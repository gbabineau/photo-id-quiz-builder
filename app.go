package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gbabineau/photo-id-quiz-builder/internal/hotspot"
	"github.com/gbabineau/photo-id-quiz-builder/internal/settings"
	"github.com/gbabineau/photo-id-quiz-builder/internal/taxonomy"
	"github.com/gbabineau/photo-id-quiz-builder/internal/trip"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the frontend.
type App struct {
	ctx   context.Context
	store settings.Store
	// confirm asks the user to approve replacing existing quizzes; nil means decline.
	confirm func(message string) bool
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if store, err := settings.DefaultStore(); err == nil {
		a.store = store
	}
	a.confirm = a.confirmReplace
}

// TripRequest describes one trip-to-quizzes run.
type TripRequest struct {
	TripPath string `json:"tripPath"`
}

// TripPreview is what the UI shows before anything is written.
type TripPreview struct {
	TripName  string         `json:"tripName"`
	OutputDir string         `json:"outputDir"`
	Quizzes   []trip.DayQuiz `json:"quizzes"`
	Warnings  []string       `json:"warnings"`
}

// WriteResult reports a write; Cancelled means the user declined to replace existing quizzes.
type WriteResult struct {
	Written   []string `json:"written"`
	Cancelled bool     `json:"cancelled"`
}

// SettingsView is what the UI shows about where files live.
type SettingsView struct {
	QuizzesDir        string `json:"quizzesDir"`
	QuizzesDirFromEnv bool   `json:"quizzesDirFromEnv"`
}

func (a *App) saveSettings(s settings.Settings) {
	if a.store.Path != "" {
		_ = a.store.Save(s)
	}
}

// GetSettings returns the folder quizzes are written to.
func (a *App) GetSettings() SettingsView {
	return SettingsView{QuizzesDir: a.store.Load().EffectiveQuizzesDir(), QuizzesDirFromEnv: settings.QuizzesDirFromEnv()}
}

// ChooseTripFile opens a file picker, starting where the last trip was chosen.
func (a *App) ChooseTripFile() (string, error) {
	s := a.store.Load()
	start := s.TripsDir
	if info, err := os.Stat(start); err != nil || !info.IsDir() {
		start = ""
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose trip JSON",
		DefaultDirectory: start,
		Filters:          []runtime.FileFilter{{DisplayName: "Trip JSON (*.json)", Pattern: "*.json"}},
	})
	if err == nil && path != "" {
		s.TripsDir = filepath.Dir(path)
		a.saveSettings(s)
	}
	return path, err
}

// ChooseQuizzesFolder lets the user pick the folder that holds one subfolder of quizzes per trip.
func (a *App) ChooseQuizzesFolder() (SettingsView, error) {
	s := a.store.Load()
	start := s.EffectiveQuizzesDir()
	if info, err := os.Stat(start); err != nil || !info.IsDir() {
		start = ""
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose quizzes folder", DefaultDirectory: start, CanCreateDirectories: true,
	})
	if err == nil && dir != "" {
		s.QuizzesDir = dir
		a.saveSettings(s)
	}
	return a.GetSettings(), err
}

// ProgressEvent is emitted while hotspot data is loaded.
const ProgressEvent = "hotspot-progress"

// PreviewTrip builds the per-day quizzes without writing files.
func (a *App) PreviewTrip(req TripRequest) (TripPreview, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	taxa, err := taxonomy.Load(ctx)
	if err != nil {
		return TripPreview{}, err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return TripPreview{}, fmt.Errorf("find user cache directory: %w", err)
	}
	src := hotspot.NewSource(ctx, filepath.Join(cacheDir, "photo-id-quiz-builder", "hotspots"), a.progress)
	defer src.Close()
	return previewTrip(req, taxa, src, a.store.Load().EffectiveQuizzesDir())
}

func (a *App) progress(message string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, ProgressEvent, message)
	}
}

func previewTrip(req TripRequest, taxa []taxonomy.Taxon, src trip.HotspotSource, quizzesDir string) (TripPreview, error) {
	if req.TripPath == "" {
		return TripPreview{}, errors.New("choose a trip JSON file")
	}
	t, err := trip.ReadTrip(req.TripPath)
	if err != nil {
		return TripPreview{}, err
	}
	result := trip.Build(t, src, taxa)
	stem := strings.TrimSuffix(filepath.Base(req.TripPath), filepath.Ext(req.TripPath))
	output := filepath.Join(quizzesDir, trip.SafeName(t.Name, stem))
	return TripPreview{TripName: t.Name, OutputDir: output, Quizzes: result.Quizzes, Warnings: nonNil(result.Warnings)}, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// WriteTripQuizzes builds the quizzes and saves them to the trip's folder. If that folder already
// holds quizzes that would be replaced, the user must confirm first.
func (a *App) WriteTripQuizzes(req TripRequest) (WriteResult, error) {
	preview, err := a.PreviewTrip(req)
	if err != nil {
		return WriteResult{}, err
	}
	return writePreview(preview, a.confirm)
}

func writePreview(preview TripPreview, confirm func(message string) bool) (WriteResult, error) {
	if existing := trip.ExistingFiles(preview.OutputDir, preview.Quizzes); len(existing) > 0 {
		if confirm == nil || !confirm(replaceMessage(preview.OutputDir, existing)) {
			return WriteResult{Written: []string{}, Cancelled: true}, nil
		}
	}
	paths, err := trip.WriteAll(preview.OutputDir, preview.Quizzes)
	return WriteResult{Written: nonNil(paths)}, err
}

func replaceMessage(dir string, existing []string) string {
	const maxListed = 10
	var b strings.Builder
	fmt.Fprintf(&b, "Building this trip will replace %d existing quiz file(s) in:\n%s\n\n", len(existing), dir)
	for i, name := range existing {
		if i == maxListed {
			fmt.Fprintf(&b, "...and %d more\n", len(existing)-maxListed)
			break
		}
		fmt.Fprintf(&b, "- %s\n", name)
	}
	b.WriteString("\nReplace them?")
	return b.String()
}

func (a *App) confirmReplace(message string) bool {
	choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Replace existing quizzes?",
		Message:       message,
		Buttons:       []string{"Yes", "No"},
		DefaultButton: "No",
		CancelButton:  "No",
	})
	return err == nil && choice == "Yes"
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gbabineau/photo-id-quiz-builder/internal/hotspot"
	"github.com/gbabineau/photo-id-quiz-builder/internal/taxonomy"
	"github.com/gbabineau/photo-id-quiz-builder/internal/trip"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the frontend.
type App struct {
	ctx context.Context
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// TripRequest describes one trip-to-quizzes run.
type TripRequest struct {
	TripPath  string `json:"tripPath"`
	OutputDir string `json:"outputDir"`
}

// TripPreview is what the UI shows before anything is written.
type TripPreview struct {
	TripName  string         `json:"tripName"`
	OutputDir string         `json:"outputDir"`
	Quizzes   []trip.DayQuiz `json:"quizzes"`
	Warnings  []string       `json:"warnings"`
}

// ChooseTripFile opens a file picker for a trip JSON file.
func (a *App) ChooseTripFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choose trip JSON",
		Filters: []runtime.FileFilter{{DisplayName: "Trip JSON (*.json)", Pattern: "*.json"}},
	})
}

// ChooseFolder opens a folder picker.
func (a *App) ChooseFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title, CanCreateDirectories: true})
}

// DefaultOutputDir suggests a folder beside the trip file.
func (a *App) DefaultOutputDir(tripPath string) string {
	return defaultOutputDir(tripPath)
}

func defaultOutputDir(tripPath string) string {
	if tripPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(tripPath), "generated_quizzes")
}

// Temporary fixed output folder until a folder picker returns.
const hardcodedOutputDir = `C:\Users\Guy\GitProjects\photo-id-quiz-builder\build\temp`

// ProgressEvent is emitted while hotspot data is loaded.
const ProgressEvent = "hotspot-progress"

// PreviewTrip builds the per-day quizzes without writing files.
func (a *App) PreviewTrip(req TripRequest) (TripPreview, error) {
	req.OutputDir = hardcodedOutputDir
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
	return previewTrip(req, taxa, src)
}

func (a *App) progress(message string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, ProgressEvent, message)
	}
}

func previewTrip(req TripRequest, taxa []taxonomy.Taxon, src trip.HotspotSource) (TripPreview, error) {
	if req.TripPath == "" {
		return TripPreview{}, errors.New("choose a trip JSON file")
	}
	t, err := trip.ReadTrip(req.TripPath)
	if err != nil {
		return TripPreview{}, err
	}
	result := trip.Build(t, src, taxa)
	output := req.OutputDir
	if output == "" {
		output = defaultOutputDir(req.TripPath)
	}
	return TripPreview{TripName: t.Name, OutputDir: output, Quizzes: result.Quizzes, Warnings: nonNil(result.Warnings)}, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// WriteTripQuizzes builds the quizzes and saves them to the output folder, returning file paths.
func (a *App) WriteTripQuizzes(req TripRequest) ([]string, error) {
	preview, err := a.PreviewTrip(req)
	if err != nil {
		return nil, err
	}
	return trip.WriteAll(preview.OutputDir, preview.Quizzes)
}

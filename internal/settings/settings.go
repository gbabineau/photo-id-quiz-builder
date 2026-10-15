// Package settings holds the builder's small per-user configuration and the shared quizzes folder.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// QuizzesDirEnv overrides the quizzes folder, for example to point at OneDrive.
const QuizzesDirEnv = "PHOTO_ID_QUIZZES_DIR"

// Settings are saved between runs.
type Settings struct {
	// TripsDir is where the last trip file was chosen; the file picker starts there.
	TripsDir string `json:"tripsDir,omitempty"`
	// QuizzesDir is the folder holding one subfolder of quizzes per trip. Empty means the default.
	QuizzesDir string `json:"quizzesDir,omitempty"`
}

// DefaultQuizzesDir is the folder shared with the quiz runner: Documents\Photo ID\Quizzes.
func DefaultQuizzesDir() string {
	return filepath.Join(documentsDir(), "Photo ID", "Quizzes")
}

// EffectiveQuizzesDir picks the environment override, then the saved setting, then the default.
func (s Settings) EffectiveQuizzesDir() string {
	if dir := strings.TrimSpace(os.Getenv(QuizzesDirEnv)); dir != "" {
		return dir
	}
	if s.QuizzesDir != "" {
		return s.QuizzesDir
	}
	return DefaultQuizzesDir()
}

// QuizzesDirFromEnv reports whether the environment variable is controlling the quizzes folder.
func QuizzesDirFromEnv() bool { return strings.TrimSpace(os.Getenv(QuizzesDirEnv)) != "" }

// Store reads and writes Settings as JSON.
type Store struct{ Path string }

// DefaultStore keeps settings in the user config folder.
func DefaultStore() (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: filepath.Join(dir, "photo-id-quiz-builder", "settings.json")}, nil
}

// Load returns the saved settings, or zero values if none are saved or the file is unreadable.
func (s Store) Load() Settings {
	var v Settings
	data, err := os.ReadFile(s.Path)
	if err != nil || json.Unmarshal(data, &v) != nil {
		return Settings{}
	}
	return v
}

// Save writes the settings.
func (s Store) Save(v Settings) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o600)
}

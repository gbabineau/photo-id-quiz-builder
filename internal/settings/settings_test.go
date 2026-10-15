package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRoundTripAndMissingFile(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "nested", "settings.json")}
	if got := store.Load(); got != (Settings{}) {
		t.Fatalf("missing file = %+v", got)
	}
	want := Settings{TripsDir: `C:\trips`, QuizzesDir: `D:\quizzes`}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	if got := store.Load(); got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestCorruptSettingsFallBackToDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte("{not json"), 0o600)
	if got := (Store{Path: path}).Load(); got != (Settings{}) {
		t.Fatalf("got %+v", got)
	}
}

func TestEffectiveQuizzesDirPrecedence(t *testing.T) {
	t.Setenv(QuizzesDirEnv, "")
	if got := (Settings{}).EffectiveQuizzesDir(); got != DefaultQuizzesDir() {
		t.Errorf("default = %q", got)
	}
	if got := (Settings{QuizzesDir: "saved"}).EffectiveQuizzesDir(); got != "saved" {
		t.Errorf("saved = %q", got)
	}
	if QuizzesDirFromEnv() {
		t.Error("env should not be active")
	}

	t.Setenv(QuizzesDirEnv, "from-env")
	if got := (Settings{QuizzesDir: "saved"}).EffectiveQuizzesDir(); got != "from-env" {
		t.Errorf("env = %q", got)
	}
	if !QuizzesDirFromEnv() {
		t.Error("env should be active")
	}
}

func TestDefaultQuizzesDirIsUnderPhotoID(t *testing.T) {
	got := DefaultQuizzesDir()
	if !strings.HasSuffix(got, filepath.Join("Photo ID", "Quizzes")) || !filepath.IsAbs(got) {
		t.Fatalf("default = %q", got)
	}
}

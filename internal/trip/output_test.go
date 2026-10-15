package trip

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gbabineau/photo-id-quiz-builder/internal/quiz"
)

func dayQuiz(name string) DayQuiz {
	return DayQuiz{Day: "1", FileName: name, Quiz: quiz.Quiz{Location: "US", Species: []quiz.Species{{CommonName: "Dipper"}}}}
}

func TestSafeName(t *testing.T) {
	cases := []struct{ name, fallback, want string }{
		{"Argentina and Chile January 2027", "x", "Argentina and Chile January 2027"},
		{`Oslo: "Spring" / Fjords?`, "x", "Oslo- -Spring- - Fjords-"},
		{"  Trip   One. ", "x", "Trip One"},
		{"", "my_trip", "my_trip"},
		{"...", "", "Untitled trip"},
		{strings.Repeat("a", 150), "x", strings.Repeat("a", 100)},
	}
	for _, c := range cases {
		if got := SafeName(c.name, c.fallback); got != c.want {
			t.Errorf("SafeName(%q, %q) = %q, want %q", c.name, c.fallback, got, c.want)
		}
	}
}

func TestRebuildReplacesPreviousQuizzesButNotOtherFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Trip")
	first := []DayQuiz{dayQuiz("Day 1 with species 1-21.json"), dayQuiz("Day 1 with species 22-41.json"), dayQuiz("Day 2 with species 1-5.json")}
	if _, err := WriteAll(dir, first); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "notes.json"), []byte("{}"), 0o644)

	second := []DayQuiz{dayQuiz("Day 1 with species 1-30.json"), dayQuiz("Day 2 with species 1-5.json")}
	if got, want := ExistingFiles(dir, second), []string{
		"Day 1 with species 1-21.json", "Day 1 with species 22-41.json", "Day 2 with species 1-5.json",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("existing = %v, want %v", got, want)
	}

	if _, err := WriteAll(dir, second); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"Day 1 with species 1-21.json", "Day 1 with species 22-41.json"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s should have been removed", gone)
		}
	}
	for _, kept := range []string{"Day 1 with species 1-30.json", "Day 2 with species 1-5.json", "notes.json"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s should exist: %v", kept, err)
		}
	}
}

func TestExistingFilesIncludesNameCollisionsWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Day 1 with species 1-5.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "unrelated.json"), []byte("{}"), 0o644)

	got := ExistingFiles(dir, []DayQuiz{dayQuiz("Day 1 with species 1-5.json"), dayQuiz("Day 2 with species 1-5.json")})
	if !reflect.DeepEqual(got, []string{"Day 1 with species 1-5.json"}) {
		t.Fatalf("existing = %v", got)
	}
	if got := ExistingFiles(filepath.Join(dir, "missing"), []DayQuiz{dayQuiz("a.json")}); len(got) != 0 {
		t.Fatalf("missing folder should have nothing to replace: %v", got)
	}
}

func TestManifestCannotDeleteOutsideTheFolder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "trip")
	os.MkdirAll(dir, 0o755)
	victim := filepath.Join(root, "precious.json")
	os.WriteFile(victim, []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, manifestName), []byte(`{"files":["../precious.json","sub/../../precious.json","notes.txt"]}`), 0o644)

	if _, err := WriteAll(dir, []DayQuiz{dayQuiz("Day 1.json")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("file outside the folder was deleted: %v", err)
	}
}

package trip

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gbabineau/photo-id-quiz-builder/internal/quiz"
)

// manifestName lists the files the builder wrote to a trip folder, so a rebuild can remove
// quizzes that no longer exist (for example a part that is no longer needed) without touching
// files the builder did not create.
const manifestName = ".quiz-builder.json"

type manifest struct {
	Files []string `json:"files"`
}

// SafeName turns a trip name into a folder name, using fallback when nothing usable remains.
func SafeName(name, fallback string) string {
	for _, candidate := range []string{name, fallback} {
		s := strings.Join(strings.Fields(unsafeFileChars.Replace(candidate)), " ")
		if r := []rune(s); len(r) > 100 {
			s = string(r[:100])
		}
		if s = strings.TrimRight(s, ". "); s != "" {
			return s
		}
	}
	return "Untitled trip"
}

func readManifest(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return nil
	}
	var m manifest
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	var files []string
	for _, f := range m.Files {
		// Only plain quiz file names inside dir are ever deleted.
		if f != "" && filepath.Base(f) == f && strings.HasSuffix(f, ".json") && f != manifestName {
			files = append(files, f)
		}
	}
	return files
}

// ExistingFiles lists the files in dir that WriteAll would replace or remove: quizzes from a
// previous build and any file sharing a name with a new quiz.
func ExistingFiles(dir string, quizzes []DayQuiz) []string {
	candidates := map[string]bool{}
	for _, f := range readManifest(dir) {
		candidates[f] = true
	}
	for _, q := range quizzes {
		candidates[q.FileName] = true
	}
	var existing []string
	for f := range candidates {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			existing = append(existing, f)
		}
	}
	sort.Strings(existing)
	return existing
}

// WriteAll saves every quiz into dir, creating it if needed, then removes quizzes left over from
// a previous build, and returns the file paths written.
func WriteAll(dir string, quizzes []DayQuiz) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create output folder: %w", err)
	}
	previous := readManifest(dir)

	paths := make([]string, 0, len(quizzes))
	names := make([]string, 0, len(quizzes))
	written := map[string]bool{}
	for _, q := range quizzes {
		path := filepath.Join(dir, q.FileName)
		if err := quiz.Write(path, q.Quiz); err != nil {
			return paths, err
		}
		paths = append(paths, path)
		names = append(names, q.FileName)
		written[q.FileName] = true
	}

	for _, f := range previous {
		if !written[f] {
			if err := os.Remove(filepath.Join(dir, f)); err != nil && !os.IsNotExist(err) {
				return paths, fmt.Errorf("remove old quiz %s: %w", f, err)
			}
		}
	}

	data, err := json.MarshalIndent(manifest{Files: names}, "", "    ")
	if err != nil {
		return paths, err
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), data, 0o644); err != nil {
		return paths, fmt.Errorf("write manifest: %w", err)
	}
	return paths, nil
}

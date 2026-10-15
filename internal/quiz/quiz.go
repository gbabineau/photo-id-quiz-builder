// Package quiz reads and writes the quiz JSON format opened by the photo-id runner.
package quiz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Species is one quiz entry. Only comName is required by the runner.
type Species struct {
	CommonName     string   `json:"comName"`
	Frequency      *float64 `json:"frequency,omitempty"`
	Notes          string   `json:"notes,omitempty"`
	SpeciesCode    string   `json:"speciesCode,omitempty"`
	ScientificName string   `json:"sciName,omitempty"`
	Category       string   `json:"category,omitempty"`
	TaxonOrder     float64  `json:"taxonOrder,omitempty"`
	BandingCodes   []string `json:"bandingCodes,omitempty"`
	ComNameCodes   []string `json:"comNameCodes,omitempty"`
	SciNameCodes   []string `json:"sciNameCodes,omitempty"`
	Order          string   `json:"order,omitempty"`
	FamilyCode     string   `json:"familyCode,omitempty"`
	FamilyComName  string   `json:"familyComName,omitempty"`
	FamilySciName  string   `json:"familySciName,omitempty"`
}

// Quiz is the runner-compatible quiz document.
type Quiz struct {
	StartMonth int       `json:"start_month"`
	EndMonth   int       `json:"end_month"`
	Location   string    `json:"location"`
	Notes      string    `json:"notes,omitempty"`
	Species    []Species `json:"species"`
}

// UnmarshalJSON accepts the legacy uppercase "Notes" key.
func (q *Quiz) UnmarshalJSON(data []byte) error {
	type plain Quiz
	aux := struct {
		*plain
		LegacyNotes string `json:"Notes"`
	}{plain: (*plain)(q)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if q.Notes == "" {
		q.Notes = aux.LegacyNotes
	}
	q.Species = DedupeByName(q.Species)
	return nil
}

// DedupeByName keeps the first entry for each case-insensitive common name.
func DedupeByName(species []Species) []Species {
	seen := make(map[string]bool, len(species))
	result := make([]Species, 0, len(species))
	for _, s := range species {
		key := strings.ToLower(strings.TrimSpace(s.CommonName))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, s)
	}
	return result
}

// Read loads a quiz file.
func Read(path string) (Quiz, error) {
	var q Quiz
	data, err := os.ReadFile(path)
	if err != nil {
		return q, fmt.Errorf("read quiz: %w", err)
	}
	if err := json.Unmarshal(data, &q); err != nil {
		return q, fmt.Errorf("parse quiz %s: %w", path, err)
	}
	return q, nil
}

// Marshal renders a quiz as indented UTF-8 JSON without HTML escaping.
func Marshal(q Quiz) ([]byte, error) {
	if q.Species == nil {
		q.Species = []Species{}
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(q); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write saves a quiz file.
func Write(path string, q Quiz) error {
	data, err := Marshal(q)
	if err != nil {
		return fmt.Errorf("encode quiz: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write quiz: %w", err)
	}
	return nil
}

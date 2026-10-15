package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbabineau/photo-id-quiz-builder/internal/taxonomy"
	"github.com/gbabineau/photo-id-quiz-builder/internal/trip"
)

func TestPreviewTripRequiresTripFile(t *testing.T) {
	if _, err := previewTrip(TripRequest{}, nil, trip.DirSource{}, t.TempDir()); err == nil {
		t.Error("expected error without trip")
	}
	if _, err := previewTrip(TripRequest{TripPath: filepath.Join(t.TempDir(), "missing.json")}, nil, trip.DirSource{}, t.TempDir()); err == nil {
		t.Error("expected error for missing trip file")
	}
}

func demoPreview(t *testing.T, tripJSON, fileName string) (TripPreview, string) {
	t.Helper()
	dir := t.TempDir()
	hotspots := filepath.Join(dir, "hotspots")
	os.Mkdir(hotspots, 0o755)
	os.WriteFile(filepath.Join(hotspots, "L1.json"), []byte(`[{"comName":"Alpha Bird","frequency":42}]`), 0o644)
	tripPath := filepath.Join(dir, fileName)
	os.WriteFile(tripPath, []byte(tripJSON), 0o644)
	quizzes := filepath.Join(dir, "Quizzes")
	p, err := previewTrip(TripRequest{TripPath: tripPath}, []taxonomy.Taxon{{CommonName: "Alpha Bird", SpeciesCode: "alpbir", Category: "species", TaxonOrder: 1}},
		trip.DirSource{Dir: hotspots}, quizzes)
	if err != nil {
		t.Fatal(err)
	}
	return p, quizzes
}

const demoTrip = `{"name":"Demo: Spring","location":"US","start_month":1,"end_month":2,"minimum_frequency":10,
	"itinerary":[{"day":"1","Full day title":"Marsh","hotspots":[{"name":"Marsh","hotspotId":"L1"}]}]}`

func TestPreviewTripEndToEnd(t *testing.T) {
	p, quizzes := demoPreview(t, demoTrip, "trip.json")
	if p.TripName != "Demo: Spring" || len(p.Quizzes) != 1 || len(p.Quizzes[0].Quiz.Species) != 1 {
		t.Fatalf("preview = %+v", p)
	}
	// The folder is named after the trip, not the file.
	if p.OutputDir != filepath.Join(quizzes, "Demo- Spring") {
		t.Errorf("output dir = %q", p.OutputDir)
	}
}

func TestUnnamedTripFolderFallsBackToFileName(t *testing.T) {
	p, quizzes := demoPreview(t, `{"location":"US","itinerary":[]}`, "oslo_may.json")
	if p.OutputDir != filepath.Join(quizzes, "oslo_may") {
		t.Errorf("output dir = %q", p.OutputDir)
	}
}

func TestWriteAsksBeforeReplacingAndHonoursCancel(t *testing.T) {
	p, _ := demoPreview(t, demoTrip, "trip.json")
	file := filepath.Join(p.OutputDir, p.Quizzes[0].FileName)

	asked := 0
	never := func(string) bool { asked++; return false }
	always := func(string) bool { asked++; return true }

	// First write: nothing exists, so there is no prompt.
	res, err := writePreview(p, never)
	if err != nil || res.Cancelled || len(res.Written) != 1 || asked != 0 {
		t.Fatalf("first write: %+v err=%v asked=%d", res, err, asked)
	}

	// Rebuild declined: prompted once, nothing changes.
	os.WriteFile(file, []byte("edited"), 0o644)
	res, err = writePreview(p, never)
	if err != nil || !res.Cancelled || asked != 1 {
		t.Fatalf("declined: %+v err=%v asked=%d", res, err, asked)
	}
	if data, _ := os.ReadFile(file); string(data) != "edited" {
		t.Fatal("declined rebuild must not touch files")
	}

	// No confirm handler at all is treated as a decline.
	if res, _ := writePreview(p, nil); !res.Cancelled {
		t.Fatal("nil confirm should cancel")
	}

	// Rebuild accepted: files are replaced.
	res, err = writePreview(p, always)
	if err != nil || res.Cancelled || len(res.Written) != 1 {
		t.Fatalf("accepted: %+v err=%v", res, err)
	}
	if data, _ := os.ReadFile(file); string(data) == "edited" {
		t.Fatal("accepted rebuild should overwrite")
	}
}

func TestReplaceMessageListsFilesAndTruncates(t *testing.T) {
	var names []string
	for i := 0; i < 13; i++ {
		names = append(names, fmt.Sprintf("Day %d.json", i))
	}
	msg := replaceMessage(`C:\Quizzes\Trip`, names)
	for _, want := range []string{"13 existing", `C:\Quizzes\Trip`, "- Day 0.json", "and 3 more", "Replace them?"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "Day 12.json") {
		t.Error("message should truncate the list")
	}
}

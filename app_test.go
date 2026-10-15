package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gbabineau/photo-id-quiz-builder/internal/taxonomy"
	"github.com/gbabineau/photo-id-quiz-builder/internal/trip"
)

func TestPreviewTripRequiresTripFile(t *testing.T) {
	if _, err := previewTrip(TripRequest{}, nil, trip.DirSource{}); err == nil {
		t.Error("expected error without trip")
	}
	if _, err := previewTrip(TripRequest{TripPath: filepath.Join(t.TempDir(), "missing.json")}, nil, trip.DirSource{}); err == nil {
		t.Error("expected error for missing trip file")
	}
}

func TestPreviewTripEndToEnd(t *testing.T) {
	dir := t.TempDir()
	hotspots := filepath.Join(dir, "hotspots")
	os.Mkdir(hotspots, 0o755)
	os.WriteFile(filepath.Join(hotspots, "L1.json"), []byte(`[{"comName":"Alpha Bird","frequency":42}]`), 0o644)
	tripPath := filepath.Join(dir, "trip.json")
	os.WriteFile(tripPath, []byte(`{"name":"Demo","location":"US","start_month":1,"end_month":2,"minimum_frequency":10,
		"itinerary":[{"day":"1","Full day title":"Marsh","hotspots":[{"name":"Marsh","hotspotId":"L1"}]}]}`), 0o644)

	p, err := previewTrip(TripRequest{TripPath: tripPath}, []taxonomy.Taxon{{CommonName: "Alpha Bird", SpeciesCode: "alpbir", Category: "species", TaxonOrder: 1}},
		trip.DirSource{Dir: hotspots})
	if err != nil {
		t.Fatal(err)
	}
	if p.TripName != "Demo" || len(p.Quizzes) != 1 || len(p.Quizzes[0].Quiz.Species) != 1 {
		t.Fatalf("preview = %+v", p)
	}
	if p.OutputDir != filepath.Join(dir, "generated_quizzes") {
		t.Errorf("output dir = %q", p.OutputDir)
	}
}

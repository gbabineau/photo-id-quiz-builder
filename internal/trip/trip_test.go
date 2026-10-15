package trip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gbabineau/photo-id-quiz-builder/internal/quiz"
	"github.com/gbabineau/photo-id-quiz-builder/internal/taxonomy"
)

type mapSource map[string][]HotspotSpecies

func (m mapSource) Species(id string, _, _ int) ([]HotspotSpecies, error) {
	list, ok := m[id]
	if !ok {
		return nil, ErrNoHotspotData
	}
	return list, nil
}

var testTaxa = []taxonomy.Taxon{
	{CommonName: "Alpha Bird", SpeciesCode: "alpbir", ScientificName: "Alpha a", Category: "species", TaxonOrder: 1},
	{CommonName: "Beta Bird", SpeciesCode: "betbir", ScientificName: "Beta b", Category: "species", TaxonOrder: 2},
	{CommonName: "Gamma Bird", SpeciesCode: "gambir", ScientificName: "Gamma g", Category: "species", TaxonOrder: 3},
}

func names(s []quiz.Species) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = v.CommonName
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHotspotID(t *testing.T) {
	cases := map[string]string{
		"L485324":                           "L485324",
		"https://ebird.org/hotspot/L472208": "L472208",
		" L1 ":                              "L1",
		"custom":                            "custom",
	}
	for in, want := range cases {
		if got := HotspotID(in); got != want {
			t.Errorf("HotspotID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAveragesFrequencyAcrossHotspotsAndFiltersByMinimum(t *testing.T) {
	src := mapSource{
		"L1": {{"Alpha Bird", 40}, {"Beta Bird", 5}},
		"L2": {{"Alpha Bird", 21}, {"Beta Bird", 30}},
	}
	tr := Trip{MinimumFrequency: 10, StartMonth: 5, EndMonth: 6, Location: "NO",
		Itinerary: []Day{{Day: "1", FullDayTitle: "Marsh", Hotspots: []Hotspot{{HotspotID: "L1"}, {HotspotID: "https://ebird.org/hotspot/L2"}}}}}

	res := Build(tr, src, testTaxa)
	q := res.Quizzes[0].Quiz
	if len(q.Species) != 2 {
		t.Fatalf("species = %v", names(q.Species))
	}
	if *q.Species[0].Frequency != 30.5 {
		t.Errorf("alpha freq = %v, want 30.5", *q.Species[0].Frequency)
	}
	// Beta only counts the 30% row, so its average is 30 rather than 17.5.
	if *q.Species[1].Frequency != 30 {
		t.Errorf("beta freq = %v, want 30", *q.Species[1].Frequency)
	}
	if q.Location != "NO" || q.StartMonth != 5 || q.EndMonth != 6 || q.Notes != "Full: Marsh." {
		t.Errorf("quiz header = %+v", q)
	}
	if q.Species[0].SpeciesCode != "alpbir" || q.Species[0].ScientificName != "Alpha a" {
		t.Errorf("taxonomy not attached: %+v", q.Species[0])
	}
}

func TestMentionedSpeciesAreAddedAndMarked(t *testing.T) {
	src := mapSource{"L1": {{"Beta Bird", 50}}}
	tr := Trip{Itinerary: []Day{{Day: "1", AMTitle: "Dawn", PMTitle: "Dusk",
		Hotspots: []Hotspot{{HotspotID: "L1"}}, Mentioned: []string{"Beta Bird", "Gamma Bird", "Unknown Bird"}}}}

	res := Build(tr, src, testTaxa)
	q := res.Quizzes[0].Quiz
	if !equal(names(q.Species), []string{"Beta Bird", "Gamma Bird"}) {
		t.Fatalf("species = %v", names(q.Species))
	}
	if q.Species[0].Notes != MentionedNote || q.Species[1].Notes != MentionedNote {
		t.Errorf("notes not set: %+v", q.Species)
	}
	if q.Species[1].Frequency != nil {
		t.Errorf("mentioned-only species should have no frequency")
	}
	if q.Notes != "Morning: Dawn. Afternoon: Dusk" {
		t.Errorf("notes = %q", q.Notes)
	}
	if len(res.Warnings) != 1 {
		t.Errorf("warnings = %v", res.Warnings)
	}
	if q.StartMonth != 1 || q.EndMonth != 12 {
		t.Errorf("month defaults = %d-%d", q.StartMonth, q.EndMonth)
	}
}

func TestRareSpeciesDroppedAndSortedTaxonomically(t *testing.T) {
	src := mapSource{"L1": {{"Gamma Bird", 50}, {"Alpha Bird", 0.5}, {"Beta Bird", 20}}}
	tr := Trip{Itinerary: []Day{{Day: "1", Hotspots: []Hotspot{{HotspotID: "L1"}}}}}
	q := Build(tr, src, testTaxa).Quizzes[0].Quiz
	if !equal(names(q.Species), []string{"Beta Bird", "Gamma Bird"}) {
		t.Fatalf("species = %v", names(q.Species))
	}
}

func TestSharedSpeciesKeptOnBestDay(t *testing.T) {
	src := mapSource{
		"L1": {{"Alpha Bird", 30}, {"Beta Bird", 20}, {"Gamma Bird", 10}},
		"L2": {{"Alpha Bird", 60}, {"Beta Bird", 20}, {"Gamma Bird", 10}},
	}
	tr := Trip{Itinerary: []Day{
		{Day: "1", Hotspots: []Hotspot{{HotspotID: "L1"}}, Mentioned: []string{"Gamma Bird"}},
		{Day: "2", Hotspots: []Hotspot{{HotspotID: "L2"}}},
	}}
	res := Build(tr, src, testTaxa)
	day1, day2 := names(res.Quizzes[0].Quiz.Species), names(res.Quizzes[1].Quiz.Species)
	// Alpha: higher on day 2. Beta: tie goes to day 1. Gamma: mentioned on day 1.
	if !equal(day1, []string{"Beta Bird", "Gamma Bird"}) || !equal(day2, []string{"Alpha Bird"}) {
		t.Fatalf("day1=%v day2=%v", day1, day2)
	}
}

func TestMissingHotspotDataWarnsOnce(t *testing.T) {
	tr := Trip{Itinerary: []Day{
		{Day: "1", Hotspots: []Hotspot{{Name: "Marsh", HotspotID: "L9"}}},
		{Day: "2", Hotspots: []Hotspot{{Name: "Marsh", HotspotID: "L9"}}},
	}}
	res := Build(tr, mapSource{}, testTaxa)
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings = %v", res.Warnings)
	}
	if res.Quizzes[0].FileName != "Day 1 with no species.json" {
		t.Errorf("file name = %q", res.Quizzes[0].FileName)
	}
}

func TestFileNamesAreSafeAndUnique(t *testing.T) {
	tr := Trip{Itinerary: []Day{{Day: "2/3"}, {Day: "2/3"}, {}}}
	res := Build(tr, mapSource{}, testTaxa)
	got := []string{res.Quizzes[0].FileName, res.Quizzes[1].FileName, res.Quizzes[2].FileName}
	want := []string{"Day 2-3 with no species.json", "Day 2-3 with no species (2).json", "Day 3 with no species.json"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestReadTripAcceptsNumericAndStringDays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trip.json")
	os.WriteFile(path, []byte(`{"location":"AR","start_month":1,"end_month":2,"minimum_frequency":10,
		"itinerary":[{"day":"0","hotspots":[{"name":"A","hotspotId":"L1"}]},{"day":3},{"day":"2-3","mentioned":["X"]}]}`), 0o644)
	tr, err := ReadTrip(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Itinerary) != 3 || tr.Itinerary[1].Day != "3" || tr.Itinerary[2].Day != "2-3" || tr.MinimumFrequency != 10 {
		t.Fatalf("trip = %+v", tr)
	}
}

func TestReadTripRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"array": `[]`, "noitin": `{"name":"x"}`, "broken": `{`} {
		path := filepath.Join(dir, name+".json")
		os.WriteFile(path, []byte(content), 0o644)
		if _, err := ReadTrip(path); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := ReadTrip(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDirSource(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "L1.json"), []byte(`[{"comName":"Alpha Bird","frequency":12.5}]`), 0o644)
	src := DirSource{Dir: dir}
	list, err := src.Species("L1", 1, 12)
	if err != nil || len(list) != 1 || list[0].Frequency != 12.5 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if _, err := src.Species("L2", 1, 12); !errors.Is(err, ErrNoHotspotData) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteAllProducesRunnerReadableFiles(t *testing.T) {
	src := mapSource{"L1": {{"Alpha Bird", 50}}}
	tr := Trip{Location: "NO", StartMonth: 5, EndMonth: 6, Itinerary: []Day{{Day: "1", Hotspots: []Hotspot{{HotspotID: "L1"}}}}}
	res := Build(tr, src, testTaxa)

	dir := filepath.Join(t.TempDir(), "generated_quizzes")
	paths, err := WriteAll(dir, res.Quizzes)
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	q, err := quiz.Read(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if q.Location != "NO" || len(q.Species) != 1 || q.Species[0].SpeciesCode != "alpbir" {
		t.Fatalf("quiz = %+v", q)
	}
}

func TestLargeDayIsSplitIntoNamedParts(t *testing.T) {
	var taxa []taxonomy.Taxon
	var rows []HotspotSpecies
	for i := 0; i < 45; i++ {
		name := fmt.Sprintf("Bird %02d", i)
		taxa = append(taxa, taxonomy.Taxon{CommonName: name, Category: "species", TaxonOrder: float64(i)})
		rows = append(rows, HotspotSpecies{CommonName: name, Frequency: 50})
	}
	tr := Trip{Itinerary: []Day{{Day: "1", Hotspots: []Hotspot{{HotspotID: "L1"}}}}}
	res := Build(tr, mapSource{"L1": rows}, taxa)

	if len(res.Quizzes) != 3 {
		t.Fatalf("quizzes = %d", len(res.Quizzes))
	}
	if res.Quizzes[0].FileName != "Day 1 with species 1-23.json" || res.Quizzes[1].FileName != "Day 1 with species 24-45.json" {
		t.Errorf("names = %q, %q", res.Quizzes[0].FileName, res.Quizzes[1].FileName)
	}
	if n := len(res.Quizzes[0].Quiz.Species) + len(res.Quizzes[1].Quiz.Species); n != 45 {
		t.Errorf("species total = %d", n)
	}
}

func TestAllSpeciesQuizCombinesEveryDayUnsplit(t *testing.T) {
	var taxa []taxonomy.Taxon
	var day1, day2 []HotspotSpecies
	for i := 0; i < 90; i++ {
		name := fmt.Sprintf("Bird %02d", i)
		taxa = append(taxa, taxonomy.Taxon{CommonName: name, Category: "species", TaxonOrder: float64(i)})
		row := HotspotSpecies{CommonName: name, Frequency: 50}
		if i%2 == 0 {
			day1 = append(day1, row)
		} else {
			day2 = append(day2, row)
		}
	}
	tr := Trip{Name: "Big Trip", Location: "NO", StartMonth: 5, EndMonth: 6, Itinerary: []Day{
		{Day: "1", Hotspots: []Hotspot{{HotspotID: "L1"}}},
		{Day: "2", Hotspots: []Hotspot{{HotspotID: "L2"}}},
	}}
	res := Build(tr, mapSource{"L1": day1, "L2": day2}, taxa)

	all := res.Quizzes[len(res.Quizzes)-1]
	if all.FileName != "All days with species 1-90.json" || all.Day != "All" {
		t.Fatalf("all quiz = %q day %q", all.FileName, all.Day)
	}
	if n := len(all.Quiz.Species); n != 90 {
		t.Fatalf("all species = %d, want 90 in a single unsplit file", n)
	}
	for i, s := range all.Quiz.Species {
		if s.TaxonOrder != float64(i) {
			t.Fatalf("species %d out of taxonomic order: %v", i, s.TaxonOrder)
		}
	}
	if all.Quiz.Notes != "All species for Big Trip" || all.Quiz.Location != "NO" || all.Quiz.StartMonth != 5 || all.Quiz.EndMonth != 6 {
		t.Errorf("header = %+v", all.Quiz)
	}
	// Each day was split in parts (45 > 40) and the combined quiz is not.
	for _, q := range res.Quizzes[:len(res.Quizzes)-1] {
		if len(q.Quiz.Species) > maxPartSize {
			t.Errorf("%s has %d species", q.FileName, len(q.Quiz.Species))
		}
	}
}

func TestAllSpeciesQuizSkippedWhenNoSpecies(t *testing.T) {
	res := Build(Trip{Itinerary: []Day{{Day: "1"}}}, mapSource{}, testTaxa)
	if len(res.Quizzes) != 1 || res.Quizzes[0].Day == "All" {
		t.Fatalf("quizzes = %+v", res.Quizzes)
	}
}

func TestAllSpeciesQuizUsesSharedSpeciesOnce(t *testing.T) {
	src := mapSource{"L1": {{"Alpha Bird", 30}}, "L2": {{"Alpha Bird", 60}, {"Beta Bird", 20}}}
	tr := Trip{Itinerary: []Day{{Day: "1", Hotspots: []Hotspot{{HotspotID: "L1"}}}, {Day: "2", Hotspots: []Hotspot{{HotspotID: "L2"}}}}}
	res := Build(tr, src, testTaxa)
	all := res.Quizzes[len(res.Quizzes)-1].Quiz
	if !equal(names(all.Species), []string{"Alpha Bird", "Beta Bird"}) || *all.Species[0].Frequency != 60 {
		t.Fatalf("all = %v", names(all.Species))
	}
	if all.Notes != "All species for the trip" {
		t.Errorf("notes = %q", all.Notes)
	}
}

func TestAllSpeciesQuizSkippedForSingleUnsplitDay(t *testing.T) {
	tr := Trip{Itinerary: []Day{{Day: "1", Hotspots: []Hotspot{{HotspotID: "L1"}}}}}
	res := Build(tr, mapSource{"L1": {{"Alpha Bird", 50}}}, testTaxa)
	if len(res.Quizzes) != 1 || res.Quizzes[0].Day == "All" {
		t.Fatalf("quizzes = %+v", res.Quizzes)
	}
}

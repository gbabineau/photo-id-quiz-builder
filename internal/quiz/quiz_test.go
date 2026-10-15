package quiz

import (
	"path/filepath"
	"testing"
)

func TestReadRunnerFixtures(t *testing.T) {
	q, err := Read(filepath.Join("..", "..", "testdata", "runner_minimal_quiz.json"))
	if err != nil {
		t.Fatal(err)
	}
	if q.Location != "EC" || q.StartMonth != 2 || len(q.Species) != 4 {
		t.Fatalf("unexpected quiz: %+v", q)
	}
	if q.Species[1].Frequency == nil || *q.Species[1].Frequency != 10 {
		t.Fatalf("frequency not read: %+v", q.Species[1])
	}
	if q.Species[2].Notes != "some notes" {
		t.Fatalf("notes not read: %+v", q.Species[2])
	}

	full, err := Read(filepath.Join("..", "..", "testdata", "runner_day_quiz.json"))
	if err != nil {
		t.Fatal(err)
	}
	first := full.Species[0]
	if first.SpeciesCode != "sponot1" || first.ScientificName != "Nothura maculosa" || first.FamilyCode != "tinami1" || first.TaxonOrder != 71 {
		t.Fatalf("taxonomy fields not read: %+v", first)
	}
}

func TestLegacyUppercaseNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q.json")
	if err := writeRaw(path, `{"start_month":1,"end_month":2,"location":"US","Notes":"legacy","species":[]}`); err != nil {
		t.Fatal(err)
	}
	q, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.Notes != "legacy" {
		t.Fatalf("Notes = %q", q.Notes)
	}
}

func TestDuplicateCommonNamesKeepFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q.json")
	raw := `{"start_month":1,"end_month":1,"location":"US","species":[
		{"comName":"Blue Jay","notes":"first"},{"comName":"blue jay","notes":"second"},{"comName":"Cardinal"}]}`
	if err := writeRaw(path, raw); err != nil {
		t.Fatal(err)
	}
	q, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Species) != 2 || q.Species[0].Notes != "first" {
		t.Fatalf("species = %+v", q.Species)
	}
}

func TestWriteRoundTrip(t *testing.T) {
	freq := 12.5
	in := Quiz{StartMonth: 3, EndMonth: 4, Location: "NO", Notes: "n", Species: []Species{{CommonName: "Dipper", Frequency: &freq, SpeciesCode: "eurdip1"}}}
	path := filepath.Join(t.TempDir(), "out.json")
	if err := Write(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.Location != "NO" || len(out.Species) != 1 || *out.Species[0].Frequency != 12.5 {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestMarshalEmptySpeciesIsArray(t *testing.T) {
	data, err := Marshal(Quiz{Location: "US"})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), `"species": []`) {
		t.Fatalf("expected empty array: %s", data)
	}
}

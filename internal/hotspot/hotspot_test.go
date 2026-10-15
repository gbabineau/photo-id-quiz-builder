package hotspot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// row builds a tab-separated line: name then 48 weekly values, with value v in weeks [from, to).
func row(name string, v float64, from, to int) string {
	cells := []string{name}
	for w := 0; w < weeksPerYear; w++ {
		x := 0.0
		if w >= from && w < to {
			x = v
		}
		cells = append(cells, fmt.Sprint(x))
	}
	return strings.Join(cells, "\t") + "\t" // eBird ends every row with a tab
}

// samplePage mimics the eBird bar chart page: markup, a sample size row, species rows, markup.
func samplePage() string {
	samples := []string{"Sample Size:"}
	for w := 0; w < weeksPerYear; w++ {
		if w < 4 {
			samples = append(samples, "10.0") // January only
		} else if w >= 44 {
			samples = append(samples, "30.0") // December only
		} else {
			samples = append(samples, "0.0")
		}
	}
	return strings.Join([]string{
		"<html><body><h1>Bird Observations</h1>",
		"Frequency of observations in the selected location(s).:",
		"Number of taxa:    4",
		"   Jan        Feb",
		strings.Join(samples, "\t"),
		"",
		row("Dolphin Gull", 0.5, 0, 48),
		row("Kelp Gull", 1.0, 44, 48),
		row("coot sp.", 0.1, 0, 48),
		row("Baird&#39;s Sandpiper", 0.2, 0, 4),
		row("Rock Pigeon (Feral Pigeon)", 0.3, 0, 48),
		"</body></html>",
	}, "\n")
}

func TestParseBarchartPage(t *testing.T) {
	chart, err := ParseBarchartPage(samplePage())
	if err != nil {
		t.Fatal(err)
	}
	if len(chart.Rows) != 5 || len(chart.Samples) != weeksPerYear {
		t.Fatalf("species=%d samples=%d", len(chart.Rows), len(chart.Samples))
	}
	if chart.Rows[3].CommonName != "Baird's Sandpiper" {
		t.Errorf("name not unescaped: %q", chart.Rows[3].CommonName)
	}
}

func TestParseBarchartPageWithoutDataFails(t *testing.T) {
	for _, page := range []string{"", "<html>Making sure you're not a bot</html>", "Sample Size:\t1.0"} {
		if _, err := ParseBarchartPage(page); err == nil {
			t.Errorf("expected error for %q", page)
		}
	}
}

func freqs(list []string, chart Barchart, start, end int) map[string]float64 {
	out := map[string]float64{}
	for _, s := range chart.Species(start, end) {
		out[s.CommonName] = s.Frequency
	}
	return out
}

func TestSpeciesFrequencyWeightsByChecklists(t *testing.T) {
	chart, _ := ParseBarchartPage(samplePage())

	jan := freqs(nil, chart, 1, 1)
	if jan["Dolphin Gull"] != 50 || jan["Baird's Sandpiper"] != 20 {
		t.Errorf("jan = %v", jan)
	}
	if _, ok := jan["Kelp Gull"]; ok {
		t.Errorf("Kelp Gull is not seen in January: %v", jan)
	}
	for _, skipped := range []string{"coot sp.", "Rock Pigeon (Feral Pigeon)"} {
		if _, ok := jan[skipped]; ok {
			t.Errorf("%q should be filtered", skipped)
		}
	}

	// Dec has 120 checklists, Jan 40: Baird's 0.2 in Jan only => 0.2*40/160 = 5%.
	wrap := freqs(nil, chart, 12, 1)
	if wrap["Baird's Sandpiper"] != 5 {
		t.Errorf("wrap = %v", wrap)
	}
	// Kelp Gull: 1.0 in Dec only => 120/160 = 75%.
	if wrap["Kelp Gull"] != 75 {
		t.Errorf("wrap kelp = %v", wrap["Kelp Gull"])
	}
}

func TestSpeciesIgnoresInvalidMonths(t *testing.T) {
	chart, _ := ParseBarchartPage(samplePage())
	if got := chart.Species(0, 99); len(got) == 0 {
		t.Fatal("invalid months should fall back to the whole year")
	}
}

type fakeFetcher struct {
	responses []int
	calls     int
	body      string
	err       error
}

func (f *fakeFetcher) Get(_ context.Context, _ string) (int, string, error) {
	if f.err != nil {
		return 0, "", f.err
	}
	status := 200
	if f.calls < len(f.responses) {
		status = f.responses[f.calls]
	}
	f.calls++
	return status, f.body, nil
}

func newSource(t *testing.T, f *fakeFetcher, opens *int) *Source {
	t.Helper()
	old := pause
	pause = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { pause = old })
	return &Source{
		Ctx:      context.Background(),
		CacheDir: t.TempDir(),
		Open: func() (Fetcher, func(), error) {
			*opens++
			return f, func() {}, nil
		},
	}
}

func TestSourceDownloadsOnceThenUsesCache(t *testing.T) {
	f, opens := &fakeFetcher{body: samplePage()}, 0
	s := newSource(t, f, &opens)

	for i := 0; i < 3; i++ {
		list, err := s.Species("L1", 1, 1)
		if err != nil || len(list) == 0 {
			t.Fatalf("list=%v err=%v", list, err)
		}
	}
	if f.calls != 1 || opens != 1 {
		t.Fatalf("calls=%d opens=%d", f.calls, opens)
	}
	if _, err := os.Stat(filepath.Join(s.CacheDir, "L1.json")); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}

	// A second Source (a later run) reads the cache and never starts a browser.
	opens2 := 0
	s2 := newSource(t, &fakeFetcher{}, &opens2)
	s2.CacheDir = s.CacheDir
	if _, err := s2.Species("L1", 5, 6); err != nil || opens2 != 0 {
		t.Fatalf("err=%v opens=%d", err, opens2)
	}
}

func TestSourceRetriesWhenRateLimited(t *testing.T) {
	f, opens := &fakeFetcher{body: samplePage(), responses: []int{429, 429, 200}}, 0
	s := newSource(t, f, &opens)
	if _, err := s.Species("L1", 1, 12); err != nil {
		t.Fatal(err)
	}
	if f.calls != 3 {
		t.Fatalf("calls = %d", f.calls)
	}
}

func TestSourceGivesUpAfterRepeatedRateLimits(t *testing.T) {
	f, opens := &fakeFetcher{body: samplePage(), responses: []int{429, 429, 429, 429}}, 0
	s := newSource(t, f, &opens)
	if _, err := s.Species("L1", 1, 12); err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("err = %v", err)
	}
}

func TestSourceReportsPagesWithoutData(t *testing.T) {
	f, opens := &fakeFetcher{body: "<html>blocked</html>"}, 0
	s := newSource(t, f, &opens)
	if _, err := s.Species("L1", 1, 12); !errors.Is(err, ErrNoBarchartData) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.CacheDir, "L1.json")); err == nil {
		t.Fatal("failed download must not be cached")
	}
}

func TestSourceRefreshesStaleCacheAndFallsBackWhenOffline(t *testing.T) {
	opens := 0
	s := newSource(t, &fakeFetcher{body: samplePage()}, &opens)
	if _, err := s.Species("L1", 1, 12); err != nil {
		t.Fatal(err)
	}

	later := time.Now().Add(cacheTTL + 24*time.Hour)
	s.Now = func() time.Time { return later }

	// Stale and the download fails: the old data is still used.
	s.Close()
	s.fetcher = &fakeFetcher{err: errors.New("offline")}
	if list, err := s.Species("L1", 1, 12); err != nil || len(list) == 0 {
		t.Fatalf("list=%v err=%v", list, err)
	}

	// Stale and the download works: the cache is rewritten with the new time.
	s.fetcher = &fakeFetcher{body: samplePage()}
	if _, err := s.Species("L1", 1, 12); err != nil {
		t.Fatal(err)
	}
	cached, err := readCache(s.cachePath("L1"))
	if err != nil || !cached.FetchedAt.Equal(later) {
		t.Fatalf("cached=%v err=%v", cached.FetchedAt, err)
	}
}

func TestBarchartURL(t *testing.T) {
	got := barchartURL("L485324", 2026)
	want := "https://ebird.org/barchart?bmo=1&byr=1900&emo=12&eyr=2026&fmt=tsv&r=L485324"
	if got != want {
		t.Fatalf("got %s", got)
	}
}

type flakyFetcher struct {
	bodies []string
	calls  int
	resets int
}

func (f *flakyFetcher) Get(context.Context, string) (int, string, error) {
	body := f.bodies[min(f.calls, len(f.bodies)-1)]
	f.calls++
	return 200, body, nil
}

func (f *flakyFetcher) Reset() { f.resets++ }

func TestSourceRetriesWhenPageHasNoChart(t *testing.T) {
	flaky, opens := &flakyFetcher{bodies: []string{"<html>Making sure you're not a bot</html>", samplePage()}}, 0
	s := newSource(t, &fakeFetcher{}, &opens)
	s.Open = func() (Fetcher, func(), error) { return flaky, func() {}, nil }
	if _, err := s.Species("L1", 1, 12); err != nil {
		t.Fatal(err)
	}
	if flaky.calls != 2 || flaky.resets != 1 {
		t.Fatalf("calls=%d resets=%d", flaky.calls, flaky.resets)
	}
}

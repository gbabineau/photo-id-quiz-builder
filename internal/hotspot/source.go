package hotspot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gbabineau/photo-id-quiz-builder/internal/trip"
)

const (
	// cacheTTL is how long a downloaded bar chart is reused; hotspot data changes slowly.
	cacheTTL      = 180 * 24 * time.Hour
	fetchAttempts = 3
)

// pause spaces out requests to eBird and the wait between retries; replaced in tests.
var pause = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Fetcher gets a page and reports its HTTP status. Browser is the production implementation.
type Fetcher interface {
	Get(ctx context.Context, url string) (status int, body string, err error)
}

// Source is a trip.HotspotSource that downloads eBird bar charts and caches them on disk.
type Source struct {
	Ctx      context.Context
	CacheDir string
	// Open starts the fetcher the first time a download is needed; its close func is called by Close.
	Open     func() (Fetcher, func(), error)
	Progress func(message string)
	Now      func() time.Time

	fetcher   Fetcher
	closeFunc func()
	fetched   bool
}

// NewSource returns a Source that downloads through headless Chromium.
func NewSource(ctx context.Context, cacheDir string, progress func(string)) *Source {
	return &Source{
		Ctx:      ctx,
		CacheDir: cacheDir,
		Progress: progress,
		Open: func() (Fetcher, func(), error) {
			b, err := NewBrowser()
			if err != nil {
				return nil, nil, err
			}
			return b, b.Close, nil
		},
	}
}

// Close releases the browser if one was started.
func (s *Source) Close() {
	if s.closeFunc != nil {
		s.closeFunc()
		s.closeFunc, s.fetcher = nil, nil
	}
}

func (s *Source) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Source) report(format string, args ...any) {
	if s.Progress != nil {
		s.Progress(fmt.Sprintf(format, args...))
	}
}

// Species implements trip.HotspotSource.
func (s *Source) Species(hotspotID string, startMonth, endMonth int) ([]trip.HotspotSpecies, error) {
	chart, err := s.load(hotspotID)
	if err != nil {
		return nil, err
	}
	return chart.Species(startMonth, endMonth), nil
}

func (s *Source) cachePath(hotspotID string) string {
	return filepath.Join(s.CacheDir, hotspotID+".json")
}

func (s *Source) load(hotspotID string) (Barchart, error) {
	cached, cacheErr := readCache(s.cachePath(hotspotID))
	if cacheErr == nil && s.now().Sub(cached.FetchedAt) < cacheTTL {
		s.report("Using cached data for %s", hotspotID)
		return cached, nil
	}

	chart, err := s.download(hotspotID)
	if err != nil {
		if cacheErr == nil {
			slog.Warn("hotspot refresh failed; using older cached data", "hotspot", hotspotID, "error", err)
			return cached, nil
		}
		return Barchart{}, err
	}
	if err := writeCache(s.cachePath(hotspotID), chart); err != nil {
		slog.Warn("could not cache hotspot data", "hotspot", hotspotID, "error", err)
	}
	return chart, nil
}

func (s *Source) download(hotspotID string) (Barchart, error) {
	if s.fetcher == nil {
		s.report("Starting browser (the first run downloads Chromium)")
		fetcher, closeFn, err := s.Open()
		if err != nil {
			return Barchart{}, err
		}
		s.fetcher, s.closeFunc = fetcher, closeFn
	}
	if s.fetched {
		if err := pause(s.Ctx, time.Second); err != nil {
			return Barchart{}, err
		}
	}
	s.fetched = true
	s.report("Downloading %s from eBird", hotspotID)

	pageURL := barchartURL(hotspotID, s.now().Year())
	for attempt := 1; ; attempt++ {
		status, body, err := s.fetcher.Get(s.Ctx, pageURL)
		if err != nil {
			return Barchart{}, err
		}
		if status == 429 || status == 503 {
			wait := 5 * time.Second << (attempt - 1)
			if attempt >= fetchAttempts {
				return Barchart{}, fmt.Errorf("eBird returned status %d after %d attempts", status, attempt)
			}
			slog.Debug("eBird rate limited; retrying", "hotspot", hotspotID, "status", status,
				"attempt", attempt, "delay", wait.String(), "delaySource", "exponential backoff")
			s.report("eBird asked us to slow down; waiting %s", wait)
			if err := pause(s.Ctx, wait); err != nil {
				return Barchart{}, err
			}
			continue
		}
		if status != 200 {
			return Barchart{}, fmt.Errorf("eBird returned status %d for %s", status, hotspotID)
		}
		chart, err := ParseBarchartPage(body)
		if errors.Is(err, ErrNoBarchartData) && attempt < fetchAttempts {
			wait := 3 * time.Second * time.Duration(attempt)
			slog.Debug("page had no bar chart; retrying", "hotspot", hotspotID, "attempt", attempt,
				"delay", wait.String(), "bodyStart", snippet(body))
			if r, ok := s.fetcher.(interface{ Reset() }); ok {
				r.Reset()
			}
			if err := pause(s.Ctx, wait); err != nil {
				return Barchart{}, err
			}
			continue
		}
		if err != nil {
			return Barchart{}, fmt.Errorf("%s: %w", hotspotID, err)
		}
		chart.HotspotID = hotspotID
		chart.FetchedAt = s.now()
		return chart, nil
	}
}

func snippet(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > 200 {
		body = body[:200]
	}
	return body
}

func barchartURL(hotspotID string, year int) string {
	q := url.Values{}
	q.Set("byr", "1900")
	q.Set("eyr", fmt.Sprint(year))
	q.Set("bmo", "1")
	q.Set("emo", "12")
	q.Set("fmt", "tsv")
	q.Set("r", hotspotID)
	return "https://ebird.org/barchart?" + q.Encode()
}

func readCache(path string) (Barchart, error) {
	var chart Barchart
	data, err := os.ReadFile(path)
	if err != nil {
		return chart, err
	}
	if err := json.Unmarshal(data, &chart); err != nil {
		return Barchart{}, err
	}
	if len(chart.Samples) != weeksPerYear {
		return Barchart{}, errors.New("cached bar chart is malformed")
	}
	return chart, nil
}

func writeCache(path string, chart Barchart) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(chart)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

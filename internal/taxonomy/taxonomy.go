// Package taxonomy loads the eBird taxonomy from a file, a local cache, or the eBird API.
package taxonomy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gbabineau/photo-id-quiz-builder/internal/ebird"
)

const (
	APIURL      = "https://api.ebird.org/v2/ref/taxonomy/ebird?fmt=json&locale=en"
	APIKeyEnv   = "EBIRDAPIKEY"
	FilePathEnv = "EBIRD_TAXONOMY_PATH"
)

// Taxon is an eBird taxonomy record.
type Taxon struct {
	CommonName     string   `json:"comName"`
	ScientificName string   `json:"sciName"`
	SpeciesCode    string   `json:"speciesCode"`
	Category       string   `json:"category"`
	TaxonOrder     float64  `json:"taxonOrder"`
	BandingCodes   []string `json:"bandingCodes"`
	ComNameCodes   []string `json:"comNameCodes"`
	SciNameCodes   []string `json:"sciNameCodes"`
	Order          string   `json:"order"`
	FamilyCode     string   `json:"familyCode"`
	FamilyComName  string   `json:"familyComName"`
	FamilySciName  string   `json:"familySciName"`
}

// Load returns the taxonomy from EBIRD_TAXONOMY_PATH, the user cache, or the eBird API
// (using the EBIRDAPIKEY environment variable), caching API results.
func Load(ctx context.Context) ([]Taxon, error) {
	if path := os.Getenv(FilePathEnv); path != "" {
		return Read(path)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("find user cache directory: %w", err)
	}
	cachePath := filepath.Join(cacheDir, "photo-id-quiz-builder", "taxonomy.json")
	cached, readErr := Read(cachePath)
	if readErr == nil && cacheIsFresh(cachePath, now()) {
		return cached, nil
	}
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return nil, readErr
	}

	apiKey := strings.TrimSpace(os.Getenv(APIKeyEnv))
	if apiKey == "" {
		if readErr == nil {
			slog.Warn("taxonomy cache is past its refresh date but no API key is set; using it", "envVar", APIKeyEnv)
			return cached, nil
		}
		return nil, fmt.Errorf("taxonomy not found: set %s (eBird API key) or %s (path to a taxonomy JSON file)", APIKeyEnv, FilePathEnv)
	}
	data, taxa, err := Download(ctx, apiKey, http.DefaultClient, APIURL)
	if err != nil {
		if readErr == nil {
			slog.Warn("taxonomy refresh failed; using older cache", "error", err)
			return cached, nil
		}
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		return nil, fmt.Errorf("create taxonomy cache directory: %w", err)
	}
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		return nil, fmt.Errorf("save taxonomy cache: %w", err)
	}
	return taxa, nil
}

var now = time.Now

func cacheIsFresh(path string, at time.Time) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return FreshAt(info.ModTime(), at)
}

// FreshAt reports whether a taxonomy downloaded at cachedAt is still current at the given time.
// eBird publishes a new taxonomy in late October, so a download is good until Oct 25. A download
// made during the Oct 25-31 window may predate the release and is replaced once, from Nov 1.
func FreshAt(cachedAt, at time.Time) bool {
	cutoff := time.Date(at.Year(), time.October, 25, 0, 0, 0, 0, at.Location())
	if at.Before(cutoff) {
		cutoff = cutoff.AddDate(-1, 0, 0)
	}
	if cachedAt.Before(cutoff) {
		return false
	}
	windowEnd := time.Date(cutoff.Year(), time.November, 1, 0, 0, 0, 0, at.Location())
	return !(cachedAt.Before(windowEnd) && !at.Before(windowEnd))
}

// Read parses a taxonomy JSON file.
func Read(path string) ([]Taxon, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read taxonomy: %w", err)
	}
	var taxa []Taxon
	if err := json.Unmarshal(data, &taxa); err != nil {
		return nil, fmt.Errorf("parse taxonomy: %w", err)
	}
	return taxa, nil
}

// Download fetches the taxonomy, returning both raw bytes (for caching) and parsed records.
func Download(ctx context.Context, apiKey string, client *http.Client, endpoint string) ([]byte, []Taxon, error) {
	data, err := ebird.Get(ctx, client, apiKey, endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("download eBird taxonomy: %w", err)
	}
	var taxa []Taxon
	if err := json.Unmarshal(data, &taxa); err != nil {
		return nil, nil, fmt.Errorf("parse eBird taxonomy response: %w", err)
	}
	if len(taxa) == 0 {
		return nil, nil, fmt.Errorf("eBird taxonomy response was empty")
	}
	return data, taxa, nil
}

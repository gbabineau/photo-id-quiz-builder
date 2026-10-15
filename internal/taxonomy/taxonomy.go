// Package taxonomy loads the eBird taxonomy from a file, a local cache, or the eBird API.
package taxonomy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	taxa, err := Read(cachePath)
	if err == nil {
		return taxa, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	apiKey := strings.TrimSpace(os.Getenv(APIKeyEnv))
	if apiKey == "" {
		return nil, fmt.Errorf("taxonomy not found: set %s (eBird API key) or %s (path to a taxonomy JSON file)", APIKeyEnv, FilePathEnv)
	}
	data, taxa, err := Download(ctx, apiKey, http.DefaultClient, APIURL)
	if err != nil {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create taxonomy request: %w", err)
	}
	req.Header.Set("X-eBirdApiToken", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("download eBird taxonomy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("download eBird taxonomy: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("read eBird taxonomy response: %w", err)
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

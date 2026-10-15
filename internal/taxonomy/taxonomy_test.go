package taxonomy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadTaxonomy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.json")
	os.WriteFile(path, []byte(`[{"comName":"Dipper","speciesCode":"dip1","taxonOrder":12.5,"category":"species"}]`), 0o600)
	taxa, err := Read(path)
	if err != nil || len(taxa) != 1 || taxa[0].TaxonOrder != 12.5 {
		t.Fatalf("taxa=%+v err=%v", taxa, err)
	}
}

func TestLoadFromEnvPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.json")
	os.WriteFile(path, []byte(`[{"comName":"Dipper"}]`), 0o600)
	t.Setenv(FilePathEnv, path)
	taxa, err := Load(context.Background())
	if err != nil || len(taxa) != 1 {
		t.Fatalf("taxa=%+v err=%v", taxa, err)
	}
}

func TestLoadWithoutKeyOrCacheExplainsConfiguration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(FilePathEnv, "")
	t.Setenv(APIKeyEnv, "")
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
	_, err := Load(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadSendsTokenAndParses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-eBirdApiToken") != "secret" {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		w.Write([]byte(`[{"comName":"Dipper"}]`))
	}))
	defer server.Close()

	_, taxa, err := Download(context.Background(), "secret", server.Client(), server.URL)
	if err != nil || len(taxa) != 1 {
		t.Fatalf("taxa=%+v err=%v", taxa, err)
	}
	if _, _, err := Download(context.Background(), "wrong", server.Client(), server.URL); err == nil {
		t.Fatal("expected error for bad key")
	}
}

func TestFreshAt(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 12, 0, 0, 0, time.UTC) }
	cases := []struct {
		name     string
		cachedAt time.Time
		at       time.Time
		want     bool
	}{
		{"downloaded last November, now mid-October", d(2025, 11, 5), d(2026, 10, 15), true},
		{"downloaded this January", d(2026, 1, 10), d(2026, 10, 24), true},
		{"downloaded before last release window", d(2025, 10, 10), d(2026, 10, 15), false},
		{"downloaded in last release window, now November", d(2025, 10, 28), d(2026, 1, 5), false},
		{"downloaded in release window, still in window", d(2026, 10, 27), d(2026, 10, 30), true},
		{"old cache after release date", d(2025, 11, 5), d(2026, 10, 26), false},
		{"downloaded in release window, now November", d(2026, 10, 27), d(2026, 11, 2), false},
		{"downloaded after window, now later", d(2026, 11, 2), d(2027, 3, 1), true},
	}
	for _, c := range cases {
		if got := FreshAt(c.cachedAt, c.at); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestLoadUsesFreshCacheWithoutNetwork(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(FilePathEnv, "")
	t.Setenv(APIKeyEnv, "")
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
	cacheDir, _ := os.UserCacheDir()
	path := filepath.Join(cacheDir, "photo-id-quiz-builder", "taxonomy.json")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte(`[{"comName":"Dipper"}]`), 0o600)

	old := now
	now = func() time.Time { return time.Now().AddDate(0, 0, 1) }
	defer func() { now = old }()
	// File mtime is "now"; with the clock a day ahead it is fresh unless inside the release window.
	if !FreshAt(time.Now(), now()) {
		t.Skip("test clock falls in the October release window")
	}
	taxa, err := Load(context.Background())
	if err != nil || len(taxa) != 1 {
		t.Fatalf("taxa=%v err=%v", taxa, err)
	}
}

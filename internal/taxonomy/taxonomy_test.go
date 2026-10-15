package taxonomy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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

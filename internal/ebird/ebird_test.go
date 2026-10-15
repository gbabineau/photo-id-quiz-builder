package ebird

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func recordSleeps(t *testing.T) *[]time.Duration {
	t.Helper()
	var waits []time.Duration
	old := sleep
	sleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	t.Cleanup(func() { sleep = old })
	return &waits
}

func TestGetRetriesAfterRetryAfterSeconds(t *testing.T) {
	waits := recordSleeps(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-eBirdApiToken") != "key" {
			t.Error("missing token")
		}
		if calls == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	data, err := Get(context.Background(), server.Client(), "key", server.URL)
	if err != nil || string(data) != "ok" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if len(*waits) != 1 || (*waits)[0] != 7*time.Second {
		t.Fatalf("waits = %v", *waits)
	}
}

func TestGetBacksOffWithoutHeaderThenGivesUp(t *testing.T) {
	waits := recordSleeps(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := Get(context.Background(), server.Client(), "key", server.URL)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("err = %v", err)
	}
	if len(*waits) != maxAttempts-1 || (*waits)[0] != 2*time.Second || (*waits)[1] != 4*time.Second {
		t.Fatalf("waits = %v", *waits)
	}
}

func TestGetFailsWhenServerAsksForTooLongAWait(t *testing.T) {
	waits := recordSleeps(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "86400")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	if _, err := Get(context.Background(), server.Client(), "key", server.URL); err == nil {
		t.Fatal("expected error")
	}
	if len(*waits) != 0 {
		t.Fatalf("should not have waited: %v", *waits)
	}
}

func TestGetDoesNotRetryOtherErrors(t *testing.T) {
	waits := recordSleeps(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	if _, err := Get(context.Background(), server.Client(), "key", server.URL); err == nil {
		t.Fatal("expected error")
	}
	if len(*waits) != 0 {
		t.Fatalf("waits = %v", *waits)
	}
}

func TestRetryDelayHTTPDate(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	d, _ := retryDelay(now.Add(30*time.Second).Format(http.TimeFormat), 1, now)
	if d != 30*time.Second {
		t.Fatalf("d = %v", d)
	}
}

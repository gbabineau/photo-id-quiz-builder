// Package ebird makes eBird API requests that honour rate limiting.
package ebird

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxAttempts  = 5
	maxWait      = 5 * time.Minute
	fallbackWait = 2 * time.Second
	maxBodyBytes = 64 << 20
)

// sleep waits for d or until ctx is done; replaced in tests.
var sleep = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Get requests endpoint with the eBird token and returns the body. On 429 or 503 it waits for
// the time given by Retry-After (or an exponential fallback) and tries again.
func Get(ctx context.Context, client *http.Client, apiKey, endpoint string) ([]byte, error) {
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create eBird request: %w", err)
		}
		req.Header.Set("X-eBirdApiToken", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("eBird request: %w", err)
		}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable
		if !retryable {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("eBird request: %s", resp.Status)
			}
			data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
			if err != nil {
				return nil, fmt.Errorf("read eBird response: %w", err)
			}
			return data, nil
		}

		status, header := resp.Status, resp.Header.Get("Retry-After")
		resp.Body.Close()
		wait, source := retryDelay(header, attempt, time.Now())
		if attempt >= maxAttempts || wait > maxWait {
			return nil, fmt.Errorf("eBird request: %s after %d attempt(s); server asked to wait %s", status, attempt, wait)
		}
		slog.Debug("eBird rate limited; retrying", "status", status, "attempt", attempt,
			"delay", wait.String(), "delaySource", source)
		if err := sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

// retryDelay reads Retry-After (seconds or HTTP date), falling back to exponential backoff.
func retryDelay(header string, attempt int, now time.Time) (time.Duration, string) {
	header = strings.TrimSpace(header)
	if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, "Retry-After seconds"
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := t.Sub(now); d > 0 {
			return d, "Retry-After date"
		}
		return 0, "Retry-After date"
	}
	return fallbackWait << (attempt - 1), "exponential backoff"
}

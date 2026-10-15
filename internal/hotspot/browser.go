package hotspot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/mxschmitt/playwright-go"
)

const (
	eBirdHome   = "https://ebird.org/home"
	userAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	pageTimeout = 60_000 // ms; leaves time for the Anubis proof-of-work to resolve
	maxAttempts = 3
)

var installOnce sync.Once
var installErr error

// Browser fetches eBird pages through headless Chromium so that Anubis proof-of-work
// challenges resolve as they would for a person. Close it when finished.
type Browser struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	page    playwright.Page
	ready   bool
}

// NewBrowser installs Chromium on first use and starts it.
func NewBrowser() (*Browser, error) {
	installOnce.Do(func() {
		installErr = playwright.Install(&playwright.RunOptions{
			Browsers:         []string{"chromium"},
			OnlyInstallShell: true,
		})
	})
	if installErr != nil {
		return nil, fmt.Errorf("install Playwright Chromium: %w", installErr)
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start Playwright: %w", err)
	}
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("launch Chromium: %w", err)
	}
	bctx, err := browser.NewContext(playwright.BrowserNewContextOptions{UserAgent: playwright.String(userAgent)})
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("create browser context: %w", err)
	}
	page, err := bctx.NewPage()
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("create page: %w", err)
	}
	page.SetDefaultTimeout(pageTimeout)
	return &Browser{pw: pw, browser: browser, page: page}, nil
}

func (b *Browser) Close() {
	b.browser.Close()
	b.pw.Stop()
}

// pass loads an ebird.org page once so the Anubis cookie is set for later same-origin fetches.
func (b *Browser) pass() error {
	if b.ready {
		return nil
	}
	slog.Debug("loading ebird.org to resolve Anubis proof-of-work")
	if _, err := b.page.Goto(eBirdHome, playwright.PageGotoOptions{Timeout: playwright.Float(pageTimeout)}); err != nil {
		return fmt.Errorf("open ebird.org: %w", err)
	}
	if err := b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle, Timeout: playwright.Float(pageTimeout),
	}); err != nil {
		return fmt.Errorf("wait for ebird.org: %w", err)
	}
	b.ready = true
	return nil
}

// Reset makes the next Get reload ebird.org first, re-solving any Anubis challenge.
func (b *Browser) Reset() { b.ready = false }

// Get fetches url from within the ebird.org page, returning the HTTP status and body.
func (b *Browser) Get(ctx context.Context, url string) (int, string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return 0, "", err
		}
		if err := b.pass(); err != nil {
			lastErr = err
			b.ready = false
			slog.Debug("eBird page load failed", "attempt", attempt, "error", err)
			continue
		}
		out, err := b.page.Evaluate(`async (u) => {
			const r = await fetch(u, {credentials: 'include'});
			return r.status + "\n" + await r.text();
		}`, url)
		if err != nil {
			lastErr = err
			b.ready = false
			slog.Debug("eBird fetch failed", "attempt", attempt, "error", err)
			continue
		}
		text, _ := out.(string)
		statusLine, body, _ := strings.Cut(text, "\n")
		var status int
		fmt.Sscanf(statusLine, "%d", &status)
		return status, body, nil
	}
	return 0, "", fmt.Errorf("eBird request failed after %d attempts: %w", maxAttempts, lastErr)
}

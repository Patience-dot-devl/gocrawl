// Package render provides the headless-rendering Fetcher backed by chromedp. It renders
// pages in a real Chromium tab, captures the post-JS DOM, records main-document
// redirects/status/headers via DevTools network events, and collects lab-mode Core Web
// Vitals (LCP, FCP, CLS, TBT, TTFB) using PerformanceObserver. On any rendering error
// the fetcher falls back to a raw HTTP fetch so a single broken page does not stall
// the crawl.
//
// Headless mode requires a Chromium-class browser (Chrome, Chromium, or Edge) on PATH.
// NewHeadlessFetcher returns an error if no compatible browser can be launched.
package render

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// HeadlessFetcher renders pages with chromedp and reports real Core Web Vitals.
type HeadlessFetcher struct {
	opts        crawler.Options
	allocCtx    context.Context
	allocCancel context.CancelFunc
	raw         *crawler.HTTPFetcher
	ua          *crawler.UAPool

	closeMu sync.Mutex
	closed  bool
}

// NewHeadlessFetcher launches the headless browser. It returns an error if no Chromium
// binary is available on PATH or the browser fails to start. Callers must Close() the
// fetcher when finished.
func NewHeadlessFetcher(opts crawler.Options) (*HeadlessFetcher, error) {
	allocOpts := append([]chromedp.ExecAllocatorOption{},
		chromedp.DefaultExecAllocatorOptions[:]...)
	ua := crawler.NewUAPool(opts)
	// Set the browser-level User-Agent to the pool's first entry. When the pool rotates, each
	// navigation overrides it per-page via network.SetUserAgentOverride in headless().
	if def := ua.Default(); def != "" {
		allocOpts = append(allocOpts, chromedp.UserAgent(def))
	}
	// Chromium takes a single proxy per browser process, so headless mode uses the first proxy
	// in the pool; per-request proxy rotation applies to raw mode only. Credentials in the URL
	// are dropped here (Chromium prompts for proxy auth via a separate CDP flow we don't drive).
	if len(opts.Proxies) > 0 {
		allocOpts = append(allocOpts, chromedp.ProxyServer(proxyServerArg(opts.Proxies[0])))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)

	// Warm the browser so a missing/broken Chromium surfaces here, not mid-crawl.
	warmCtx, warmCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(quietErrorf))
	defer warmCancel()
	if err := chromedp.Run(warmCtx); err != nil {
		allocCancel()
		return nil, fmt.Errorf("launching headless browser: %w", err)
	}

	return &HeadlessFetcher{
		opts:        opts,
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		raw:         crawler.NewHTTPFetcher(opts),
		ua:          ua,
	}, nil
}

// proxyServerArg renders a proxy URL as Chromium's --proxy-server value: scheme://host[:port],
// without any userinfo (Chromium rejects credentials embedded in the flag).
func proxyServerArg(u *url.URL) string {
	return u.Scheme + "://" + u.Host
}

// hostOf extracts the hostname from rawURL for sticky-host User-Agent rotation. A parse
// failure yields "" (the sticky hash then maps everything to one agent, which is harmless).
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// quietErrorf is chromedp's error logger with the benign "unhandled node event"
// noise filtered out. chromedp logs that for CDP DOM events its version doesn't
// model (e.g. *dom.EventAdoptedStyleSheetsModified, emitted by sites using
// constructable stylesheets). It has no effect on rendering, so we drop it and
// pass everything else through to the default logger.
func quietErrorf(format string, args ...interface{}) {
	if strings.Contains(format, "unhandled node event") {
		return
	}
	log.Printf(format, args...)
}

// Close terminates the browser. Safe to call more than once.
func (h *HeadlessFetcher) Close() error {
	h.closeMu.Lock()
	defer h.closeMu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	if h.allocCancel != nil {
		h.allocCancel()
	}
	return nil
}

// Fetch renders rawURL in a fresh tab. On rendering failure it falls back to a raw
// HTTP fetch and annotates the page's RenderResult with the failure reason.
func (h *HeadlessFetcher) Fetch(ctx context.Context, rawURL string) (*crawler.Page, error) {
	page, err := h.headless(ctx, rawURL)
	if err == nil {
		return page, nil
	}
	return h.fallback(ctx, rawURL, err)
}

func (h *HeadlessFetcher) fallback(ctx context.Context, rawURL string, cause error) (*crawler.Page, error) {
	page, err := h.raw.Fetch(ctx, rawURL)
	if page != nil {
		page.Render = &crawler.RenderResult{
			Implemented: false,
			Note:        fmt.Sprintf("headless render failed: %s", cause),
		}
	}
	return page, err
}

// maxCapturedRequests bounds how many outbound request URLs a single render records. A
// typical page issues well under this; the cap guards against runaway pages (infinite
// pollers, ad refreshes) without losing the analytics beacons we care about.
const maxCapturedRequests = 400

// navState tracks main-document network activity across redirects, plus every outbound
// request URL (bounded) so analyzers can tell which tags actually fired.
type navState struct {
	mu        sync.Mutex
	navID     network.RequestID
	current   string
	redirects []crawler.Redirect
	status    int
	headers   network.Headers
	requests  []string
}

func (h *HeadlessFetcher) headless(ctx context.Context, rawURL string) (*crawler.Page, error) {
	start := time.Now()

	timeout := h.opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	tabCtx, tabCancel := chromedp.NewContext(h.allocCtx, chromedp.WithErrorf(quietErrorf))
	defer tabCancel()
	runCtx, runCancel := context.WithTimeout(tabCtx, timeout)
	defer runCancel()
	// Propagate caller cancellation (e.g. global crawl ctx) without overriding the timeout.
	stop := context.AfterFunc(ctx, runCancel)
	defer stop()

	// Fetch the pre-JS HTML in parallel with rendering so the GEO JS-dependency check can
	// compare what a non-executing AI crawler sees against the post-JS DOM. This overlaps the
	// render's settle time, so it adds little wall-clock; failure is non-fatal (RawBody stays nil).
	rawCh := make(chan []byte, 1)
	go func() {
		if rawPage, rerr := h.raw.Fetch(ctx, rawURL); rerr == nil && rawPage != nil {
			rawCh <- rawPage.Body
			return
		}
		rawCh <- nil
	}()

	state := &navState{current: rawURL}
	chromedp.ListenTarget(runCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			state.mu.Lock()
			defer state.mu.Unlock()
			if len(state.requests) < maxCapturedRequests {
				state.requests = append(state.requests, e.Request.URL)
			}
			if e.Type != network.ResourceTypeDocument {
				return
			}
			if state.navID == "" {
				state.navID = e.RequestID
			}
			if e.RequestID != state.navID {
				return
			}
			if e.RedirectResponse != nil {
				state.redirects = append(state.redirects, crawler.Redirect{
					From:   state.current,
					To:     e.DocumentURL,
					Status: int(e.RedirectResponse.Status),
				})
				state.current = e.DocumentURL
			}
		case *network.EventResponseReceived:
			if e.Type != network.ResourceTypeDocument {
				return
			}
			state.mu.Lock()
			defer state.mu.Unlock()
			if state.navID == "" || e.RequestID != state.navID {
				return
			}
			state.status = int(e.Response.Status)
			state.headers = e.Response.Headers
		}
	})

	var (
		htmlBody string
		metrics  cwvJS
		dlJSON   string
	)
	actions := []chromedp.Action{network.Enable()}
	// When the UA pool rotates, pick this page's agent and override it for the navigation. A
	// single-agent pool is already set browser-wide at allocation, so we skip the override.
	if h.ua.Rotates() {
		if ua := h.ua.Next(hostOf(rawURL)); ua != "" {
			actions = append(actions, emulation.SetUserAgentOverride(ua))
		}
	}
	actions = append(actions,
		chromedp.ActionFunc(func(c context.Context) error {
			_, err := cdppage.AddScriptToEvaluateOnNewDocument(cwvBootstrap).Do(c)
			return err
		}),
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		// Allow paint, layout shifts, long tasks, and tag-manager pushes to settle before
		// reading metrics and the dataLayer.
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(cwvReadScript, &metrics),
		chromedp.Evaluate(dataLayerReadScript, &dlJSON),
		chromedp.OuterHTML("html", &htmlBody, chromedp.ByQuery),
	)
	if err := chromedp.Run(runCtx, actions...); err != nil {
		return nil, err
	}
	dlPresent, dlEntries := parseDataLayer(dlJSON)

	p := &crawler.Page{
		RequestedURL: rawURL,
		FetchedAt:    start,
		Duration:     time.Since(start),
		Body:         []byte(htmlBody),
		RawBody:      <-rawCh,
		Redirects:    state.redirects,
	}
	state.mu.Lock()
	p.FinalURL = state.current
	p.StatusCode = state.status
	p.Header = headersToHTTP(state.headers)
	state.mu.Unlock()
	p.ContentType = p.Header.Get("Content-Type")
	if p.ContentType == "" || isHTMLContentType(p.ContentType) {
		if doc, derr := goquery.NewDocumentFromReader(strings.NewReader(htmlBody)); derr == nil {
			p.Doc = doc
		}
	}
	state.mu.Lock()
	reqs := append([]string(nil), state.requests...)
	state.mu.Unlock()
	p.Render = &crawler.RenderResult{
		Implemented:      true,
		LCP:              metrics.LCP,
		FCP:              metrics.FCP,
		CLS:              metrics.CLS,
		TBT:              metrics.TBT,
		TTFB:             metrics.TTFB,
		DataLayerPresent: dlPresent,
		DataLayer:        dlEntries,
		Requests:         reqs,
	}
	return p, nil
}

type cwvJS struct {
	LCP  float64 `json:"lcp"`
	FCP  float64 `json:"fcp"`
	CLS  float64 `json:"cls"`
	TBT  float64 `json:"tbt"`
	TTFB float64 `json:"ttfb"`
}

// cwvBootstrap is injected before navigation so PerformanceObservers are armed for
// every event the page emits. Layout-shift entries with hadRecentInput are excluded
// per the CLS spec; long-task entries contribute (duration-50)ms to TBT above 50ms.
const cwvBootstrap = `
(() => {
  window.__cwv = { lcp: 0, fcp: 0, cls: 0, tbt: 0, ttfb: 0 };
  const safeObserve = (type, cb) => {
    try { new PerformanceObserver(cb).observe({ type, buffered: true }); } catch (_) {}
  };
  safeObserve('largest-contentful-paint', (list) => {
    const e = list.getEntries();
    const last = e[e.length - 1];
    if (last) window.__cwv.lcp = last.renderTime || last.startTime;
  });
  safeObserve('paint', (list) => {
    for (const e of list.getEntries()) {
      if (e.name === 'first-contentful-paint') window.__cwv.fcp = e.startTime;
    }
  });
  safeObserve('layout-shift', (list) => {
    for (const e of list.getEntries()) {
      if (!e.hadRecentInput) window.__cwv.cls += e.value;
    }
  });
  safeObserve('longtask', (list) => {
    for (const e of list.getEntries()) {
      if (e.duration > 50) window.__cwv.tbt += e.duration - 50;
    }
  });
})();
`

const cwvReadScript = `
(() => {
  const out = window.__cwv || { lcp: 0, fcp: 0, cls: 0, tbt: 0, ttfb: 0 };
  const nav = performance.getEntriesByType('navigation')[0];
  if (nav) out.ttfb = Math.max(0, nav.responseStart - nav.requestStart);
  return out;
})()
`

// dataLayerReadScript serializes window.dataLayer to a JSON string after the page has
// settled. GTM's dataLayer is append-only (entries are never removed once pushed), so the
// final array is the full event history — no push hook is needed. The replacer drops
// functions and breaks cycles so a single un-serializable entry can't fail the whole read.
// It returns a string (not an object) because chromedp serializes deep page objects
// unreliably; we re-parse the string on the Go side.
const dataLayerReadScript = `
(() => {
  try {
    const dl = window.dataLayer;
    if (!Array.isArray(dl)) return JSON.stringify({ present: false, entries: [] });
    const seen = new WeakSet();
    const replacer = (k, v) => {
      if (typeof v === 'function') return undefined;
      if (typeof v === 'object' && v !== null) {
        if (seen.has(v)) return undefined;
        seen.add(v);
      }
      return v;
    };
    return JSON.stringify({ present: true, entries: dl }, replacer);
  } catch (e) {
    return JSON.stringify({ present: false, entries: [] });
  }
})()
`

// parseDataLayer decodes the dataLayerReadScript output into a presence flag and the raw
// JSON of each entry. A blank or malformed payload yields (false, nil) so callers degrade
// gracefully.
func parseDataLayer(s string) (bool, []json.RawMessage) {
	if strings.TrimSpace(s) == "" {
		return false, nil
	}
	var snap struct {
		Present bool              `json:"present"`
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal([]byte(s), &snap); err != nil {
		return false, nil
	}
	return snap.Present, snap.Entries
}

func headersToHTTP(hs network.Headers) http.Header {
	out := http.Header{}
	for k, v := range hs {
		switch val := v.(type) {
		case string:
			out.Set(k, val)
		case []interface{}:
			for _, item := range val {
				if s, ok := item.(string); ok {
					out.Add(k, s)
				}
			}
		}
	}
	return out
}

func isHTMLContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

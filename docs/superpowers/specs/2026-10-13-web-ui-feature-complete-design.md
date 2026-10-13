---
id: "2026-10-13-web-ui-feature-complete-design"
title: "Feature-complete web UI"
scope: "Proposal to bring every CLI capability into the web UI, with live sitemap-based progress, pause/resume, crawl auth, and a new visual system; approved, not yet built"
topic: "spec"
status: "current"
superseded_by: []
updated: 2026-10-13
depends_on: []
sources: []
---
# Feature-complete web UI

Date: 2026-10-13

## Goal

Make the `gocrawl serve` UI a complete, self-contained GUI for gocrawl: everything the CLI does,
plus live progress, without a terminal. This is sub-project 1 of 2. Sub-project 2 wraps this UI
in a native desktop window with `webview/webview_go` and ships installers for macOS, Windows and
Linux. It gets its own spec, and nothing in this one depends on it beyond two constraints called
out below (no `localStorage`, no reliance on browser downloads).

Audience: public open-source users on all three desktop platforms, including SEO/SEA people who
never open a terminal.

## Decisions taken while brainstorming

- Order: feature-complete web UI first, then the desktop shell.
- Desktop shell approach: `webview_go`, a separate CGO binary; the CLI stays `CGO_ENABLED=0`.
- New capabilities: compare crawls, check redirects, live progress, settings and profiles,
  report import, crawl auth (bearer token and interactive browser login), pause and resume.
- Progress is measured against the site's sitemap, not the page budget, and the UI warns when
  the budget is smaller than the sitemap.
- Visual direction "crawl tape", dark by default, WCAG 2.2 AA, with HTTP-status colors
  (2xx green, 3xx yellow, 4xx dark orange, 5xx red) and violet for interface chrome.
- Crawl controls: green start/resume, yellow pause, red stop.
- Out of scope: auth for the GUI itself, scripted form login, saved auth profiles, deleting
  history entries, SSE or websockets, editing YAML config files from the UI, seeding the crawl
  frontier from the sitemap.

## Constraints

- The UI must keep working when served from a different `127.0.0.1` port on every launch (the
  desktop shell does this), so nothing persists in `localStorage`; UI state that must survive
  lives server-side in `~/.gocrawl/gui.json`.
- Embedded webviews handle file downloads unreliably. Exports keep their current download links
  for the browser; the desktop spec will add a native save path. No feature in this spec may
  depend on a download completing.
- Everything stays in the existing guard model (`internal/webserver/guard.go`): loopback `Host`
  check, same-origin check on non-`GET`, JSON content type on JSON bodies. New `POST`, `PUT`
  and `DELETE` routes go through it, and multipart routes get the same origin check.
- No new frontend runtime dependencies. Fonts are bundled, never loaded from a CDN.

## Backend

### 1. Live progress

**Engine hook.** `crawler.Options` gains `OnProgress func(Progress)`, nil by default. The engine
calls it after each page is recorded (where `result.Pages` is appended in `Engine.Crawl`):

```go
type Progress struct {
    Fetched    int    // pages recorded so far
    Queued     int    // frontier length
    CurrentURL string // URL just recorded
    Status     int    // its HTTP status (0 on fetch error)
    Rate       float64 // limiter's current requests per second
    Throttled  int    // 429/503 responses in the last 60s
    Paused     bool
    PauseReason string // "", "user", "rate_limited"
}
```

The callback runs on a worker goroutine; it must not block. The web job copies it into its
snapshot under the job mutex.

**Runner.** `runner.Run(ctx, cfg, seed, opts ...RunOption)`. Existing callers are unchanged.
`WithProgress(fn func(runner.Progress))` passes `crawler.Progress` through and adds the phase:
`sitemap` (discovery pre-pass), `crawling`, `analyzing`.

**Sitemap pre-pass.** Before the crawl starts, the runner discovers the site's sitemap once:
robots.txt `Sitemap:` lines, then `/sitemap.xml` and `/sitemap_index.xml`, following indexes,
through the existing `sitemap.Fetch` (`internal/analyze/sitemap/sitemap.go`) on the governed
fetcher. It counts only URLs inside the crawl scope, using a new exported
`(*crawler.Engine).InScope(rawURL string) bool` that applies the same host, subdomain,
include and exclude rules as `enqueue` (not robots.txt or depth). The `sitemap` analyzer is
given the pre-fetched URL set (a new functional option on `sitemap.New`) so the sitemap is
fetched once per crawl. The candidate-building code in `Analyze` moves into an exported
`sitemap.Discover(ctx, fetcher, seed, robots)` used by both.

The pre-pass runs for every crawl, CLI and MCP included, because the analyzer needs the set
anyway.

**Sitemap totals.** The runner holds the sitemap set, so it wraps the engine callback and
reports a `runner.Progress` that embeds `crawler.Progress` and adds `Phase`, plus:

- `SitemapTotal`: in-scope sitemap URLs (0 when no sitemap was found).
- `SitemapFetched`: fetched pages whose normalized URL is in that set.
- `OffSitemap`: `Fetched - SitemapFetched`.

`report.Coverage` gains `SitemapTotal` and `SitemapFetched` (JSON `sitemap_total`,
`sitemap_fetched`, omitted when no sitemap was found), so History and Compare can show
sitemap coverage per crawl.

**Budget below sitemap.** When `MaxPages > 0` and `MaxPages < SitemapTotal`, the runner appends
a note to `Report.Notes`: "The page budget (500) is below the sitemap total (1,240); this crawl
covered at most 40% of the sitemap." The CLI, HTML report and MCP response get it for free.

**Sitemap preview.** `POST /api/sitemap-preview` with `{url, include, exclude,
allow_subdomains}` runs only the discovery step and returns `{found, total, sources[]}`. The New
crawl form calls it (debounced) when the URL or scope fields change. It respects robots.txt and
uses the same User-Agent as a crawl would.

**Tape stream.** The job keeps two strings that grow by one character per recorded page:

- `tape`: status class, one of `2`, `3`, `4`, `5`, or `x` for a fetch error.
- `tape_off`: `1` when that page is not in the sitemap, otherwise `0`.

`GET /api/crawls/{id}?tape_from=N` returns only the characters from index `N`, so polling stays
cheap (one byte per page per field). The runner also saves the tape in the report
(`Coverage.tape`, `Coverage.tape_off`, same encoding), so saved crawls show the same tape as
the live view. Crawl list items carry a downsampled `tape_preview` (at most 160 ticks, worst
status wins) for the mini tapes. Reports from before this change fall back to a tape grouped
by status from `summary.pages_by_status`.

**API.** `JobView` gains `progress` (the latest snapshot plus `phase`). The frontend keeps
polling `GET /api/crawls/{id}` every 1.5s; no SSE.

### 2. Pause and resume

**Engine.** `(*Engine).Pause()` and `(*Engine).Resume(rate float64)` (rate 0 keeps the current
one). While paused, `frontier.next` blocks; requests already in flight finish and are recorded.
Cancelling a paused crawl works (the existing cancellation broadcast wakes the waiters).
`MaxDuration` counts active time only: the `context.WithTimeout` in `Engine.Crawl` is replaced
by a timer that stops while paused.

**Auto-pause on sustained rate limiting.** New `crawler.Options.PauseOnThrottle bool`. When set,
and adaptive backoff has already lowered the rate at least twice, the engine pauses itself once
it sees 10 or more 429/503 responses within 60 seconds. It records the latest `Retry-After` in
the progress snapshot. The web API sets this option on; CLI and MCP leave it off because nobody
is there to resume.

**Runner.** `runner.Run` hides the engine, so a `WithControl(*runner.Control)` option hands the
caller `Pause()` and `Resume(rate)` that forward to the engine once it exists (calls before the
crawl phase starts are queued; calls after it ends are no-ops).

**API.** `POST /api/crawls/{id}/pause` and `POST /api/crawls/{id}/resume` (JSON body, optional
`rate`). Job status gains `paused`. Pausing a finished job is `409`.

### 3. Compare

`GET /api/compare?base=<ref>&current=<ref>` resolves both refs through `store.Resolve` (so IDs,
`latest` and bare hosts work, as on the CLI) and returns `diff.Compare`'s `Diff` as JSON. `400`
when a ref is missing, `404` when one doesn't resolve.

### 4. Report import (the `render` equivalent)

`POST /api/reports/import`, multipart with one JSON file (limit 64 MB). It decodes into
`report.Report`, rejects files that don't parse or have no `seed`, and saves through
`store.Save`. The imported crawl then appears in History and works with export and compare, so
no separate render view is needed.

### 5. Redirect checks as jobs

- `jobManager` becomes generic over its result type so crawl and redirect-check jobs share one
  implementation; each kind keeps its own concurrency cap (4 crawls, 2 redirect checks).
- `redirectcheck.RunOptions` gains `OnRow func(done, total int)`. `RowResult` gains JSON tags.
- `POST /api/redirect-checks`: multipart with the CSV file (limit 10 MB) plus `domain`,
  optional `sitemap_url`, `concurrency`, `rate`, `adaptive_delay`. Parses with
  `redirectcheck.ParseCSV` up front, so a bad header is a `400`, not a failed job. Returns `202`.
- `GET /api/redirect-checks/{id}`: status, `done/total`, and the rows once finished.
- `POST /api/redirect-checks/{id}/cancel`.
- `GET /api/redirect-checks/{id}/export`: CSV through `redirectcheck.WriteCSV`.
- Results are kept in memory only, like unsaved crawls.

### 6. Settings, profiles and crawl parameters

**Crawl parameters.** `crawlrequest.Params` gains `strip_query`, `adaptive_delay`,
`follow_nofollow`, `timeout` and `bearer_token`. MCP shares `Params`, so it gets them too.

**Bearer token.** Set as an `Authorization: Bearer …` header through the existing custom-headers
path, which `HTTPFetcher.authHostAllowed` already confines to the seed host. Combining it with
Basic Auth or a custom `Authorization` header is rejected (`400` on the API, an error on the
CLI). New CLI flag `--bearer-token` and config key `crawl.bearer_token`.

**Header profiles.** New `config.ListHeaderProfiles`, `SaveHeaderProfile` and
`DeleteHeaderProfile` on `HeaderProfileDir()`. Names use the existing name regex; bodies are
validated with `crawler.ParseHeaderLines`; files are written with `0600` through
`atomicfile`. Routes: `GET /api/header-profiles`, and `GET`, `PUT`, `DELETE`
`/api/header-profiles/{name}`.

**GUI settings.** `GET` and `PUT /api/settings`, stored in `~/.gocrawl/gui.json` (atomic write):

- `theme`: `dark` (default), `light` or `system`.
- `crawl_defaults`: a `Params` value that pre-fills the New crawl form.

`gui.json` never holds secrets: `bearer_token`, `basic_auth` and `cookie` are stripped on save.
The store directory is shown read-only.

### 7. Interactive browser login

For sites behind a login (staging, member areas, a Shopify password page), the user logs in
once in a real browser and the crawl reuses that session.

1. `POST /api/login-sessions` with `{url}` opens a visible Chromium window (chromedp,
   `headless=false`, throwaway profile directory) on that URL. Returns `202` with a session id.
2. The user logs in normally, so SSO, 2FA and CAPTCHAs work.
3. `POST /api/login-sessions/{id}/capture` reads the cookie jar over CDP (HttpOnly included),
   keeps the cookies whose domain matches the seed host, closes the browser and deletes the
   profile directory.
4. Cookies stay in server memory. The API only returns cookie names, the count, and the
   earliest expiry. `GET /api/login-sessions/{id}` reports state (`waiting`, `captured`,
   `closed`, `expired`).
5. A crawl request references `login_session: <id>`; the server turns it into the existing
   `Cookie` option, so the existing host restriction applies. Like `--cookie`, it works in raw
   mode only: headless render refuses cookies, Basic Auth and custom headers because Chromium
   can't scope them to the seed host (`internal/render/render.go`). The API returns `400` for
   a login session or bearer token combined with headless render, and the form disables the
   combination with that explanation.
6. Sessions expire after 1 hour or when the process exits. Closing the browser window before
   capture moves the session to `closed`, and the UI says so.
7. Logout guard: a crawl that uses a login session adds the exclude pattern
   `(?i)(log|sign)-?out` unless the user removes it in the form, where it is shown.
8. Web and desktop only, not MCP. Without a Chromium-class browser the button is disabled with
   a hint, the same detection headless render uses.

Chromium launching sits behind a `loginBrowser` interface so handler tests use a fake.

## Frontend

### Information architecture

Left navigation rail: **New crawl**, **Crawls** (today's History), **Compare**, **Redirects**,
**Settings**. Hash routing as today; no router dependency.

### Views

- **New crawl.** Every crawl option, grouped as today, plus `strip_query`, `adaptive_delay`,
  `follow_nofollow`, `timeout`, bearer token, a header-profile dropdown filled from the API,
  and **Log in with browser…**. Pre-filled from `crawl_defaults`; **Save as defaults** writes
  the current form back. Under the URL: the sitemap-preview line ("Found sitemap.xml via
  robots.txt: 1,240 pages in scope"), and, when the budget is smaller, a notice with a
  **Raise budget to 1,240** button. Primary action **Start crawl**.
- **Crawl (running).** Header with host, phase pill, **Pause**, **Stop crawl**. Budget notice
  when it applies. The tape, then counts: sitemap pages fetched of total, fetched pages not in
  the sitemap, queue, elapsed active time, request rate. Status-class totals. A "Fetching now"
  table of the last few URLs with status and timing.
- **Crawl (paused).** Phase pill **Paused**. When the pause was automatic, a notice explains
  the 429s, shows the server's `Retry-After`, and offers a **Resume at** rate field pre-filled
  at half the current rate. Actions **Resume crawl**, **Stop crawl**.
- **Crawl (finished).** Today's report view restyled: tape at the top (outlined ticks now read
  "Not reached by links"), coverage banner, summary, issue table with filters and
  explanations, site map, notes, export links.
- **Crawls.** Saved and in-memory crawls, each with a small tape and sitemap coverage.
  Checkboxes to pick two and open Compare. **Import report…** file picker.
- **Compare.** Two pickers defaulting to the newest two crawls of the same host. Both tapes,
  summary deltas by severity and analyzer, tabs for new, resolved and persisting issues (the
  report's issue table and filters), added and removed pages.
- **Redirects.** CSV picker, domain, optional sitemap URL, rate and concurrency, **Check
  redirects**. Then a `done/total` progress bar, then a results table filterable by verdict and
  scope, and **Export CSV**.
- **Settings.** Theme, crawl defaults, header profiles (list plus a textarea editor with save
  and delete), store path (read-only).

### Visual system

**Principle.** Color means HTTP status, and violet means "this is interface". The one
distinctive element is the crawl tape; everything around it stays quiet.

**Type.** Atkinson Hyperlegible Next for text and Atkinson Hyperlegible Mono for URLs and status
codes, bundled as WOFF2 under `web/src/fonts/` (SIL OFL; the license file ships alongside).
Chosen because it keeps 0/O and 1/l/I distinct, which matters when reading URLs. Sentence case
throughout; no all-caps labels.

**Color tokens** (contrast against the panel surface; all pass WCAG 2.2 AA):

| Token | Dark (default) | Ratio | Light | Ratio |
| --- | --- | --- | --- | --- |
| ground | `#12161d` | | `#f2f3f7` | |
| surface | `#1a2029` | | `#ffffff` | |
| ink (text) | `#e6eaf0` | 13.56 | `#1b2330` | 15.79 |
| ink-2 (secondary text) | `#9aa5b4` | 6.56 | `#4f5a6b` | 6.98 |
| field (input borders) | `#64728a` | 3.37 | `#7b8696` | 3.69 |
| 2xx green | `#4fae6e` | 5.94 | `#1f7a3f` | 5.37 |
| 3xx yellow | `#e3c13b` | 9.32 | `#8a6a00` | 5.07 |
| 4xx dark orange | `#e07b24` | 5.48 | `#b4500c` | 5.13 |
| 5xx red | `#f45d5a` | 5.11 | `#c8221f` | 5.67 |
| violet (UI) | `#a78bfa` | 6.02 | `#6d3fd6` | 6.30 |
| not fetched outline | `#6b7a8f` | 3.75 | `#7b8696` | 3.69 |
| past budget outline | `#a27a1f` | 4.16 | `#8a6a00` | 5.07 |

Text on filled buttons: dark theme uses ground `#12161d` (6.57 on green, 5.66 on red, 6.66 on
violet); light theme uses white (5.37, 5.67, 6.30).

**Controls.** Start crawl and Resume crawl: green fill. Pause: yellow outline. Stop crawl: red
fill. Phase pill: green pulsing dot while crawling, yellow static dot while paused, violet
while analyzing. Violet also marks the active navigation item, links and the other primary
actions (Save, Check redirects). Focus ring: 2px ink outline.

**The tape.** One tick per page, 3px wide, in fetch order:

| Tick | Meaning |
| --- | --- |
| short green | 2xx |
| mid yellow | 3xx |
| tall orange | 4xx |
| tall red with a light cap | 5xx |
| notched (a small gap near the base), any color | fetched page not in the sitemap |
| short outline, slate | sitemap page not fetched yet (after the crawl: not reached by links) |
| short outline, amber | sitemap page past the page budget |

Height carries status as well as color, so the tape still reads for color-blind users (orange
and red are 1.17:1 apart). Above 20,000 pages, each tick aggregates consecutive pages and takes
the worst status among them. The tape is `role="img"` with a text summary, and every count it
shows also appears as text.

**Motion.** Two moments only: new ticks drop in while crawling, and the crawling dot pulses.
Both are off under `prefers-reduced-motion`.

**Copy.** Actions keep one name through the flow (**Stop crawl** produces "Crawl stopped").
Errors say what happened and what to do. Empty states invite the next action: "No saved crawls
yet. Turn on 'Save to Crawls' when you start a crawl."

### Accessibility guard

`web/scripts/contrast.mjs` reads the token file (`web/src/styles/tokens.css`) for both themes and
fails when any pair from the table above drops below its threshold (4.5:1 text, 3:1 non-text).
`npm run lint` runs it.

## Error handling

- Every new endpoint returns JSON errors (`{"error": "..."}`) with the existing status
  conventions: `400` invalid input, `404` unknown id, `409` wrong state (pause a finished job,
  capture a closed login session), `413` oversized upload, `415` wrong content type, `429` job
  cap reached.
- No sitemap found is not an error: progress falls back to the page budget, or a plain count
  when there is no budget, and the form says "No sitemap found".
- The sitemap pre-pass failing (timeout, robots-disallowed) never fails the crawl.
- The browser login failing to launch returns `503` with "No Chromium-class browser found".

## Testing

- Engine: `OnProgress` counts match `len(result.Pages)`; pause blocks new fetches and resume
  continues; cancel while paused returns a partial result; `MaxDuration` excludes paused time;
  auto-pause triggers on a fake server that keeps returning 429.
- Runner: the sitemap is fetched once per crawl (counting fetcher); the budget note appears
  only when the budget is below the sitemap total; `Coverage` sitemap fields.
- `sitemap`: `Discover` matches the analyzer's previous behavior (existing tests keep passing).
- `redirectcheck`: `OnRow` reaches `total`.
- `config`: header profile list, save, delete, name and body validation, file mode.
- Webserver (`httptest`): every new route, including the error codes above, the guard on new
  `POST`/`PUT`/`DELETE` and multipart routes, `tape_from` slicing, secrets stripped from
  `gui.json`, and login sessions through the fake `loginBrowser`. One real-chromedp login test
  is skipped when Chromium is missing, like the render tests.
- Frontend: `tsc -b`, oxlint, and the contrast check.
- `make check` passes after every PR.

## Documentation

`docs/web.md` (views and API), `docs/mcp.md` (new `crawl` parameters), `docs/configuration.md`
(`--bearer-token`, `crawl.bearer_token`), `docs/output.md` (`Coverage.sitemap_total`,
`sitemap_fetched`), `docs/architecture.md` (progress hook, sitemap pre-pass), and a
`CHANGELOG.md` line per PR.

## Delivery

One plan, delivered as separate PRs in this order:

1. Visual system: tokens, bundled fonts, left rail, dark default, restyle of the existing
   three views, contrast check.
2. Live progress: engine hook, sitemap pre-pass shared with the analyzer, `Coverage` sitemap
   fields, budget note, sitemap preview, the tape.
3. Pause, resume and auto-pause on sustained rate limiting.
4. Crawl parameters, bearer token, settings (theme, defaults), header profiles.
5. Compare and report import.
6. Redirect-check jobs.
7. Interactive browser login.

The desktop shell follows as sub-project 2 with its own spec.

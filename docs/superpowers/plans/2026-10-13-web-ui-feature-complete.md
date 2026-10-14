---
id: "2026-10-13-web-ui-feature-complete"
title: "Feature-Complete Web UI Implementation Plan"
scope: "Task-by-task plan for the feature-complete web UI spec, in four waves"
topic: "plan"
status: "archived"
superseded_by: []
updated: 2026-10-14
depends_on: ["../specs/2026-10-13-web-ui-feature-complete-design.md"]
sources: []
---
# Feature-Complete Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `gocrawl serve` a complete, self-contained GUI for everything the CLI does, plus
live sitemap-based progress with the crawl tape, pause and resume (with auto-pause on sustained
rate limiting), compare, report import, redirect-check jobs, settings and header profiles, a
bearer token and interactive browser login, all in a new dark-by-default WCAG 2.2 AA visual
system.

**Architecture:** Wave 0 reshapes the seams so tracks never edit each other's code: a generic
`jobManager[T]` shared by crawl and redirect-check jobs, one route file per feature in
`internal/webserver`, `runner.Run(ctx, cfg, seed, runOpts ...RunOption)`, and a split frontend
(`api/<feature>.ts`, `types/<feature>.ts`, `routes.ts`, stub views). The crawl engine reports
per-page progress through `Options.OnProgress`; `runner.Run` adds a sitemap pre-pass, run
phases and sitemap counts, records the per-page tape (saved in `coverage.tape`) and exposes
`WithProgress`, `WithControl` and `WithPauseOnThrottle`; the web server keeps progress and the
tape per job and serves them through polling with `tape_from`. The frontend gets tokens,
bundled fonts, a left rail and shared `ui/` and `tape/` components first, then each feature's
views on top.

**Tech Stack:** Go 1.27 (stdlib `net/http` ServeMux, chromedp v0.19.1 / cdproto v0.157.6,
`golang.org/x/time/rate`, `golang.org/x/net/idna`, all already in `go.mod`), React 19 +
TypeScript 7 + Vite 8, oxlint, a Node-builtin contrast script; Atkinson Hyperlegible Next and
Mono (SIL OFL) bundled as WOFF2.

**Spec:** [Feature-complete web UI](../specs/2026-10-13-web-ui-feature-complete-design.md)

## Execution changes (2026-10-14)

Decided after wave 1 merged, to cut token use. The tasks below are unchanged; how they run changed.

- Each remaining track runs as one implementer and one review at merge time, one track at a time. Sonnet by default; Opus only for the pause-backend review and the final branch review; Haiku for the API-module tasks (W2-SET-1, W2-LOGIN-1, W2-CMP-1) and all docs-and-changelog tasks, each batched into one run.
- Accessibility work beyond the contrast guard, existing accessibility code and native elements is deferred to one dedicated pass after wave 3. Task steps that ask for ARIA, live regions, focus management, keyboard flows or screen-reader text are skipped until that pass.

## Global Constraints

- Go 1.27 (`go 1.27`, toolchain go1.27.2 in `go.mod`); no new Go modules.
- The CLI keeps building with `CGO_ENABLED=0`; nothing here adds cgo (the `webview_go` desktop shell is sub-project 2).
- No new frontend runtime dependencies; `web/scripts/contrast.mjs` uses Node built-ins only.
- Fonts are bundled under `web/src/fonts/` (WOFF2 + `OFL.txt`), never loaded from a CDN.
- No `localStorage`/`sessionStorage`: UI state that must survive lives in `~/.gocrawl/gui.json` through `GET`/`PUT /api/settings` (the desktop shell changes port every launch).
- No feature depends on a browser download completing; exports stay plain links.
- WCAG 2.2 AA: `npm run lint` runs `contrast.mjs` on `web/src/styles/tokens.css` for both themes, 4.5:1 for text, 3:1 for non-text, with the spec's values below.
- Colors mean HTTP status (2xx green, 3xx yellow, 4xx dark orange, 5xx red, also for `StatusCode` text); violet means interface; filled-button text is ground `#12161d` (dark) or white (light).
- Controls: Start crawl / Resume crawl green fill, Pause yellow outline, Stop crawl red fill, other primary actions violet; focus ring 2px ink; motion (tick drop-in, crawling pulse) off under `prefers-reduced-motion`.
- Copy: sentence case, no all-caps; an action keeps one name through the flow (**Stop crawl** → "Crawl stopped"); errors say what happened and what to do; empty states invite the next action.
- Guard model: every new `POST`/`PUT`/`DELETE` and multipart route goes through `guard.go` (loopback `Host`, same-origin on non-`GET`), JSON bodies through `requireJSON` (415), uploads through `parseMultipart` (415/413); errors are `{"error": "..."}` with 400/404/409/413/415/429/503.
- Secrets (bearer token, Basic Auth, cookies, header values) never appear in an error, a log line, `gui.json`, or a login-session or job response.
- `make check` passes at the end of every task; frontend tasks also run `cd web && npm run build && npm run lint`.
- Each track's docs task adds exactly one line under `## [Unreleased]` in `CHANGELOG.md`, updates its docs, bumps `updated:` and runs `make docs`.
- Work on track branches merged into the feature branch `Patience-dot-devl/gui-self-contained-build`; never commit to or force-push `main`; never commit a built `internal/webserver/webui/dist/index.html`.
- Every commit ends with `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

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

Ratios are against the panel surface. Text on filled buttons: dark 6.57 on green, 5.66 on
red, 6.66 on violet; light 5.37, 5.67, 6.30.

## Review Focus

Five input classes and failure modes most likely to slip through, each pinned by a test in
the task that owns it:

- **Very large crawls (20,000 to 100,000+ pages).** The tape aggregates (worst status wins),
  polls only the new slice, and lists carry a 160-character preview instead of the full tape:
  `TestTapePreview`, `TestTapePreviewLargeCrawl` (W1-PROG-6b),
  `TestRecordProgressGrowsTapeAndSlicesByTapeFrom`,
  `TestListCarriesStatusCountsAndSitemapCoverage` (W1-PROG-8); the 50,000-page aggregation is
  checked by hand in W1-VIS-5.
- **A sitemap on another host or CDN, or one that lists only out-of-scope URLs** (www vs apex):
  `TestDiscoverUsesRobotsDeclaredSitemap` (W1-PROG-4), `TestRunDoesNotLeakBasicAuthToSitemapHost`
  (existing, re-run in W1-PROG-5), `TestRunCountsOnlyInScopeSitemapURLs` (W1-PROG-6b).
- **The server restarted while a crawl ran.** The poll gets a JSON 404 and the UI says the
  unsaved crawl is gone; saved crawls reopen with the same tape: `TestGetCrawlForgottenJobIs404`
  (W1-PROG-8), `TestRunSavesTheLiveTapeInCoverage` (W1-PROG-6b).
- **Unicode (IDN) hosts in the login flow.** The user types `bücher.example`, Chromium stores
  cookies for `xn--bcher-kva.example`: `TestLoginSessionMatchesIDNHosts` (W1-LOGIN-4), with
  the domain rules in `TestCookiesForHostFollowsBrowserDomainRules` (W1-LOGIN-2). Sitemap URLs
  in the other IDN form are not matched (see Open decisions).
- **Concurrent pause, resume, cancel and polling.** `TestPauseResumeWhilePollingDoesNotDeadlock`,
  `TestCancelPausedCrawl` (W2-PAUSE-6), `TestEngineCancelWhilePausedReturnsPartialResult`
  (W2-PAUSE-3), `TestControlQueuesPauseBeforeTheCrawlPhase`,
  `TestRunCanceledWhileQueuedPauseHolds` (W2-PAUSE-5), all under `go test -race`.

## Decisions taken during reconciliation

- **Login session and bearer token are raw-mode only.** `NewHeadlessFetcher` rejects cookies,
  Basic Auth and custom headers (`internal/render/render.go`) because Chromium can't scope them
  to the seed host. The plan keeps the 400 (W1-LOGIN-4) and the disabled button with its hint
  (W2-LOGIN-2); the spec says so. Setting captured cookies in Chromium's jar with
  `network.SetCookies` scoped to the seed host is a possible follow-up, not part of this plan.
- **IDN in sitemap matching (known limitation).** Crawl URL normalization keeps the host as typed, so a sitemap
  listing `xn--…` URLs for a Unicode seed (or the reverse) counts them out of scope. Fixing it
  touches `crawler.normalizeURL` for every crawl; left out of this plan.
- **Spec wording.** The spec was updated to match this plan: saved crawls keep their tape in
  `coverage.tape`, tokens live in `web/src/styles/tokens.css`, and crawl auth is raw-mode only.

## Reconciliation log

How the four plan sections were made consistent with each other, with wave 0, and with the
visual system's actual code. The sections below already include every change.

1. Coordinator decision 1, tape persisted: new task W1-PROG-6b (`coverage.tape` / `tape_off`,
   `crawler.TapeChar`, `crawler.TapePreview`, `Progress.TapeOffChar`, TypeScript `Coverage`
   fields); W1-PROG-8 adds `tape_preview` to list items and store entries (never the full
   tape); W2-PROG-UI-3 (finished view), W2-PROG-UI-5 (mini tapes) and W2-CMP-3 (Compare) read
   it, with the `pages_by_status` grouping only as a fallback for older reports.
2. Coordinator decision 2: `StatusCode` 2xx is `--s2` green (W1-VIS-4); green on surface is an
   already-checked pair (5.94 dark, 5.37 light).
3. Coordinator decision 3 (allow login + headless) not applied: its premise is false
   (`render.go:58` rejects `opts.Cookie`). Kept the 400; see Decisions taken.
4. Coordinator decision 4: W1-SET-6 extends wave 0's `settings.go` and keeps its
   `WithGUISettingsPath`; only `DefaultGUISettingsPath`, the types and the routes are new.
5. Coordinator decision 5: pause/resume answer 204; `Control.Pause` queues during the sitemap
   phase. Contract updated.
6. Coordinator decision 6: `store.ResolveStored` adopted; no section calls `store.Resolve` from
   a web handler.
7. Coordinator decision 7: every `Notice` call site passes `{tone, children, action?}` and every
   `Button` call site `variant` + native props, matching W1-VIS-4; no call site needed changes.
8. W1-PROG-8 edited `JobView`/`crawlListItem`/`jobListItem` in `job.go`; wave 0 put them in
   `crawl_job.go`. Retargeted.
9. W1-PROG-5 re-applied run options by hand and told the implementer to delete wave 0's
   helper; it now calls wave 0's `newRunSettings` (pinned by a wave-0 test, and nil-safe).
10. W2-PROG-UI-2 created a second phase pill (`progress/PhasePill.tsx` + `.phase-pill` CSS);
    replaced by `RunPhasePill` on top of the visual system's `ui/PhasePill` (also in W3).
11. `tapeText.ts` duplicated `status.ts` and labeled `x` "Fetch failed"; it now reuses
    `status.ts` (`No response`, "without a response").
12. W2-PROG-UI-1 added `'paused'` to `CrawlStatus` without the `STATUS_PILL` entry the visual
    system's `Record` requires (tsc would fail); the entry is added in the same task.
13. W2-SET-3/4/5 and W2-LOGIN-2 used pre-visual classes (`card`, `form-section`, `checkbox`,
    `form-hint`, `form-actions`) that W1-VIS-8 deletes with `index.css`, and only described
    how to keep the progress track's sitemap line. Rewritten in the `cf-` markup with the
    preview line in code, plus `SettingsView.css` and a `.cf-login` panel.
14. W2-CMP-2 extracted `IssueTable` from `main`'s pre-visual markup; rebuilt from W1-VIS-7's
    `cr-` markup (and the import cleanup now drops `useRef` too).
15. Compare and Redirects views used unstyled pre-visual classes (`card`, `empty`, `tabs`,
    `table-scroll`, `url-cell`, `filter-count`); switched to `muted`, `cr-tabs`, `redirect-*`
    and `btn btn-secondary` for the export link.
16. W2-RED-3 edited `App.tsx` to pass `id` to `RedirectView`, which wave 0 already does: no-op.
17. W2-CMP-4 assumed an early-return empty state in `History.tsx`; adapted to W1-VIS-8's
    ternary, and its type predicate no longer depends on `CrawlListItem` vs the progress
    track's `CrawlListItemWithProgress`.
18. Progress UI rendered errors as `<p className="error">` and used a second stop-confirm
    wording; now `Notice` (with restart copy) and the visual system's "The report keeps the
    pages fetched so far."
19. The finished and running tapes had no visible key for notches and outlines; both now
    render the visual system's `TapeLegend` (`finished` reads "Not reached by links").
20. Review-focus gaps closed with tests: forgotten job 404 (W1-PROG-8), out-of-scope sitemap
    (W1-PROG-6b), IDN login hosts with `asciiHost` (W1-LOGIN-4), queued pause then cancel
    (W2-PAUSE-5).

## Execution order: waves and tracks

Tracks in one wave run in parallel, each on its own branch (a git worktree per track works
well), cut from the feature branch `Patience-dot-devl/gui-self-contained-build` once every wave
it depends on is merged there. A track merges back through a pull request into the feature
branch; the feature branch goes to `main` at the end, as one PR per spec delivery item or in
one piece. Branch names below replace the examples inside the sections.

| Wave | Track | Tasks | Depends on | Branch / worktree |
| --- | --- | --- | --- | --- |
| 0 | Prep | W0-1 … W0-5 | — | feature branch directly |
| 1 | Visual system | W1-VIS-1 … W1-VIS-8 | wave 0 | `Patience-dot-devl/web-ui-visual` |
| 1 | Progress backend | W1-PROG-1 … W1-PROG-6, W1-PROG-6b, W1-PROG-7 … W1-PROG-10 | wave 0 | `Patience-dot-devl/web-ui-progress` |
| 1 | Settings backend | W1-SET-1 … W1-SET-8 | wave 0 | `Patience-dot-devl/web-ui-settings` |
| 1 | Login backend | W1-LOGIN-1 … W1-LOGIN-5 | wave 0 | `Patience-dot-devl/web-ui-login` |
| 1 | Compare/import backend | W1-CMP-1 … W1-CMP-4 | wave 0 | `Patience-dot-devl/web-ui-compare` |
| 1 | Redirects backend | W1-RED-1 … W1-RED-4 | wave 0 | `Patience-dot-devl/web-ui-redirects` |
| 2 | Progress UI | W2-PROG-UI-1 … W2-PROG-UI-6 | wave 1 (VIS, PROG) | `Patience-dot-devl/web-ui-progress-ui` |
| 2 | Pause backend | W2-PAUSE-1 … W2-PAUSE-7 | wave 1 (PROG) | `Patience-dot-devl/web-ui-pause` |
| 2 | Settings + login UI | W2-SET-1 … W2-SET-6, then W2-LOGIN-1 … W2-LOGIN-3 | wave 1 (VIS, SET, LOGIN); W2-SET-3 needs W2-PROG-UI-4 merged | `Patience-dot-devl/web-ui-settings-ui` |
| 2 | Compare UI | W2-CMP-1 … W2-CMP-5 | wave 1 (VIS, PROG, CMP); W2-CMP-2 needs W2-PROG-UI-2/3, W2-CMP-4 needs W2-PROG-UI-5 merged | `Patience-dot-devl/web-ui-compare-ui` |
| 2 | Redirects UI | W2-RED-1 … W2-RED-4 | wave 1 (VIS, RED) | `Patience-dot-devl/web-ui-redirects-ui` |
| 3 | Pause UI | W3-PAUSE-UI-1, W3-PAUSE-UI-2 | wave 2 (Progress UI, Pause backend, Compare UI) | `Patience-dot-devl/web-ui-pause-ui` |

Task counts: wave 0: 5, wave 1: 40, wave 2: 31, wave 3: 2 (78 tasks).

### Merge order inside each wave

- **Wave 1:** Visual system → Progress backend → Settings backend → Login backend →
  Compare/import backend → Redirects backend. Progress goes first among the backends because
  it changes `handleStartCrawl` the most (`MaxPages`, run options, `handleGetCrawl`, list items)
  and `internal/store/store.go`; Settings then adds its `cfg.ToOptions()` block after
  `req.ToConfig()`, and Login inserts its `login_session` block between the two (W1-LOGIN-4
  says so) and adds `startCrawlRequest.LoginSession`. Compare adds `ResolveStored` to the
  `store.go` that Progress already changed (`Entry`, `entryFor`).
- **Wave 2:** Progress UI → Pause backend → Settings + login UI → Compare UI → Redirects UI.
  Progress UI first because W2-SET-3 rewrites `CrawlForm.tsx` around the sitemap preview line
  that W2-PROG-UI-4 adds, and W2-CMP-2 extracts `IssueTable` from the `CrawlReport.tsx` that
  W2-PROG-UI-2/3 changed. The settings-UI branch may run W2-SET-1/2 early but rebases onto the
  merged progress UI before W2-SET-3; the compare-UI branch rebases before W2-CMP-2.
- **Wave 3:** Pause UI alone, after every wave-2 track merged.

### Shared files to merge carefully

| File | Edited by | Note |
| --- | --- | --- |
| `internal/webserver/crawls.go` | W0-2, W1-PROG-8, W1-SET-5, W1-LOGIN-4, W2-PAUSE-6 | Final `handleStartCrawl` order: decode → `ToConfig` → `login_session` → `ToOptions` → `NewControl` → `start(crawlJob{Seed, MaxPages, control})` → `Run(…, s.crawlRunOptions(job.ID, ctl)...)`. |
| `internal/webserver/crawl_job.go` | W0-1, W1-PROG-8, W2-PAUSE-6 | `crawlJob`, `JobView`, `crawlListItem`, `jobView`, `jobListItem` all live here. |
| `internal/store/store.go`, `store_test.go` | W1-PROG-8, W1-CMP-1 | Different functions; keep both. |
| `internal/runner/runner.go`, `options.go` | W0-3, W1-PROG-5/6/6b, W2-PAUSE-5 | Same lineage (progress then pause branch). |
| `internal/crawler/*` | W1-PROG-1…6b, W2-PAUSE-1…4 | Same lineage. |
| `cmd/gocrawl/serve.go` | W1-SET-6, W1-LOGIN-3 | Different lines (`opts` slice; `web := …; defer web.Close()`). |
| `web/src/types/crawls.ts` | W0-4, W1-PROG-6b, W2-PROG-UI-1, W2-SET-1, W2-LOGIN-1 | Additive fields only. |
| `web/src/components/CrawlForm.tsx` (+ `CrawlForm.css`) | W1-VIS-6, W2-PROG-UI-4, W2-SET-3/4, W2-LOGIN-2 | W2-SET-3 rewrites it and carries the preview line over. |
| `web/src/components/CrawlReport.tsx` | W1-VIS-7, W2-PROG-UI-2/3, W2-CMP-2, W3-PAUSE-UI-1 | In that order. |
| `web/src/components/History.tsx` | W1-VIS-8, W2-PROG-UI-5, W2-CMP-4 | In that order. |
| `web/src/App.tsx`, `web/src/main.tsx` | W0-5, W1-VIS-1/2/3/8, W2-SET-2, W2-CMP-3 | Small, line-level edits. |
| `docs/web.md`, `docs/architecture.md`, `docs/output.md`, `CHANGELOG.md`, `docs/README.md` | every track's docs task | Keep both sides of a conflict, then `make docs` regenerates frontmatter links and the index. |

### Cross-track contract (final)

Spec: docs/superpowers/specs/2026-10-13-web-ui-feature-complete-design.md
Every track uses exactly these names. This version folds in every gap the four plan sections
filled and every coordinator decision; where it differs from the first draft, the change is
marked *(adopted)*.

#### Execution model

- Wave 0 (prep) lands first on the feature branch. Wave 1 runs the visual system next to the
  backend tracks; wave 2 the UI tracks and the pause backend; wave 3 the pause UI.
- Each track runs on its own branch/worktree cut from the feature branch after the waves it
  depends on, then merges back. Tracks keep edits to shared files minimal: register routes in
  their own file, add their own Server fields only where wave 0 left a placeholder, add their
  own frontend modules.
- Each track's last task updates its docs + one CHANGELOG line under `## [Unreleased]`.
- `make check` must pass at the end of every task. Frontend: `cd web && npm run build && npm run lint`.
- Frontend commits start with `git restore internal/webserver/webui/dist/index.html` (never
  commit a built `index.html`).

#### Go

##### internal/webserver (wave 0 creates)

```go
// job.go — generic over kind-specific data
type Status string
const (StatusRunning="running"; StatusPaused="paused"; StatusDone="done"; StatusError="error"; StatusCanceled="canceled")

type Job[T any] struct {
    ID         string
    Status     Status
    Err        string
    StartedAt  time.Time
    FinishedAt time.Time
    Data       T
    cancel          context.CancelFunc
    cancelRequested bool
}
func (j *Job[T]) active() bool                           // running or paused
type jobManager[T any] struct { mu sync.Mutex; jobs map[string]*Job[T] }
func newJobManager[T any]() *jobManager[T]
// start: running AND paused jobs count toward maxRunning.
func (m *jobManager[T]) start(data T, cancel context.CancelFunc, maxRunning int) (Job[T], bool)
func (m *jobManager[T]) get(id string) (Job[T], bool)      // shallow snapshot
func (m *jobManager[T]) list() []Job[T]
// with runs fn under the lock on the live job; false if unknown. fn must not block or call back into m.
func (m *jobManager[T]) with(id string, fn func(j *Job[T])) bool
// finish sets FinishedAt, Status (error / canceled when cancelRequested / done), Err,
// and calls apply(&j.Data) under the lock when non-nil, for every outcome.
func (m *jobManager[T]) finish(id string, err error, apply func(d *T))
func (m *jobManager[T]) cancel(id string) bool              // running or paused -> cancelRequested
func formatTime(t time.Time) string

// crawl_job.go — crawlJob AND the crawl views live here (adopted: not in job.go)
type crawlJob struct {
    Seed     string
    Report   *report.Report
    StoreID  string
    MaxPages int              // progress track (adopted)
    Progress *runner.Progress // progress track; replaced, never written through
    Tape     []byte           // '2','3','4','5','x' per recorded page (crawler.TapeChar)
    TapeOff  []byte           // runner.Progress.TapeOffChar per page
    control  *runner.Control  // pause track
}
type JobView struct { ID, Seed, Status, Err, StartedAt, FinishedAt; Persisted bool; Report *report.Report; Progress *progressView `json:"progress,omitempty"` }
type crawlListItem struct { …; StatusClasses map[string]int `json:"status_classes,omitempty"`; SitemapTotal, SitemapFetched int (omitempty); TapePreview string `json:"tape_preview,omitempty"` }
func jobView(j Job[crawlJob]) JobView
func jobListItem(j Job[crawlJob]) crawlListItem
```

The crawl goroutine saves the report before `finish` and sets `Report` and `StoreID` in
`finish`'s `apply` (wave 0; tracks keep that order). `startCrawlRequest` (crawls.go) is
`crawlrequest.Params` + `Save bool` + `LoginSession string` (login track).

Server after wave 0 (`New` builds `crawls`, `redirects`, `logins` before applying options):

```go
type Server struct {
    store           *store.Store
    crawls          *jobManager[crawlJob]
    redirects       *jobManager[redirectJob] // redirects.go
    logins          *loginManager            // login.go; newLoginManager()
    guiSettingsPath string                   // settings.go
    mux             *http.ServeMux
    allowAnyHost    bool
    maxRunning      int
}
func WithGUISettingsPath(path string) Option // wave 0, settings.go; the settings track must not redeclare it (adopted)
```

`routes()` calls, in order: `s.crawlRoutes()` (crawls.go), `s.sitemapRoutes()`,
`s.compareRoutes()`, `s.importRoutes()`, `s.redirectRoutes()`, `s.settingsRoutes()`,
`s.profileRoutes()`, `s.loginRoutes()`, then the frontend handler. Wave 0 creates each as an
empty method in its own file: sitemap.go, compare.go, import.go, redirects.go, settings.go,
profiles.go, login.go.

Helpers that exist and must be reused: `writeJSON`, `writeError`, `formatTime`, the guard,
`requireJSON(w, r) bool` (415 unless application/json) and
`parseMultipart(w, r, maxBytes int64) bool` (415 unless multipart/form-data, 413 over the cap,
400 malformed; caller `defer r.MultipartForm.RemoveAll()`). Test helpers in `server_test.go`:
`newTestServer(t)`, `doJSON`, `startCrawlTarget`, `pollUntilFinished` (treats `paused` as live
after W2-PAUSE-6).

##### internal/runner (wave 0 creates the mechanism only)

```go
type RunOption func(*runSettings)
type runSettings struct {
    progress        func(Progress) // progress track
    control         *Control       // pause track
    pauseOnThrottle bool           // pause track
}
func newRunSettings(opts []RunOption) runSettings // wave 0; skips nil options
// The variadic parameter is runOpts (adopted): Run already has a local named opts.
func Run(ctx context.Context, cfg config.Config, seed string, runOpts ...RunOption) (*report.Report, error)
```

##### Progress track produces

```go
// internal/crawler
type Progress struct {
    Fetched           int     `json:"fetched"`
    Queued            int     `json:"queued"`
    CurrentURL        string  `json:"current_url"`
    FinalURL          string  `json:"final_url,omitempty"`   // adopted
    Status            int     `json:"status"`                // first redirect hop's status when redirected (adopted)
    DurationMS        int64   `json:"duration_ms"`           // adopted
    Rate              float64 `json:"rate"`
    Throttled         int     `json:"throttled"`             // 429/503 in last 60s
    Paused            bool    `json:"paused"`
    PauseReason       string  `json:"pause_reason,omitempty"` // PauseReasonUser "user" | PauseReasonRateLimited "rate_limited"
    RetryAfterSeconds int     `json:"retry_after_seconds,omitempty"`
}
// Options.OnProgress func(Progress) — serialized, in recording order, under engine locks
func (e *Engine) InScope(rawURL string) bool
func (e *Engine) SetSeed(seed string) (string, error)          // adopted
func (e *Engine) Normalize(rawURL string) string               // adopted
func (e *Engine) RobotsFor(ctx context.Context, rawURL string) *RobotsData // adopted
// AdaptiveLimiter: RecentThrottled, LastRetryAfter, EffectiveRate (+ SetRate, ResetRecent from the pause track)
// Coverage gains (report.Coverage is *crawler.Coverage):
SitemapTotal   int    `json:"sitemap_total,omitempty"`
SitemapFetched int    `json:"sitemap_fetched,omitempty"`
Tape           string `json:"tape,omitempty"`      // adopted (coordinator decision 1)
TapeOff        string `json:"tape_off,omitempty"`  // adopted
func TapeChar(status int) byte                     // adopted: '2'..'5', 'x'
func TapePreview(tape string, limit int) string    // adopted: worst status wins per run
const TapePreviewLen = 160                         // adopted

// internal/analyze/sitemap
type Option func(*Analyzer)
func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer
func Discover(ctx context.Context, fetcher crawler.Fetcher, seed string, robots map[string]*crawler.RobotsData) (urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string)
func DiscoverAll(ctx context.Context, fetcher crawler.Fetcher, seed string, robots map[string]*crawler.RobotsData) Discovery // adopted
type Discovery struct { URLs map[string]bool; Parsed int; InvalidDeclared, TruncatedDeclared, Sources []string; ViaRobots bool } // adopted
func WithURLs(urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string) Option
// Fetch keeps its signature (internal/redirectcheck uses it).

// internal/runner
type Phase string // PhaseSitemap "sitemap" | PhaseCrawling "crawling" | PhaseAnalyzing "analyzing"
type Progress struct {
    crawler.Progress
    Phase          Phase `json:"phase"`
    SitemapFound   bool  `json:"sitemap_found"`
    SitemapTotal   int   `json:"sitemap_total"`
    SitemapFetched int   `json:"sitemap_fetched"`
    OffSitemap     int   `json:"off_sitemap"`
    InSitemap      bool  `json:"in_sitemap"` // for CurrentURL
}
func (p Progress) TapeOffChar() byte  // adopted: '1' when SitemapFound && !InSitemap
func WithProgress(fn func(Progress)) RunOption
// Run always installs its progress reporter (it records Coverage.Tape), with or without WithProgress (adopted).
type RegistryOptions struct { …; SitemapOptions []sitemap.Option } // adopted
type SitemapPreview struct { Found bool; Total int; Sources []string; ViaRobots bool } // adopted (json: found,total,sources,via_robots)
func PreviewSitemap(ctx context.Context, cfg config.Config, seed string) (SitemapPreview, error) // adopted
```

`internal/store.Entry` gains `ByStatus` (`pages_by_status`), `SitemapTotal`, `SitemapFetched`,
`TapePreview` (`tape_preview`), all `omitempty` *(adopted)*.

API (progress track): `JobView.Progress *progressView` (`json:"progress,omitempty"`):
runner.Progress fields + `tape string`, `tape_off string`, `tape_from int`, `max_pages int`,
`active_ms int64` *(adopted)*. `GET /api/crawls/{id}?tape_from=N` (400 for negative or
non-numeric; 404 for an id that is neither a job nor a stored crawl). `GET /api/crawls` items
carry `status_classes`, `sitemap_total`, `sitemap_fetched`, `tape_preview` (never the full
tape). `POST /api/sitemap-preview` body `{url, include, exclude, allow_subdomains}` ->
`{found bool, total int, sources []string, via_robots bool}`.

##### Pause track produces

```go
func (e *Engine) Pause()
func (e *Engine) Resume(rate float64) // 0 keeps current rate
func (e *Engine) Paused() bool                 // adopted
func (e *Engine) ActiveTime() time.Duration    // adopted
// crawler.Options.PauseOnThrottle bool
type Control struct { /* unexported */ }
func NewControl() *Control
func (c *Control) Pause() bool            // queued (true) before the crawl phase, false once it ended (adopted: spec wins)
func (c *Control) Resume(rate float64) bool
func (c *Control) Active() time.Duration  // adopted; lock-free, for progress.active_ms
func WithControl(c *Control) RunOption
func WithPauseOnThrottle() RunOption
```

API: `POST /api/crawls/{id}/pause`, `POST /api/crawls/{id}/resume` JSON `{rate?: number}` ->
`204` *(adopted)*; `404` unknown; `409` when not running / not paused respectively (or the crawl
phase is over); resume `400` for `rate <= 0` or bad JSON, `415` non-JSON. Lock order is binding
(see the progress section): never call `Control.Pause/Resume` inside `s.crawls.with`.

##### Settings track produces

```go
// internal/crawlrequest.Params gains
StripQuery     bool   `json:"strip_query,omitempty"`
AdaptiveDelay  *bool  `json:"adaptive_delay,omitempty"` // nil = default (true)
FollowNofollow bool   `json:"follow_nofollow,omitempty"`
Timeout        string `json:"timeout,omitempty"`        // Go duration, same convention as max_duration
BearerToken    string `json:"bearer_token,omitempty"`
// internal/config
func ListHeaderProfiles() ([]string, error)
func ReadHeaderProfile(name string) (string, error)
func SaveHeaderProfile(name, body string) error
func DeleteHeaderProfile(name string) error
var ErrInvalidHeaderProfile error // adopted: 400; missing profiles wrap fs.ErrNotExist (404)
// CrawlConfig.BearerToken string `mapstructure:"bearer_token"`; CLI --bearer-token
// internal/webserver/settings.go (extends the wave-0 file)
type GUISettings struct {
    Theme         string              `json:"theme"` // "dark" (default) | "light" | "system"
    CrawlDefaults crawlrequest.Params `json:"crawl_defaults"`
}
func DefaultGUISettingsPath() string // adopted: ~/.gocrawl/gui.json, used by serve.go
```
API: `GET/PUT /api/settings` (responses add read-only `store_dir` *(adopted)*; PUT without a
settings path is 503); `GET /api/header-profiles` -> `{profiles: string[]}`;
`GET/PUT/DELETE /api/header-profiles/{name}` (PUT JSON `{body}` -> 204 *(adopted)*; GET ->
`{name, body}`). `POST /api/crawls` compiles `cfg.ToOptions()` up front, so every option error
(bearer conflicts included) is a 400 *(adopted)*.

##### Login track produces

`startCrawlRequest` gains `LoginSession string json:"login_session,omitempty"` (web-only, NOT in
crawlrequest.Params). 400 when combined with `cookie`, with `render: headless` (kept pending a
human decision, see the plan), or when the session's host isn't the seed host (hosts compare
in ASCII/punycode, *adopted*); 404 unknown; 409 not captured. Resolved into cfg.Crawl.Cookie.
```go
type loginBrowser interface { Open(ctx context.Context, url string) (loginWindow, error) }
type loginWindow interface { Cookies(ctx context.Context) ([]*http.Cookie, error); Done() <-chan struct{}; Close() error }
type loginProber interface { Available() error } // adopted, optional on a loginBrowser
func WithLoginBrowser(b loginBrowser) Option
func (s *Server) Close()                         // adopted; serve.go defers it
// internal/render: FindBrowser, LoginAvailable, OpenLoginWindow, ErrNoBrowser, ErrNoDisplay, ErrLoginWindowClosed (adopted)
```
API: `POST /api/login-sessions` `{url}` -> 202 `{id, state}` (429 when two windows wait, 503
without a browser); `GET /api/login-sessions/{id}`; `POST /api/login-sessions/{id}/capture`;
`DELETE /api/login-sessions/{id}`; `GET /api/login-sessions/available` -> `{available bool, reason?}`.
States: `waiting`, `captured`, `closed`, `expired`. View: `{id, state, host, cookie_names[], cookie_count, expires_at?}`.

##### Compare / import / redirects tracks produce

`store.ResolveStored(ref)` *(adopted)*: resolves IDs, `latest` and bare hosts inside the store,
never a file path; `store.Resolve` (CLI) delegates to it. No web handler calls `store.Resolve`.
`GET /api/compare?base=&current=` -> diff.Diff JSON; refs may also be crawl-job IDs of this
process (running/paused -> 409, failed -> 404) *(adopted)*. `POST /api/reports/import`
multipart field `file` (64 MB) -> 201 `{id}`. Redirect jobs:
`type redirectJob struct { Domain, FileName string; Total, Done int; Rules []redirectcheck.Rule; Rows []redirectcheck.RowResult }`
(`FileName` adopted), cap 2, routes per spec section 5 plus `GET /api/redirect-checks` ->
`{checks: [...]}` *(adopted)*; multipart fields `file, domain, sitemap_url, concurrency, rate, adaptive_delay`.
`redirectcheck.RunOptions.OnRow func(done, total int)`; `RowResult` and `Rule` JSON tags
snake_case; `redirectcheck.NewFetcher`, `DefaultTimeout`, `DefaultUserAgent` *(adopted)*.

#### Frontend (web/src)

Wave 0 creates:
- `api/client.ts`: `request<T>(path, init?)` (204 -> undefined; throws `Error(body.error)`),
  `postJSON<T>(path, body, method='POST')`, `postForm<T>(path, form: FormData)`.
- `api/crawls.ts` (`listAnalyzers`, `listExplanations`, `startCrawl`, `listCrawls`, `getCrawl`,
  `cancelCrawl`, `exportURL`), `types/crawls.ts` (existing types moved). `api.ts` and `types.ts`
  are removed. Each track adds `api/<track>.ts` + `types/<track>.ts`.
- `routes.ts`: `type Route = {view:'new'} | {view:'crawls'} | {view:'crawl', id} | {view:'compare', base?, current?} | {view:'redirects'} | {view:'redirect', id} | {view:'settings'}`;
  `parseHash(hash): Route`, `href(route): string`, `navigate(route)` *(adopted)*. `#/history` still parses to crawls.
- `views/` stubs `CompareView.tsx`, `RedirectsView.tsx`, `RedirectView.tsx` (`{id}`),
  `SettingsView.tsx`. App.tsx routes all of them (`<RedirectView key={route.id} id={route.id} />`).

Visual system (wave 1) creates and others reuse (all default exports):
- `styles/tokens.css` *(adopted path; the spec's `web/src/tokens.css` is superseded)*: CSS vars
  on `:root, :root[data-theme=dark]` (default), `[data-theme=light]`, and
  `[data-theme=system]` under `prefers-color-scheme: light`:
  `--ground --surface --raised --rule --field --ink --ink-2 --s2 --s3 --s4 --s5 --violet --on-fill --todo --cut --notice-bg`.
  `styles/fonts.css` adds `--font-text`, `--font-mono`; `styles/base.css` has global element
  styles and `.muted`, `.num`, `.mono`, `.visually-hidden`, `.layout`.
- `theme.ts`: `type Theme='dark'|'light'|'system'; applyTheme(t: Theme): void; isTheme(v)`.
- `fonts/` WOFF2 + OFL.txt.
- `status.ts` *(adopted)*: `StatusClass`, `STATUS_CLASSES`, `statusClass(code)`, `STATUS_LABEL`.
- `components/ui/Button.tsx`: `variant: 'go'|'stop'|'pause'|'primary'|'secondary'` (default
  secondary, `type='button'`) + every native `<button>` prop. Links styled as buttons use
  `className="btn btn-secondary"`.
- `components/ui/Notice.tsx`: exactly `{ tone: 'warning'|'error'|'info'; children; action? }`.
- `components/ui/StatusCode.tsx`: `{code:number}`, 2xx `--s2` green *(adopted)*, 3xx/4xx/5xx
  `--s3`/`--s4`/`--s5`, 0 reads "Error".
- `components/ui/PhasePill.tsx` *(adopted)*: `{ phase: 'crawling'|'paused'|'analyzing'|'done'|'stopped'|'failed'; children }`.
- `crawlStatus.ts` *(adopted)*: `STATUS_PILL: Record<CrawlStatus, {phase, label}>` (the
  progress track adds `paused`).
- `components/tape/Tape.tsx`: `{ tape: string; tapeOff?: string; notFetched?: number; pastBudget?: number; summary: string; compact?: boolean; live?: boolean }`
  — `role="img"` `aria-label={summary}`, aggregates above 20000 ticks (above 160 when compact),
  worst status wins (5 > x > 4 > 3 > 2 > not fetched > past budget). `tapeModel.ts`
  (`buildTicks`, `countTape`, `describeTape`, `MAX_TICKS`, `MAX_COMPACT_TICKS`) and
  `TapeLegend.tsx` (`{sitemap?, finished?}`) *(adopted)*.
- `components/Rail.tsx` left navigation.
- Styling: every component imports its own colocated stylesheet with a per-view prefix
  (`cf-` New crawl form, `cr-` crawl report, `hist-` Crawls, `sm-` site map, `rc-`/`ft-`/`sp-`
  progress, `set-` Settings, `compare-`, `redirect-`). `index.css` is deleted in W1-VIS-8;
  pre-visual classes (`card`, `empty`, `tabs`, `form-section`, …) must not be used.
- `scripts/contrast.mjs` + `npm run lint` runs oxlint and it.

Later tracks add (UI waves): `components/progress/*` (`useCrawlPoll`, `RunningCrawl`,
`RunPhasePill` over `ui/PhasePill`, `FinishedTape`, `SitemapPreviewLine`, `MiniTape`,
`CrawlControls`, `AutoPauseNotice`, `tapeText.ts`), `components/IssueTable.tsx`,
`components/crawlform/*` (`values.ts`, `CrawlFields.tsx`, `LoginPanel.tsx`),
`settings/*` (`SettingsProvider`, `useSettings`), `views/compare/*`. TypeScript `Coverage`
gains `sitemap_total?`, `sitemap_fetched?`, `tape?`, `tape_off?` (W1-PROG-6b); `CrawlStatus`
gains `'paused'` (W2-PROG-UI-1); `StartCrawlParams` gains the five settings params and
`login_session?`.

## Track briefs

Each section of the plan opened with context for its tracks: files touched, conventions, the
baseline it builds on and (for progress and pause) the binding lock order. They are kept here
as written, after reconciliation.

### Brief: prep and visual system

Plan section for the feature-complete web UI
(spec: `docs/superpowers/specs/2026-10-13-web-ui-feature-complete-design.md`, binding names:
`.context/plan/00-contract.md`). Wave 0 ("prep") reshapes the web server, runner, and
frontend so the later tracks can work in parallel without touching each other's files.
Wave 1 ("visual system") is spec delivery item 1: tokens, bundled fonts, the left rail, dark
default, restyle of the existing views, and the contrast check.

Every code block in this section was compiled, linted, and tested against a scratch copy of
the repository at `3075b13` (`go test -race`, golangci-lint v2.14.0, `tsc -b`, `vite build`,
oxlint 1.86, `make check`, `make docs`), so it can be pasted as written.

#### Files touched

| Path | W0-1 | W0-2 | W0-3 | W0-4 | W0-5 | VIS-1 | VIS-2 | VIS-3 | VIS-4 | VIS-5 | VIS-6 | VIS-7 | VIS-8 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `internal/webserver/job.go` | rewrite | | | | | | | | | | | | |
| `internal/webserver/job_test.go` | create | | | | | | | | | | | | |
| `internal/webserver/crawl_job.go` | create | | | | | | | | | | | | |
| `internal/webserver/hardening_test.go` | modify | | | | | | | | | | | | |
| `internal/webserver/server.go` | modify | rewrite | | | | | | | | | | | |
| `internal/webserver/crawls.go` | | create | | | | | | | | | | | |
| `internal/webserver/request.go`, `request_test.go` | | create | | | | | | | | | | | |
| `internal/webserver/{sitemap,compare,import,redirects,settings,profiles,login}.go` | | create (stubs) | | | | | | | | | | | |
| `internal/runner/options.go`, `options_test.go` | | | create | | | | | | | | | | |
| `internal/runner/runner.go` | | | modify | | | | | | | | | | |
| `web/src/api/client.ts`, `web/src/api/crawls.ts` | | | | create | | | | | | | | | |
| `web/src/api.ts` | | | | delete | | | | | | | | | |
| `web/src/types.ts` -> `web/src/types/crawls.ts` | | | | move | | | | | | | | | |
| `web/src/routes.ts` | | | | | create | | | | | | | | |
| `web/src/views/{Compare,Redirects,Redirect,Settings}View.tsx` | | | | | create | | | | | | | | |
| `web/src/App.tsx` | | | | | rewrite | | | rewrite | | | | | |
| `web/src/main.tsx` | | | | | | modify | modify | modify | | | | | modify |
| `web/src/styles/tokens.css`, `web/src/theme.ts`, `web/scripts/contrast.mjs` | | | | | | create | | | | | | | |
| `web/package.json`, `web/README.md` | | | | | | modify | | | | | | | |
| `web/src/fonts/*.woff2`, `web/src/fonts/OFL.txt`, `web/src/styles/fonts.css` | | | | | | | create | | | | | | |
| `web/src/styles/base.css`, `web/src/components/Rail.{tsx,css}` | | | | | | | | create | | | | | |
| `web/src/index.css` | | | | | | | modify | modify | | | | | delete |
| `web/src/status.ts`, `web/src/components/ui/*` | | | | | | | | | create | | | | |
| `web/src/components/tape/*` | | | | | | | | | | create | | | |
| `web/src/components/CrawlForm.{tsx,css}` | | | | modify | | | | | | | rewrite | | |
| `web/src/components/CrawlReport.{tsx,css}`, `web/src/crawlStatus.ts` | | | | modify | | | | | | | | rewrite | |
| `web/src/components/History.{tsx,css}` | | | | modify | | | | | | | | | rewrite |
| `web/src/components/SiteMap.{tsx,css}` | | | | modify | | | | | | | | | modify |
| `docs/web.md`, `docs/architecture.md`, `docs/README.md`, `CHANGELOG.md` | | | | | modify | | | | | | | | modify |

#### Conventions for every task

- Work on the feature branch (today `Patience-dot-devl/gui-self-contained-build`), never on
  `main`. All commands run from the repository root unless a step says `cd web`.
- `npm run build` writes into `internal/webserver/webui/dist/` and overwrites the committed
  placeholder `index.html`. Every frontend commit step therefore starts with
  `git restore internal/webserver/webui/dist/index.html`; the built assets next to it are
  gitignored. Never commit a built `index.html`.
- The frontend has no JavaScript test framework. Its test cycle is
  `cd web && npm run build && npm run lint` (type check, bundle, oxlint, and from VIS-1 on
  the contrast check) plus the manual check named in the task.
- Every task ends with `make check` passing (gofmt, `go mod verify`, `go vet`, `go build`,
  `go test -race`, golangci-lint, docs-drift tests).
- `sed` commands write to a temporary file and move it back, so they behave the same with
  BSD (macOS) and GNU sed.

### Brief: progress and pause

Spec: `docs/superpowers/specs/2026-10-13-web-ui-feature-complete-design.md` (Backend 1 "Live
progress", Backend 2 "Pause and resume", Frontend "Crawl (running/paused/finished)", "Crawls",
"New crawl" sitemap line, Testing). Contract: `.context/plan/00-contract.md` (names used exactly).

Work on this track's branch, cut after wave 0 and the visual system landed. Never commit to
`main`. Every task ends with `make check` passing; frontend tasks also run
`cd web && npm run build && npm run lint`.

#### Files touched

| File | Tasks | Kind |
| --- | --- | --- |
| `internal/crawler/adaptive.go` | W1-PROG-1, W2-PAUSE-4 | modify |
| `internal/crawler/adaptive_test.go` | W1-PROG-1, W2-PAUSE-4 | modify |
| `internal/crawler/progress.go` | W1-PROG-2, W2-PAUSE-3 | create, then modify |
| `internal/crawler/types.go` (`Options`, `Coverage`) | W1-PROG-2, W1-PROG-5, W1-PROG-6b, W2-PAUSE-3, W2-PAUSE-4 | modify |
| `internal/crawler/tape.go`, `tape_test.go` | W1-PROG-6b | create |
| `internal/crawler/frontier.go` | W1-PROG-2, W2-PAUSE-1 | modify |
| `internal/crawler/frontier_test.go` | W2-PAUSE-1 | modify |
| `internal/crawler/engine.go` | W1-PROG-2, W1-PROG-3, W2-PAUSE-3, W2-PAUSE-4 | modify |
| `internal/crawler/progress_test.go` | W1-PROG-2 | create |
| `internal/crawler/inscope_test.go` | W1-PROG-3 | create |
| `internal/crawler/clock.go`, `clock_test.go` | W2-PAUSE-2 | create |
| `internal/crawler/pause.go`, `pause_test.go` | W2-PAUSE-3, W2-PAUSE-4 | create, then modify |
| `internal/analyze/sitemap/sitemap.go`, `sitemap_test.go` | W1-PROG-4 | modify |
| `internal/runner/runner.go` (`Run`, `RegistryOptions`, `BuildRegistry`) | W1-PROG-5, W1-PROG-6, W1-PROG-6b, W2-PAUSE-5 | modify |
| `internal/runner/sitemap.go`, `sitemap_test.go` | W1-PROG-5, W1-PROG-7 | create, then modify |
| `internal/runner/progress.go`, `progress_test.go` | W1-PROG-6, W1-PROG-6b | create, then modify |
| `internal/runner/options.go` (wave 0 file) | W1-PROG-6, W2-PAUSE-5 | modify |
| `internal/runner/control.go`, `control_test.go` | W2-PAUSE-5 | create |
| `internal/store/store.go`, `store_test.go` (`Entry`) | W1-PROG-8 | modify |
| `internal/webserver/crawl_job.go` (wave 0: `crawlJob`, `JobView`, `crawlListItem`, `jobListItem`) | W1-PROG-8, W2-PAUSE-6 | modify |
| `internal/webserver/crawls.go` (wave 0: crawl handlers, `crawlRoutes`) | W1-PROG-8, W2-PAUSE-6 | modify |
| `internal/webserver/progress.go`, `progress_test.go` | W1-PROG-8, W2-PAUSE-6 | create, then modify |
| `internal/webserver/sitemap.go` (wave 0 stub), `sitemap_test.go` | W1-PROG-9 | modify, create |
| `internal/webserver/pause.go`, `pause_test.go` | W2-PAUSE-6 | create |
| `internal/webserver/server_test.go` (`pollUntilFinished`) | W2-PAUSE-6 | modify |
| `web/src/types/progress.ts`, `web/src/api/progress.ts` | W2-PROG-UI-1 | create |
| `web/src/types/crawls.ts` (wave 0) | W1-PROG-6b (`Coverage` fields), W2-PROG-UI-1 (`'paused'`) | modify |
| `web/src/crawlStatus.ts` (visual system) | W2-PROG-UI-1 | modify (`paused` entry) |
| `web/src/components/progress/tapeText.ts` | W2-PROG-UI-1 | create |
| `web/src/components/progress/useCrawlPoll.ts`, `RunningCrawl.tsx`, `RunPhasePill.tsx`, `progress.css` | W2-PROG-UI-2, W3-PAUSE-UI-1 | create, then modify |
| `web/src/components/progress/FinishedTape.tsx` | W2-PROG-UI-3 | create |
| `web/src/components/progress/SitemapPreviewLine.tsx` | W2-PROG-UI-4 | create |
| `web/src/components/progress/MiniTape.tsx` | W2-PROG-UI-5 | create |
| `web/src/components/CrawlReport.tsx` | W2-PROG-UI-2, W2-PROG-UI-3, W3-PAUSE-UI-1 | modify |
| `web/src/components/CrawlForm.tsx` | W2-PROG-UI-4 | modify (2 imports, 1 element) |
| `web/src/components/History.tsx` | W2-PROG-UI-5 | modify |
| `web/src/api/pause.ts`, `components/progress/CrawlControls.tsx`, `AutoPauseNotice.tsx` | W3-PAUSE-UI-1 | create |
| `docs/web.md`, `docs/output.md`, `docs/architecture.md`, `docs/README.md` (generated), `CHANGELOG.md` | W1-PROG-10, W2-PROG-UI-6, W2-PAUSE-7, W3-PAUSE-UI-2 | modify |

#### Baseline this section assumes (wave 0 + visual system)

Exactly as the contract describes. Where the contract leaves a name open, these are assumed and
called out again under "Contract gaps" at the end:

- `internal/webserver/crawls.go` holds `crawlRoutes()`, `handleStartCrawl`, `handleListCrawls`,
  `handleGetCrawl`, `handleCancelCrawl`, `handleExportCrawl`, moved unchanged from `server.go`
  except that they use `s.crawls` (`*jobManager[crawlJob]`).
- `internal/webserver/crawl_job.go` holds `crawlJob`, `JobView`, `crawlListItem`,
  `func jobView(j Job[crawlJob]) JobView` and `func jobListItem(j Job[crawlJob]) crawlListItem`
  (wave 0 moved them out of `job.go`, which now holds only the generic `Job[T]`/`jobManager[T]`).
- `handleStartCrawl` calls `s.crawls.start(crawlJob{Seed: seed}, cancel, s.maxRunning)` and,
  in its goroutine, `runner.Run(ctx, cfg, seed)`, then `s.crawls.finish(job.ID, runErr, ...)`.
- `crawlJob` has `Seed`, `Report`, `StoreID`. The `Progress`/`Tape`/`TapeOff` and `control`
  fields cannot exist before `runner.Progress` / `runner.Control` do, so this section adds them.
- `internal/runner/options.go` holds `type RunOption func(*runSettings)`, an empty
  `type runSettings struct{}` and `func newRunSettings(opts []RunOption) runSettings` (skips nil
  options; pinned by wave 0's `TestNewRunSettingsAppliesEveryOption`). `Run`'s variadic
  parameter is named `runOpts`, and wave 0's `Run` starts with
  `settings := newRunSettings(runOpts)` followed by `_ = settings`.
- `requireJSON(w, r) bool`, `writeJSON`, `writeError`, `formatTime` exist in the webserver
  package; `server_test.go` keeps `newTestServer`, `doJSON`, `startCrawlTarget`,
  `pollUntilFinished`.
- Frontend: `api/client.ts` `request<T>(path, init?)` forwards `init` (including `signal`) to
  `fetch` and returns `undefined` on `204`; `postJSON<T>(path, body, method='POST')`.
  `api/crawls.ts` exports `cancelCrawl(id): Promise<void>`. `types/crawls.ts` exports
  `JobView`, `CrawlListItem`, `Report`, `Coverage`, `CrawlStatus`. `components/ui/Button.tsx`
  (default export) takes `variant` plus every native `<button>` prop. `components/ui/Notice.tsx`
  (default export) takes `tone`, `children`, `action`. `components/ui/PhasePill.tsx` takes
  `{ phase: 'crawling'|'paused'|'analyzing'|'done'|'stopped'|'failed'; children }`.
  `web/src/status.ts` exports `StatusClass`, `STATUS_CLASSES`, `statusClass`, `STATUS_LABEL`.
  `web/src/crawlStatus.ts` exports `STATUS_PILL: Record<CrawlStatus, {phase, label}>`.
  `Tape` renders ticks only, accepts `'x'` for a fetch error and aggregates (worst status wins)
  above 20,000 ticks, or above 160 when `compact`; `TapeLegend` (`{sitemap, finished}`) is the
  separate legend.

#### Lock order (binding for every task below; the race detector runs in CI)

```
engine result mu  ->  Engine.progMu  ->  limiter.mu | Engine.ps.mu
Engine.ps.mu      ->  frontier.mu | activeClock.mu | limiter.mu      (never the reverse)
Engine.progMu     ->  OnProgress  ->  runner progressReporter.mu  ->  webserver jobManager.mu
runner Control.mu ->  Engine.ps.mu, Engine.progMu (through Engine.Pause/Resume)
webserver jobManager.mu -> Control.Active() -> Engine.ps.mu -> activeClock.mu
```

Rules that follow from it, each pinned by a test:

1. `Engine.pauseWith` / `Engine.Resume` release `ps.mu` before emitting a progress event.
2. `OnProgress` must never call `Engine.Pause`/`Resume` synchronously (it runs under `progMu`).
3. The webserver never calls `Control.Pause`/`Control.Resume` inside `s.crawls.with`;
   `Control.Active` takes no `Control` lock (atomics), so calling it inside `with` is safe.

### Brief: settings and login

Spec: `docs/superpowers/specs/2026-10-13-web-ui-feature-complete-design.md` sections 6 and 7,
the New crawl and Settings views, error handling, testing. Binding names:
`.context/plan/00-contract.md`.


**Goal:** crawl parameters `strip_query`, `adaptive_delay`, `follow_nofollow`, `timeout`,
`bearer_token` everywhere `crawlrequest.Params` goes; header profiles, GUI settings and an
interactive browser login in the web API; a Settings view and a complete New crawl form in the UI.

**Architecture:** config owns the bearer-token header and header-profile files; `crawlrequest`
maps the new params; `internal/render` owns everything chromedp (browser lookup, the login
window); `internal/webserver` gets `settings.go`, `profiles.go`, `login.go` (a `loginManager`
behind the `loginBrowser` interface, so handler tests use a fake). The frontend splits the
crawl form into a reusable field set (`components/crawlform/`) used by New crawl and Settings.

**Tech stack:** Go 1.27, chromedp v0.19.1 / cdproto v0.157.6 (as already in go.mod), React 19 +
TypeScript, oxlint. No new dependencies.

#### Assumptions about wave 0 and the visual system (from the contract)

- `internal/webserver/settings.go`, `profiles.go`, `login.go` exist with empty
  `settingsRoutes()`, `profileRoutes()`, `loginRoutes()`; `login.go` also holds
  `type loginManager struct{}` and `func newLoginManager() *loginManager`.
- `Server` has `crawls *jobManager[crawlJob]`, `logins *loginManager`, `guiSettingsPath string`;
  `New` builds `logins: newLoginManager()` **before** applying options (like `jobs` today).
  `WithLoginBrowser` relies on that; `TestLoginSessionFlow` fails loudly if it is not so.
- The crawl-start handler moved to `internal/webserver/crawls.go`, still named
  `handleStartCrawl`, still decoding into `startCrawlRequest` and calling `req.ToConfig()`.
- `requireJSON(w, r) bool`, `writeJSON`, `writeError`, `formatTime`, and the test helpers
  `newTestServer`, `doJSON`, `pollUntilFinished` exist with today's signatures.
- Frontend: `web/src/api/client.ts` exports `request<T>`, `postJSON<T>(path, body, method='POST')`;
  `web/src/api/crawls.ts` exports `listAnalyzers`, `startCrawl`; `web/src/types/crawls.ts`
  exports `StartCrawlParams`, `AnalyzerInfo`; `web/src/theme.ts` exports `type Theme` and
  `applyTheme`; `components/ui/Button.tsx` and `components/ui/Notice.tsx` are **default exports**
  (confirmed against W1-VIS-4): `Button` takes `variant` + every `<button>` attribute;
  `Notice` takes exactly `tone`, `children`, optional `action`. The New crawl form's markup
  and `CrawlForm.css` (`cf-` classes) come from W1-VIS-6; `index.css` and its old classes
  (`card`, `form-section`, `checkbox`, …) are gone after W1-VIS-8, so every new UI file here
  uses `cf-` classes or its own `set-` stylesheet.

#### Files touched

| Path | Wave/task | What |
| --- | --- | --- |
| `internal/config/config.go` | W1-SET-1, W1-SET-4 | `CrawlConfig.BearerToken`, env default, bearer header + conflicts in `ToOptions`; `ErrInvalidHeaderProfile` in `HeaderProfilePath` |
| `internal/config/example.go`, `configs/example.yaml` | W1-SET-1 | `bearer_token:` in the `gocrawl init` template and its checked-in copy (also the copy in `docs/configuration.md`) |
| `internal/config/profiles.go` (new) | W1-SET-4 | `ListHeaderProfiles`, `ReadHeaderProfile`, `SaveHeaderProfile`, `DeleteHeaderProfile` |
| `internal/config/headers_test.go`, `config_test.go`, `profiles_test.go` (new) | W1-SET-1, W1-SET-4 | tests |
| `internal/render/render.go` | W1-SET-1 | headless rejection message names `--bearer-token` |
| `internal/render/login.go` (new), `login_test.go` (new) | W1-LOGIN-1 | `FindBrowser`, `LoginAvailable`, `ErrNoBrowser`, `ErrNoDisplay`, `LoginWindow` |
| `internal/crawlrequest/crawlrequest.go`, `_test.go` | W1-SET-2 | five new `Params` fields |
| `internal/mcpserver/tools_test.go` | W1-SET-2 | schema test (no code change: `CrawlInput = crawlrequest.Params`) |
| `cmd/gocrawl/crawl.go`, `interactive.go`, `crawl_test.go` | W1-SET-3 | `--bearer-token`, interactive field |
| `cmd/gocrawl/serve.go` | W1-SET-6, W1-LOGIN-3 | `WithGUISettingsPath(DefaultGUISettingsPath())`, `defer web.Close()` |
| `internal/webserver/crawls.go` | W1-SET-5, W1-LOGIN-4 | early `cfg.ToOptions()` validation; `LoginSession` field + resolution |
| `internal/webserver/settings.go` (wave 0 file), `settings_test.go` (new) | W1-SET-6 | `GUISettings`, `DefaultGUISettingsPath`, routes (keeps wave 0's `WithGUISettingsPath`) |
| `internal/webserver/profiles.go`, `profiles_test.go` (new) | W1-SET-7 | header-profile routes |
| `internal/webserver/crawl_auth_test.go` (new) | W1-SET-5 | bearer conflicts / end-to-end |
| `internal/webserver/login.go`, `login_test.go`, `login_routes_test.go`, `crawl_login_test.go` (new) | W1-LOGIN-2..4 | manager, routes, `WithLoginBrowser`, `Server.Close`, crawl resolution |
| `docs/configuration.md`, `docs/mcp.md`, `docs/web.md`, `docs/README.md` (generated), `CHANGELOG.md` | W1-SET-8, W1-LOGIN-5, W2-SET-6, W2-LOGIN-3 | docs |
| `web/src/types/crawls.ts` | W2-SET-1, W2-LOGIN-1 | new `StartCrawlParams` fields |
| `web/src/types/settings.ts`, `types/profiles.ts`, `types/login.ts` (new) | W2-SET-1, W2-LOGIN-1 | types |
| `web/src/api/settings.ts`, `api/profiles.ts`, `api/login.ts` (new) | W2-SET-1, W2-LOGIN-1 | API calls |
| `web/src/settings/context.ts`, `settings/SettingsProvider.tsx` (new), `web/src/main.tsx` | W2-SET-2 | load settings at startup, apply theme |
| `web/src/components/crawlform/values.ts`, `CrawlFields.tsx`, `LoginPanel.tsx` (new) | W2-SET-3, W2-LOGIN-2 | form model, reusable field set, login panel |
| `web/src/components/CrawlForm.tsx` | W2-SET-3, W2-SET-4, W2-LOGIN-2 | uses the field set (keeps the visual markup and the progress track's sitemap preview); defaults; login |
| `web/src/components/CrawlForm.css` (visual system) | W2-SET-4, W2-LOGIN-2 | `.cf-actions` row, `.cf-login` panel |
| `web/src/views/SettingsView.tsx`, `SettingsView.css` (new) | W2-SET-5 | replaces the wave-0 stub |

Decision recorded here (the contract left it open): **the store path is returned by
`GET /api/settings` (and `PUT`) as a read-only `store_dir` field next to `theme` and
`crawl_defaults`**; `PUT` ignores it.

#### Conventions for every task

- Work on the track branch cut after wave 0 (e.g. `feat/web-ui-settings` and
  `feat/web-ui-login`); never commit to `main`.
- Commit with `git commit -F -` and a heredoc; every message ends with
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.
- Frontend tasks: `make web-build` overwrites `internal/webserver/webui/dist/index.html`;
  run `git checkout -- internal/webserver/webui/dist/index.html` before committing.
- Manual UI checks use a throwaway home so they never touch your real `~/.gocrawl`:
  `make web-build && go build -o /tmp/gocrawl-dev ./cmd/gocrawl && HOME="$(mktemp -d)" /tmp/gocrawl-dev serve`
  (the same as `go run ./cmd/gocrawl serve`, with a temporary `HOME`), then open
  http://127.0.0.1:8080.
- Secrets (bearer token, Basic Auth, cookies, header values) never appear in an error
  message, a log line, `gui.json`, or a login-session response. Tests pin each of these.
- Error strings start lower-case and have no trailing period (staticcheck ST1005 runs in
  `make check`).

### Brief: compare, import and redirects

Spec: `docs/superpowers/specs/2026-10-13-web-ui-feature-complete-design.md` sections 3, 4 and 5,
plus the Crawls, Compare and Redirects views, error handling and testing.
Contract: `.context/plan/00-contract.md`. The names in that file are used exactly as written.
Additions are listed under "Contract gaps filled" at the end.

This section has two tracks, and each track has a backend wave (W1) and a UI wave (W2):

- **Compare/import track:** W1-CMP-1..4 (backend), then W2-CMP-1..5 (UI).
- **Redirects track:** W1-RED-1..4 (backend), then W2-RED-1..4 (UI).

The two tracks share no files except `docs/web.md` and `CHANGELOG.md`. Each track writes its
own test helpers with track-specific names, so the tracks can run on separate branches and
merge without conflicts.

#### Assumptions about wave 0 and the visual system

These come from the contract. If one does not hold, fix wave 0. Do not work around it here.

- `internal/webserver/job.go` has the generic `Job[T]` and `jobManager[T]`, with `start`, `get`,
  `list`, `with`, `finish(id, err, apply)` and `cancel`. `finish` sets `StatusError` when `err != nil`.
  Otherwise it sets `StatusCanceled` when `cancelRequested` is true, and `StatusDone` in every other case.
- `Server` has `store *store.Store`, `crawls *jobManager[crawlJob]` and
  `redirects *jobManager[redirectJob]`. `New` initializes `redirects` with
  `newJobManager[redirectJob]()`. `crawlJob` has `Seed`, `Report` and `StoreID`.
- `compare.go`, `import.go` and `redirects.go` exist. Each one holds an empty
  `compareRoutes()`, `importRoutes()` or `redirectRoutes()`. `redirects.go` also declares
  `type redirectJob struct{}`.
- `parseMultipart(w, r, maxBytes int64) bool` writes a 415 when the request is not
  `multipart/form-data` and a 413 when the body is larger than `maxBytes`. It writes a 400 for
  any other parse error. After it returns true, `r.FormValue` and `r.FormFile` work.
- `writeJSON`, `writeError`, `formatTime`, `DefaultMaxRunning`, and the test helpers
  `newTestServer` and `doJSON` (in `server_test.go`) keep their current signatures.
- Frontend: `web/src/api/client.ts` exports `request<T>`, `postJSON<T>` and `postForm<T>`.
  `request` returns `undefined` on a 204 and throws `Error(body.error)` when the response is
  not ok. `web/src/api/crawls.ts` exports `listCrawls`, `getCrawl` and `listExplanations`.
  `web/src/types/crawls.ts` exports `Issue`, `Report`, `Explanation` and `CrawlListItem`.
  `web/src/routes.ts` exports `href(route)`. The `Route` union includes
  `{view:'compare', base?, current?}`, `{view:'crawls'}`, `{view:'crawl', id}`,
  `{view:'redirects'}` and `{view:'redirect', id}`.
- `Button`, `Notice`, `StatusCode` and `Tape` are **default exports** at the contract paths
  (confirmed against W1-VIS-4 and W1-VIS-5). `Button` passes native `<button>` props through
  (`type`, `disabled`, `onClick`); `Notice` takes `{tone, children, action?}`.

`npm run build` writes into `internal/webserver/webui/dist/`. Never commit that output. Every
frontend commit below runs `git add` on explicit `web/src` paths.

#### Files touched

| File | Task(s) | Change |
| --- | --- | --- |
| `internal/store/store.go` | W1-CMP-1 | Add `ResolveStored`; `Resolve` delegates to it |
| `internal/store/store_test.go` | W1-CMP-1 | Test `ResolveStored` |
| `internal/webserver/compare.go` | W1-CMP-2 | `GET /api/compare` |
| `internal/webserver/compare_test.go` | W1-CMP-2 | New |
| `internal/webserver/import.go` | W1-CMP-3 | `POST /api/reports/import` |
| `internal/webserver/import_test.go` | W1-CMP-3 | New |
| `internal/redirectcheck/types.go` | W1-RED-1 | JSON tags on `RowResult` |
| `internal/redirectcheck/parse.go` | W1-RED-1 | JSON tags on `Rule` |
| `internal/redirectcheck/run.go` | W1-RED-1 | `RunOptions.OnRow` |
| `internal/redirectcheck/fetcher.go` | W1-RED-1 | New: `NewFetcher`, `DefaultTimeout`, `DefaultUserAgent` |
| `internal/redirectcheck/run_test.go`, `json_test.go`, `fetcher_test.go` | W1-RED-1 | Tests |
| `cmd/gocrawl/checkredirects.go` | W1-RED-1 | Use `redirectcheck.NewFetcher` and its defaults |
| `internal/webserver/redirects.go` | W1-RED-2, W1-RED-3 | Fill `redirectJob`; add list/start/get/cancel/export routes |
| `internal/webserver/redirects_test.go` | W1-RED-2, W1-RED-3 | New |
| `docs/web.md` | W1-CMP-4, W1-RED-4, W2-CMP-5, W2-RED-4 | API rows, views |
| `docs/redirect-check.md` | W1-RED-4 | "In the web UI" section |
| `CHANGELOG.md` | W1-CMP-4, W1-RED-4, W2-CMP-5, W2-RED-4 | One line each |
| `web/src/types/compare.ts`, `web/src/api/compare.ts`, `web/src/api/import.ts` | W2-CMP-1 | New |
| `web/src/components/IssueTable.tsx` | W2-CMP-2 | New (extracted from CrawlReport) |
| `web/src/components/CrawlReport.tsx` | W2-CMP-2 | Use `IssueTable` |
| `web/src/views/compare/statusTape.ts`, `web/src/views/compare/pairs.ts` | W2-CMP-3 | New helpers |
| `web/src/views/CompareView.tsx`, `web/src/views/compare.css` | W2-CMP-3 | Replace stub; new CSS |
| `web/src/App.tsx` | W2-CMP-3 | Pass `base`/`current` to `CompareView` (`RedirectView` already gets `id` from wave 0) |
| `web/src/components/History.tsx` | W2-CMP-4 | Compare checkboxes, **Import report…** |
| `web/src/types/redirects.ts`, `web/src/api/redirects.ts` | W2-RED-1 | New |
| `web/src/views/RedirectsView.tsx`, `web/src/views/RedirectView.tsx`, `web/src/views/redirects.css` | W2-RED-2, W2-RED-3 | Replace stubs; new CSS |

## Wave 0: prep

### Track: prep (feature branch)

#### Task W0-1: Generic job manager and crawlJob

**Files:**
- Modify (rewrite): `internal/webserver/job.go`
- Create: `internal/webserver/crawl_job.go`
- Modify: `internal/webserver/server.go`, `internal/webserver/hardening_test.go`
- Test: `internal/webserver/job_test.go`

**Interfaces:**
- Consumes: `report.Report`, `runner.Run(ctx, cfg, seed)` (unchanged in this task).
- Produces (contract names):
  - `type Status string`; `StatusRunning`, `StatusPaused`, `StatusDone`, `StatusError`, `StatusCanceled`
  - `type Job[T any] struct { ID string; Status Status; Err string; StartedAt, FinishedAt time.Time; Data T; cancel context.CancelFunc; cancelRequested bool }`
  - `func (j *Job[T]) active() bool` (running or paused)
  - `type jobManager[T any] struct { mu sync.Mutex; jobs map[string]*Job[T] }`
  - `func newJobManager[T any]() *jobManager[T]`
  - `func (m *jobManager[T]) start(data T, cancel context.CancelFunc, maxRunning int) (Job[T], bool)`
  - `func (m *jobManager[T]) get(id string) (Job[T], bool)`
  - `func (m *jobManager[T]) list() []Job[T]`
  - `func (m *jobManager[T]) with(id string, fn func(j *Job[T])) bool`
  - `func (m *jobManager[T]) finish(id string, err error, apply func(d *T))`
  - `func (m *jobManager[T]) cancel(id string) bool`
  - `type crawlJob struct { Seed string; Report *report.Report; StoreID string }`
  - `func jobView(j Job[crawlJob]) JobView`, `func jobListItem(j Job[crawlJob]) crawlListItem`
  - `Server.crawls *jobManager[crawlJob]` (replaces `Server.jobs`)

- [ ] **Step 1: Write the failing test.** Create `internal/webserver/job_test.go`:

```go
package webserver

import (
	"errors"
	"testing"
)

type testJobData struct {
	N int
}

func noopCancel() {}

func TestJobManagerCapCountsRunningAndPausedJobs(t *testing.T) {
	m := newJobManager[testJobData]()
	first, ok := m.start(testJobData{N: 1}, noopCancel, 2)
	if !ok {
		t.Fatal("first start refused")
	}
	if _, ok := m.start(testJobData{N: 2}, noopCancel, 2); !ok {
		t.Fatal("second start refused")
	}
	m.with(first.ID, func(j *Job[testJobData]) { j.Status = StatusPaused })
	if _, ok := m.start(testJobData{N: 3}, noopCancel, 2); ok {
		t.Fatal("start accepted with one running and one paused job at cap 2; paused jobs must count")
	}
	m.finish(first.ID, nil, nil)
	if _, ok := m.start(testJobData{N: 3}, noopCancel, 2); !ok {
		t.Fatal("start refused after the paused job finished")
	}
}

func TestJobManagerStartReturnsRunningSnapshot(t *testing.T) {
	m := newJobManager[testJobData]()
	j, ok := m.start(testJobData{N: 7}, noopCancel, 1)
	if !ok {
		t.Fatal("start refused")
	}
	if j.ID == "" || j.Status != StatusRunning || j.StartedAt.IsZero() || j.Data.N != 7 {
		t.Fatalf("start returned %+v, want an id, status running, a start time and the data", j)
	}
}

func TestJobManagerWithMutatesLiveJob(t *testing.T) {
	m := newJobManager[testJobData]()
	j, _ := m.start(testJobData{N: 1}, noopCancel, 1)
	before, _ := m.get(j.ID)

	if !m.with(j.ID, func(live *Job[testJobData]) { live.Data.N = 42 }) {
		t.Fatal("with returned false for a known job")
	}
	after, _ := m.get(j.ID)
	if after.Data.N != 42 {
		t.Fatalf("Data.N = %d after with, want 42", after.Data.N)
	}
	if before.Data.N != 1 {
		t.Fatalf("earlier snapshot changed to %d; get must return a copy", before.Data.N)
	}
	if m.with("missing", func(*Job[testJobData]) { t.Error("fn called for an unknown id") }) {
		t.Fatal("with returned true for an unknown id")
	}
}

func TestJobManagerFinishAppliesData(t *testing.T) {
	m := newJobManager[testJobData]()
	ok1, _ := m.start(testJobData{}, noopCancel, 2)
	bad, _ := m.start(testJobData{}, noopCancel, 2)

	m.finish(ok1.ID, nil, func(d *testJobData) { d.N = 5 })
	got, _ := m.get(ok1.ID)
	if got.Status != StatusDone || got.Data.N != 5 || got.FinishedAt.IsZero() || got.Err != "" {
		t.Fatalf("after a clean finish: %+v, want done, N=5, a finish time and no error", got)
	}

	m.finish(bad.ID, errors.New("boom"), func(d *testJobData) { d.N = 9 })
	got, _ = m.get(bad.ID)
	if got.Status != StatusError || got.Err != "boom" || got.Data.N != 9 {
		t.Fatalf("after a failed finish: %+v, want error \"boom\" and apply still run (N=9)", got)
	}

	m.finish("missing", nil, func(*testJobData) { t.Error("apply called for an unknown id") })
}

func TestJobManagerCancel(t *testing.T) {
	m := newJobManager[testJobData]()
	cancelled := 0
	cancel := func() { cancelled++ }

	running, _ := m.start(testJobData{}, cancel, 3)
	paused, _ := m.start(testJobData{}, cancel, 3)
	m.with(paused.ID, func(j *Job[testJobData]) { j.Status = StatusPaused })
	done, _ := m.start(testJobData{}, cancel, 3)
	m.finish(done.ID, nil, nil)

	if !m.cancel(running.ID) {
		t.Error("cancel(running) = false, want true")
	}
	if !m.cancel(paused.ID) {
		t.Error("cancel(paused) = false, want true")
	}
	if m.cancel(done.ID) {
		t.Error("cancel(done) = true, want false")
	}
	if m.cancel("missing") {
		t.Error("cancel(unknown) = true, want false")
	}
	if cancelled != 2 {
		t.Errorf("cancel func called %d times, want 2", cancelled)
	}

	m.finish(running.ID, nil, nil)
	m.finish(paused.ID, nil, nil)
	for _, id := range []string{running.ID, paused.ID} {
		if j, _ := m.get(id); j.Status != StatusCanceled {
			t.Errorf("job %s status = %s after cancel + finish, want canceled", id, j.Status)
		}
	}
}

func TestJobManagerList(t *testing.T) {
	m := newJobManager[testJobData]()
	m.start(testJobData{N: 1}, noopCancel, 5)
	m.start(testJobData{N: 2}, noopCancel, 5)
	if got := len(m.list()); got != 2 {
		t.Fatalf("list returned %d jobs, want 2", got)
	}
	if _, ok := m.get("missing"); ok {
		t.Fatal("get(unknown) ok = true")
	}
}
```

- [ ] **Step 2: Run it and watch it fail.**
  `go test ./internal/webserver -run TestJobManager`
  Expected: build failure, starting with
  `job_test.go:15:20: cannot index newJobManager (value of type func() *jobManager)` and
  `undefined: StatusPaused`.

- [ ] **Step 3: Replace `internal/webserver/job.go` with the generic manager.** The whole file:

```go
package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Status is the lifecycle state of a background job: a crawl or a redirect check.
type Status string

const (
	StatusRunning  Status = "running"
	StatusPaused   Status = "paused"
	StatusDone     Status = "done"
	StatusError    Status = "error"
	StatusCanceled Status = "canceled"
)

// Job is one background job started through the web API. The lifecycle fields are shared by
// every kind of job; Data holds what is specific to one kind (crawlJob, redirectJob).
type Job[T any] struct {
	ID         string
	Status     Status
	Err        string
	StartedAt  time.Time
	FinishedAt time.Time
	Data       T

	cancel          context.CancelFunc
	cancelRequested bool
}

// active reports whether the job still occupies a slot under its manager's concurrency cap.
// A paused job holds its slot: it has an engine, a fetcher and a frontier in memory, and it
// can resume at any moment.
func (j *Job[T]) active() bool {
	return j.Status == StatusRunning || j.Status == StatusPaused
}

// jobManager holds every job of one kind that the server has started, in memory, for the
// life of the process. Crawls and redirect checks each get their own manager, and so their
// own cap.
type jobManager[T any] struct {
	mu   sync.Mutex
	jobs map[string]*Job[T]
}

func newJobManager[T any]() *jobManager[T] {
	return &jobManager[T]{jobs: make(map[string]*Job[T])}
}

func newJobID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// start, get, and list all return Job by value: a snapshot copied out while holding the lock.
// The canonical, mutable *Job lives only in m.jobs and is never handed to a caller except
// inside with, so a read of a snapshot can never race against finish/cancel/with mutating
// that same struct. The copy is shallow: slices and pointers in Data still share their
// backing memory, so code that reads something another goroutine appends to (the crawl
// tape) reads it inside with instead.
//
// start refuses (ok == false) when maxRunning jobs are already running or paused, so the
// count and the insert happen under one lock and two concurrent starts cannot both slip
// past the cap.
func (m *jobManager[T]) start(data T, cancel context.CancelFunc, maxRunning int) (Job[T], bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	active := 0
	for _, j := range m.jobs {
		if j.active() {
			active++
		}
	}
	if active >= maxRunning {
		return Job[T]{}, false
	}
	j := &Job[T]{
		ID:        newJobID(),
		Status:    StatusRunning,
		StartedAt: time.Now(),
		Data:      data,
		cancel:    cancel,
	}
	m.jobs[j.ID] = j
	return *j, true
}

func (m *jobManager[T]) get(id string) (Job[T], bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Job[T]{}, false
	}
	return *j, true
}

// list returns a snapshot of every job, in no particular order.
func (m *jobManager[T]) list() []Job[T] {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job[T], 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, *j)
	}
	return out
}

// with runs fn on the live job under the manager's lock and reports whether the job exists.
// Use it to update Data while the job runs (progress, tape) and to build views that must
// not race with those updates. fn must not block and must not call back into m.
func (m *jobManager[T]) with(id string, fn func(j *Job[T])) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return false
	}
	fn(j)
	return true
}

// finish records the outcome of a job: the finish time, the final status, and the error
// text. apply, when non-nil, runs under the lock on the job's Data, whatever the outcome, so
// the result lands in the same critical section as the status change and a poller never
// sees "done" without it.
//
// A canceled context does not make runner.Run return an error (see
// crawler.Coverage.Interrupted): it still produces a valid partial report, so the only way
// to tell a user-requested cancellation apart from a normal finish is the cancelRequested
// flag set by cancel below.
func (m *jobManager[T]) finish(id string, err error, apply func(d *T)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return
	}
	j.FinishedAt = time.Now()
	switch {
	case err != nil:
		j.Status = StatusError
		j.Err = err.Error()
	case j.cancelRequested:
		j.Status = StatusCanceled
	default:
		j.Status = StatusDone
	}
	if apply != nil {
		apply(&j.Data)
	}
}

// cancel requests that a running or paused job stop. It is a no-op (false) if the job is
// already finished or unknown.
func (m *jobManager[T]) cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || !j.active() {
		return false
	}
	j.cancelRequested = true
	j.cancel()
	return true
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
```

- [ ] **Step 4: Create `internal/webserver/crawl_job.go`** with the crawl-specific data and the
  view builders that used to live in `job.go`:

```go
package webserver

import (
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// crawlJob is the crawl-specific part of a Job: what the web API knows about one crawl
// beyond its lifecycle. Later features add their fields here (live progress and the tape,
// the pause control).
type crawlJob struct {
	Seed    string
	Report  *report.Report
	StoreID string // set once the report is saved, so history and job views agree on one ID
}

// JobView is the API representation of a crawl job.
type JobView struct {
	ID         string         `json:"id"`
	Seed       string         `json:"seed"`
	Status     string         `json:"status"`
	Err        string         `json:"error,omitempty"`
	StartedAt  string         `json:"started_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Persisted  bool           `json:"persisted"`
	Report     *report.Report `json:"report,omitempty"`
}

func jobView(j Job[crawlJob]) JobView {
	return JobView{
		ID:         j.ID,
		Seed:       j.Data.Seed,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Persisted:  j.Data.StoreID != "",
		Report:     j.Data.Report,
	}
}

// crawlListItem is the lightweight API representation used by the crawl list endpoint —
// summary counts only, not the full report.
type crawlListItem struct {
	ID           string         `json:"id"`
	Seed         string         `json:"seed"`
	Status       string         `json:"status"`
	Err          string         `json:"error,omitempty"`
	StartedAt    string         `json:"started_at,omitempty"`
	FinishedAt   string         `json:"finished_at,omitempty"`
	PagesCrawled int            `json:"pages_crawled,omitempty"`
	BySeverity   map[string]int `json:"by_severity,omitempty"`
	Persisted    bool           `json:"persisted"`
}

func jobListItem(j Job[crawlJob]) crawlListItem {
	item := crawlListItem{
		ID:         j.ID,
		Seed:       j.Data.Seed,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Persisted:  j.Data.StoreID != "",
	}
	if rep := j.Data.Report; rep != nil {
		item.PagesCrawled = rep.PagesCrawled
		item.BySeverity = rep.Summary.BySeverity
	}
	return item
}
```

- [ ] **Step 5: Point `server.go` at the generic manager.** Apply these exact replacements in
  `internal/webserver/server.go`:

  1. In `type Server struct`, replace `	jobs         *jobManager` with
     `	crawls       *jobManager[crawlJob]`.
  2. In `New`, replace `		jobs:       newJobManager(),` with
     `		crawls:     newJobManager[crawlJob](),`.
  3. In `handleStartCrawl`, replace
     `	job, ok := s.jobs.start(seed, cancel, s.maxRunning)` with
     `	job, ok := s.crawls.start(crawlJob{Seed: seed}, cancel, s.maxRunning)`.
  4. In `handleStartCrawl`, replace the goroutine body

     ```go
     		rep, runErr := runner.Run(ctx, cfg, seed)
     		s.jobs.finish(job.ID, rep, runErr)
     		if runErr == nil && req.Save {
     			if id, saveErr := s.store.Save(rep); saveErr == nil {
     				s.jobs.setStoreID(job.ID, id)
     			}
     		}
     ```

     with (save first, then record report and store ID in the same locked update, so a
     poller that sees the job finish also sees it persisted):

     ```go
     		rep, runErr := runner.Run(ctx, cfg, seed)
     		// Save before finishing, so a poller that sees the job finish also sees it persisted.
     		var storeID string
     		if runErr == nil && req.Save {
     			if id, saveErr := s.store.Save(rep); saveErr == nil {
     				storeID = id
     			}
     		}
     		s.crawls.finish(job.ID, runErr, func(d *crawlJob) {
     			d.Report = rep
     			d.StoreID = storeID
     		})
     ```
  5. In `handleListCrawls`, replace `	jobs := s.jobs.list()` with `	jobs := s.crawls.list()`,
     and replace

     ```go
     		if j.StoreID != "" {
     			saved[j.StoreID] = true
     		}
     ```

     with

     ```go
     		if j.Data.StoreID != "" {
     			saved[j.Data.StoreID] = true
     		}
     ```
  6. In `handleGetCrawl`, replace `	if job, ok := s.jobs.get(id); ok {` with
     `	if job, ok := s.crawls.get(id); ok {`.
  7. In `handleCancelCrawl`, replace `	if !s.jobs.cancel(id) {` with `	if !s.crawls.cancel(id) {`.
  8. In `handleExportCrawl`, replace

     ```go
     	if job, ok := s.jobs.get(id); ok {
     		rep = job.Report
     	}
     ```

     with

     ```go
     	if job, ok := s.crawls.get(id); ok {
     		rep = job.Data.Report
     	}
     ```

  In `internal/webserver/hardening_test.go`, replace `srv.jobs.list()` with
  `srv.crawls.list()` (one occurrence, in `TestRejectsCrossOriginMutation`).

- [ ] **Step 6: Run the package tests.**
  `go test -race ./internal/webserver`
  Expected: `ok  	github.com/Patience-dot-devl/gocrawl/internal/webserver`. The new
  `TestJobManager*` tests pass and the existing API tests (start, poll, export, cancel, 429
  cap, guard) still pass.

- [ ] **Step 7: Run the full check.** `make check` — expected to end with `0 issues.`

- [ ] **Step 8: Commit.**

  ```sh
  git add internal/webserver/job.go internal/webserver/job_test.go internal/webserver/crawl_job.go \
          internal/webserver/server.go internal/webserver/hardening_test.go
  git commit -F - <<'EOF'
  refactor(webserver): make the job manager generic over job data

  jobManager[T] holds the lifecycle (status, error, times, cancel) for any kind of
  background job; crawlJob holds what is specific to a crawl. Paused jobs count toward the
  concurrency cap, with() gives locked access to a live job, and finish() applies the result
  in the same critical section as the status change. The crawl handler now saves before
  finishing, so "done" and "persisted" can no longer be observed apart.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W0-2: Route files per feature, request helpers, Server placeholders

**Files:**
- Modify (rewrite): `internal/webserver/server.go`
- Create: `internal/webserver/crawls.go`, `internal/webserver/request.go`,
  `internal/webserver/sitemap.go`, `internal/webserver/compare.go`,
  `internal/webserver/import.go`, `internal/webserver/redirects.go`,
  `internal/webserver/settings.go`, `internal/webserver/profiles.go`,
  `internal/webserver/login.go`
- Test: `internal/webserver/request_test.go`

**Interfaces:**
- Consumes: W0-1's `jobManager[T]`, `crawlJob`, `jobView`, `jobListItem`.
- Produces (contract names):
  - `func requireJSON(w http.ResponseWriter, r *http.Request) bool` — 415 unless `application/json`
  - `func parseMultipart(w http.ResponseWriter, r *http.Request, maxBytes int64) bool` — 415 unless `multipart/form-data`, 413 over `maxBytes`, 400 malformed; caller must `defer r.MultipartForm.RemoveAll()`
  - `Server` fields `crawls *jobManager[crawlJob]`, `redirects *jobManager[redirectJob]`, `logins *loginManager`, `guiSettingsPath string`
  - `type redirectJob struct{}` (redirects.go), `type loginManager struct{}` and `func newLoginManager() *loginManager` (login.go)
  - `func WithGUISettingsPath(path string) Option` (settings.go; see Self-review, gap 3)
  - Empty route methods, each in its own file: `crawlRoutes` (crawls.go, holds the existing routes), `sitemapRoutes`, `compareRoutes`, `importRoutes`, `redirectRoutes`, `settingsRoutes`, `profileRoutes`, `loginRoutes`; `routes()` calls them in that order, then registers the frontend handler.

- [ ] **Step 1: Write the failing test.** Create `internal/webserver/request_test.go`:

```go
package webserver

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireJSON(t *testing.T) {
	for _, tc := range []struct {
		contentType string
		want        bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/plain", false},
		{"application/x-www-form-urlencoded", false},
		{"", false},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(`{}`))
		if tc.contentType != "" {
			req.Header.Set("Content-Type", tc.contentType)
		}
		rec := httptest.NewRecorder()
		if got := requireJSON(rec, req); got != tc.want {
			t.Errorf("Content-Type %q: requireJSON = %v, want %v", tc.contentType, got, tc.want)
		}
		if !tc.want && rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("Content-Type %q: status = %d, want 415", tc.contentType, rec.Code)
		}
	}
}

// multipartBody builds a multipart/form-data body with one text field and one file part.
func multipartBody(t *testing.T, fileContent string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("domain", "example.com"); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", "rules.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(fileContent)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func TestParseMultipartAcceptsFormWithinLimit(t *testing.T) {
	body, ct := multipartBody(t, "source,target\n/a,/b\n")
	req := httptest.NewRequest(http.MethodPost, "/api/x", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	if !parseMultipart(rec, req, 1<<20) {
		t.Fatalf("parseMultipart = false, status %d, body %s", rec.Code, rec.Body.String())
	}
	defer req.MultipartForm.RemoveAll()
	if got := req.FormValue("domain"); got != "example.com" {
		t.Errorf("domain = %q, want example.com", got)
	}
	if _, _, err := req.FormFile("file"); err != nil {
		t.Errorf("FormFile(file): %v", err)
	}
}

func TestParseMultipartRejectsWrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if parseMultipart(rec, req, 1<<20) {
		t.Fatal("parseMultipart = true for a JSON body")
	}
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestParseMultipartRejectsOversizedBody(t *testing.T) {
	body, ct := multipartBody(t, strings.Repeat("x", 4096))
	req := httptest.NewRequest(http.MethodPost, "/api/x", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	if parseMultipart(rec, req, 1024) {
		t.Fatal("parseMultipart = true for a body over the limit")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestParseMultipartRejectsMalformedBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	rec := httptest.NewRecorder()
	if parseMultipart(rec, req, 1<<20) {
		t.Fatal("parseMultipart = true for a malformed body")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run it and watch it fail.**
  `go test ./internal/webserver -run 'TestRequireJSON|TestParseMultipart'`
  Expected: build failure `undefined: requireJSON` and `undefined: parseMultipart`.

- [ ] **Step 3: Create `internal/webserver/request.go`:**

```go
package webserver

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
)

// multipartMemory is how much of a multipart body ParseMultipartForm keeps in memory; larger
// file parts spill to temporary files, which the caller removes with
// r.MultipartForm.RemoveAll.
const multipartMemory = 8 << 20

// requireJSON writes 415 and returns false unless the request body is declared as JSON. A
// cross-origin HTML form can only send text/plain or form encodings without a CORS
// preflight, so insisting on JSON is the second half of the CSRF guard in guard.go. Every
// JSON-bodied POST, PUT, and DELETE handler calls it first.
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, errors.New("Content-Type must be application/json"))
		return false
	}
	return true
}

// parseMultipart parses a multipart/form-data upload with the body capped at maxBytes. On
// failure it writes the error response (415 for another content type, 413 over the cap, 400
// for a malformed body) and returns false. On success r.MultipartForm is populated; the
// caller must `defer r.MultipartForm.RemoveAll()` to delete parts spilled to disk. The
// origin check for these routes is the guard's, like every other non-GET route.
func parseMultipart(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "multipart/form-data" {
		writeError(w, http.StatusUnsupportedMediaType, errors.New("Content-Type must be multipart/form-data"))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(min(maxBytes, multipartMemory)); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("upload is larger than the %s limit", byteSize(maxBytes)))
			return false
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("reading multipart form: %w", err))
		return false
	}
	return true
}

// byteSize renders an upload limit for an error message: whole megabytes as "10 MB",
// anything else in bytes.
func byteSize(n int64) string {
	if n >= 1<<20 && n%(1<<20) == 0 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d bytes", n)
}
```

- [ ] **Step 4: Run the helper tests.**
  `go test ./internal/webserver -run 'TestRequireJSON|TestParseMultipart' -v`
  Expected: all five tests `PASS`.

- [ ] **Step 5: Replace `internal/webserver/server.go`** with the slimmed server (struct, `New`,
  `Handler`, `routes`, `writeJSON`, `writeError`):

```go
// Package webserver exposes gocrawl over HTTP: an API to start and inspect crawls, backed by
// the same runner.Run seam the CLI and MCP server use, plus the embedded single-page app
// (see assets.go) that drives it from a browser.
//
// Each feature registers its routes from its own file (crawls.go, sitemap.go, compare.go,
// import.go, redirects.go, settings.go, profiles.go, login.go), so features can be added
// without editing each other's handlers.
package webserver

import (
	"encoding/json"
	"net/http"

	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// Server serves the gocrawl web API and embedded frontend.
type Server struct {
	store           *store.Store
	crawls          *jobManager[crawlJob]
	redirects       *jobManager[redirectJob]
	logins          *loginManager
	guiSettingsPath string
	mux             *http.ServeMux
	allowAnyHost    bool
	maxRunning      int
}

// New builds a Server backed by st for crawl history (persisted via --save). By default it
// only answers requests whose Host header is a loopback address and runs at most
// DefaultMaxRunning crawls at once; see the Option values to change either.
func New(st *store.Store, opts ...Option) *Server {
	s := &Server{
		store:      st,
		crawls:     newJobManager[crawlJob](),
		redirects:  newJobManager[redirectJob](),
		logins:     newLoginManager(),
		maxRunning: DefaultMaxRunning,
	}
	for _, o := range opts {
		o(s)
	}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

// Handler returns the http.Handler serving the API and frontend.
func (s *Server) Handler() http.Handler {
	return s.guard(s.mux)
}

// routes registers every feature's API routes, then the frontend as the catch-all.
func (s *Server) routes() {
	s.crawlRoutes()
	s.sitemapRoutes()
	s.compareRoutes()
	s.importRoutes()
	s.redirectRoutes()
	s.settingsRoutes()
	s.profileRoutes()
	s.loginRoutes()
	s.mux.Handle("/", frontendHandler())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
```

- [ ] **Step 6: Create `internal/webserver/crawls.go`** holding every existing crawl handler,
  moved unchanged except that `handleStartCrawl` now calls `requireJSON`:

```go
package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// crawlRoutes registers the crawl job API plus the analyzer and explanation listings the
// New crawl form and the report view read.
func (s *Server) crawlRoutes() {
	s.mux.HandleFunc("GET /api/analyzers", s.handleListAnalyzers)
	s.mux.HandleFunc("GET /api/explanations", s.handleListExplanations)
	s.mux.HandleFunc("POST /api/crawls", s.handleStartCrawl)
	s.mux.HandleFunc("GET /api/crawls", s.handleListCrawls)
	s.mux.HandleFunc("GET /api/crawls/{id}", s.handleGetCrawl)
	s.mux.HandleFunc("POST /api/crawls/{id}/cancel", s.handleCancelCrawl)
	s.mux.HandleFunc("GET /api/crawls/{id}/export", s.handleExportCrawl)
}

func (s *Server) handleListAnalyzers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"analyzers": runner.ListAnalyzers()})
}

// handleListExplanations returns the same code -> what/impact/fix text the HTML report bakes
// in per issue, so the live report view can render identical explanations without duplicating
// internal/report/explanations.go in the frontend.
func (s *Server) handleListExplanations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"explanations": report.Explanations()})
}

// startCrawlRequest is the POST /api/crawls body: the shared crawl params plus web-only
// options.
type startCrawlRequest struct {
	crawlrequest.Params
	Save bool `json:"save"`
}

func (s *Server) handleStartCrawl(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var req startCrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	cfg, seed, err := req.ToConfig()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	job, ok := s.crawls.start(crawlJob{Seed: seed}, cancel, s.maxRunning)
	if !ok {
		cancel()
		writeError(w, http.StatusTooManyRequests, fmt.Errorf("%d crawls already running; wait for one to finish or cancel it", s.maxRunning))
		return
	}

	go func() {
		defer cancel()
		rep, runErr := runner.Run(ctx, cfg, seed)
		// Save before finishing, so a poller that sees the job finish also sees it persisted.
		var storeID string
		if runErr == nil && req.Save {
			if id, saveErr := s.store.Save(rep); saveErr == nil {
				storeID = id
			}
		}
		s.crawls.finish(job.ID, runErr, func(d *crawlJob) {
			d.Report = rep
			d.StoreID = storeID
		})
	}()

	writeJSON(w, http.StatusAccepted, jobView(job))
}

func (s *Server) handleListCrawls(w http.ResponseWriter, _ *http.Request) {
	jobs := s.crawls.list()
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].StartedAt.After(jobs[k].StartedAt) })

	saved := make(map[string]bool, len(jobs))
	items := make([]crawlListItem, 0, len(jobs))
	for _, j := range jobs {
		if j.Data.StoreID != "" {
			saved[j.Data.StoreID] = true
		}
		items = append(items, jobListItem(j))
	}

	entries, err := s.store.List("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, e := range entries {
		if saved[e.ID] {
			continue
		}
		items = append(items, crawlListItem{
			ID:           e.ID,
			Seed:         e.Seed,
			Status:       string(StatusDone),
			FinishedAt:   e.FinishedAt,
			PagesCrawled: e.PagesCrawled,
			BySeverity:   e.BySeverity,
			Persisted:    true,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"crawls": items})
}

func (s *Server) handleGetCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if job, ok := s.crawls.get(id); ok {
		writeJSON(w, http.StatusOK, jobView(job))
		return
	}
	rep, err := s.store.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, JobView{
		ID:        id,
		Seed:      rep.Seed,
		Status:    string(StatusDone),
		Persisted: true,
		Report:    rep,
	})
}

func (s *Server) handleCancelCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.crawls.cancel(id) {
		writeError(w, http.StatusNotFound, fmt.Errorf("no running crawl with id %q", id))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExportCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var rep *report.Report
	if job, ok := s.crawls.get(id); ok {
		rep = job.Data.Report
	}
	if rep == nil {
		loaded, err := s.store.Load(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		rep = loaded
	}
	if rep == nil {
		writeError(w, http.StatusConflict, errors.New("crawl has not finished yet"))
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	contentType, ext := exportContentType(format)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="gocrawl-report.%s"`, ext))
	if err := report.For(format).Write(w, rep); err != nil {
		writeError(w, http.StatusInternalServerError, err)
	}
}

func exportContentType(format string) (contentType, ext string) {
	switch format {
	case "csv":
		return "text/csv", "csv"
	case "html":
		return "text/html", "html"
	default:
		return "application/json", "json"
	}
}
```

- [ ] **Step 7: Create the per-feature placeholder files.** Each later track fills in its own
  file and touches nothing else in the package.

  `internal/webserver/sitemap.go`:

```go
package webserver

// sitemapRoutes registers POST /api/sitemap-preview, which runs only the sitemap discovery
// step for the New crawl form. Empty until the live-progress feature fills it in.
func (s *Server) sitemapRoutes() {}
```

  `internal/webserver/compare.go`:

```go
package webserver

// compareRoutes registers GET /api/compare, which diffs two stored crawls. Empty until the
// compare feature fills it in.
func (s *Server) compareRoutes() {}
```

  `internal/webserver/import.go`:

```go
package webserver

// importRoutes registers POST /api/reports/import, which saves an uploaded JSON report to
// the store. Empty until the report-import feature fills it in.
func (s *Server) importRoutes() {}
```

  `internal/webserver/redirects.go`:

```go
package webserver

// redirectJob is the redirect-check-specific part of a Job, run by its own jobManager with
// its own concurrency cap. The redirect-check feature fills in its fields.
type redirectJob struct{}

// redirectRoutes registers the /api/redirect-checks routes. Empty until the redirect-check
// feature fills it in.
func (s *Server) redirectRoutes() {}
```

  `internal/webserver/settings.go` (the `guiSettingsPath` field needs a writer, or the
  `unused` linter fails the build; see Self-review, gap 3):

```go
package webserver

// WithGUISettingsPath sets the file GET and PUT /api/settings read and write (gocrawl serve
// passes ~/.gocrawl/gui.json). An empty path, the default, means settings are not stored.
func WithGUISettingsPath(path string) Option {
	return func(s *Server) { s.guiSettingsPath = path }
}

// settingsRoutes registers GET and PUT /api/settings. Empty until the settings feature fills
// it in.
func (s *Server) settingsRoutes() {}
```

  `internal/webserver/profiles.go`:

```go
package webserver

// profileRoutes registers the /api/header-profiles routes. Empty until the settings feature
// fills it in.
func (s *Server) profileRoutes() {}
```

  `internal/webserver/login.go`:

```go
package webserver

// loginManager tracks interactive browser-login sessions. The browser-login feature fills
// in its fields.
type loginManager struct{}

func newLoginManager() *loginManager {
	return &loginManager{}
}

// loginRoutes registers the /api/login-sessions routes. Empty until the browser-login
// feature fills it in.
func (s *Server) loginRoutes() {}
```

- [ ] **Step 8: Run the package tests.**
  `go test -race ./internal/webserver`
  Expected: `ok`. `TestStartCrawlRequiresJSONContentType` still gets `415`, now from
  `requireJSON`.

- [ ] **Step 9: Run the full check.** `make check` — expected `0 issues.` (golangci-lint's
  `unused` accepts the placeholders because `New` sets `redirects` and `logins` and
  `WithGUISettingsPath` sets `guiSettingsPath`).

- [ ] **Step 10: Commit.**

  ```sh
  git add internal/webserver/
  git commit -F - <<'EOF'
  refactor(webserver): one route file per feature, plus request helpers

  The crawl routes move to crawls.go and routes() calls one registration method per
  feature, each in its own file, so the upcoming features (sitemap preview, compare,
  import, redirect checks, settings, header profiles, browser login) don't edit each
  other's code. requireJSON and parseMultipart give every new JSON and upload route the
  same 415/413 handling as POST /api/crawls.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W0-3: Runner options plumbing

**Files:**
- Create: `internal/runner/options.go`
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/options_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces (contract names):
  - `type RunOption func(*runSettings)`
  - `type runSettings struct{}` (no fields yet; the progress and pause tracks add theirs in `options.go`)
  - `func newRunSettings(opts []RunOption) runSettings` (skips nil options)
  - `func Run(ctx context.Context, cfg config.Config, seed string, runOpts ...RunOption) (*report.Report, error)` — the variadic parameter is named `runOpts` because `opts` already names the `crawler.Options` local inside `Run`. Callers in `cmd/gocrawl/crawl.go`, `cmd/gocrawl/interactive.go`, `internal/mcpserver/server.go`, and `internal/webserver/crawls.go` compile unchanged.

- [ ] **Step 1: Write the failing test.** Create `internal/runner/options_test.go`:

```go
package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestNewRunSettingsAppliesEveryOption(t *testing.T) {
	calls := 0
	count := RunOption(func(*runSettings) { calls++ })
	newRunSettings([]RunOption{count, nil, count})
	if calls != 2 {
		t.Fatalf("options applied %d times, want 2 (nil options are skipped)", calls)
	}
}

// TestRunAcceptsOptions: Run takes RunOptions as a variadic tail, so the CLI and MCP callers
// compile unchanged, and it applies each one exactly once per call.
func TestRunAcceptsOptions(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<html><head><title>Home</title></head><body>hello</body></html>")
	}))
	defer site.Close()

	cfg := config.Default()
	cfg.Crawl.MaxDepth = 0
	cfg.Crawl.MaxPages = 1

	calls := 0
	opt := RunOption(func(*runSettings) { calls++ })
	rep, err := Run(context.Background(), cfg, site.URL, opt)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.PagesCrawled != 1 {
		t.Errorf("PagesCrawled = %d, want 1", rep.PagesCrawled)
	}
	if calls != 1 {
		t.Errorf("option applied %d times, want 1", calls)
	}
}
```

- [ ] **Step 2: Run it and watch it fail.**
  `go test ./internal/runner -run 'TestNewRunSettings|TestRunAcceptsOptions'`
  Expected: build failure `undefined: RunOption`, `undefined: runSettings`,
  `undefined: newRunSettings`.

- [ ] **Step 3: Create `internal/runner/options.go`:**

```go
package runner

// RunOption customizes one Run call. The CLI and MCP server call Run without any; the web
// server uses options for what only it needs (live progress, pause and resume), so Run's
// signature stays the same for every caller.
type RunOption func(*runSettings)

// runSettings collects the RunOption values for one Run call. Each feature adds its own
// field here together with the RunOption constructor that sets it, never a new positional
// parameter on Run.
type runSettings struct{}

// newRunSettings applies opts in order, skipping nil entries so a caller can pass an
// optional option without a branch.
func newRunSettings(opts []RunOption) runSettings {
	var rs runSettings
	for _, o := range opts {
		if o != nil {
			o(&rs)
		}
	}
	return rs
}
```

- [ ] **Step 4: Give `Run` the variadic options.** In `internal/runner/runner.go`, replace

  ```go
  // Run performs a full crawl + analysis for the given config and seed, returning a Report.
  // The seed argument overrides cfg.Seed when non-empty.
  func Run(ctx context.Context, cfg config.Config, seed string) (*report.Report, error) {
  	if seed == "" {
  ```

  with

  ```go
  // Run performs a full crawl + analysis for the given config and seed, returning a Report.
  // The seed argument overrides cfg.Seed when non-empty. runOpts customize this one call (see
  // RunOption); the CLI and MCP server pass none.
  func Run(ctx context.Context, cfg config.Config, seed string, runOpts ...RunOption) (*report.Report, error) {
  	settings := newRunSettings(runOpts)
  	_ = settings // read by the progress and pause features once they add their fields
  	if seed == "" {
  ```

- [ ] **Step 5: Run the tests and the build.**
  `go build ./... && go test ./internal/runner -run 'TestNewRunSettings|TestRunAcceptsOptions' -v`
  Expected: `go build` succeeds (all four `runner.Run` callers unchanged) and both tests
  `PASS`.

- [ ] **Step 6: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 7: Commit.**

  ```sh
  git add internal/runner/options.go internal/runner/options_test.go internal/runner/runner.go
  git commit -F - <<'EOF'
  feat(runner): accept RunOption values in Run

  Run gains a variadic RunOption tail so the web server can ask for live progress and pause
  control without changing the call for the CLI and MCP server. runSettings has no fields
  yet; each feature adds its own next to its option constructor.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W0-4: Split the frontend API client and types per feature

**Files:**
- Create: `web/src/api/client.ts`, `web/src/api/crawls.ts`
- Move: `web/src/types.ts` -> `web/src/types/crawls.ts` (content unchanged)
- Delete: `web/src/api.ts`
- Modify: `web/src/components/CrawlForm.tsx`, `web/src/components/CrawlReport.tsx`,
  `web/src/components/History.tsx`, `web/src/components/SiteMap.tsx` (import paths only)

**Interfaces:**
- Consumes: the existing API routes.
- Produces (contract names):
  - `request<T>(path: string, init?: RequestInit): Promise<T>`
  - `postJSON<T>(path: string, body: unknown, method: string = 'POST'): Promise<T>`
  - `postForm<T>(path: string, form: FormData): Promise<T>`
  - `api/crawls.ts`: `listAnalyzers`, `listExplanations`, `startCrawl`, `listCrawls`, `getCrawl`, `cancelCrawl`, `exportURL` (same signatures as before)
  - `types/crawls.ts`: every type formerly in `types.ts`, unchanged (`CrawlStatus` is still `'running' | 'done' | 'error' | 'canceled'`; the pause track adds `'paused'`)

- [ ] **Step 1: Install the frontend toolchain once** (skip when `web/node_modules` exists):
  `cd web && npm ci`

- [ ] **Step 2: Create `web/src/api/client.ts`:**

```ts
// Shared fetch helpers for every API module (api/<feature>.ts). A non-2xx response throws an
// Error carrying the server's {"error": "..."} message when there is one, so views can show
// it as-is.

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new Error(body?.error || `${res.status} ${res.statusText}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

// postJSON sends body as JSON. The server's guard insists on application/json for JSON
// routes (a 415 otherwise), so always go through here rather than a bare fetch.
export function postJSON<T>(path: string, body: unknown, method: string = 'POST'): Promise<T> {
  return request<T>(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

// postForm sends a multipart upload. No Content-Type header: the browser sets
// multipart/form-data with the boundary itself.
export function postForm<T>(path: string, form: FormData): Promise<T> {
  return request<T>(path, { method: 'POST', body: form })
}
```

- [ ] **Step 3: Move the types and create `web/src/api/crawls.ts`.**
  `git mv web/src/types.ts web/src/types/crawls.ts` (run `mkdir -p web/src/types` first),
  then create `web/src/api/crawls.ts`:

```ts
import { postJSON, request } from './client'
import type { AnalyzerInfo, CrawlListItem, Explanation, JobView, StartCrawlParams } from '../types/crawls'

export function listAnalyzers(): Promise<{ analyzers: AnalyzerInfo[] }> {
  return request('/api/analyzers')
}

export function listExplanations(): Promise<{ explanations: Record<string, Explanation> }> {
  return request('/api/explanations')
}

export function startCrawl(params: StartCrawlParams): Promise<JobView> {
  return postJSON('/api/crawls', params)
}

export function listCrawls(): Promise<{ crawls: CrawlListItem[] }> {
  return request('/api/crawls')
}

export function getCrawl(id: string): Promise<JobView> {
  return request(`/api/crawls/${encodeURIComponent(id)}`)
}

export function cancelCrawl(id: string): Promise<void> {
  return request(`/api/crawls/${encodeURIComponent(id)}/cancel`, { method: 'POST' })
}

export function exportURL(id: string, format: 'json' | 'csv' | 'html'): string {
  return `/api/crawls/${encodeURIComponent(id)}/export?format=${format}`
}
```

  Then delete the old module: `git rm web/src/api.ts`.

- [ ] **Step 4: Update the imports in the four components.**

  ```sh
  for f in CrawlForm CrawlReport History SiteMap; do
    p="web/src/components/$f.tsx"
    sed -e "s#from '../api'#from '../api/crawls'#" -e "s#from '../types'#from '../types/crawls'#" "$p" > "$p.tmp" && mv "$p.tmp" "$p"
  done
  grep -nE "from '\.\./(api|types)" web/src/components/*.tsx
  ```

  Expected `grep` output: only `'../api/crawls'` and `'../types/crawls'` imports.

- [ ] **Step 5: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: `tsc -b` passes, `vite build` ends with `✓ built in …`, oxlint prints nothing and
  exits 0.

- [ ] **Step 6: Manual check.** `make web-build && go run ./cmd/gocrawl serve`, open
  `http://localhost:8080`, start a crawl of `https://example.com` with depth 1: it starts
  (proves `postJSON` sends `Content-Type: application/json`, otherwise the server answers
  415), the report appears, and History lists it. Stop the server with Ctrl-C.

- [ ] **Step 7: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 8: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src
  git commit -F - <<'EOF'
  refactor(web): split the API client and types per feature

  api/client.ts holds request, postJSON, and postForm; api/crawls.ts and types/crawls.ts
  hold what api.ts and types.ts held. Later features add api/<feature>.ts and
  types/<feature>.ts next to them instead of growing one shared file.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W0-5: Hash routes, stub views, App routing, wave 0 docs

**Files:**
- Create: `web/src/routes.ts`, `web/src/views/CompareView.tsx`,
  `web/src/views/RedirectsView.tsx`, `web/src/views/RedirectView.tsx`,
  `web/src/views/SettingsView.tsx`
- Modify (rewrite): `web/src/App.tsx`
- Modify: `docs/web.md`, `docs/architecture.md`, `docs/README.md` (generated), `CHANGELOG.md`

**Interfaces:**
- Consumes: W0-4's `api/crawls.ts`; existing `CrawlForm`, `CrawlReport`, `History`.
- Produces (contract names):
  - `type Route = {view:'new'} | {view:'crawls'} | {view:'crawl'; id: string} | {view:'compare'; base?: string; current?: string} | {view:'redirects'} | {view:'redirect'; id: string} | {view:'settings'}`
  - `parseHash(hash: string): Route` (`#/history` parses to `{view:'crawls'}`; unknown hashes parse to `{view:'new'}`)
  - `href(route: Route): string`
  - `navigate(route: Route): void` (addition: sets `window.location.hash = href(route)`)
  - `views/CompareView.tsx`, `views/RedirectsView.tsx`, `views/RedirectView.tsx` (`{ id: string }`), `views/SettingsView.tsx`: default-exported components rendering an `<h1>` only

  Addresses: `#/`, `#/crawls`, `#/crawl/<id>` (id URL-encoded; store IDs contain a slash),
  `#/compare?base=<ref>&current=<ref>`, `#/redirects`, `#/redirect/<id>`, `#/settings`.

- [ ] **Step 1: Create `web/src/routes.ts`:**

```ts
// Hash routes for the single-page app. There is no router dependency: parseHash turns
// location.hash into a Route and href turns a Route back into a hash, so every link and
// redirect goes through these two functions. Hash routing also keeps working when the
// desktop shell serves the UI from a different 127.0.0.1 port on every launch.

export type Route =
  | { view: 'new' }
  | { view: 'crawls' }
  | { view: 'crawl'; id: string }
  | { view: 'compare'; base?: string; current?: string }
  | { view: 'redirects' }
  | { view: 'redirect'; id: string }
  | { view: 'settings' }

// Store IDs contain a slash (host/timestamp). href encodes it; parseHash also accepts the
// raw form, which older links and the first version of this UI used.
function decode(s: string): string {
  try {
    return decodeURIComponent(s)
  } catch {
    return s
  }
}

export function parseHash(hash: string): Route {
  const clean = hash.replace(/^#\/?/, '')
  const q = clean.indexOf('?')
  const path = q === -1 ? clean : clean.slice(0, q)
  const params = new URLSearchParams(q === -1 ? '' : clean.slice(q + 1))
  const slash = path.indexOf('/')
  const head = slash === -1 ? path : path.slice(0, slash)
  const rest = slash === -1 ? '' : decode(path.slice(slash + 1))

  switch (head) {
    case 'crawls':
    case 'history': // the first version of the UI called this view History
      return { view: 'crawls' }
    case 'crawl':
      return rest ? { view: 'crawl', id: rest } : { view: 'crawls' }
    case 'compare':
      return {
        view: 'compare',
        base: params.get('base') || undefined,
        current: params.get('current') || undefined,
      }
    case 'redirects':
      return { view: 'redirects' }
    case 'redirect':
      return rest ? { view: 'redirect', id: rest } : { view: 'redirects' }
    case 'settings':
      return { view: 'settings' }
    default:
      return { view: 'new' }
  }
}

export function href(route: Route): string {
  switch (route.view) {
    case 'new':
      return '#/'
    case 'crawls':
      return '#/crawls'
    case 'crawl':
      return `#/crawl/${encodeURIComponent(route.id)}`
    case 'compare': {
      const params = new URLSearchParams()
      if (route.base) params.set('base', route.base)
      if (route.current) params.set('current', route.current)
      const qs = params.toString()
      return qs ? `#/compare?${qs}` : '#/compare'
    }
    case 'redirects':
      return '#/redirects'
    case 'redirect':
      return `#/redirect/${encodeURIComponent(route.id)}`
    case 'settings':
      return '#/settings'
  }
}

// navigate moves to route; App re-renders from the hashchange event.
export function navigate(route: Route): void {
  window.location.hash = href(route)
}
```

- [ ] **Step 2: Create the four stub views.**

  `web/src/views/CompareView.tsx`:

```tsx
export default function CompareView() {
  return <h1>Compare crawls</h1>
}
```

  `web/src/views/RedirectsView.tsx`:

```tsx
export default function RedirectsView() {
  return <h1>Check redirects</h1>
}
```

  `web/src/views/RedirectView.tsx`:

```tsx
export default function RedirectView({ id }: { id: string }) {
  return <h1>Redirect check {id}</h1>
}
```

  `web/src/views/SettingsView.tsx`:

```tsx
export default function SettingsView() {
  return <h1>Settings</h1>
}
```

- [ ] **Step 3: Replace `web/src/App.tsx`** (routes through `routes.ts`; the header nav lists
  all five entries until VIS-3 replaces it with the rail):

```tsx
import { useEffect, useState } from 'react'
import CrawlForm from './components/CrawlForm'
import CrawlReport from './components/CrawlReport'
import History from './components/History'
import { href, navigate, parseHash, type Route } from './routes'
import CompareView from './views/CompareView'
import RedirectView from './views/RedirectView'
import RedirectsView from './views/RedirectsView'
import SettingsView from './views/SettingsView'

// The navigation entries, in order, and which routes count as "inside" each one.
const NAV: { label: string; route: Route; owns: Route['view'][] }[] = [
  { label: 'New crawl', route: { view: 'new' }, owns: ['new'] },
  { label: 'Crawls', route: { view: 'crawls' }, owns: ['crawls', 'crawl'] },
  { label: 'Compare', route: { view: 'compare' }, owns: ['compare'] },
  { label: 'Redirects', route: { view: 'redirects' }, owns: ['redirects', 'redirect'] },
  { label: 'Settings', route: { view: 'settings' }, owns: ['settings'] },
]

export default function App() {
  const [route, setRoute] = useState<Route>(() => parseHash(window.location.hash))

  useEffect(() => {
    const onHashChange = () => setRoute(parseHash(window.location.hash))
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  return (
    <div className="app">
      <header className="app-header">
        <h1>gocrawl</h1>
        <nav>
          {NAV.map((item) => (
            <a key={item.label} href={href(item.route)} className={item.owns.includes(route.view) ? 'active' : ''}>
              {item.label}
            </a>
          ))}
        </nav>
      </header>

      <main>
        {route.view === 'new' && <CrawlForm onStarted={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawls' && <History onSelect={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawl' && <CrawlReport key={route.id} id={route.id} />}
        {route.view === 'compare' && <CompareView />}
        {route.view === 'redirects' && <RedirectsView />}
        {route.view === 'redirect' && <RedirectView key={route.id} id={route.id} />}
        {route.view === 'settings' && <SettingsView />}
      </main>
    </div>
  )
}
```

- [ ] **Step 4: Build and lint (the route test).** `cd web && npm run build && npm run lint`
  Expected: `tsc -b` passes (it checks that `href` covers every `Route` variant, since a
  missing `case` makes the function's return type include `undefined`), `✓ built`, oxlint
  silent.

- [ ] **Step 5: Manual check.** `make web-build && go run ./cmd/gocrawl serve`, then in the
  browser:
  - `http://localhost:8080/#/history` shows the crawl list and highlights "Crawls".
  - `#/compare`, `#/redirects`, `#/redirect/abc`, `#/settings` show their headings
    ("Compare crawls", "Check redirects", "Redirect check abc", "Settings").
  - Starting a crawl lands on `#/crawl/<id>`; after it finishes with "Save" on, clicking it in
    Crawls opens `#/crawl/<host>%2F<timestamp>` and loads the saved report.
  - `#/nonsense` shows the New crawl form.

- [ ] **Step 6: Update the docs.**

  `docs/architecture.md`: set `updated: 2026-10-13` in the frontmatter and replace the
  paragraph

  ```markdown
  [`runner.Run`](../internal/runner/runner.go) is the single orchestration point used by both
  the CLI (`gocrawl crawl`) and the MCP server. It compiles the config into
  `crawler.Options`, builds the engine and analyzer registry, runs the crawl, runs the selected
  analyzers over the result, and hands everything to the report builder.
  ```

  with

  ```markdown
  [`runner.Run`](../internal/runner/runner.go) is the single orchestration point used by the
  CLI (`gocrawl crawl`), the MCP server, and the web API. It compiles the config into
  `crawler.Options`, builds the engine and analyzer registry, runs the crawl, runs the selected
  analyzers over the result, and hands everything to the report builder. A caller that needs
  more than the finished report passes `RunOption` values
  ([`internal/runner/options.go`](../internal/runner/options.go)) instead of new parameters;
  the CLI and MCP server pass none.
  ```

  `docs/web.md`: set `updated: 2026-10-13`, then make three replacements.

  1. Replace

     ```markdown
     ## What the UI does

     Three views, all against the API below:
     ```

     with

     ```markdown
     ## What the UI does

     The navigation lists New crawl, Crawls, Compare, Redirects, and Settings. Compare,
     Redirects, and Settings are placeholders until those features land. The working views, all
     against the API below:
     ```

  2. Replace

     ```markdown
     - **History** — every crawl saved to the store (`save: true` on start), reopen-able as a
       report.
     ```

     with

     ```markdown
     - **Crawls** — every crawl saved to the store (`save: true` on start), reopen-able as a
       report. Its old address, `#/history`, still opens it.
     ```

  3. Insert this section directly above `## API`:

     ```markdown
     ## Source layout

     The frontend in [`web/src`](../web/src) is split so features can be added side by side:

     - [`routes.ts`](../web/src/routes.ts) is the hash router: `parseHash` and `href` convert
       between `location.hash` and a `Route`. The addresses are `#/` (New crawl), `#/crawls`,
       `#/crawl/<id>`, `#/compare?base=<ref>&current=<ref>`, `#/redirects`, `#/redirect/<id>`,
       and `#/settings`.
     - [`api/client.ts`](../web/src/api/client.ts) holds the fetch helpers (`request`,
       `postJSON`, `postForm`). Each feature has its own `api/<feature>.ts` and
       `types/<feature>.ts`, starting with [`api/crawls.ts`](../web/src/api/crawls.ts) and
       [`types/crawls.ts`](../web/src/types/crawls.ts).
     - `views/` holds one component per top-level view; `components/` holds the pieces views
       share.

     The server side in [`internal/webserver`](../internal/webserver) follows the same split: each
     feature registers its routes from its own file (`crawls.go`, `sitemap.go`, `compare.go`,
     `import.go`, `redirects.go`, `settings.go`, `profiles.go`, `login.go`). Crawls run as jobs in
     a generic in-memory job manager ([`job.go`](../internal/webserver/job.go)) that later job
     kinds share, each kind with its own concurrency cap. A paused job keeps its slot under the
     cap.
     ```

  Then regenerate the frontmatter links and the index: `make docs`. Expected: `ok` and new
  entries in the `sources:` lists of both documents (for example
  `"../internal/runner/options.go"`, `"../web/src/routes.ts"`).

  `CHANGELOG.md`: under `## [Unreleased]`, in the existing `### Changed` list, add as its
  first bullet:

  ```markdown
  - The web UI's History view is now called Crawls and lives at `#/crawls` (`#/history` still
    works). The navigation also lists Compare, Redirects, and Settings, which are placeholders
    for now.
  ```

- [ ] **Step 7: Run the full check.** `make check` — expected `0 issues.` (the docs-drift tests
  in `internal/repocheck` confirm every new relative link resolves and the frontmatter is
  current).

- [ ] **Step 8: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src docs CHANGELOG.md
  git commit -F - <<'EOF'
  feat(web): hash routes for every view, with stub pages for the new ones

  routes.ts owns parsing and building hash addresses, including #/history as an alias of
  #/crawls and URL-encoded store IDs. Compare, Redirects, Redirect, and Settings get stub
  views so their tracks only fill in their own files.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

## Wave 1: visual system and backends

### Track: visual system

#### Task W1-VIS-1: Design tokens, theme switch, contrast guard

**Files:**
- Create: `web/src/styles/tokens.css`, `web/src/theme.ts`, `web/scripts/contrast.mjs`
- Modify: `web/src/main.tsx`, `web/package.json`, `web/README.md`

**Interfaces:**
- Consumes: nothing.
- Produces (contract names):
  - `styles/tokens.css`: on `:root, :root[data-theme='dark']` (default), `:root[data-theme='light']`, and `:root[data-theme='system']` inside `@media (prefers-color-scheme: light)`: `--ground --surface --raised --rule --field --ink --ink-2 --s2 --s3 --s4 --s5 --violet --on-fill --todo --cut --notice-bg`, plus `color-scheme`
  - `theme.ts`: `type Theme = 'dark' | 'light' | 'system'`, `applyTheme(t: Theme): void`, and (addition) `isTheme(value: unknown): value is Theme`
  - `scripts/contrast.mjs`, run by `npm run lint` after oxlint; exit 1 on any failure

- [ ] **Step 1: Write the guard first.** Create `web/scripts/contrast.mjs`:

```js
// Contrast guard for the web UI's design tokens (run by `npm run lint`).
//
// Reads src/styles/tokens.css, pulls the custom properties of the dark, light, and
// system-light theme blocks, and fails (exit 1) when any foreground/background pair the UI
// relies on drops below its WCAG 2.2 AA threshold: 4.5:1 for text, 3:1 for non-text marks
// (input borders, outline ticks). It also fails when the 'system' block under
// prefers-color-scheme: light drifts from the light block, since CSS can't share one.
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const TEXT = 4.5
const NON_TEXT = 3

// [foreground, background, minimum ratio, what it is]
const PAIRS = [
  ['--ink', '--surface', TEXT, 'text'],
  ['--ink-2', '--surface', TEXT, 'secondary text'],
  ['--field', '--surface', NON_TEXT, 'input borders'],
  ['--s2', '--surface', TEXT, '2xx green'],
  ['--s3', '--surface', TEXT, '3xx yellow'],
  ['--s4', '--surface', TEXT, '4xx dark orange'],
  ['--s5', '--surface', TEXT, '5xx red'],
  ['--violet', '--surface', TEXT, 'violet (UI)'],
  ['--todo', '--surface', NON_TEXT, 'not fetched outline'],
  ['--cut', '--surface', NON_TEXT, 'past budget outline'],
  ['--on-fill', '--s2', TEXT, 'text on green (Start, Resume)'],
  ['--on-fill', '--s5', TEXT, 'text on red (Stop)'],
  ['--on-fill', '--violet', TEXT, 'text on violet (Save, Check redirects)'],
  ['--ink', '--ground', TEXT, 'text on the page ground'],
  ['--ink-2', '--ground', TEXT, 'secondary text on the page ground'],
  ['--violet', '--ground', TEXT, 'links on the page ground'],
  ['--ink', '--raised', TEXT, 'active rail item'],
  ['--ink-2', '--raised', TEXT, 'secondary text on raised controls'],
  ['--ink', '--notice-bg', TEXT, 'notice text'],
  ['--s3', '--notice-bg', NON_TEXT, 'warning notice border'],
  ['--s2', '--ground', TEXT, '2xx green on the page ground'],
  ['--s3', '--ground', TEXT, '3xx yellow on the page ground'],
  ['--s4', '--ground', TEXT, '4xx dark orange on the page ground'],
  ['--s5', '--ground', TEXT, '5xx red on the page ground'],
]

const here = dirname(fileURLToPath(import.meta.url))
const tokensPath = join(here, '..', 'src', 'styles', 'tokens.css')

let css
try {
  css = readFileSync(tokensPath, 'utf8')
} catch (err) {
  console.error(`contrast: cannot read ${tokensPath}: ${err.message}`)
  process.exit(1)
}
css = css.replace(/\/\*[\s\S]*?\*\//g, '')

// Innermost rules only: a selector run with no braces, then a body with no braces. The
// @media wrapper's own brace is skipped, so the system block is found by its selector.
function themeBlock(attr) {
  const rule = /([^{}]+)\{([^{}]*)\}/g
  for (const m of css.matchAll(rule)) {
    const selectors = m[1].split(',').map((s) => s.trim())
    if (!selectors.includes(`:root[data-theme='${attr}']`)) continue
    const vars = {}
    for (const decl of m[2].matchAll(/(--[a-z0-9-]+)\s*:\s*([^;]+);/g)) {
      vars[decl[1]] = decl[2].trim().toLowerCase()
    }
    return vars
  }
  return null
}

function luminance(hex) {
  const m = /^#([0-9a-f]{6})$/.exec(hex)
  if (!m) throw new Error(`not a #rrggbb color: ${hex}`)
  const [r, g, b] = [0, 2, 4].map((i) => {
    const c = parseInt(m[1].slice(i, i + 2), 16) / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function ratio(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

const failures = []
const themes = { dark: themeBlock('dark'), light: themeBlock('light') }

for (const [name, vars] of Object.entries(themes)) {
  if (!vars) {
    failures.push(`${name}: no :root[data-theme='${name}'] block in tokens.css`)
    continue
  }
  for (const [fg, bg, min, what] of PAIRS) {
    if (!vars[fg] || !vars[bg]) {
      failures.push(`${name}: ${fg} or ${bg} is not defined`)
      continue
    }
    let r
    try {
      r = ratio(vars[fg], vars[bg])
    } catch (err) {
      failures.push(`${name}: ${err.message}`)
      continue
    }
    const line = `${name.padEnd(5)} ${what.padEnd(40)} ${fg} on ${bg}: ${r.toFixed(2)} (min ${min})`
    if (r < min) failures.push(line)
    else console.log(`ok    ${line}`)
  }
}

const system = themeBlock('system')
if (!system) {
  failures.push("system: no :root[data-theme='system'] block in tokens.css")
} else if (themes.light) {
  const keys = new Set([...Object.keys(themes.light), ...Object.keys(system)])
  for (const k of keys) {
    if (themes.light[k] !== system[k]) {
      failures.push(`system: ${k} is ${system[k] ?? 'missing'}, light block has ${themes.light[k] ?? 'missing'}`)
    }
  }
}

if (failures.length > 0) {
  for (const f of failures) console.error(`FAIL  ${f}`)
  process.exit(1)
}
console.log('contrast: all token pairs pass WCAG 2.2 AA')
```

- [ ] **Step 2: Run it and watch it fail.** `cd web && node scripts/contrast.mjs; echo "exit $?"`
  Expected: `contrast: cannot read …/web/src/styles/tokens.css: ENOENT: no such file or directory` and `exit 1`.

- [ ] **Step 3: Create `web/src/styles/tokens.css`** with the spec's values (plus the
  mockup's `--raised`, `--rule`, `--notice-bg`):

```css
/*
 * Design tokens for the gocrawl web UI. Color means HTTP status (--s2..--s5); violet means
 * "this is interface". Dark is the default: a bare :root (no data-theme yet) gets it too.
 * theme.ts sets data-theme on <html>; 'system' follows prefers-color-scheme.
 *
 * scripts/contrast.mjs parses this file (npm run lint): it checks the WCAG 2.2 AA pairs from
 * the design spec for both themes and that the 'system' light block matches the light block.
 * Keep one declaration per line in the form `--name: #rrggbb;`.
 */

:root,
:root[data-theme='dark'] {
  color-scheme: dark;
  --ground: #12161d;
  --surface: #1a2029;
  --raised: #212834;
  --rule: #2c3542;
  --field: #64728a;
  --ink: #e6eaf0;
  --ink-2: #9aa5b4;
  --s2: #4fae6e;
  --s3: #e3c13b;
  --s4: #e07b24;
  --s5: #f45d5a;
  --violet: #a78bfa;
  --on-fill: #12161d;
  --todo: #6b7a8f;
  --cut: #a27a1f;
  --notice-bg: #241f12;
}

:root[data-theme='light'] {
  color-scheme: light;
  --ground: #f2f3f7;
  --surface: #ffffff;
  --raised: #eceaf6;
  --rule: #d9dde4;
  --field: #7b8696;
  --ink: #1b2330;
  --ink-2: #4f5a6b;
  --s2: #1f7a3f;
  --s3: #8a6a00;
  --s4: #b4500c;
  --s5: #c8221f;
  --violet: #6d3fd6;
  --on-fill: #ffffff;
  --todo: #7b8696;
  --cut: #8a6a00;
  --notice-bg: #fff8e1;
}

@media (prefers-color-scheme: light) {
  :root[data-theme='system'] {
    color-scheme: light;
    --ground: #f2f3f7;
    --surface: #ffffff;
    --raised: #eceaf6;
    --rule: #d9dde4;
    --field: #7b8696;
    --ink: #1b2330;
    --ink-2: #4f5a6b;
    --s2: #1f7a3f;
    --s3: #8a6a00;
    --s4: #b4500c;
    --s5: #c8221f;
    --violet: #6d3fd6;
    --on-fill: #ffffff;
    --todo: #7b8696;
    --cut: #8a6a00;
    --notice-bg: #fff8e1;
  }
}
```

- [ ] **Step 4: Run the guard.** `cd web && node scripts/contrast.mjs; echo "exit $?"`
  Expected: 48 `ok` lines (24 per theme) whose ratios match the spec's table (for example
  `dark  text  --ink on --surface: 13.56`, `light 3xx yellow --s3 on --surface: 5.07`,
  `dark  text on violet … --on-fill on --violet: 6.66`), then
  `contrast: all token pairs pass WCAG 2.2 AA` and `exit 0`.

- [ ] **Step 5: Prove the guard bites, then undo.**

  ```sh
  cd web
  cp src/styles/tokens.css /tmp/tokens.bak
  sed -e 's/--ink-2: #9aa5b4;/--ink-2: #555b66;/' /tmp/tokens.bak > src/styles/tokens.css
  node scripts/contrast.mjs; echo "exit $?"   # expect FAIL lines for --ink-2 and exit 1
  sed -e 's/    --violet: #6d3fd6;/    --violet: #6d3fd7;/' /tmp/tokens.bak > src/styles/tokens.css   # 4-space indent: only the system block
  node scripts/contrast.mjs; echo "exit $?"   # expect "FAIL  system: --violet is #6d3fd7, light block has #6d3fd6" and exit 1
  cp /tmp/tokens.bak src/styles/tokens.css
  node scripts/contrast.mjs > /dev/null; echo "exit $?"   # expect exit 0
  ```

- [ ] **Step 6: Create `web/src/theme.ts`:**

```ts
// Theme switching. styles/tokens.css defines the palettes on :root[data-theme=...]: dark (also
// the default before any attribute is set), light, and system, which follows the operating
// system's prefers-color-scheme. This module only sets the attribute. The Settings view
// stores the choice server-side (gui.json), never in localStorage: the desktop shell serves
// the UI from a different port on every launch, so localStorage would start empty each time.

export type Theme = 'dark' | 'light' | 'system'

export function isTheme(value: unknown): value is Theme {
  return value === 'dark' || value === 'light' || value === 'system'
}

export function applyTheme(t: Theme): void {
  document.documentElement.dataset.theme = t
}
```

- [ ] **Step 7: Load the tokens and apply the default theme.** Replace `web/src/main.tsx` with:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/tokens.css'
import './index.css'
import App from './App.tsx'
import { applyTheme } from './theme'

// Dark until the Settings feature loads the saved theme from the server.
applyTheme('dark')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 8: Run the guard from `npm run lint`.** In `web/package.json` replace
  `"lint": "oxlint",` with `"lint": "oxlint && node scripts/contrast.mjs",`. In
  `web/README.md`, in the command block, add after the `npm run build` line:

  ```sh
  npm run lint     # oxlint, then the WCAG contrast check on src/styles/tokens.css
  ```

- [ ] **Step 9: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: `✓ built`, then the contrast report ending
  `contrast: all token pairs pass WCAG 2.2 AA`, exit 0.

- [ ] **Step 10: Manual check.** `make web-build && go run ./cmd/gocrawl serve`; in the
  browser console `document.documentElement.dataset.theme` returns `"dark"`. (The old
  stylesheet still paints the page; the new look arrives in VIS-3.)

- [ ] **Step 11: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 12: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src/styles/tokens.css web/src/theme.ts web/src/main.tsx web/scripts/contrast.mjs web/package.json web/README.md
  git commit -F - <<'EOF'
  feat(web): design tokens for dark, light, and system themes, with a contrast guard

  tokens.css carries the spec's palette (dark by default); theme.ts switches it through
  data-theme on <html>. scripts/contrast.mjs, run by npm run lint, fails when any text or
  control pair in either theme drops below WCAG 2.2 AA, or when the system block drifts
  from the light block.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W1-VIS-2: Bundled Atkinson Hyperlegible fonts

**Files:**
- Create: `web/src/fonts/AtkinsonHyperlegibleNext.woff2`,
  `web/src/fonts/AtkinsonHyperlegibleMono.woff2`, `web/src/fonts/OFL.txt`,
  `web/src/styles/fonts.css`
- Modify: `web/src/main.tsx`, `web/src/index.css` (one line, until VIS-3 removes it)

**Interfaces:**
- Consumes: nothing.
- Produces: `@font-face` families `'Atkinson Hyperlegible Next'` and
  `'Atkinson Hyperlegible Mono'` (variable, weights 200–800), and the custom properties
  `--font-text`, `--font-mono` on `:root` (addition to the contract's token list).

- [ ] **Step 1: Download the official variable fonts and license, and convert to WOFF2.** The
  source is the Google Fonts repository, pinned to a revision. Requires `curl` and Python 3.

  ```sh
  REV=95f4904fc8bcf26d3420fe315560c96417c6dec7
  BASE="https://raw.githubusercontent.com/google/fonts/$REV/ofl"
  T=$(mktemp -d)
  curl -fsSL -o "$T/next.ttf" "$BASE/atkinsonhyperlegiblenext/AtkinsonHyperlegibleNext%5Bwght%5D.ttf"
  curl -fsSL -o "$T/mono.ttf" "$BASE/atkinsonhyperlegiblemono/AtkinsonHyperlegibleMono%5Bwght%5D.ttf"
  curl -fsSL -o "$T/OFL-next.txt" "$BASE/atkinsonhyperlegiblenext/OFL.txt"
  curl -fsSL -o "$T/OFL-mono.txt" "$BASE/atkinsonhyperlegiblemono/OFL.txt"
  shasum -a 256 "$T/next.ttf" "$T/mono.ttf"
  ```

  Expected checksums:
  `5a455d1cfa099b601ab70751bb9673e8fe1854dc4500c80e1a220d0d75e31745` (next.ttf) and
  `5ce8b1698d1ded7dff2178c1a3ad159470085a58ea239e8b2cb88f4fb4a6f646` (mono.ttf). Stop if
  they differ.

  ```sh
  diff <(tail -n +2 "$T/OFL-next.txt") <(tail -n +2 "$T/OFL-mono.txt") && echo "license bodies identical"
  python3 -m venv "$T/venv" && "$T/venv/bin/pip" install --quiet fonttools brotli
  mkdir -p web/src/fonts
  "$T/venv/bin/fonttools" ttLib.woff2 compress -o web/src/fonts/AtkinsonHyperlegibleNext.woff2 "$T/next.ttf"
  "$T/venv/bin/fonttools" ttLib.woff2 compress -o web/src/fonts/AtkinsonHyperlegibleMono.woff2 "$T/mono.ttf"
  { head -n 1 "$T/OFL-next.txt"; head -n 1 "$T/OFL-mono.txt"; tail -n +2 "$T/OFL-next.txt"; } > web/src/fonts/OFL.txt
  rm -rf "$T"
  ls -l web/src/fonts && head -3 web/src/fonts/OFL.txt
  ```

  Expected: `license bodies identical`; two WOFF2 files of about 48 KB (Next) and 26 KB
  (Mono); `OFL.txt` starts with the two copyright lines (Next, then Mono) followed by the
  shared SIL Open Font License 1.1 text.

- [ ] **Step 2: Create `web/src/styles/fonts.css`:**

```css
/*
 * Atkinson Hyperlegible Next (text) and Atkinson Hyperlegible Mono (URLs, status codes),
 * bundled so the UI never loads a font from a CDN and works offline in the desktop shell.
 * They keep 0/O and 1/l/I distinct, which matters when reading URLs. Both are variable fonts:
 * one file covers weights 200 to 800. Converted to WOFF2 from google/fonts at revision
 * 95f4904fc8bcf26d3420fe315560c96417c6dec7 (ofl/atkinsonhyperlegiblenext,
 * ofl/atkinsonhyperlegiblemono). SIL Open Font License 1.1: see ../fonts/OFL.txt.
 */

@font-face {
  font-family: 'Atkinson Hyperlegible Next';
  src: url('../fonts/AtkinsonHyperlegibleNext.woff2') format('woff2');
  font-weight: 200 800;
  font-style: normal;
  font-display: swap;
}

@font-face {
  font-family: 'Atkinson Hyperlegible Mono';
  src: url('../fonts/AtkinsonHyperlegibleMono.woff2') format('woff2');
  font-weight: 200 800;
  font-style: normal;
  font-display: swap;
}

:root {
  --font-text: 'Atkinson Hyperlegible Next', system-ui, sans-serif;
  --font-mono: 'Atkinson Hyperlegible Mono', ui-monospace, monospace;
}
```

- [ ] **Step 3: Load it.** In `web/src/main.tsx`, replace
  `import './styles/tokens.css'` with the two lines

  ```tsx
  import './styles/tokens.css'
  import './styles/fonts.css'
  ```

  and make the old stylesheet use the bundled face until VIS-3 replaces it:

  ```sh
  p=web/src/index.css
  sed -e "s#  font: var(--fs-body-lg)/1.5 system-ui, 'Segoe UI', Roboto, sans-serif;#  font: var(--fs-body-lg)/1.5 var(--font-text);#" "$p" > "$p.tmp" && mv "$p.tmp" "$p"
  sed -n 23p "$p"   # expect:   font: var(--fs-body-lg)/1.5 var(--font-text);
  ```

- [ ] **Step 4: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: the build lists `assets/AtkinsonHyperlegibleNext-<hash>.woff2` (about 48 kB) and
  `assets/AtkinsonHyperlegibleMono-<hash>.woff2` (about 26 kB), then `✓ built`; lint passes.

- [ ] **Step 5: Manual check.** `make web-build && go run ./cmd/gocrawl serve`; with the
  browser's network panel open, reload: both `.woff2` files load from
  `localhost:8080/assets/`, nothing from a font CDN. The inspector's "Rendered fonts" for body
  text shows "Atkinson Hyperlegible Next". Then disconnect from the network and reload: the
  text still renders in Atkinson Hyperlegible.

- [ ] **Step 6: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 7: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src/fonts web/src/styles/fonts.css web/src/main.tsx web/src/index.css
  git commit -F - <<'EOF'
  feat(web): bundle Atkinson Hyperlegible Next and Mono

  Variable WOFF2 builds of both families, converted from google/fonts at a pinned
  revision, ship under web/src/fonts with their SIL OFL license, so the UI never loads a
  font from a CDN and works offline in the desktop shell.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W1-VIS-3: Base styles and the navigation rail

**Files:**
- Create: `web/src/styles/base.css`, `web/src/components/Rail.tsx`,
  `web/src/components/Rail.css`
- Modify (rewrite): `web/src/App.tsx`
- Modify: `web/src/main.tsx`, `web/src/index.css`

**Interfaces:**
- Consumes: tokens (VIS-1), fonts (VIS-2), `href` and `Route` (W0-5).
- Produces: `Rail` (`components/Rail.tsx`, props `{ current: Route }`); global classes
  `.layout`, `.layout-main`, `.muted`, `.num`, `.mono`, `.visually-hidden`; global focus ring
  (2px `--ink` outline); form-control and table base styles.

- [ ] **Step 1: Create `web/src/styles/base.css`:**

```css
/*
 * Global element styles and the page layout, on the design tokens (tokens.css) and bundled
 * fonts (fonts.css). Component styles live next to their component (components/.../Name.css)
 * and use the same tokens. Sentence case everywhere: no text-transform, no all-caps labels.
 */

*,
*::before,
*::after {
  box-sizing: border-box;
}

html {
  background: var(--ground);
}

body {
  margin: 0;
  font: 15px/1.5 var(--font-text);
  color: var(--ink);
  background: var(--ground);
}

h1,
h2,
h3 {
  color: var(--ink);
  line-height: 1.25;
  letter-spacing: -0.01em;
}

h1 {
  font-size: 24px;
  font-weight: 700;
  margin: 0 0 16px;
}

h2 {
  font-size: 18px;
  font-weight: 700;
  margin: 24px 0 10px;
}

h3 {
  font-size: 15px;
  font-weight: 700;
  margin: 16px 0 8px;
}

a {
  color: var(--violet);
  text-underline-offset: 2px;
}

code,
kbd,
pre,
.mono {
  font-family: var(--font-mono);
  font-size: 0.92em;
}

:focus-visible {
  outline: 2px solid var(--ink);
  outline-offset: 2px;
}

input,
select,
textarea,
button {
  font: inherit;
  color: inherit;
}

input[type='text'],
input[type='number'],
input[type='password'],
input[type='search'],
input[type='url'],
select,
textarea {
  min-height: 32px;
  padding: 6px 8px;
  color: var(--ink);
  background: var(--surface);
  border: 1px solid var(--field);
  border-radius: 5px;
}

textarea {
  resize: vertical;
}

::placeholder {
  color: var(--ink-2);
  opacity: 1;
}

input[type='checkbox'],
input[type='radio'] {
  width: 16px;
  height: 16px;
  margin: 0;
  accent-color: var(--violet);
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}

th {
  text-align: left;
  font-weight: 600;
  color: var(--ink-2);
}

.muted {
  color: var(--ink-2);
}

.num {
  font-variant-numeric: tabular-nums;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
  border: 0;
}

/* Page layout: the navigation rail on the left, the current view on the right. */
.layout {
  display: grid;
  grid-template-columns: 168px minmax(0, 1fr);
  min-height: 100vh;
}

.layout-main {
  min-width: 0;
  max-width: 1200px;
  padding: 22px 28px 48px;
}

@media (max-width: 720px) {
  .layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .layout-main {
    padding: 16px;
  }
}
```

- [ ] **Step 2: Strip the old global rules from `web/src/index.css` and alias its variables.**
  The old file's lines 1–129 are its `:root` palette, element styles, `.app` header and
  `.card`; lines 245–260 are the global `button` rules. Check the anchors, then cut them:

  ```sh
  p=web/src/index.css
  sed -n '1p;128p;245p;259p;261p' "$p"
  # expect, one per line:  ":root {"  "}"  "button {"  "}"  ".error {"
  sed -e '245,260d' -e '1,129d' "$p" > "$p.tmp" && mv "$p.tmp" "$p"
  ```

  Then prepend this block (create it as `/tmp/alias.css` and run
  `cat /tmp/alias.css web/src/index.css > /tmp/index.css && mv /tmp/index.css web/src/index.css`):

  ```css
/*
 * Transitional: maps the first UI's variable names onto the design tokens
 * (styles/tokens.css) until each view is restyled with its own stylesheet. This file is
 * deleted in the last visual-system task.
 */
:root {
  --text: var(--ink-2);
  --text-h: var(--ink);
  --bg: var(--ground);
  --card-bg: var(--surface);
  --border: var(--rule);
  --accent: var(--violet);
  --accent-2: var(--violet);
  --error: var(--s5);
  --warning: var(--s3);
  --info: var(--ink-2);
  --success: var(--s2);
  --shadow: none;
  --fs-caption: 13px;
  --fs-body: 14px;
  --fs-body-lg: 15px;
  --fs-h3: 15px;
  --fs-h2: 18px;
  --fs-stat: 28px;
}
  ```

  `grep -nE '^(button|\.app|\.card|h2)' web/src/index.css` must print nothing.

- [ ] **Step 3: Create `web/src/components/Rail.tsx`:**

```tsx
import { href, type Route } from '../routes'
import './Rail.css'

// The navigation entries, in order, and which views count as "inside" each one (a single
// crawl belongs to Crawls, a single redirect check to Redirects).
const ITEMS: { label: string; route: Route; owns: Route['view'][] }[] = [
  { label: 'New crawl', route: { view: 'new' }, owns: ['new'] },
  { label: 'Crawls', route: { view: 'crawls' }, owns: ['crawls', 'crawl'] },
  { label: 'Compare', route: { view: 'compare' }, owns: ['compare'] },
  { label: 'Redirects', route: { view: 'redirects' }, owns: ['redirects', 'redirect'] },
  { label: 'Settings', route: { view: 'settings' }, owns: ['settings'] },
]

export default function Rail({ current }: { current: Route }) {
  return (
    <nav className="rail" aria-label="Main">
      <a className="rail-mark" href={href({ view: 'new' })}>
        gocrawl
      </a>
      <ul>
        {ITEMS.map((item) => {
          const active = item.owns.includes(current.view)
          return (
            <li key={item.label}>
              <a
                href={href(item.route)}
                className={active ? 'rail-item rail-item-on' : 'rail-item'}
                aria-current={active ? 'page' : undefined}
              >
                {item.label}
              </a>
            </li>
          )
        })}
      </ul>
    </nav>
  )
}
```

- [ ] **Step 4: Create `web/src/components/Rail.css`:**

```css
/* Left navigation rail. Violet marks the active item; focus is the global 2px ink ring,
   pulled inside so the rail's edge doesn't clip it. */
.rail {
  position: sticky;
  top: 0;
  align-self: start;
  min-height: 100vh;
  padding: 18px 0;
  background: var(--surface);
  border-right: 1px solid var(--rule);
}

.rail-mark {
  display: block;
  padding: 0 18px 18px;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--ink);
  text-decoration: none;
}

.rail ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.rail-item {
  display: block;
  padding: 7px 18px;
  color: var(--ink-2);
  text-decoration: none;
  border-left: 3px solid transparent;
}

.rail-item:hover {
  color: var(--ink);
}

.rail-item-on {
  font-weight: 600;
  color: var(--ink);
  background: var(--raised);
  border-left-color: var(--violet);
}

.rail a:focus-visible {
  outline-offset: -4px;
}

@media (max-width: 720px) {
  .rail {
    position: static;
    min-height: 0;
    padding: 10px 0 0;
    border-right: 0;
    border-bottom: 1px solid var(--rule);
  }

  .rail-mark {
    padding-bottom: 8px;
  }

  .rail ul {
    display: flex;
    flex-wrap: wrap;
  }

  .rail-item {
    border-left: 0;
    border-bottom: 3px solid transparent;
  }

  .rail-item-on {
    border-bottom-color: var(--violet);
  }
}
```

- [ ] **Step 5: Replace `web/src/App.tsx`** (rail plus main column):

```tsx
import { useEffect, useState } from 'react'
import CrawlForm from './components/CrawlForm'
import CrawlReport from './components/CrawlReport'
import History from './components/History'
import Rail from './components/Rail'
import { navigate, parseHash, type Route } from './routes'
import CompareView from './views/CompareView'
import RedirectView from './views/RedirectView'
import RedirectsView from './views/RedirectsView'
import SettingsView from './views/SettingsView'

export default function App() {
  const [route, setRoute] = useState<Route>(() => parseHash(window.location.hash))

  useEffect(() => {
    const onHashChange = () => setRoute(parseHash(window.location.hash))
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  return (
    <div className="layout">
      <Rail current={route} />
      <main className="layout-main">
        {route.view === 'new' && <CrawlForm onStarted={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawls' && <History onSelect={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawl' && <CrawlReport key={route.id} id={route.id} />}
        {route.view === 'compare' && <CompareView />}
        {route.view === 'redirects' && <RedirectsView />}
        {route.view === 'redirect' && <RedirectView key={route.id} id={route.id} />}
        {route.view === 'settings' && <SettingsView />}
      </main>
    </div>
  )
}
```

  In `web/src/main.tsx`, replace `import './styles/fonts.css'` with

  ```tsx
  import './styles/fonts.css'
  import './styles/base.css'
  ```

  (`index.css` stays imported after `base.css` until VIS-8.)

- [ ] **Step 6: Build and lint.** `cd web && npm run build && npm run lint` — expected
  `✓ built`, lint passes. (A `*/` inside a CSS comment, such as a glob, ends the comment and
  breaks the build; the comments above avoid it.)

- [ ] **Step 7: Manual check.** `make web-build && go run ./cmd/gocrawl serve`:
  - Dark page (`#12161d`), a 168px rail on the left with "gocrawl" and the five entries;
    the current entry has a violet left edge and raised background, and `aria-current="page"`
    in the inspector.
  - `#/crawl/<id>` highlights "Crawls"; `#/redirect/x` highlights "Redirects".
  - Tab through the rail: each link shows a 2px light outline inside the rail.
  - Narrow the window below 720px: the rail becomes a horizontal bar above the content.

- [ ] **Step 8: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 9: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src/styles/base.css web/src/components/Rail.tsx web/src/components/Rail.css web/src/App.tsx web/src/main.tsx web/src/index.css
  git commit -F - <<'EOF'
  feat(web): left navigation rail and base styles on the design tokens

  base.css sets the type, colors, focus ring, and form controls from the tokens and lays
  out the rail beside the current view. The old header nav, card kit, and global button
  rules are gone; the remaining view styles read the tokens through aliases until each view
  is restyled.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W1-VIS-4: Button, Notice, StatusCode, PhasePill

**Files:**
- Create: `web/src/status.ts`, `web/src/components/ui/Button.tsx`,
  `web/src/components/ui/Button.css`, `web/src/components/ui/Notice.tsx`,
  `web/src/components/ui/Notice.css`, `web/src/components/ui/StatusCode.tsx`,
  `web/src/components/ui/StatusCode.css`, `web/src/components/ui/PhasePill.tsx`,
  `web/src/components/ui/PhasePill.css`

**Interfaces:**
- Consumes: tokens.
- Produces:
  - `status.ts` (addition): `type StatusClass = '2'|'3'|'4'|'5'|'x'`, `STATUS_CLASSES: StatusClass[]`, `statusClass(code: number): StatusClass`, `STATUS_LABEL: Record<StatusClass, string>`
  - `Button` (contract): props `ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'go'|'stop'|'pause'|'primary'|'secondary' }`, default `variant='secondary'`, `type='button'`; exported `type ButtonVariant`. Anchors styled as buttons use `className="btn btn-secondary"`.
  - `Notice` (contract): `{ tone: 'warning'|'error'|'info'; children: ReactNode; action?: ReactNode }`; exported `type NoticeTone`
  - `StatusCode` (contract): `{ code: number }`
  - `PhasePill` (addition): `{ phase: Phase; children: ReactNode }`, `type Phase = 'crawling'|'paused'|'analyzing'|'done'|'stopped'|'failed'`

- [ ] **Step 1: Create `web/src/status.ts`:**

```ts
// HTTP status classes, shared by StatusCode, the crawl tape, and anything else that colors by
// status. 'x' is "no response": a fetch error, recorded with status 0.

export type StatusClass = '2' | '3' | '4' | '5' | 'x'

export const STATUS_CLASSES: StatusClass[] = ['2', '3', '4', '5', 'x']

export function statusClass(code: number): StatusClass {
  if (code >= 200 && code < 300) return '2'
  if (code >= 300 && code < 400) return '3'
  if (code >= 400 && code < 500) return '4'
  if (code >= 500 && code < 600) return '5'
  return 'x'
}

// Sentence-case names for legends and summaries.
export const STATUS_LABEL: Record<StatusClass, string> = {
  '2': 'OK',
  '3': 'Redirected',
  '4': 'Not found or blocked',
  '5': 'Server error',
  x: 'No response',
}
```

- [ ] **Step 2: Create the button.** `web/src/components/ui/Button.tsx`:

```tsx
import type { ButtonHTMLAttributes } from 'react'
import './Button.css'

// go: Start crawl and Resume crawl (green fill). stop: Stop crawl (red fill). pause: Pause
// (yellow outline). primary: the other main actions, such as Save and Check redirects
// (violet fill). secondary: everything else. Links that act as buttons (export downloads)
// use the same classes: className="btn btn-secondary".
export type ButtonVariant = 'go' | 'stop' | 'pause' | 'primary' | 'secondary'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }

export default function Button({ variant = 'secondary', className, type = 'button', ...rest }: ButtonProps) {
  const classes = className ? `btn btn-${variant} ${className}` : `btn btn-${variant}`
  return <button type={type} className={classes} {...rest} />
}
```

  `web/src/components/ui/Button.css`:

```css
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 32px;
  margin: 0;
  padding: 6px 14px;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.2;
  white-space: nowrap;
  text-decoration: none;
  color: var(--ink);
  background: var(--raised);
  border: 1px solid var(--field);
  border-radius: 5px;
  cursor: pointer;
}

.btn:hover:not(:disabled) {
  border-color: var(--ink-2);
}

.btn:disabled {
  opacity: 0.55;
  cursor: default;
}

.btn-secondary {
  font-weight: 400;
}

.btn-go {
  color: var(--on-fill);
  background: var(--s2);
  border-color: var(--s2);
}

.btn-stop {
  color: var(--on-fill);
  background: var(--s5);
  border-color: var(--s5);
}

.btn-primary {
  color: var(--on-fill);
  background: var(--violet);
  border-color: var(--violet);
}

.btn-pause {
  color: var(--ink);
  background: transparent;
  border-color: var(--s3);
}

.btn-go:hover:not(:disabled),
.btn-stop:hover:not(:disabled),
.btn-primary:hover:not(:disabled),
.btn-pause:hover:not(:disabled) {
  border-color: var(--ink);
}
```

- [ ] **Step 3: Create the notice.** `web/src/components/ui/Notice.tsx`:

```tsx
import type { ReactNode } from 'react'
import './Notice.css'

// A boxed message with a colored left edge: warning (yellow, e.g. partial coverage or a
// budget below the sitemap), error (red), info (violet, e.g. run notes). action holds a
// button or a small form that resolves the notice, such as "Raise budget".
export type NoticeTone = 'warning' | 'error' | 'info'

export default function Notice({ tone, children, action }: { tone: NoticeTone; children: ReactNode; action?: ReactNode }) {
  return (
    <div className={`notice notice-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      <div className="notice-body">{children}</div>
      {action && <div className="notice-action">{action}</div>}
    </div>
  )
}
```

  `web/src/components/ui/Notice.css`:

```css
.notice {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 10px 14px;
  margin: 16px 0;
  padding: 10px 14px;
  color: var(--ink);
  background: var(--surface);
  border: 1px solid var(--rule);
  border-left-width: 4px;
  border-radius: 5px;
}

.notice-body {
  flex: 1 1 340px;
  min-width: 0;
}

.notice-body > :first-child {
  margin-top: 0;
}

.notice-body > :last-child {
  margin-bottom: 0;
}

.notice-action {
  flex: none;
}

.notice-warning {
  background: var(--notice-bg);
  border-color: var(--s3);
}

.notice-error {
  border-color: var(--s5);
}

.notice-info {
  border-color: var(--violet);
}
```

- [ ] **Step 4: Create the status code.** `web/src/components/ui/StatusCode.tsx`:

```tsx
import { statusClass } from '../../status'
import './StatusCode.css'

// An HTTP status code in the mono face, colored by class: 2xx green, 3xx yellow, 4xx orange,
// 5xx red (the spec's status colors, as on the tape). 0 means the fetch failed and reads
// "Error".
export default function StatusCode({ code }: { code: number }) {
  return <span className={`status-code status-code-${statusClass(code)}`}>{code > 0 ? code : 'Error'}</span>
}
```

  `web/src/components/ui/StatusCode.css`:

```css
.status-code {
  font-family: var(--font-mono);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.status-code-2 {
  color: var(--s2);
}

.status-code-3 {
  color: var(--s3);
}

.status-code-4 {
  color: var(--s4);
}

.status-code-5,
.status-code-x {
  color: var(--s5);
}
```

- [ ] **Step 5: Create the phase pill.** `web/src/components/ui/PhasePill.tsx`:

```tsx
import type { ReactNode } from 'react'
import './PhasePill.css'

// The state pill next to a crawl's heading. crawling: a green dot that pulses (static under
// prefers-reduced-motion). paused: a yellow dot. analyzing: a violet dot. done, stopped,
// failed: the finished states, with a green, slate, or red dot.
export type Phase = 'crawling' | 'paused' | 'analyzing' | 'done' | 'stopped' | 'failed'

export default function PhasePill({ phase, children }: { phase: Phase; children: ReactNode }) {
  return <span className={`phase phase-${phase}`}>{children}</span>
}
```

  `web/src/components/ui/PhasePill.css`:

```css
.phase {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 1px 10px 1px 8px;
  font-size: 13px;
  color: var(--ink);
  border: 1px solid var(--field);
  border-radius: 999px;
}

.phase::before {
  content: '';
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--violet);
}

.phase-crawling::before,
.phase-done::before {
  background: var(--s2);
}

.phase-paused {
  border-color: var(--s3);
}

.phase-paused::before {
  background: var(--s3);
}

.phase-stopped::before {
  background: var(--todo);
}

.phase-failed {
  border-color: var(--s5);
}

.phase-failed::before {
  background: var(--s5);
}

@media (prefers-reduced-motion: no-preference) {
  .phase-crawling::before {
    animation: phase-pulse 1.6s ease-in-out infinite;
  }
}

@keyframes phase-pulse {
  50% {
    opacity: 0.35;
  }
}
```

- [ ] **Step 6: Build and lint.** `cd web && npm run build && npm run lint` — expected `✓ built`
  and lint passes. Every color these components use is a pair the contrast guard checks
  (`--on-fill` on `--s2`, `--s5`, `--violet`; `--ink` on `--notice-bg`; `--s3` border on
  `--notice-bg`; status-code text `--s2`/`--s3`/`--s4`/`--s5` on `--surface` and `--ground`,
  for example 2xx green on the dark surface at 5.94). The components get their first callers in VIS-6 to VIS-8, where they are
  checked by eye.

- [ ] **Step 7: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 8: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src/status.ts web/src/components/ui
  git commit -F - <<'EOF'
  feat(web): Button, Notice, StatusCode, and PhasePill components

  Button carries the crawl-control colors (green go, yellow pause outline, red stop) and
  violet for other primary actions. Notice, StatusCode, and PhasePill give every view the
  same warning boxes, status coloring, and state pill.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W1-VIS-5: The crawl tape

**Files:**
- Create: `web/src/components/tape/tapeModel.ts`, `web/src/components/tape/Tape.tsx`,
  `web/src/components/tape/TapeLegend.tsx`, `web/src/components/tape/Tape.css`

**Interfaces:**
- Consumes: `status.ts` (VIS-4), tokens.
- Produces:
  - `Tape` (contract): `{ tape: string; tapeOff?: string; notFetched?: number; pastBudget?: number; summary: string; compact?: boolean; live?: boolean }`, `role="img"`, `aria-label={summary}`
  - `tapeModel.ts` (additions): `MAX_TICKS = 20000`, `MAX_COMPACT_TICKS = 160`, `type TickKind`, `interface Tick { kind: TickKind; off: boolean }`, `buildTicks(tape, tapeOff = '', notFetched = 0, pastBudget = 0, max = MAX_TICKS): Tick[]`, `interface TapeCounts`, `countTape(tape, tapeOff = ''): TapeCounts`, `describeTape(counts, notFetched = 0, pastBudget = 0, finished = false): string`
  - `TapeLegend` (addition): `{ sitemap?: boolean; finished?: boolean }`

  Encoding (spec, "The tape"): one 3px tick per page in fetch order; 2xx short green (9px),
  3xx mid yellow (15px), 4xx tall orange (22px), 5xx tall red with a 5px `--ink` cap,
  fetch error (`x`) tall slate (`--ink-2`); notch 3–5px above the base for pages not in the
  sitemap; short slate outline (`--todo`) for sitemap pages not fetched yet; short amber
  outline (`--cut`) for sitemap pages past the budget. Above 20,000 ticks (160 in compact
  mode) each tick stands for a run of consecutive pages and takes the worst kind
  (5xx > x > 4xx > 3xx > 2xx > not fetched > past budget) and is notched if any page in the
  run is off-sitemap. New fetched ticks drop in when `live`; the animation is inside
  `@media (prefers-reduced-motion: no-preference)`.

- [ ] **Step 1: Create the model** `web/src/components/tape/tapeModel.ts`:

```ts
// Pure model behind the crawl tape (Tape.tsx). The API sends the tape as two strings that
// grow by one character per recorded page: `tape` holds the status class ('2', '3', '4', '5',
// or 'x' for a fetch error) and `tapeOff` holds '1' when that page is not in the sitemap.
// Outline ticks follow the fetched ones: sitemap pages not fetched yet, then sitemap pages
// past the page budget.

import { STATUS_CLASSES, type StatusClass } from '../../status'

// Above this many ticks, each tick stands for a run of consecutive pages.
export const MAX_TICKS = 20000
// A compact tape (a row in a list) is one short line, so it aggregates much sooner.
export const MAX_COMPACT_TICKS = 160

export type TickKind = StatusClass | 'todo' | 'cut'

export interface Tick {
  kind: TickKind
  // A fetched page that is not in the sitemap (aggregated: any page in the run).
  off: boolean
}

// Worst first. A run takes the worst status among its pages; any fetched page outranks the
// outline ticks, and "not fetched yet" outranks "past the budget".
const RANK: Record<TickKind, number> = { '5': 6, x: 5, '4': 4, '3': 3, '2': 2, todo: 1, cut: 0 }

function charKind(c: string): StatusClass {
  return c === '2' || c === '3' || c === '4' || c === '5' ? c : 'x'
}

function whole(n: number): number {
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0
}

export function buildTicks(tape: string, tapeOff = '', notFetched = 0, pastBudget = 0, max = MAX_TICKS): Tick[] {
  const todo = whole(notFetched)
  const cut = whole(pastBudget)
  const total = tape.length + todo + cut
  const per = Math.max(1, Math.ceil(total / Math.max(1, max)))
  const kindAt = (i: number): TickKind => {
    if (i < tape.length) return charKind(tape[i])
    return i < tape.length + todo ? 'todo' : 'cut'
  }

  const ticks: Tick[] = []
  for (let start = 0; start < total; start += per) {
    const end = Math.min(total, start + per)
    let kind = kindAt(start)
    let off = false
    for (let i = start; i < end; i++) {
      const k = kindAt(i)
      if (RANK[k] > RANK[kind]) kind = k
      if (i < tape.length && tapeOff[i] === '1') off = true
    }
    ticks.push({ kind, off })
  }
  return ticks
}

export interface TapeCounts {
  fetched: number
  byClass: Record<StatusClass, number>
  off: number
}

export function countTape(tape: string, tapeOff = ''): TapeCounts {
  const byClass: Record<StatusClass, number> = { '2': 0, '3': 0, '4': 0, '5': 0, x: 0 }
  let off = 0
  for (let i = 0; i < tape.length; i++) {
    byClass[charKind(tape[i])]++
    if (tapeOff[i] === '1') off++
  }
  return { fetched: tape.length, byClass, off }
}

// How each status class reads after a count, singular and plural.
const PHRASE: Record<StatusClass, [string, string]> = {
  '2': ['OK', 'OK'],
  '3': ['redirected', 'redirected'],
  '4': ['not found or blocked', 'not found or blocked'],
  '5': ['server error', 'server errors'],
  x: ['without a response', 'without a response'],
}

function n(count: number): string {
  return count.toLocaleString('en-US')
}

function pages(count: number): string {
  return count === 1 ? 'page' : 'pages'
}

// describeTape is the tape's text alternative (its aria-label), and the sentence a view can
// show under it: every count the tape draws, in words. finished switches the slate outline's
// meaning from "not fetched yet" to "not reached by links".
export function describeTape(counts: TapeCounts, notFetched = 0, pastBudget = 0, finished = false): string {
  const parts = STATUS_CLASSES.filter((k) => counts.byClass[k] > 0).map((k) => {
    const c = counts.byClass[k]
    return `${n(c)} ${PHRASE[k][c === 1 ? 0 : 1]}`
  })
  let text = `${n(counts.fetched)} ${pages(counts.fetched)} fetched${parts.length ? `: ${parts.join(', ')}.` : '.'}`
  if (counts.off > 0) {
    text += ` ${n(counts.off)} fetched ${counts.off === 1 ? 'page is' : 'pages are'} not in the sitemap.`
  }
  const todo = whole(notFetched)
  if (todo > 0) {
    text += ` ${n(todo)} sitemap ${pages(todo)} ${finished ? 'not reached by links' : 'not fetched yet'}.`
  }
  const cut = whole(pastBudget)
  if (cut > 0) {
    text += ` ${n(cut)} sitemap ${pages(cut)} past the page budget.`
  }
  return text
}
```

- [ ] **Step 2: Create the component** `web/src/components/tape/Tape.tsx`:

```tsx
import { useMemo } from 'react'
import { buildTicks, MAX_COMPACT_TICKS, MAX_TICKS } from './tapeModel'
import './Tape.css'

export interface TapeProps {
  // One status-class character per fetched page, in fetch order.
  tape: string
  // '1' where that page is not in the sitemap; drawn as a notch near the tick's base.
  tapeOff?: string
  // Sitemap pages not fetched (yet): slate outline ticks after the fetched ones.
  notFetched?: number
  // Sitemap pages beyond the page budget: amber outline ticks at the end.
  pastBudget?: number
  // Text alternative; build it with describeTape so every drawn count is also in words.
  summary: string
  // One short row, for lists (Crawls, Compare pickers).
  compact?: boolean
  // New fetched ticks drop in (off under prefers-reduced-motion).
  live?: boolean
}

// The crawl tape: one 3px tick per page. Height carries the status as well as color, so the
// tape still reads without color: short 2xx, mid 3xx, tall 4xx, tall 5xx with a light cap.
// Fetched ticks and outline ticks have different keys, so when a page is fetched its tick
// remounts and the drop-in animation plays only for it.
export default function Tape({ tape, tapeOff = '', notFetched = 0, pastBudget = 0, summary, compact = false, live = false }: TapeProps) {
  const ticks = useMemo(
    () => buildTicks(tape, tapeOff, notFetched, pastBudget, compact ? MAX_COMPACT_TICKS : MAX_TICKS),
    [tape, tapeOff, notFetched, pastBudget, compact],
  )
  const classes = `tape${compact ? ' tape-compact' : ''}${live ? ' tape-live' : ''}`
  return (
    <div className={classes} role="img" aria-label={summary}>
      {ticks.map((t, i) => {
        const fetched = t.kind !== 'todo' && t.kind !== 'cut'
        return (
          <i
            key={fetched ? `f${i}` : `o${i}`}
            className={`tick tick-${t.kind}${t.off ? ' tick-off' : ''}${fetched ? ' tick-fetched' : ''}`}
          />
        )
      })}
    </div>
  )
}
```

- [ ] **Step 3: Create the legend** `web/src/components/tape/TapeLegend.tsx`:

```tsx
import { STATUS_CLASSES, STATUS_LABEL } from '../../status'
import './Tape.css'

// The key under a full-size tape. sitemap adds the notch and outline entries, which only a
// live crawl with a sitemap draws (saved crawls rebuild a status-only tape). finished renames
// the slate outline from "Not fetched yet" to "Not reached by links".
export default function TapeLegend({ sitemap = false, finished = false }: { sitemap?: boolean; finished?: boolean }) {
  return (
    <ul className="tape-legend">
      {STATUS_CLASSES.map((k) => (
        <li key={k}>
          <i className={`tick tick-${k}`} aria-hidden="true" />
          {STATUS_LABEL[k]}
        </li>
      ))}
      {sitemap && (
        <>
          <li>
            <i className="tick tick-2 tick-off" aria-hidden="true" />
            Not in the sitemap (notched)
          </li>
          <li>
            <i className="tick tick-todo" aria-hidden="true" />
            {finished ? 'Not reached by links' : 'Not fetched yet'}
          </li>
          <li>
            <i className="tick tick-cut" aria-hidden="true" />
            Past the page budget
          </li>
        </>
      )}
    </ul>
  )
}
```

- [ ] **Step 4: Create the styles** `web/src/components/tape/Tape.css`:

```css
/*
 * The crawl tape. --u scales every tick height, so the compact tape is the same drawing at
 * half size. Tick encoding (design spec, "The tape"):
 *   2xx short green · 3xx mid yellow · 4xx tall orange · 5xx tall red with a light cap
 *   no response (fetch error) tall slate · notch near the base: fetched, not in the sitemap
 *   slate outline: sitemap page not fetched yet · amber outline: sitemap page past the budget
 */
.tape {
  --u: 1px;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  align-content: flex-start;
  gap: 2px 1px;
  max-height: 50vh;
  overflow-y: auto;
  padding: 6px;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.tape-compact {
  --u: 0.5px;
  flex-wrap: nowrap;
  gap: 1px;
  max-height: none;
  overflow: hidden;
  padding: 3px;
}

.tick {
  display: block;
  flex: none;
  width: 3px;
  height: calc(9 * var(--u));
}

.tick-2 {
  height: calc(9 * var(--u));
  background: var(--s2);
}

.tick-3 {
  height: calc(15 * var(--u));
  background: var(--s3);
}

.tick-4 {
  height: calc(22 * var(--u));
  background: var(--s4);
}

.tick-5 {
  height: calc(22 * var(--u));
  background: linear-gradient(var(--ink) 0 calc(5 * var(--u)), var(--s5) calc(5 * var(--u)));
}

.tick-x {
  height: calc(22 * var(--u));
  background: var(--ink-2);
}

.tick-todo {
  height: calc(9 * var(--u));
  box-shadow: inset 0 0 0 1px var(--todo);
}

.tick-cut {
  height: calc(9 * var(--u));
  box-shadow: inset 0 0 0 1px var(--cut);
}

.tick-off {
  -webkit-mask-image: linear-gradient(
    to top,
    #000 0 calc(3 * var(--u)),
    transparent calc(3 * var(--u)) calc(5 * var(--u)),
    #000 calc(5 * var(--u))
  );
  mask-image: linear-gradient(
    to top,
    #000 0 calc(3 * var(--u)),
    transparent calc(3 * var(--u)) calc(5 * var(--u)),
    #000 calc(5 * var(--u))
  );
}

@media (prefers-reduced-motion: no-preference) {
  .tape-live .tick-fetched {
    animation: tick-drop 240ms ease-out;
  }
}

@keyframes tick-drop {
  from {
    opacity: 0;
    transform: translateY(-6px);
  }
}

.tape-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 6px 18px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
  font-size: 13px;
  color: var(--ink-2);
}

.tape-legend li {
  display: inline-flex;
  align-items: flex-end;
  gap: 6px;
}

.tape-legend .tick {
  --u: 1px;
  width: 4px;
}
```

- [ ] **Step 5: Build and lint.** `cd web && npm run build && npm run lint` — expected `✓ built`,
  lint passes.

- [ ] **Step 6: Manual check with a throwaway page.** Temporarily replace
  `web/src/views/CompareView.tsx` with this (do not commit it):

  ```tsx
  import Tape from '../components/tape/Tape'
  import TapeLegend from '../components/tape/TapeLegend'
  import { countTape, describeTape } from '../components/tape/tapeModel'

  const tape = '2'.repeat(240) + '3'.repeat(12) + '4'.repeat(6) + '5'.repeat(3) + 'x'
  const tapeOff = '0'.repeat(100) + '1'.repeat(5) + '0'.repeat(157)
  const big = '2'.repeat(49990) + '5'.repeat(10)

  export default function CompareView() {
    return (
      <>
        <h1>Tape check</h1>
        <Tape
          tape={tape}
          tapeOff={tapeOff}
          notFetched={300}
          pastBudget={120}
          live
          summary={describeTape(countTape(tape, tapeOff), 300, 120)}
        />
        <TapeLegend sitemap />
        <h2>Aggregated</h2>
        <Tape tape={big} summary={describeTape(countTape(big))} />
        <h2>Compact</h2>
        <Tape tape={tape} compact summary={describeTape(countTape(tape))} />
      </>
    )
  }
  ```

  Run `make web-build && go run ./cmd/gocrawl serve`, open `#/compare`, and check:
  - First tape: 240 short green, 12 mid yellow, 6 tall orange, 3 tall red with a light cap,
    1 tall slate, then 300 slate outlines and 120 amber outlines; ticks 101–105 show a gap
    near the base. The ticks drop in once on load.
  - In the console, `document.querySelectorAll('.tape')[1].children.length` is `16667`
    (50,000 pages, 3 per tick, at most 20,000) and its last tick is red with a cap.
  - `document.querySelectorAll('.tape')[2].children.length` is `131` and fits on one row.
  - The accessibility inspector shows the first tape as an image named "262 pages fetched:
    240 OK, 12 redirected, 6 not found or blocked, 3 server errors, 1 without a response.
    5 fetched pages are not in the sitemap. 300 sitemap pages not fetched yet. 120 sitemap
    pages past the page budget."
  - With the rendering panel's "Emulate prefers-reduced-motion: reduce", reloading shows no
    drop-in.
  - With `document.documentElement.dataset.theme = 'light'` the colors switch to the light
    palette.

  Then discard the throwaway page: `git checkout -- web/src/views/CompareView.tsx`.

- [ ] **Step 7: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 8: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git status --short web/src/views   # expect no output
  git add web/src/components/tape
  git commit -F - <<'EOF'
  feat(web): crawl tape component

  One 3px tick per page whose height and color give the HTTP status class, a notch for
  pages not in the sitemap, and slate and amber outlines for sitemap pages not fetched yet
  and past the budget. Above 20,000 pages each tick aggregates a run and takes its worst
  status. The tape is role="img" with a full text summary from describeTape, and its
  drop-in animation respects prefers-reduced-motion.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W1-VIS-6: Restyle the New crawl form

**Files:**
- Modify (rewrite): `web/src/components/CrawlForm.tsx`
- Create: `web/src/components/CrawlForm.css`

**Interfaces:**
- Consumes: `Button` (`go`, `secondary`), `Notice` (`error`), base styles.
- Produces: unchanged component API `CrawlForm({ onStarted }: { onStarted: (id: string) => void })`;
  CSS class prefix `cf-`. Behavior and request body are unchanged; only markup, copy (sentence
  case, "Site URL", "Save to Crawls when finished"), and styles change.

- [ ] **Step 1: Replace `web/src/components/CrawlForm.tsx`.** State, `splitList`,
  `toggleAnalyzer`, and `handleSubmit` are unchanged; the imports gain `Button`, `Notice`, and
  the stylesheet, and the markup uses fieldsets with legends instead of the old uppercase
  section titles:

```tsx
import { useEffect, useState } from 'react'
import { listAnalyzers, startCrawl } from '../api/crawls'
import type { AnalyzerInfo, StartCrawlParams } from '../types/crawls'
import Button from './ui/Button'
import Notice from './ui/Notice'
import './CrawlForm.css'

export default function CrawlForm({ onStarted }: { onStarted: (id: string) => void }) {
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [url, setUrl] = useState('')
  const [depth, setDepth] = useState('')
  const [maxPages, setMaxPages] = useState('')
  const [concurrency, setConcurrency] = useState('')
  const [rate, setRate] = useState('')
  const [maxDuration, setMaxDuration] = useState('')
  const [render, setRender] = useState('raw')
  const [respectRobots, setRespectRobots] = useState(true)
  const [subdomains, setSubdomains] = useState(false)
  const [followExternal, setFollowExternal] = useState(false)
  const [specialized, setSpecialized] = useState(false)
  const [securityAudit, setSecurityAudit] = useState(false)
  const [ignoreExternalTagging, setIgnoreExternalTagging] = useState(true)
  const [save, setSave] = useState(true)
  const [include, setInclude] = useState('')
  const [exclude, setExclude] = useState('')
  const [userAgent, setUserAgent] = useState('')
  const [userAgents, setUserAgents] = useState('')
  const [userAgentRotation, setUserAgentRotation] = useState('round-robin')
  const [proxy, setProxy] = useState('')
  const [proxies, setProxies] = useState('')
  const [proxyRotation, setProxyRotation] = useState('round-robin')
  const [basicAuthUser, setBasicAuthUser] = useState('')
  const [basicAuthPass, setBasicAuthPass] = useState('')
  const [cookieAuth, setCookieAuth] = useState(false)
  const [cookie, setCookie] = useState('')
  const [customHeaders, setCustomHeaders] = useState(false)
  const [headers, setHeaders] = useState('')
  const [headerProfile, setHeaderProfile] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e) => setError(String(e)))
  }, [])

  // Newline-separated rather than comma-separated: a comma is a completely ordinary character
  // in a User-Agent string ("... (KHTML, like Gecko) ...") and in a regex ("{2,4}"), so
  // splitting on it would silently corrupt otherwise-valid entries.
  function splitList(s: string): string[] {
    return s
      .split('\n')
      .map((v) => v.trim())
      .filter(Boolean)
  }

  function toggleAnalyzer(name: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  function selectAllAnalyzers() {
    setSelected(new Set(analyzers.map((a) => a.name)))
  }

  function selectNoAnalyzers() {
    setSelected(new Set())
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!url.trim()) return
    setSubmitting(true)
    setError('')
    const params: StartCrawlParams = {
      url: url.trim(),
      render,
      respect_robots: respectRobots,
      subdomains,
      follow_external: followExternal,
      specialized,
      security_audit: securityAudit,
      ignore_external_tagging: ignoreExternalTagging,
      save,
    }
    if (depth) params.depth = Number(depth)
    if (maxPages) params.max_pages = Number(maxPages)
    if (concurrency) params.concurrency = Number(concurrency)
    if (rate) params.rate = Number(rate)
    if (maxDuration.trim()) params.max_duration = maxDuration.trim()
    if (selected.size > 0) params.analyzers = Array.from(selected)
    if (include.trim()) params.include = splitList(include)
    if (exclude.trim()) params.exclude = splitList(exclude)
    if (userAgent.trim()) params.user_agent = userAgent.trim()
    if (userAgents.trim()) {
      params.user_agents = splitList(userAgents)
      params.user_agent_rotation = userAgentRotation
    }
    if (proxy.trim()) params.proxy = proxy.trim()
    if (proxies.trim()) {
      params.proxies = splitList(proxies)
      params.proxy_rotation = proxyRotation
    }
    if (basicAuthUser.trim()) params.basic_auth = `${basicAuthUser.trim()}:${basicAuthPass}`
    if (cookieAuth && cookie.trim()) params.cookie = cookie.trim()
    if (customHeaders) {
      if (headers.trim()) params.headers = splitList(headers)
      if (headerProfile.trim()) params.header_profile = headerProfile.trim()
    }
    try {
      const job = await startCrawl(params)
      onStarted(job.id)
    } catch (err) {
      setError(String(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="cf" onSubmit={handleSubmit}>
      <h1>New crawl</h1>
      <label className="cf-field cf-url">
        <span className="cf-label">Site URL</span>
        <input
          type="text"
          inputMode="url"
          placeholder="https://example.com"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
        />
      </label>

      <fieldset className="cf-section">
        <legend>Crawl behavior</legend>
        <div className="cf-grid">
          <label className="cf-field">
            <span className="cf-label">Depth</span>
            <input type="number" min="0" placeholder="Unlimited" value={depth} onChange={(e) => setDepth(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max pages</span>
            <input type="number" min="0" placeholder="500" value={maxPages} onChange={(e) => setMaxPages(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Concurrency</span>
            <input type="number" min="1" placeholder="4" value={concurrency} onChange={(e) => setConcurrency(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Rate limit (requests per second)</span>
            <input type="number" min="0" step="any" placeholder="Unlimited" value={rate} onChange={(e) => setRate(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max duration</span>
            <input type="text" placeholder="Unlimited, e.g. 90m" value={maxDuration} onChange={(e) => setMaxDuration(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Render</span>
            <select value={render} onChange={(e) => setRender(e.target.value)}>
              <option value="raw">Raw HTML</option>
              <option value="headless">Headless browser</option>
            </select>
          </label>
        </div>
        <label className="cf-check">
          <input type="checkbox" checked={save} onChange={(e) => setSave(e.target.checked)} />
          Save to Crawls when finished
        </label>
      </fieldset>

      <fieldset className="cf-section">
        <legend>Scope</legend>
        <label className="cf-check">
          <input type="checkbox" checked={respectRobots} onChange={(e) => setRespectRobots(e.target.checked)} />
          Respect robots.txt
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={subdomains} onChange={(e) => setSubdomains(e.target.checked)} />
          Follow links to subdomains
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={followExternal} onChange={(e) => setFollowExternal(e.target.checked)} />
          Crawl links that leave the site
        </label>
      </fieldset>

      <fieldset className="cf-section">
        <legend>Analysis</legend>
        <label className="cf-check">
          <input type="checkbox" checked={specialized} onChange={(e) => setSpecialized(e.target.checked)} />
          Specialized checks (AEO and GEO heuristics, WordPress security probes)
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={securityAudit} onChange={(e) => setSecurityAudit(e.target.checked)} />
          Security audit (TLS and certificate, cookies, response headers)
        </label>
        <label className="cf-check">
          <input
            type="checkbox"
            checked={ignoreExternalTagging}
            onChange={(e) => setIgnoreExternalTagging(e.target.checked)}
          />
          Ignore UTM tagging issues on links that leave the site
        </label>

        {analyzers.length > 0 && (
          <fieldset className="cf-analyzers">
            <legend>Analyzers</legend>
            <p className="cf-hint">None checked runs all of them.</p>
            <div className="cf-analyzer-actions">
              <Button onClick={selectAllAnalyzers}>Select all</Button>
              <Button onClick={selectNoAnalyzers}>Select none</Button>
            </div>
            <div className="cf-analyzer-grid">
              {analyzers.map((a) => (
                <label key={a.name} className="cf-check" title={a.description}>
                  <input type="checkbox" checked={selected.has(a.name)} onChange={() => toggleAnalyzer(a.name)} />
                  <span className="mono">{a.name}</span>
                </label>
              ))}
            </div>
          </fieldset>
        )}
      </fieldset>

      <details className="cf-advanced">
        <summary>Advanced</summary>
        <div className="cf-grid cf-grid-wide">
          <label className="cf-field">
            <span className="cf-label">Include (regex, one per line)</span>
            <span className="cf-hint">
              Matched against the full normalized URL. The seed must match too, or the crawl returns zero pages. No
              trailing slash.
            </span>
            <textarea rows={2} placeholder="/blog" value={include} onChange={(e) => setInclude(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Exclude (regex, one per line)</span>
            <span className="cf-hint">Matched against the full URL.</span>
            <textarea rows={2} placeholder="/admin/" value={exclude} onChange={(e) => setExclude(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent</span>
            <input type="text" value={userAgent} onChange={(e) => setUserAgent(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent pool (one per line)</span>
            <textarea rows={2} value={userAgents} onChange={(e) => setUserAgents(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent rotation</span>
            <select value={userAgentRotation} onChange={(e) => setUserAgentRotation(e.target.value)}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
            </select>
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy</span>
            <input type="text" placeholder="http(s):// or socks5://" value={proxy} onChange={(e) => setProxy(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy pool (one per line)</span>
            <textarea rows={2} value={proxies} onChange={(e) => setProxies(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy rotation</span>
            <select value={proxyRotation} onChange={(e) => setProxyRotation(e.target.value)}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
              <option value="sticky-host">Sticky per host</option>
            </select>
          </label>
          <label className="cf-field">
            <span className="cf-label">Basic Auth username</span>
            <input type="text" value={basicAuthUser} onChange={(e) => setBasicAuthUser(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Basic Auth password</span>
            <input type="password" value={basicAuthPass} onChange={(e) => setBasicAuthPass(e.target.value)} />
          </label>
        </div>

        <label className="cf-check">
          <input type="checkbox" checked={cookieAuth} onChange={(e) => setCookieAuth(e.target.checked)} />
          The site needs a session cookie (for example a Shopify storefront password page)
        </label>
        {cookieAuth && (
          <label className="cf-field">
            <span className="cf-label">Cookie header</span>
            <span className="cf-hint">
              Paste the session cookie from a browser that is already past the gate (DevTools, Application or Storage,
              Cookies). Sent only to the seed host. Not supported with headless rendering.
            </span>
            <textarea
              rows={2}
              className="mono"
              placeholder="_shopify_essential=:AZ_..."
              value={cookie}
              onChange={(e) => setCookie(e.target.value)}
            />
          </label>
        )}

        <label className="cf-check">
          <input type="checkbox" checked={customHeaders} onChange={(e) => setCustomHeaders(e.target.checked)} />
          Send custom request headers (for example a Shopify crawler access signature)
        </label>
        {customHeaders && (
          <>
            <label className="cf-field">
              <span className="cf-label">Header profile</span>
              <span className="cf-hint">
                Name of a file at ~/.gocrawl/headers/&lt;name&gt;.headers on this machine: the way to use a stored
                secret without pasting it here.
              </span>
              <input type="text" placeholder="uk" value={headerProfile} onChange={(e) => setHeaderProfile(e.target.value)} />
            </label>
            <label className="cf-field">
              <span className="cf-label">Extra headers (one &quot;Name: value&quot; per line)</span>
              <span className="cf-hint">
                Sent only to the seed host. They override the profile by name. Not supported with headless rendering.
              </span>
              <textarea
                rows={3}
                className="mono"
                placeholder="X-Access-Token: abc123"
                value={headers}
                onChange={(e) => setHeaders(e.target.value)}
              />
            </label>
          </>
        )}
      </details>

      {error && <Notice tone="error">{error}</Notice>}
      <div className="cf-actions">
        <Button type="submit" variant="go" disabled={submitting}>
          {submitting ? 'Starting…' : 'Start crawl'}
        </Button>
      </div>
    </form>
  )
}
```

- [ ] **Step 2: Create `web/src/components/CrawlForm.css`:**

```css
/* New crawl form. Labels sit above their field in sentence case; hints are secondary ink. */
.cf {
  max-width: 920px;
}

.cf-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 12px 0 0;
}

.cf-field input,
.cf-field select,
.cf-field textarea {
  width: 100%;
}

.cf-url input {
  font-family: var(--font-mono);
  font-size: 15px;
}

.cf-label {
  font-size: 14px;
  font-weight: 600;
}

.cf-hint {
  margin: 0;
  font-size: 13px;
  color: var(--ink-2);
}

.cf-section {
  margin: 24px 0 0;
  padding: 0;
  border: 0;
}

.cf-section > legend,
.cf-analyzers > legend {
  padding: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--ink);
}

.cf-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 0 16px;
}

.cf-grid-wide {
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
}

.cf-check {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 10px 0 0;
  font-size: 14px;
}

.cf-analyzers {
  margin: 16px 0 0;
  padding: 12px 14px 14px;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.cf-analyzers > legend {
  padding: 0 4px;
  font-size: 15px;
}

.cf-analyzer-actions {
  display: flex;
  gap: 8px;
  margin: 8px 0 4px;
}

.cf-analyzer-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 0 12px;
}

.cf-advanced {
  margin: 24px 0 0;
  padding: 10px 14px 14px;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.cf-advanced > summary {
  font-weight: 700;
  cursor: pointer;
}

.cf-actions {
  margin: 24px 0 0;
}
```

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint` — expected `✓ built`,
  lint passes.

- [ ] **Step 4: Manual check.** `make web-build && go run ./cmd/gocrawl serve`, open `#/`:
  - "New crawl" heading; "Crawl behavior", "Scope", "Analysis" as sentence-case section
    headings; no all-caps text anywhere on the page.
  - The URL field uses the mono face; inputs have the `--field` border; checkboxes are violet.
  - "Start crawl" is a green filled button with dark text; it starts a crawl and lands on its
    page.
  - Stop the server, submit again: a red-edged error notice appears above the button.

- [ ] **Step 5: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 6: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src/components/CrawlForm.tsx web/src/components/CrawlForm.css
  git commit -F - <<'EOF'
  feat(web): restyle the New crawl form

  Fieldsets with sentence-case legends replace the uppercase section titles, the green
  Start crawl button and an error notice replace the old purple button and red text, and the
  form's styles move to CrawlForm.css on the design tokens.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W1-VIS-7: Restyle the crawl view

**Files:**
- Modify (rewrite): `web/src/components/CrawlReport.tsx`
- Create: `web/src/components/CrawlReport.css`, `web/src/crawlStatus.ts`

**Interfaces:**
- Consumes: `Button` (`stop`, `secondary`), `Notice` (`warning`, `info`, `error`),
  `PhasePill`, `StatusCode`.
- Produces:
  - `crawlStatus.ts` (addition): `STATUS_PILL: Record<CrawlStatus, { phase: Phase; label: string }>` — running "Crawling", done "Finished", canceled "Crawl stopped", error "Failed". Because it is a `Record` over `CrawlStatus`, adding `'paused'` to `CrawlStatus` (pause track) fails `tsc` until an entry is added here.
  - Unchanged component API `CrawlReport({ id }: { id: string })`; CSS class prefix `cr-`;
    global severity classes `.sev-error` (`--s5`), `.sev-warning` (`--s3`), `.sev-info`
    (`--ink-2`).
  - Copy: "Cancel" becomes **Stop crawl** (red fill) with the confirm "Stop this crawl? The
    report keeps the pages fetched so far."; a stopped crawl reads "Crawl stopped".

- [ ] **Step 1: Create `web/src/crawlStatus.ts`:**

```ts
import type { Phase } from './components/ui/PhasePill'
import type { CrawlStatus } from './types/crawls'

// How a crawl job's status reads in a PhasePill. Actions keep one name through the flow:
// "Stop crawl" leads to "Crawl stopped".
export const STATUS_PILL: Record<CrawlStatus, { phase: Phase; label: string }> = {
  running: { phase: 'crawling', label: 'Crawling' },
  done: { phase: 'done', label: 'Finished' },
  canceled: { phase: 'stopped', label: 'Crawl stopped' },
  error: { phase: 'failed', label: 'Failed' },
}
```

- [ ] **Step 2: Replace `web/src/components/CrawlReport.tsx`.** Polling, filtering, sorting,
  review state, and the reviewed export are unchanged; the header gets the host as `<h1>`,
  the phase pill, and button-styled actions; the coverage banner and notes become notices;
  page statuses use `StatusCode`; filters and the issue table use `cr-` classes and
  `aria-pressed` / `aria-sort`:

```tsx
import { useEffect, useMemo, useRef, useState } from 'react'
import { cancelCrawl, exportURL, getCrawl, listExplanations } from '../api/crawls'
import SiteMap from './SiteMap'
import { useSort } from '../sort'
import { STATUS_PILL } from '../crawlStatus'
import type { Explanation, Issue, JobView } from '../types/crawls'
import Button from './ui/Button'
import Notice from './ui/Notice'
import PhasePill from './ui/PhasePill'
import StatusCode from './ui/StatusCode'
import './CrawlReport.css'

const POLL_MS = 1500
const SEVERITY_RANK: Record<string, number> = { error: 0, warning: 1, info: 2 }

interface ReviewState {
  resolved: boolean
  nonissue: boolean
  note: string
}

type IssueRow = Issue & { _key: number }

function formatElapsed(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000))
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

function hostOf(seed: string): string {
  try {
    return new URL(seed).host
  } catch {
    return seed
  }
}

function issueSortValue(row: IssueRow, key: string): string | number {
  switch (key) {
    case 'severity':
      return SEVERITY_RANK[row.severity] ?? 9
    case 'analyzer':
      return row.analyzer
    case 'code':
      return row.code
    case 'url':
      return row.url
    case 'message':
      return row.message
    default:
      return ''
  }
}

export default function CrawlReport({ id }: { id: string }) {
  const [job, setJob] = useState<JobView | null>(null)
  const [error, setError] = useState('')
  const [severityOn, setSeverityOn] = useState<Record<string, boolean>>({ error: true, warning: true, info: true })
  const [analyzerFilter, setAnalyzerFilter] = useState('')
  const [search, setSearch] = useState('')
  const [codeOn, setCodeOn] = useState<Record<string, boolean>>({})
  const [codeMenuOpen, setCodeMenuOpen] = useState(false)
  const [hideResolved, setHideResolved] = useState(false)
  const [hideNonissue, setHideNonissue] = useState(false)
  const [review, setReview] = useState<Record<number, ReviewState>>({})
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [tab, setTab] = useState<'issues' | 'sitemap'>('issues')
  const [now, setNow] = useState(() => Date.now())
  const [explanations, setExplanations] = useState<Record<string, Explanation>>({})
  const timer = useRef<number | undefined>(undefined)
  const selectAllRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    listExplanations()
      .then((r) => setExplanations(r.explanations))
      .catch(() => undefined)
  }, [])

  useEffect(() => {
    let stopped = false
    async function poll() {
      try {
        const j = await getCrawl(id)
        if (stopped) return
        setJob(j)
        if (j.status === 'running') {
          timer.current = window.setTimeout(poll, POLL_MS)
        }
      } catch (err) {
        if (!stopped) setError(String(err))
      }
    }
    poll()
    return () => {
      stopped = true
      window.clearTimeout(timer.current)
    }
  }, [id])

  // The server attaches the report once, when the crawl finishes, and App keys this component
  // by crawl id, so the report-derived UI state below starts fresh for every report without a
  // reset effect. A code missing from codeOn counts as shown.
  const report = job?.report

  useEffect(() => {
    if (job?.status !== 'running') return
    const t = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(t)
  }, [job?.status])

  useEffect(() => {
    if (!codeMenuOpen) return
    function close() {
      setCodeMenuOpen(false)
    }
    document.addEventListener('click', close)
    return () => document.removeEventListener('click', close)
  }, [codeMenuOpen])

  const codeCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    report?.issues.forEach((i) => {
      counts[i.code] = (counts[i.code] ?? 0) + 1
    })
    return counts
  }, [report])
  const codeList = useMemo(() => Object.keys(codeCounts).sort(), [codeCounts])
  const hiddenCodeCount = codeList.filter((c) => codeOn[c] === false).length

  const rows: IssueRow[] = useMemo(
    () => (report ? report.issues.map((i, originalIndex) => ({ ...i, _key: originalIndex })) : []),
    [report],
  )

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase()
    return rows.filter((i) => {
      if (severityOn[i.severity] === false) return false
      if (analyzerFilter && i.analyzer !== analyzerFilter) return false
      if (codeOn[i.code] === false) return false
      if (term && !(i.url.toLowerCase().includes(term) || i.message.toLowerCase().includes(term) || i.code.toLowerCase().includes(term))) {
        return false
      }
      const r = review[i._key]
      if (hideResolved && r?.resolved) return false
      if (hideNonissue && r?.nonissue) return false
      return true
    })
  }, [rows, severityOn, analyzerFilter, codeOn, search, hideResolved, hideNonissue, review])

  const { sorted, toggle, dirFor } = useSort(filtered, issueSortValue, { key: 'severity' })

  const selectedShownCount = sorted.filter((r) => selected.has(r._key)).length
  useEffect(() => {
    if (!selectAllRef.current) return
    selectAllRef.current.indeterminate = selectedShownCount > 0 && selectedShownCount < sorted.length
  }, [selectedShownCount, sorted.length])

  if (error) return <Notice tone="error">{error}</Notice>
  if (!job) return <p className="muted">Loading…</p>

  const emptyReview: ReviewState = { resolved: false, nonissue: false, note: '' }

  function setReviewFlag(key: number, patch: Partial<ReviewState>) {
    setReview((prev) => ({
      ...prev,
      [key]: { ...emptyReview, ...prev[key], ...patch },
    }))
  }

  function toggleSelected(key: number) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function selectShown(checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      sorted.forEach((r) => (checked ? next.add(r._key) : next.delete(r._key)))
      return next
    })
  }

  function bulkApply(kind: 'resolved' | 'nonissue' | 'clear') {
    setReview((prev) => {
      const next = { ...prev }
      sorted.forEach((r) => {
        if (!selected.has(r._key)) return
        if (kind === 'clear') {
          next[r._key] = { ...emptyReview, note: prev[r._key]?.note ?? '' }
        } else {
          next[r._key] = { ...emptyReview, ...prev[r._key], [kind]: true }
        }
      })
      return next
    })
    setSelected(new Set())
  }

  function clearFilters() {
    setSeverityOn({ error: true, warning: true, info: true })
    setAnalyzerFilter('')
    setSearch('')
    setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))
    setHideResolved(false)
    setHideNonissue(false)
  }

  function exportReviewed() {
    if (!report) return
    const annotated = {
      ...report,
      issues: report.issues.map((issue, idx) => {
        const r = review[idx]
        if (!r || (!r.resolved && !r.nonissue && !r.note.trim())) return issue
        return { ...issue, review: r }
      }),
    }
    const blob = new Blob([JSON.stringify(annotated, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'gocrawl-report-reviewed.json'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(a.href)
  }

  const filtersActive =
    search.trim() !== '' ||
    analyzerFilter !== '' ||
    hiddenCodeCount > 0 ||
    hideResolved ||
    hideNonissue ||
    Object.values(severityOn).some((v) => v === false)

  const pill = STATUS_PILL[job.status]

  return (
    <div className="cr">
      <header className="cr-head">
        <h1>{hostOf(job.seed)}</h1>
        <PhasePill phase={pill.phase}>{pill.label}</PhasePill>
        <div className="cr-actions">
          {job.status === 'running' && (
            <Button
              variant="stop"
              onClick={() => {
                if (!window.confirm('Stop this crawl? The report keeps the pages fetched so far.')) return
                cancelCrawl(id).catch((e) => setError(String(e)))
              }}
            >
              Stop crawl
            </Button>
          )}
          {report && (
            <>
              <a className="btn btn-secondary" href={exportURL(id, 'json')}>
                Export JSON
              </a>
              <a className="btn btn-secondary" href={exportURL(id, 'csv')}>
                Export CSV
              </a>
              <a className="btn btn-secondary" href={exportURL(id, 'html')}>
                Export HTML
              </a>
              <Button onClick={exportReviewed} title="Download this report with your review marks and comments merged in">
                Export reviewed JSON
              </Button>
            </>
          )}
        </div>
      </header>
      <p className="cr-seed mono">{job.seed}</p>

      {job.status === 'running' && job.started_at && (
        <div className="cr-stats">
          <div className="cr-stat">
            <b className="num">{formatElapsed(now - Date.parse(job.started_at))}</b>
            <span>elapsed</span>
          </div>
        </div>
      )}

      {job.error && <Notice tone="error">{job.error}</Notice>}

      {report && (
        <>
          {report.coverage && !report.coverage.complete && (
            <Notice tone="warning">
              <p>
                <strong>Partial coverage: this crawl did not reach the whole site.</strong>{' '}
                {report.coverage.duration_limit_reached ? (
                  <>
                    The crawl stopped when it reached its max duration. Findings cover only what was fetched in that
                    time, and many pages may not have been discovered at all. Raise the max duration (or leave it
                    empty for no limit) and crawl again for full coverage.
                  </>
                ) : report.coverage.interrupted ? (
                  <>
                    The crawl was stopped before it finished (for example with Stop crawl). Findings cover only what
                    was fetched before that, and many pages may not have been discovered at all. Crawl again and let
                    it finish for full coverage.
                  </>
                ) : (
                  <>
                    {report.coverage.discovered_not_crawled ?? 0} in-scope URL
                    {report.coverage.discovered_not_crawled !== 1 ? 's were' : ' was'} discovered but not crawled
                    {report.coverage.page_limit_reached && (
                      <>
                        , because the page limit (<code>--max-pages {report.coverage.max_pages}</code>) was reached
                      </>
                    )}
                    {report.coverage.depth_limit_reached && (
                      <>
                        {report.coverage.page_limit_reached ? ' and' : ', because'} the depth limit (
                        <code>--depth {report.coverage.max_depth}</code>) was reached
                      </>
                    )}
                    . Page-level findings, broken links especially, may be incomplete. Crawl again with a higher
                    limit (or <code>0</code> for no limit) for full coverage.
                  </>
                )}
              </p>
            </Notice>
          )}

          {(() => {
            // runner.Run appends the same coverageNote() text to report.notes whenever
            // coverage is incomplete, always prefixed "partial coverage: ", which the
            // notice above already covers more richly. Filter it out here so the web view
            // doesn't show it twice; report.html.tmpl never renders .notes at all, so this
            // duplication is specific to the live view.
            const otherNotes = (report.notes ?? []).filter((n) => !n.startsWith('partial coverage:'))
            return (
              otherNotes.length > 0 && (
                <Notice tone="info">
                  <ul className="cr-notes">
                    {otherNotes.map((n, i) => (
                      <li key={i}>{n}</li>
                    ))}
                  </ul>
                </Notice>
              )
            )
          })()}

          <div className="cr-stats">
            <div className="cr-stat">
              <b className="num">{report.pages_crawled.toLocaleString()}</b>
              <span>pages crawled</span>
            </div>
            {(['error', 'warning', 'info'] as const)
              .filter((sev) => report.summary.by_severity[sev] !== undefined)
              .map((sev) => (
                <div className="cr-stat" key={sev}>
                  <b className={`num sev-${sev}`}>{report.summary.by_severity[sev].toLocaleString()}</b>
                  <span>{sev === 'error' ? 'errors' : sev === 'warning' ? 'warnings' : 'info'}</span>
                </div>
              ))}
          </div>

          <div className="cr-panels">
            <section className="cr-panel">
              <h2>By analyzer</h2>
              {Object.keys(report.summary.by_analyzer).length > 0 ? (
                <table>
                  <tbody>
                    {Object.entries(report.summary.by_analyzer).map(([a, count]) => (
                      <tr key={a}>
                        <td className="mono">{a}</td>
                        <td className="num cr-count">{count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="muted">No analyzer reported an issue.</p>
              )}
            </section>
            <section className="cr-panel">
              <h2>Pages by status</h2>
              {Object.keys(report.summary.pages_by_status).length > 0 ? (
                <table>
                  <tbody>
                    {Object.entries(report.summary.pages_by_status).map(([s, count]) => (
                      <tr key={s}>
                        <td>{/^\d+$/.test(s) ? <StatusCode code={Number(s)} /> : s}</td>
                        <td className="num cr-count">{count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="muted">No pages were crawled.</p>
              )}
            </section>
          </div>

          {report.site_map && (
            <div className="cr-tabs">
              <button
                type="button"
                className={`cr-tab${tab === 'issues' ? ' cr-tab-on' : ''}`}
                aria-pressed={tab === 'issues'}
                onClick={() => setTab('issues')}
              >
                Issues
              </button>
              <button
                type="button"
                className={`cr-tab${tab === 'sitemap' ? ' cr-tab-on' : ''}`}
                aria-pressed={tab === 'sitemap'}
                onClick={() => setTab('sitemap')}
              >
                Site map
              </button>
            </div>
          )}

          {tab === 'sitemap' && report.site_map ? (
            <SiteMap siteMap={report.site_map} />
          ) : (
            <>
              <div className="cr-filters">
                <div className="cr-sevs" role="group" aria-label="Severity">
                  {(['error', 'warning', 'info'] as const).map((sev) => (
                    <button
                      key={sev}
                      type="button"
                      className={`cr-sev sev-${sev}`}
                      aria-pressed={severityOn[sev]}
                      onClick={() => setSeverityOn((prev) => ({ ...prev, [sev]: !prev[sev] }))}
                    >
                      {sev.charAt(0).toUpperCase() + sev.slice(1)}
                    </button>
                  ))}
                </div>
                <label className="cr-inline">
                  Analyzer
                  <select value={analyzerFilter} onChange={(e) => setAnalyzerFilter(e.target.value)}>
                    <option value="">All</option>
                    {Object.keys(report.summary.by_analyzer).map((a) => (
                      <option key={a} value={a}>
                        {a}
                      </option>
                    ))}
                  </select>
                </label>
                <div className={`cr-codes${hiddenCodeCount > 0 ? ' cr-codes-hidden' : ''}`}>
                  <Button
                    aria-expanded={codeMenuOpen}
                    onClick={(e) => {
                      e.stopPropagation()
                      setCodeMenuOpen((v) => !v)
                    }}
                  >
                    {hiddenCodeCount > 0 ? `Codes (${hiddenCodeCount} hidden)` : 'Codes'}
                    <span aria-hidden="true">▾</span>
                  </Button>
                  <div className="cr-codes-panel" hidden={!codeMenuOpen} onClick={(e) => e.stopPropagation()}>
                    <div className="cr-codes-actions">
                      <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))}>
                        Show all
                      </Button>
                      <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, false])))}>
                        Hide all
                      </Button>
                    </div>
                    {codeList.map((c) => (
                      <label key={c} className="cr-code-option">
                        <input
                          type="checkbox"
                          checked={codeOn[c] !== false}
                          onChange={(e) => setCodeOn((prev) => ({ ...prev, [c]: e.target.checked }))}
                        />
                        <code>{c}</code>
                        <span className="num muted">{codeCounts[c]}</span>
                      </label>
                    ))}
                  </div>
                </div>
                <input
                  type="search"
                  className="cr-search"
                  placeholder="Search URL, message, code…"
                  aria-label="Search issues"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                <label className="cr-inline">
                  <input type="checkbox" checked={hideResolved} onChange={(e) => setHideResolved(e.target.checked)} />
                  Hide resolved
                </label>
                <label className="cr-inline">
                  <input type="checkbox" checked={hideNonissue} onChange={(e) => setHideNonissue(e.target.checked)} />
                  Hide non-issues
                </label>
                <span className="cr-count-line num muted">
                  Showing {sorted.length.toLocaleString()} of {rows.length.toLocaleString()} issues
                </span>
                {filtersActive && <Button onClick={clearFilters}>Clear filters</Button>}
              </div>

              <div className="cr-bulk">
                <label className="cr-inline">
                  <input
                    ref={selectAllRef}
                    type="checkbox"
                    checked={sorted.length > 0 && selectedShownCount === sorted.length}
                    onChange={(e) => selectShown(e.target.checked)}
                  />
                  Select shown
                </label>
                <Button disabled={selected.size === 0} onClick={() => bulkApply('nonissue')}>
                  Mark as non-issue
                </Button>
                <Button disabled={selected.size === 0} onClick={() => bulkApply('resolved')}>
                  Mark as resolved
                </Button>
                <Button disabled={selected.size === 0} onClick={() => bulkApply('clear')}>
                  Clear review
                </Button>
                {selected.size > 0 && <span className="num muted">{selected.size} selected</span>}
              </div>

              <div className="cr-table-scroll">
                <table className="cr-issues">
                  <thead>
                    <tr>
                      {(['severity', 'analyzer', 'code', 'url', 'message'] as const).map((col) => (
                        <th
                          key={col}
                          className="cr-sortable"
                          onClick={() => toggle(col)}
                          data-dir={dirFor(col)}
                          aria-sort={dirFor(col) === 'asc' ? 'ascending' : dirFor(col) === 'desc' ? 'descending' : undefined}
                        >
                          {col === 'url' ? 'URL' : col.charAt(0).toUpperCase() + col.slice(1)}
                        </th>
                      ))}
                      <th>Review</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sorted.map((i) => {
                      const explanation = explanations[i.code]
                      const r = review[i._key]
                      return (
                        <tr key={i._key} className={r?.resolved || r?.nonissue ? 'cr-reviewed' : undefined}>
                          <td className={`sev-${i.severity}`}>{i.severity}</td>
                          <td className="mono">{i.analyzer}</td>
                          <td>
                            <code>{i.code}</code>
                          </td>
                          <td className="cr-url mono" title={i.url}>
                            {i.url}
                          </td>
                          <td>
                            {i.message}
                            {explanation && (
                              <details className="cr-explain">
                                <summary>What this means and how to fix it</summary>
                                <dl>
                                  <dt>What it is</dt>
                                  <dd>{explanation.what}</dd>
                                  <dt>Impact</dt>
                                  <dd>{explanation.impact}</dd>
                                  <dt>How to fix it</dt>
                                  <dd>{explanation.fix}</dd>
                                </dl>
                              </details>
                            )}
                            {i.data && (
                              <details className="cr-explain">
                                <summary>Data</summary>
                                <pre>{JSON.stringify(i.data, null, 2)}</pre>
                              </details>
                            )}
                          </td>
                          <td className="cr-review">
                            <label className="cr-inline">
                              <input type="checkbox" checked={selected.has(i._key)} onChange={() => toggleSelected(i._key)} />
                              Select
                            </label>
                            <label className="cr-inline">
                              <input
                                type="checkbox"
                                checked={!!r?.resolved}
                                onChange={(e) => setReviewFlag(i._key, { resolved: e.target.checked })}
                              />
                              Resolved
                            </label>
                            <label className="cr-inline">
                              <input
                                type="checkbox"
                                checked={!!r?.nonissue}
                                onChange={(e) => setReviewFlag(i._key, { nonissue: e.target.checked })}
                              />
                              Non-issue
                            </label>
                            <textarea
                              rows={2}
                              placeholder="Add a comment…"
                              aria-label="Comment"
                              value={r?.note ?? ''}
                              onChange={(e) => setReviewFlag(i._key, { note: e.target.value })}
                            />
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 3: Create `web/src/components/CrawlReport.css`:**

```css
/* Crawl view: heading with state and actions, notices, counts, then the issues table. */
.cr-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 14px;
}

.cr-head h1 {
  margin: 0;
}

.cr-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-left: auto;
}

.cr-seed {
  margin: 4px 0 0;
  color: var(--ink-2);
  overflow-wrap: anywhere;
}

.cr-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 32px;
  margin: 16px 0 6px;
}

.cr-stat b {
  display: block;
  font-size: 28px;
  font-weight: 700;
  line-height: 1.1;
}

.cr-stat span {
  font-size: 13px;
  color: var(--ink-2);
}

.sev-error {
  color: var(--s5);
}

.sev-warning {
  color: var(--s3);
}

.sev-info {
  color: var(--ink-2);
}

.cr-notes {
  margin: 0;
  padding-left: 18px;
}

.cr-panels {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin: 16px 0;
}

.cr-panel {
  flex: 1 1 16rem;
  padding: 10px 14px;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.cr-panel h2 {
  margin: 0 0 6px;
  font-size: 15px;
}

.cr-panel td {
  padding: 3px 0;
  border-top: 1px solid var(--rule);
}

.cr-panel tr:first-child td {
  border-top: 0;
}

.cr-count {
  text-align: right;
  font-weight: 600;
}

.cr-tabs {
  display: flex;
  gap: 4px;
  margin: 20px 0 12px;
  border-bottom: 1px solid var(--rule);
}

.cr-tab {
  margin: 0 0 -1px;
  padding: 8px 12px;
  font-weight: 600;
  color: var(--ink-2);
  background: none;
  border: 0;
  border-bottom: 3px solid transparent;
  cursor: pointer;
}

.cr-tab:hover {
  color: var(--ink);
}

.cr-tab-on {
  color: var(--ink);
  border-bottom-color: var(--violet);
}

.cr-filters,
.cr-bulk {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin: 0 0 10px;
  font-size: 14px;
}

.cr-inline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.cr-sevs {
  display: flex;
  gap: 6px;
}

.cr-sev {
  min-height: 32px;
  padding: 4px 10px;
  font-weight: 600;
  background: var(--surface);
  border: 1px solid var(--field);
  border-radius: 5px;
  cursor: pointer;
}

.cr-sev[aria-pressed='true'] {
  background: var(--raised);
  border-color: currentColor;
}

.cr-sev[aria-pressed='false'] {
  color: var(--ink-2);
  text-decoration: line-through;
}

.cr-codes {
  position: relative;
}

.cr-codes-hidden > .btn {
  border-color: var(--violet);
}

.cr-codes-panel {
  position: absolute;
  z-index: 30;
  top: calc(100% + 4px);
  left: 0;
  min-width: 16rem;
  max-width: 24rem;
  max-height: 20rem;
  overflow: auto;
  padding: 6px;
  background: var(--surface);
  border: 1px solid var(--field);
  border-radius: 5px;
}

.cr-codes-panel[hidden] {
  display: none;
}

.cr-codes-actions {
  display: flex;
  gap: 8px;
  padding: 4px 4px 8px;
  margin: 0 0 6px;
  border-bottom: 1px solid var(--rule);
}

.cr-code-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
}

.cr-code-option:hover {
  background: var(--raised);
}

.cr-code-option .num {
  margin-left: auto;
}

.cr-search {
  min-width: 14rem;
}

.cr-count-line {
  margin-left: auto;
}

.cr-table-scroll {
  max-height: 70vh;
  overflow: auto;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.cr-issues th,
.cr-issues td {
  padding: 6px 10px;
  vertical-align: top;
  border-top: 1px solid var(--rule);
}

.cr-issues thead th {
  position: sticky;
  top: 0;
  z-index: 5;
  white-space: nowrap;
  background: var(--surface);
  border-top: 0;
  border-bottom: 1px solid var(--rule);
}

.cr-sortable {
  cursor: pointer;
  user-select: none;
}

.cr-sortable:hover {
  color: var(--ink);
}

.cr-sortable[data-dir='asc']::after {
  content: ' \25B2';
  font-size: 0.7em;
}

.cr-sortable[data-dir='desc']::after {
  content: ' \25BC';
  font-size: 0.7em;
}

.cr-url {
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cr-explain {
  margin-top: 4px;
}

.cr-explain > summary {
  width: fit-content;
  font-size: 13px;
  font-weight: 600;
  color: var(--violet);
  cursor: pointer;
}

.cr-explain dl,
.cr-explain pre {
  margin: 6px 0 0;
  padding: 8px 12px;
  font-size: 13px;
  background: var(--ground);
  border: 1px solid var(--rule);
  border-left: 3px solid var(--violet);
  border-radius: 5px;
}

.cr-explain pre {
  overflow-x: auto;
}

.cr-explain dt {
  margin-top: 8px;
  font-weight: 700;
}

.cr-explain dt:first-child {
  margin-top: 0;
}

.cr-explain dd {
  margin: 2px 0 0;
}

.cr-review {
  white-space: nowrap;
  font-size: 13px;
}

.cr-review .cr-inline {
  display: flex;
  margin-bottom: 3px;
}

.cr-review textarea {
  width: 12rem;
  max-width: 100%;
  margin-top: 4px;
  font-size: 13px;
}

.cr-reviewed {
  opacity: 0.55;
}

.cr-reviewed:hover {
  opacity: 1;
}

.cr-reviewed .cr-url {
  text-decoration: line-through;
}
```

- [ ] **Step 4: Build and lint.** `cd web && npm run build && npm run lint` — expected `✓ built`,
  lint passes.

- [ ] **Step 5: Manual check.** `make web-build && go run ./cmd/gocrawl serve`:
  - Start a crawl of `https://example.com` with max pages 5. While it runs: heading
    `example.com`, a "Crawling" pill with a pulsing green dot, a red "Stop crawl" button, an
    elapsed counter. With reduced motion emulated, the dot does not pulse.
  - After it finishes: pill "Finished"; export links and "Export reviewed JSON" styled as
    buttons; counts with errors in red and warnings in yellow; "Pages by status" shows `200`
    in the mono face.
  - Start a crawl of a large site, press "Stop crawl", confirm: pill "Crawl stopped" and a
    yellow-edged partial-coverage notice that mentions Stop crawl.
  - Severity toggles show pressed state and strike through when off; sorting a column shows
    the arrow and `aria-sort`; "What this means and how to fix it" expands.

- [ ] **Step 6: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 7: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src/crawlStatus.ts web/src/components/CrawlReport.tsx web/src/components/CrawlReport.css
  git commit -F - <<'EOF'
  feat(web): restyle the crawl view

  The crawl page leads with the host, a phase pill, and a red Stop crawl button; the
  partial-coverage banner and run notes become notices, page statuses use StatusCode, and
  the filters and issue table move to CrawlReport.css on the design tokens.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

#### Task W1-VIS-8: Restyle Crawls and the site map, retire index.css, wave 1 docs

**Files:**
- Modify (rewrite): `web/src/components/History.tsx`
- Create: `web/src/components/History.css`, `web/src/components/SiteMap.css`
- Modify: `web/src/components/SiteMap.tsx`, `web/src/main.tsx`
- Delete: `web/src/index.css`
- Modify: `docs/web.md`, `docs/README.md` (generated), `CHANGELOG.md`

**Interfaces:**
- Consumes: `Notice`, `PhasePill`, `Button`, `StatusCode`, `STATUS_PILL`, `href`.
- Produces: unchanged component APIs `History({ onSelect })` and `SiteMap({ siteMap })`; CSS
  class prefixes `hist-` and `sm-`. Empty state: "No saved crawls yet. Turn on 'Save to
  Crawls' when you start a crawl." (with a link to New crawl).

- [ ] **Step 1: Replace `web/src/components/History.tsx`:**

```tsx
import { useEffect, useState } from 'react'
import { listCrawls } from '../api/crawls'
import { STATUS_PILL } from '../crawlStatus'
import { href } from '../routes'
import { useSort } from '../sort'
import type { CrawlListItem } from '../types/crawls'
import Notice from './ui/Notice'
import PhasePill from './ui/PhasePill'
import './History.css'

function historySortValue(item: CrawlListItem, key: string): string | number {
  switch (key) {
    case 'seed':
      return item.seed
    case 'status':
      return item.status
    case 'pages_crawled':
      return item.pages_crawled ?? -1
    case 'errors':
      return item.by_severity?.error ?? 0
    case 'warnings':
      return item.by_severity?.warning ?? 0
    case 'finished_at':
      return item.finished_at ?? ''
    default:
      return ''
  }
}

function formatFinished(finishedAt?: string): string {
  if (!finishedAt) return '—'
  const d = new Date(finishedAt)
  return Number.isNaN(d.getTime()) ? finishedAt : d.toLocaleString()
}

const COLUMNS: { key: string; label: string; numeric?: boolean }[] = [
  { key: 'seed', label: 'Site' },
  { key: 'status', label: 'Status' },
  { key: 'pages_crawled', label: 'Pages', numeric: true },
  { key: 'errors', label: 'Errors', numeric: true },
  { key: 'warnings', label: 'Warnings', numeric: true },
  { key: 'finished_at', label: 'Finished' },
]

export default function History({ onSelect }: { onSelect: (id: string) => void }) {
  const [items, setItems] = useState<CrawlListItem[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')

  useEffect(() => {
    listCrawls()
      .then((r) => setItems(r.crawls))
      .catch((e) => setError(String(e)))
      .finally(() => setLoaded(true))
  }, [])

  const filtered = items.filter((item) => item.seed.toLowerCase().includes(search.trim().toLowerCase()))
  const { sorted, toggle, dirFor } = useSort(filtered, historySortValue)

  if (error) return <Notice tone="error">Could not load your crawls: {error}. Reload the page to try again.</Notice>
  if (!loaded) return <p className="muted">Loading…</p>

  return (
    <div className="hist">
      <h1>Crawls</h1>
      {items.length === 0 ? (
        <p className="hist-empty">
          No saved crawls yet. Turn on &lsquo;Save to Crawls&rsquo; when you <a href={href({ view: 'new' })}>start a crawl</a>.
        </p>
      ) : (
        <>
          <div className="hist-filters">
            <input
              type="search"
              placeholder="Search by site URL…"
              aria-label="Search crawls"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <span className="num muted">
              Showing {sorted.length.toLocaleString()} of {items.length.toLocaleString()}
            </span>
          </div>
          <div className="hist-table">
            <table>
              <thead>
                <tr>
                  {COLUMNS.map((col) => (
                    <th
                      key={col.key}
                      className={`hist-sortable${col.numeric ? ' hist-num' : ''}`}
                      onClick={() => toggle(col.key)}
                      data-dir={dirFor(col.key)}
                      aria-sort={dirFor(col.key) === 'asc' ? 'ascending' : dirFor(col.key) === 'desc' ? 'descending' : undefined}
                    >
                      {col.label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {sorted.map((item) => {
                  const pill = STATUS_PILL[item.status]
                  return (
                    <tr key={item.id} className="hist-row" onClick={() => onSelect(item.id)}>
                      <td>
                        <a className="mono" href={href({ view: 'crawl', id: item.id })} onClick={(e) => e.stopPropagation()}>
                          {item.seed}
                        </a>
                      </td>
                      <td>
                        <PhasePill phase={pill.phase}>{pill.label}</PhasePill>
                      </td>
                      <td className="hist-num num">{item.pages_crawled?.toLocaleString() ?? '—'}</td>
                      <td className="hist-num num">{item.by_severity?.error ?? 0}</td>
                      <td className="hist-num num">{item.by_severity?.warning ?? 0}</td>
                      <td className="muted">{formatFinished(item.finished_at)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Create `web/src/components/History.css`:**

```css
/* Crawls list. */
.hist-empty {
  color: var(--ink-2);
}

.hist-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin: 0 0 12px;
  font-size: 14px;
}

.hist-filters input {
  min-width: 16rem;
}

.hist-table {
  overflow-x: auto;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.hist-table th,
.hist-table td {
  padding: 7px 12px;
  border-top: 1px solid var(--rule);
}

.hist-table thead th {
  white-space: nowrap;
  border-top: 0;
}

.hist-num {
  text-align: right;
}

.hist-sortable {
  cursor: pointer;
  user-select: none;
}

.hist-sortable:hover {
  color: var(--ink);
}

.hist-sortable[data-dir='asc']::after {
  content: ' \25B2';
  font-size: 0.7em;
}

.hist-sortable[data-dir='desc']::after {
  content: ' \25BC';
  font-size: 0.7em;
}

.hist-row {
  cursor: pointer;
}

.hist-row:hover {
  background: var(--raised);
}
```

- [ ] **Step 3: Move the site-map styles out of `index.css` onto the tokens.** The site-map
  block runs from the `/* Site map: node-link` comment to the end of `index.css`. Extract it,
  drop the rules the shared components now own (`.sm-controls button`, the `.sm-st` status
  badges), remove uppercase, letter-spacing, and shadows, and rename the old variables to
  tokens:

  ```sh
  cd web/src
  sed -n '/^\/\* Site map: node-link/,$p' index.css > /tmp/sm.css
  sed -e '/^\.sm-controls button {$/,/^}$/d' \
      -e '/^\.sm-st[ .]/,/^}$/d' \
      -e '/text-transform: uppercase;/d' \
      -e '/letter-spacing: 0\.0[0-9]*em;/d' \
      -e '/box-shadow: var(--shadow);/d' \
      -e 's/var(--card-bg)/var(--surface)/g' \
      -e 's/var(--border)/var(--rule)/g' \
      -e 's/var(--text-h)/var(--ink)/g' \
      -e 's/var(--text)/var(--ink-2)/g' \
      -e 's/var(--bg)/var(--ground)/g' \
      -e 's/var(--accent-2)/var(--violet)/g' \
      -e 's/var(--accent)/var(--violet)/g' \
      -e 's/var(--error)/var(--s5)/g' \
      -e 's/var(--warning)/var(--s3)/g' \
      -e 's/var(--info)/var(--ink-2)/g' \
      -e 's/var(--success)/var(--s2)/g' \
      -e 's/var(--fs-body-lg)/15px/g' \
      -e 's/var(--fs-body)/14px/g' \
      -e 's/var(--fs-caption)/13px/g' \
      /tmp/sm.css | cat -s > components/SiteMap.css
  grep -o 'var(--[a-z0-9-]*)' components/SiteMap.css | sort -u
  grep -nE 'sm-st|uppercase|var\(--shadow\)' components/SiteMap.css
  cd ../..
  ```

  Expected: the first `grep` lists only `--ground`, `--ink`, `--ink-2`, `--rule`, `--s2`,
  `--s3`, `--s5`, `--surface`, `--violet`; the second prints nothing.

- [ ] **Step 4: Edit `web/src/components/SiteMap.tsx`.**
  1. Replace the import block and the local `statusClass` function

     ```tsx
     import { useState } from 'react'
     import type { PageIssue, SevCounts, SiteMap as SiteMapData, SiteMapNode } from '../types/crawls'

     function statusClass(status?: number): string {
       if (!status) return 'st-none'
       if (status >= 200 && status < 300) return 'st-ok'
       if (status >= 300 && status < 400) return 'st-redirect'
       return 'st-error'
     }
     ```

     with

     ```tsx
     import { useState } from 'react'
     import type { PageIssue, SevCounts, SiteMap as SiteMapData, SiteMapNode } from '../types/crawls'
     import Button from './ui/Button'
     import StatusCode from './ui/StatusCode'
     import './SiteMap.css'
     ```
  2. Replace
     `          {node.status ? <span className={`sm-st ${statusClass(node.status)}`}>{node.status}</span> : null}`
     with `          {node.status ? <StatusCode code={node.status} /> : null}`.
  3. Replace

     ```tsx
             <button type="button" onClick={expandAll}>
               Expand all
             </button>
             <button type="button" onClick={collapseAll}>
               Collapse all
             </button>
     ```

     with

     ```tsx
             <Button onClick={expandAll}>Expand all</Button>
             <Button onClick={collapseAll}>Collapse all</Button>
     ```
  4. Replace `title="collapse / expand subtree"` with `title="Collapse or expand this branch"`.
  5. Replace
     `<span className="sm-hint">Click a node to see its issues · click a label to open the page · −/+ collapses a branch</span>`
     with
     `<span className="sm-hint">Click a page to see its issues. Click its label to open it. − and + collapse a branch.</span>`.
  6. Replace `<p className="empty">No site map available.</p>` with
     `<p className="muted">No site map available.</p>`.

- [ ] **Step 5: Retire the old stylesheet.** `git rm web/src/index.css`, then remove the line
  `import './index.css'` from `web/src/main.tsx`. The final `web/src/main.tsx`:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/tokens.css'
import './styles/fonts.css'
import './styles/base.css'
import App from './App.tsx'
import { applyTheme } from './theme'

// Dark until the Settings feature loads the saved theme from the server.
applyTheme('dark')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 6: Build and lint.** `cd web && npm run build && npm run lint` — expected `✓ built`,
  lint passes. Check that no old class names are left without styles:
  `grep -rnE 'className="(card|error|empty|stat|filters|form-section)"' web/src` prints
  nothing.

- [ ] **Step 7: Manual check, both themes.** `make web-build`, then
  `go run ./cmd/gocrawl serve --store-dir "$(mktemp -d)"`:
  - `#/crawls` shows "No saved crawls yet. Turn on 'Save to Crawls' when you start a crawl."
    with "start a crawl" linking to `#/`.
  - Run two saved crawls; the list shows the seed in mono as a link, a status pill, right-
    aligned counts; rows highlight on hover; Tab reaches the seed links.
  - Open one, choose "Site map": node status codes use the status colors in the mono face,
    "Expand all" and "Collapse all" are standard buttons, the issue pop-up header is sentence
    case.
  - In the console run `document.documentElement.dataset.theme = 'light'` and revisit every
    view (New crawl, Crawls, a crawl, Site map, the stubs): light palette, all text readable.
    Then `'system'` with the rendering panel's "prefers-color-scheme: light" and "dark"
    emulations: the page follows the emulation.

- [ ] **Step 8: Check the embedded-UI flow is untouched.**

  ```sh
  make web-build
  git status --short internal/webserver/webui/dist   # expect only: " M internal/webserver/webui/dist/index.html"
  git restore internal/webserver/webui/dist/index.html
  git ls-files internal/webserver/webui/dist          # expect only: internal/webserver/webui/dist/index.html
  go build ./... && make build                        # embeds whatever dist/ holds; no Node needed
  ```

- [ ] **Step 9: Update the docs.** In `docs/web.md` (its `updated:` is already `2026-10-13`
  from W0-5), insert this section directly above `## Source layout`:

  ```markdown
  ## Look and feel

  The UI is dark by default. Color means HTTP status (2xx green, 3xx yellow, 4xx dark orange,
  5xx red) and violet means interface: navigation, links, focus, and the main actions. Crawl
  controls are green to start or resume, yellow to pause, and red to stop. Text is set in
  Atkinson Hyperlegible Next and URLs and status codes in Atkinson Hyperlegible Mono, both
  bundled into the binary under the SIL Open Font License
  ([`web/src/fonts/OFL.txt`](../web/src/fonts/OFL.txt)), so nothing loads from a CDN.

  For frontend work:

  - Colors come from the tokens in [`styles/tokens.css`](../web/src/styles/tokens.css), one
    block per theme: dark, light, and system (follows the operating system).
    [`theme.ts`](../web/src/theme.ts) switches theme by setting `data-theme` on `<html>`.
  - `npm run lint` runs oxlint and then [`scripts/contrast.mjs`](../web/scripts/contrast.mjs),
    which fails when a text or control color in either theme drops below WCAG 2.2 AA against
    the background it is used on (4.5:1 for text, 3:1 for input borders and outline ticks).
  - Shared building blocks live in `components/ui/` (`Button`, `Notice`, `StatusCode`,
    `PhasePill`) and `components/tape/` (the crawl tape: one tick per page, its height and
    color giving the status, so it reads without color too). Each component imports the
    stylesheet next to it; global element styles and the page layout are in
    [`styles/base.css`](../web/src/styles/base.css).
  - Sentence case throughout: no all-caps labels.
  ```

  In the same file, in the **Report** bullet of "What the UI does", replace
  `polls the running job, then shows` with
  `polls the running job (with a Stop crawl button while it runs), then shows`.

  Run `make docs` (expected `ok`; `docs/web.md`'s `sources:` gains the five new links).

  `CHANGELOG.md`: under `## [Unreleased]`, in `### Changed`, add as the first bullet:

  ```markdown
  - The web UI has a new look: dark by default (light and system themes are built in), a
    left navigation rail, bundled Atkinson Hyperlegible fonts, HTTP-status colors, and green,
    yellow, and red crawl controls. Cancel is now Stop crawl. `npm run lint` in `web/` also
    checks every color pair against WCAG 2.2 AA.
  ```

- [ ] **Step 10: Run the full check.** `make check` — expected `0 issues.`

- [ ] **Step 11: Commit.**

  ```sh
  git restore internal/webserver/webui/dist/index.html
  git add web/src docs CHANGELOG.md
  git commit -F - <<'EOF'
  feat(web): restyle Crawls and the site map, retire the old stylesheet

  The Crawls list gets status pills, mono seed links, and an empty state that says how to
  save a crawl; the site map moves to SiteMap.css on the tokens and uses StatusCode and
  Button. index.css and its variable aliases are gone. docs/web.md describes the visual
  system and how the contrast check guards it.

  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  EOF
  ```

---

### Track: progress backend

#### Task W1-PROG-1: Throttle window, Retry-After memory and effective rate on AdaptiveLimiter

**Files:**
- Modify: `internal/crawler/adaptive.go`
- Test: `internal/crawler/adaptive_test.go`

**Interfaces:**
- Consumes: existing `AdaptiveLimiter`, `retryAfterSeconds`.
- Produces:
  - `const throttleWindow = 60 * time.Second`
  - `func (a *AdaptiveLimiter) RecentThrottled() int` — 429/503 responses seen in the last 60s,
    counted even when adaptive delay is off.
  - `func (a *AdaptiveLimiter) LastRetryAfter() int` — latest Retry-After (seconds, rounded up,
    clamped to 5 min), 0 if none.
  - `func (a *AdaptiveLimiter) EffectiveRate() float64` — rate limit in effect (0 = unlimited).

- [ ] **Step 1: Write the failing tests.** Add `"time"` to the import block of
  `internal/crawler/adaptive_test.go`, then append:

```go
func TestAdaptiveLimiterRecentThrottledCountsEvenWhenDisabled(t *testing.T) {
	l := NewAdaptiveLimiter(0, false)
	l.OnResponse(http.StatusTooManyRequests, nil)
	l.OnResponse(http.StatusServiceUnavailable, nil)
	l.OnResponse(http.StatusOK, nil)
	if got := l.RecentThrottled(); got != 2 {
		t.Fatalf("RecentThrottled = %d, want 2 (429 and 503 count, 200 does not)", got)
	}
}

func TestAdaptiveLimiterRecentThrottledDropsEntriesOlderThanWindow(t *testing.T) {
	l := NewAdaptiveLimiter(0, true)
	l.mu.Lock()
	l.recent = []time.Time{time.Now().Add(-2 * throttleWindow)}
	l.mu.Unlock()
	l.OnResponse(http.StatusTooManyRequests, nil)
	if got := l.RecentThrottled(); got != 1 {
		t.Fatalf("RecentThrottled = %d, want 1 (the old entry is outside the window)", got)
	}
}

func TestAdaptiveLimiterLastRetryAfter(t *testing.T) {
	l := NewAdaptiveLimiter(0, true)
	if got := l.LastRetryAfter(); got != 0 {
		t.Fatalf("LastRetryAfter before any 429 = %d, want 0", got)
	}
	h := http.Header{}
	h.Set("Retry-After", "120")
	l.OnResponse(http.StatusTooManyRequests, h)
	if got := l.LastRetryAfter(); got != 120 {
		t.Fatalf("LastRetryAfter = %d, want 120", got)
	}
	l.OnResponse(http.StatusTooManyRequests, nil) // no header: the last value stays
	if got := l.LastRetryAfter(); got != 120 {
		t.Fatalf("LastRetryAfter after a 429 without Retry-After = %d, want 120", got)
	}
}

func TestAdaptiveLimiterEffectiveRate(t *testing.T) {
	if got := NewAdaptiveLimiter(0, true).EffectiveRate(); got != 0 {
		t.Fatalf("unlimited EffectiveRate = %v, want 0", got)
	}
	l := NewAdaptiveLimiter(4, true)
	if got := l.EffectiveRate(); got != 4 {
		t.Fatalf("EffectiveRate before backoff = %v, want the configured 4", got)
	}
	l.OnResponse(http.StatusTooManyRequests, nil)
	if got := l.EffectiveRate(); got != 2 {
		t.Fatalf("EffectiveRate after one backoff = %v, want 2", got)
	}
}
```

- [ ] **Step 2: Run them and watch them fail.**
  `go test ./internal/crawler -run 'TestAdaptiveLimiter(RecentThrottled|LastRetryAfter|EffectiveRate)'`
  Expected: build failure, `l.RecentThrottled undefined (type *AdaptiveLimiter has no field or method RecentThrottled)`.

- [ ] **Step 3: Implement.** In `internal/crawler/adaptive.go`:

  Add to the `const` block (after `maxRetryAfter`):

```go
	// throttleWindow is how far back RecentThrottled counts 429/503 responses. Live progress
	// shows the count, and auto-pause (Options.PauseOnThrottle) triggers on it.
	throttleWindow = 60 * time.Second
```

  Add two fields at the end of `AdaptiveLimiter` (inside the `mu`-guarded group):

```go
	// recent holds the arrival time of every 429/503 response within throttleWindow, oldest
	// first. Pruned on every write and read.
	recent []time.Time
	// lastRetryAfter is the most recent Retry-After (seconds, clamped) a 429/503 carried.
	lastRetryAfter float64
```

  Replace the whole `OnResponse` method with:

```go
// OnResponse inspects a fetch outcome and, if it signals overload (429/503), halves the current
// rate down to backoffMinRate, never faster than any Retry-After header demands. It reports
// whether this call triggered an adjustment (false if disabled, the status wasn't 429/503, or a
// trigger already landed within backoffDebounce) and the resulting rate. Every 429/503 is also
// recorded for RecentThrottled and LastRetryAfter, whether or not adaptive delay is enabled.
func (a *AdaptiveLimiter) OnResponse(statusCode int, header http.Header) (adjusted bool, newRate float64) {
	if statusCode != http.StatusTooManyRequests && statusCode != http.StatusServiceUnavailable {
		return false, 0
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	// Recorded whether or not adaptive delay is on: live progress reports how hard the server
	// is pushing back even when the operator turned backoff off.
	a.recent = append(pruneBefore(a.recent, now.Add(-throttleWindow)), now)
	ra := retryAfterSeconds(header)
	if ra > 0 {
		a.lastRetryAfter = ra
	}

	if !a.enabled {
		return false, 0
	}
	if !a.lastAdjust.IsZero() && now.Sub(a.lastAdjust) < backoffDebounce {
		return false, 0
	}
	a.lastAdjust = now

	prev := a.curRate
	if prev <= 0 {
		prev = a.baseRate
	}
	var next float64
	if prev <= 0 {
		next = backoffStartRate // was previously unrestricted
	} else {
		next = prev / 2
	}
	// The floor only bounds our halving heuristic.
	if next < backoffMinRate {
		next = backoffMinRate
	}
	// An explicit Retry-After is a direct server instruction, so honor it even below the
	// heuristic floor: never go faster than it asks.
	if ra > 0 {
		if byRetry := 1.0 / ra; byRetry < next {
			next = byRetry
		}
	}
	a.curRate = next
	a.throttleCount++
	a.limiter.SetLimit(rate.Limit(next))
	return true, next
}

// pruneBefore drops the leading timestamps older than cutoff. ts is oldest first.
func pruneBefore(ts []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(ts) && ts[i].Before(cutoff) {
		i++
	}
	return ts[i:]
}

// RecentThrottled reports how many 429/503 responses arrived within the last throttleWindow.
func (a *AdaptiveLimiter) RecentThrottled() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.recent = pruneBefore(a.recent, time.Now().Add(-throttleWindow))
	return len(a.recent)
}

// LastRetryAfter reports the most recent Retry-After a 429/503 carried, in whole seconds
// (rounded up, clamped to maxRetryAfter), or 0 when none did.
func (a *AdaptiveLimiter) LastRetryAfter() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return int(math.Ceil(a.lastRetryAfter))
}

// EffectiveRate reports the requests-per-second limit in effect: the backed-off rate when
// adaptive delay has lowered it, otherwise the configured rate. 0 means unlimited.
func (a *AdaptiveLimiter) EffectiveRate() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.curRate > 0 {
		return a.curRate
	}
	return a.baseRate
}
```

- [ ] **Step 4: Run the tests.**
  `go test -race ./internal/crawler -run 'TestAdaptiveLimiter|TestRetryAfter|TestThrottle'`
  Expected: `ok  github.com/Patience-dot-devl/gocrawl/internal/crawler`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/adaptive.go internal/crawler/adaptive_test.go
git commit -F - <<'EOF'
feat(crawler): count recent 429/503 responses and expose the effective rate

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-2: crawler.Progress and Options.OnProgress

**Files:**
- Create: `internal/crawler/progress.go`, `internal/crawler/progress_test.go`
- Modify: `internal/crawler/types.go` (`Options`), `internal/crawler/engine.go`,
  `internal/crawler/frontier.go`

**Interfaces:**
- Consumes: `AdaptiveLimiter.EffectiveRate`, `AdaptiveLimiter.RecentThrottled` (W1-PROG-1).
- Produces:
  - `type Progress struct { Fetched, Queued int; CurrentURL, FinalURL string; Status int; DurationMS int64; Rate float64; Throttled int; Paused bool; PauseReason string; RetryAfterSeconds int }`
    with the contract's JSON tags plus `final_url,omitempty` and `duration_ms`.
  - `Options.OnProgress func(Progress)`
  - `func (f *frontier) queued() int`
  - unexported `(*Engine).emitPage(Progress)`, `(*Engine).fillLive(*Progress)`,
    `progressStatus(*Page) int`, fields `Engine.progMu`, `Engine.lastProgress`.

- [ ] **Step 1: Write the failing test** `internal/crawler/progress_test.go`:

```go
package crawler

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// TestEngineOnProgressMatchesRecordedPages: one event per recorded page, delivered in
// recording order (Fetched = 1, 2, 3, ...), so a consumer can append one tape tick per call.
func TestEngineOnProgressMatchesRecordedPages(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	var (
		mu     sync.Mutex
		events []Progress
	)
	opts := DefaultOptions()
	opts.MaxDepth = 1
	opts.Concurrency = 2
	opts.OnProgress = func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	}
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) != len(result.Pages) {
		t.Fatalf("got %d progress events, want one per recorded page (%d)", len(events), len(result.Pages))
	}
	byURL := map[string]Progress{}
	for i, ev := range events {
		if ev.Fetched != i+1 {
			t.Errorf("event %d has Fetched=%d, want %d (events must arrive in recording order)", i, ev.Fetched, i+1)
		}
		byURL[ev.CurrentURL] = ev
	}
	if ev := byURL[ts.URL+"/missing"]; ev.Status != http.StatusNotFound {
		t.Errorf("/missing reported status %d, want 404", ev.Status)
	}
	redir := byURL[ts.URL+"/redir"]
	if redir.Status != http.StatusMovedPermanently {
		t.Errorf("/redir reported status %d, want the first hop's 301", redir.Status)
	}
	if redir.FinalURL != ts.URL+"/target" {
		t.Errorf("/redir FinalURL = %q, want %q", redir.FinalURL, ts.URL+"/target")
	}
	if last := events[len(events)-1]; last.Queued != 0 {
		t.Errorf("last event Queued = %d, want 0", last.Queued)
	}
}

func TestEngineWithoutOnProgressStillCrawls(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	opts := DefaultOptions()
	opts.MaxDepth = 1
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil || len(result.Pages) == 0 {
		t.Fatalf("crawl without a progress callback: pages=%d err=%v", len(result.Pages), err)
	}
}
```

- [ ] **Step 2: Run and watch it fail.**
  `go test ./internal/crawler -run TestEngineOnProgress`
  Expected: build failure, `undefined: Progress` and `opts.OnProgress undefined`.

- [ ] **Step 3: Implement.**

  Create `internal/crawler/progress.go`:

```go
package crawler

// Progress is a snapshot of a running crawl, delivered through Options.OnProgress after each
// page is recorded (and, from the pause track on, on every pause and resume).
type Progress struct {
	// Fetched is the number of pages recorded so far, including this one.
	Fetched int `json:"fetched"`
	// Queued is the number of URLs waiting in the frontier.
	Queued int `json:"queued"`
	// CurrentURL is the URL just recorded, as requested; FinalURL is where it landed after
	// redirects.
	CurrentURL string `json:"current_url"`
	FinalURL   string `json:"final_url,omitempty"`
	// Status is the HTTP status of the page just recorded: the first hop's status when the
	// fetch was redirected (so a redirect reads as 3xx), otherwise the final status, and 0
	// when the fetch failed.
	Status int `json:"status"`
	// DurationMS is how long that fetch took.
	DurationMS int64 `json:"duration_ms"`
	// Rate is the requests-per-second limit in effect (0 = unlimited), after any backoff.
	Rate float64 `json:"rate"`
	// Throttled counts HTTP 429/503 responses in the last 60 seconds.
	Throttled int `json:"throttled"`
	// Paused, PauseReason ("user" or "rate_limited") and RetryAfterSeconds describe a pause.
	Paused            bool   `json:"paused"`
	PauseReason       string `json:"pause_reason,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

// progressStatus is the status a page reports in Progress.Status.
func progressStatus(p *Page) int {
	if len(p.Redirects) > 0 {
		return p.Redirects[0].Status
	}
	return p.StatusCode
}

// emitPage delivers a page event. The caller holds the crawl's result mutex, so page events
// reach OnProgress in recording order; progMu serializes them with state-only events.
func (e *Engine) emitPage(p Progress) {
	if e.opts.OnProgress == nil {
		return
	}
	e.progMu.Lock()
	defer e.progMu.Unlock()
	e.fillLive(&p)
	e.lastProgress = p
	e.opts.OnProgress(p)
}

// fillLive copies the fields that change between pages: the rate limit and recent throttling.
func (e *Engine) fillLive(p *Progress) {
	p.Rate = e.limiter.EffectiveRate()
	p.Throttled = e.limiter.RecentThrottled()
}
```

  In `internal/crawler/types.go`, add to the end of `Options` (after `MaxDuration`):

```go
	// OnProgress, when non-nil, is called after each page is recorded with a snapshot of the
	// crawl's progress. Calls are serialized and arrive in recording order, so Fetched rises by
	// exactly one per page event. It runs on a worker goroutine while the engine holds its
	// internal locks: it must return quickly and must never call back into the Engine.
	OnProgress func(Progress)
```

  In `internal/crawler/frontier.go`, add after `push`:

```go
// queued reports how many tasks are waiting to be handed to a worker.
func (f *frontier) queued() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queue)
}
```

  In `internal/crawler/engine.go`:

  1. Add to the `Engine` struct, after `seedHost string`:

```go

	// progMu serializes OnProgress calls and guards lastProgress, the snapshot that
	// state-only events (pause, resume) re-send with fresh live fields.
	progMu       sync.Mutex
	lastProgress Progress
```

  2. In `Crawl`, right after the `result := &Result{...}` literal, add:

```go
	e.progMu.Lock()
	e.lastProgress = Progress{}
	e.progMu.Unlock()
```

  3. In `process`, replace

```go
			visited[finalNorm] = true
		}
		mu.Unlock()
```

  with

```go
			visited[finalNorm] = true
		}
		if e.opts.OnProgress != nil {
			// Emitted under mu so events arrive in recording order: Fetched rises by exactly
			// one per event, which is what lets a consumer append one tape tick per call.
			e.emitPage(Progress{
				Fetched:    len(result.Pages),
				Queued:     fr.queued(),
				CurrentURL: page.RequestedURL,
				FinalURL:   page.FinalURL,
				Status:     progressStatus(page),
				DurationMS: page.Duration.Milliseconds(),
			})
		}
		mu.Unlock()
```

- [ ] **Step 4: Run.** `go test -race ./internal/crawler -run 'TestEngine'`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/progress.go internal/crawler/progress_test.go internal/crawler/types.go internal/crawler/engine.go internal/crawler/frontier.go
git commit -F - <<'EOF'
feat(crawler): report per-page progress through Options.OnProgress

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-3: Engine.SetSeed, InScope, Normalize and RobotsFor

**Files:**
- Modify: `internal/crawler/engine.go`
- Test: `internal/crawler/inscope_test.go` (create)

**Interfaces:**
- Produces:
  - `func (e *Engine) InScope(rawURL string) bool` (contract)
  - `func (e *Engine) SetSeed(seed string) (string, error)` (gap: InScope needs the seed host
    before `Crawl` sets it)
  - `func (e *Engine) Normalize(rawURL string) string` (gap: sitemap membership must use the
    crawl's own URL normalization)
  - `func (e *Engine) RobotsFor(ctx context.Context, rawURL string) *RobotsData` (gap: the
    pre-pass needs robots.txt `Sitemap:` lines before the crawl)

- [ ] **Step 1: Write the failing test** `internal/crawler/inscope_test.go`:

```go
package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestEngineInScopeBeforeCrawl(t *testing.T) {
	opts := DefaultOptions()
	opts.Exclude = []*regexp.Regexp{regexp.MustCompile(`/private`)}
	e := New(opts, NewHTTPFetcher(opts))
	seed, err := e.SetSeed("HTTPS://Example.com:443/")
	if err != nil {
		t.Fatalf("SetSeed: %v", err)
	}
	if seed != "https://example.com/" {
		t.Fatalf("SetSeed returned %q, want the normalized https://example.com/", seed)
	}
	cases := map[string]bool{
		"https://example.com/a":         true,
		"https://EXAMPLE.com/b/":        true,
		"https://shop.example.com/a":    false,
		"https://other.org/":            false,
		"https://example.com/private/x": false,
		"mailto:hi@example.com":         false,
	}
	for u, want := range cases {
		if got := e.InScope(u); got != want {
			t.Errorf("InScope(%q) = %v, want %v", u, got, want)
		}
	}

	opts.AllowSubdomains = true
	sub := New(opts, NewHTTPFetcher(opts))
	if _, err := sub.SetSeed("https://example.com/"); err != nil {
		t.Fatalf("SetSeed: %v", err)
	}
	if !sub.InScope("https://shop.example.com/a") {
		t.Error("with AllowSubdomains, a subdomain URL should be in scope")
	}
}

func TestEngineNormalize(t *testing.T) {
	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	if got := e.Normalize("https://Example.com/a/#top"); got != "https://example.com/a" {
		t.Errorf("Normalize = %q, want https://example.com/a", got)
	}
	opts.StripQuery = true
	strip := New(opts, NewHTTPFetcher(opts))
	if got := strip.Normalize("https://example.com/a?page=2"); got != "https://example.com/a" {
		t.Errorf("Normalize with StripQuery = %q, want https://example.com/a", got)
	}
}

func TestEngineRobotsForReadsSitemapLines(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\nSitemap: https://cdn.example/sitemap.xml\n")
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	rd := e.RobotsFor(context.Background(), ts.URL+"/")
	if rd == nil || !rd.Found {
		t.Fatalf("RobotsFor = %+v, want a found robots.txt", rd)
	}
	if len(rd.Sitemaps) != 1 || rd.Sitemaps[0] != "https://cdn.example/sitemap.xml" {
		t.Errorf("Sitemaps = %v, want [https://cdn.example/sitemap.xml]", rd.Sitemaps)
	}
	if got := e.RobotsFor(context.Background(), "::bad"); got != nil {
		t.Errorf("RobotsFor on an unparseable URL = %+v, want nil", got)
	}
}
```

- [ ] **Step 2: Run and watch it fail.** `go test ./internal/crawler -run 'TestEngine(InScope|Normalize|RobotsFor)'`
  Expected: build failure, `e.SetSeed undefined`.

- [ ] **Step 3: Implement** in `internal/crawler/engine.go`.

  In `Crawl`, replace

```go
	seed = normalizeURL(seed, e.opts.StripQuery)
	su, err := url.Parse(seed)
	if err != nil {
		return nil, err
	}
	e.seedHost = su.Host
```

  with

```go
	seed, err := e.SetSeed(seed)
	if err != nil {
		return nil, err
	}
```

  Add after `authHostAllowed`:

```go
// SetSeed normalizes seed and fixes the host scope that InScope, enqueue and the credential
// guard check against. Crawl calls it itself; call it first only when something needs InScope
// or RobotsFor before the crawl starts (runner.Run's sitemap pre-pass). It must not be called
// while a crawl is running. It returns the normalized seed.
func (e *Engine) SetSeed(seed string) (string, error) {
	seed = normalizeURL(seed, e.opts.StripQuery)
	su, err := url.Parse(seed)
	if err != nil {
		return "", err
	}
	e.seedHost = su.Host
	return seed, nil
}

// InScope reports whether rawURL falls inside the crawl scope: the same scheme, host,
// subdomain, include and exclude rules enqueue applies, after the same normalization. It does
// not consult robots.txt or depth. Call SetSeed (or Crawl) first.
func (e *Engine) InScope(rawURL string) bool {
	u, err := url.Parse(normalizeURL(rawURL, e.opts.StripQuery))
	if err != nil {
		return false
	}
	return e.inScope(u)
}

// Normalize canonicalizes rawURL the way the crawl does for deduplication, so a URL from
// outside the crawl (a sitemap <loc>) can be matched against recorded pages.
func (e *Engine) Normalize(rawURL string) string {
	return normalizeURL(rawURL, e.opts.StripQuery)
}

// RobotsFor returns the parsed robots.txt for rawURL's scheme and host, fetching and caching
// it the way the crawl does (the crawl reuses the cached copy). Nil when rawURL has no host.
func (e *Engine) RobotsFor(ctx context.Context, rawURL string) *RobotsData {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return nil
	}
	return e.robots.get(ctx, u)
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/crawler`
  Expected: `ok` (existing scope tests such as `TestIncludeAppliesToSeedAndNormalizedURL` still pass).

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/engine.go internal/crawler/inscope_test.go
git commit -F - <<'EOF'
feat(crawler): expose InScope, Normalize, RobotsFor and SetSeed on Engine

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-4: sitemap.Discover and the WithURLs option

**Files:**
- Modify: `internal/analyze/sitemap/sitemap.go`
- Test: `internal/analyze/sitemap/sitemap_test.go`

**Interfaces:**
- Produces:
  - `func Discover(ctx context.Context, fetcher crawler.Fetcher, seed string, robots map[string]*crawler.RobotsData) (urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string)` (contract)
  - `func WithURLs(urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string) Option` (contract)
  - `type Option func(*Analyzer)`; `func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer`
  - `type Discovery struct { URLs map[string]bool; Parsed int; InvalidDeclared, TruncatedDeclared, Sources []string; ViaRobots bool }` and
    `func DiscoverAll(ctx context.Context, fetcher crawler.Fetcher, seed string, robots map[string]*crawler.RobotsData) Discovery` (gap: the preview needs `sources` and whether robots.txt declared them)
- `Fetch` keeps its signature (used by `internal/redirectcheck`).

- [ ] **Step 1: Write the failing tests.** Add `"sync"` to the imports of
  `internal/analyze/sitemap/sitemap_test.go` and append:

```go
// countingFetcher records every URL it is asked for and serves from inner.
type countingFetcher struct {
	inner fakeFetcher
	mu    sync.Mutex
	calls []string
}

func (c *countingFetcher) Fetch(ctx context.Context, rawURL string) (*crawler.Page, error) {
	c.mu.Lock()
	c.calls = append(c.calls, rawURL)
	c.mu.Unlock()
	return c.inner.Fetch(ctx, rawURL)
}

func TestDiscoverUsesRobotsDeclaredSitemap(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://cdn.example/sm.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap)},
	}}
	robots := map[string]*crawler.RobotsData{
		"example.com": {Host: "example.com", Found: true, Sitemaps: []string{"https://cdn.example/sm.xml"}},
	}
	urls, parsed, invalid, truncated := sitemap.Discover(context.Background(), ff, "https://example.com/", robots)
	if parsed != 1 || len(urls) != 2 || !urls["https://example.com/a"] || !urls["https://example.com/b"] {
		t.Fatalf("Discover: parsed=%d urls=%v", parsed, urls)
	}
	if len(invalid) != 0 || len(truncated) != 0 {
		t.Errorf("unexpected invalid=%v truncated=%v", invalid, truncated)
	}
}

func TestDiscoverAllReportsSourcesAndRobotsOrigin(t *testing.T) {
	declared := fakeFetcher{pages: map[string]*crawler.Page{
		"https://cdn.example/sm.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap)},
	}}
	robots := map[string]*crawler.RobotsData{"example.com": {Host: "example.com", Sitemaps: []string{"https://cdn.example/sm.xml"}}}
	d := sitemap.DiscoverAll(context.Background(), declared, "https://example.com/", robots)
	if !d.ViaRobots || len(d.Sources) != 1 || d.Sources[0] != "https://cdn.example/sm.xml" {
		t.Errorf("declared sitemap: ViaRobots=%v Sources=%v", d.ViaRobots, d.Sources)
	}

	guessed := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap)},
	}}
	g := sitemap.DiscoverAll(context.Background(), guessed, "https://example.com/", nil)
	if g.ViaRobots || len(g.Sources) != 1 || g.Sources[0] != "https://example.com/sitemap.xml" {
		t.Errorf("guessed sitemap: ViaRobots=%v Sources=%v", g.ViaRobots, g.Sources)
	}
}

func TestDiscoverAllWithoutHostFindsNothing(t *testing.T) {
	d := sitemap.DiscoverAll(context.Background(), fakeFetcher{}, "not a url", nil)
	if d.Parsed != 0 || d.URLs == nil || len(d.URLs) != 0 {
		t.Errorf("DiscoverAll on a host-less seed = %+v, want an empty, non-nil URL set", d)
	}
}

func TestWithURLsSkipsFetching(t *testing.T) {
	cf := &countingFetcher{inner: fakeFetcher{pages: map[string]*crawler.Page{}}}
	urls := map[string]bool{"https://example.com/a": true, "https://example.com/b": true}
	a := sitemap.New(cf, sitemap.WithURLs(urls, 1, nil, nil))
	res := &crawler.Result{
		Seed:  "https://example.com/",
		Pages: []*crawler.Page{{FinalURL: "https://example.com/a", StatusCode: 200}},
	}
	issues := a.Analyze(context.Background(), res)
	if len(cf.calls) != 0 {
		t.Fatalf("analyzer fetched %v despite a pre-fetched sitemap", cf.calls)
	}
	cov, ok := find(issues, "sitemap-coverage")
	if !ok {
		t.Fatal("expected sitemap-coverage from the pre-fetched set")
	}
	if cov.Data["sitemap_urls"] != 2 || cov.Data["in_sitemap_not_crawled"] != 1 {
		t.Errorf("coverage data = %v", cov.Data)
	}
}

func TestWithURLsReportsPrefetchedProblems(t *testing.T) {
	a := sitemap.New(&countingFetcher{}, sitemap.WithURLs(map[string]bool{}, 0, []string{"https://example.com/bad.xml"}, nil))
	issues := a.Analyze(context.Background(), &crawler.Result{Seed: "https://example.com/"})
	if _, ok := find(issues, "sitemap-invalid"); !ok {
		t.Error("expected sitemap-invalid for the pre-fetched invalid declared sitemap")
	}
	if _, ok := find(issues, "sitemap-missing"); !ok {
		t.Error("expected sitemap-missing when nothing parsed")
	}
}
```

- [ ] **Step 2: Run and watch them fail.** `go test ./internal/analyze/sitemap`
  Expected: build failure, `undefined: sitemap.Discover`.

- [ ] **Step 3: Implement.** In `internal/analyze/sitemap/sitemap.go`:

  Add `"sort"` to the imports. Replace the `Analyzer` type and `New` with:

```go
// Analyzer fetches and validates sitemaps. It uses a Fetcher to retrieve sitemap files,
// unless it was handed a pre-fetched set through WithURLs.
type Analyzer struct {
	fetcher crawler.Fetcher
	pre     *prefetched
}

// prefetched is a sitemap set discovered before the crawl (runner.Run's pre-pass).
type prefetched struct {
	urls                               map[string]bool
	parsed                             int
	invalidDeclared, truncatedDeclared []string
}

// Option configures an Analyzer.
type Option func(*Analyzer)

// WithURLs hands the analyzer a sitemap set someone already discovered (the values Discover
// returns), so it cross-checks coverage without fetching the sitemap a second time.
func WithURLs(urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string) Option {
	return func(a *Analyzer) {
		a.pre = &prefetched{urls: urls, parsed: parsed, invalidDeclared: invalidDeclared, truncatedDeclared: truncatedDeclared}
	}
}

// New returns a sitemap analyzer that fetches sitemaps with the given fetcher.
func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer {
	a := &Analyzer{fetcher: fetcher}
	for _, o := range opts {
		o(a)
	}
	return a
}
```

  Replace the whole `Fetch` function with:

```go
// Fetch retrieves and parses the sitemap(s) reachable from candidates (URL -> declared, where
// declared is true for sitemaps named in robots.txt or a parent index, false for a
// conventional path merely guessed at). It follows sitemap indexes up to two levels deep.
// parsed counts candidates that successfully parsed as an urlset or index (0 means nothing
// usable was found at any candidate); invalidDeclared lists declared candidates whose response
// parsed as neither (a guessed path that fails to parse is almost always a soft-404 and isn't
// reported); truncatedDeclared lists declared candidates whose body was cut short by the
// fetcher's size cap before it could be parsed, so they aren't misreported as invalid.
func Fetch(ctx context.Context, fetcher crawler.Fetcher, candidates map[string]bool) (urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string) {
	r := fetchAll(ctx, fetcher, candidates)
	return r.urls, r.parsed, r.invalidDeclared, r.truncatedDeclared
}

// fetchResult is Fetch's outcome plus the top-level candidates that parsed (sources).
type fetchResult struct {
	urls                                        map[string]bool
	parsed                                      int
	invalidDeclared, truncatedDeclared, sources []string
}

func fetchAll(ctx context.Context, fetcher crawler.Fetcher, candidates map[string]bool) fetchResult {
	r := fetchResult{urls: map[string]bool{}}
	visited := map[string]bool{}

	var fetchOne func(smURL string, depth int, declared bool)
	fetchOne = func(smURL string, depth int, declared bool) {
		if depth > 2 || visited[smURL] {
			return
		}
		visited[smURL] = true

		page, ferr := fetcher.Fetch(ctx, smURL)
		if ferr != nil || page == nil || page.StatusCode != 200 || len(page.Body) == 0 {
			return
		}
		if page.Truncated {
			if declared {
				r.truncatedDeclared = append(r.truncatedDeclared, smURL)
			}
			return
		}
		body, gzErr := maybeGunzip(smURL, page)
		if gzErr != nil {
			if declared {
				r.invalidDeclared = append(r.invalidDeclared, smURL)
			}
			return
		}

		var idx sitemapindex
		if xml.Unmarshal(body, &idx) == nil && len(idx.Sitemaps) > 0 {
			r.parsed++
			if depth == 0 {
				r.sources = append(r.sources, smURL)
			}
			for _, s := range idx.Sitemaps {
				if loc := strings.TrimSpace(s.Loc); loc != "" {
					// Children referenced by an index are declared by it.
					fetchOne(loc, depth+1, true)
				}
			}
			return
		}
		var us urlset
		if xml.Unmarshal(body, &us) == nil {
			r.parsed++
			if depth == 0 {
				r.sources = append(r.sources, smURL)
			}
			for _, u := range us.URLs {
				if loc := strings.TrimSpace(u.Loc); loc != "" {
					r.urls[normalize(loc)] = true
				}
			}
			return
		}
		// The response did not parse as a sitemap. Only flag it when the sitemap was
		// explicitly declared (robots.txt or an index); a guessed conventional path that
		// returns HTML is a soft-404, not a broken sitemap, so reporting it is noise.
		if declared {
			r.invalidDeclared = append(r.invalidDeclared, smURL)
		}
	}

	for c, declared := range candidates {
		fetchOne(c, 0, declared)
	}
	return r
}

// Discovery is everything DiscoverAll learned about a site's sitemap.
type Discovery struct {
	URLs                               map[string]bool // every <loc>, normalized; never nil
	Parsed                             int
	InvalidDeclared, TruncatedDeclared []string
	// Sources are the top-level candidates that parsed as an urlset or index, sorted.
	Sources []string
	// ViaRobots is true when at least one source was declared in robots.txt.
	ViaRobots bool
}

// Discover finds the sitemap for seed: the Sitemap: lines of every robots.txt in robots, then
// the conventional /sitemap.xml and /sitemap_index.xml on the seed's host, following indexes.
// The analyzer uses it when it has no pre-fetched set; runner.Run's pre-pass uses DiscoverAll.
func Discover(ctx context.Context, fetcher crawler.Fetcher, seed string, robots map[string]*crawler.RobotsData) (urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string) {
	d := DiscoverAll(ctx, fetcher, seed, robots)
	return d.URLs, d.Parsed, d.InvalidDeclared, d.TruncatedDeclared
}

// DiscoverAll is Discover plus where the sitemap came from.
func DiscoverAll(ctx context.Context, fetcher crawler.Fetcher, seed string, robots map[string]*crawler.RobotsData) Discovery {
	su, err := url.Parse(seed)
	if err != nil || su.Host == "" {
		return Discovery{URLs: map[string]bool{}}
	}
	// Candidate sitemap URLs: those declared in robots.txt (declared==true) plus common
	// conventional paths we merely guess at (declared==false). The distinction matters for
	// error reporting: a declared sitemap that won't parse is a real misconfiguration worth
	// flagging, but a guessed path that returns non-XML is almost always a soft-404 (the
	// server answers 200 with an HTML page for any unknown path), so we stay silent about it.
	candidates := map[string]bool{}
	for _, data := range robots {
		if data == nil {
			continue
		}
		for _, sm := range data.Sitemaps {
			candidates[sm] = true
		}
	}
	base := su.Scheme + "://" + su.Host
	for _, path := range []string{"/sitemap.xml", "/sitemap_index.xml"} {
		if _, ok := candidates[base+path]; !ok {
			candidates[base+path] = false
		}
	}

	r := fetchAll(ctx, fetcher, candidates)
	sort.Strings(r.sources)
	d := Discovery{
		URLs:              r.urls,
		Parsed:            r.parsed,
		InvalidDeclared:   r.invalidDeclared,
		TruncatedDeclared: r.truncatedDeclared,
		Sources:           r.sources,
	}
	for _, s := range d.Sources {
		if candidates[s] {
			d.ViaRobots = true
		}
	}
	return d
}
```

  Replace the start of `Analyze` (from `func (a Analyzer) Analyze` through the line
  `sitemapURLs, parsed, invalidDeclared, truncatedDeclared := Fetch(ctx, a.fetcher, candidates)`)
  with:

```go
func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	seed, err := url.Parse(result.Seed)
	if err != nil {
		return nil
	}
	base := seed.Scheme + "://" + seed.Host
	var issues []analyze.Issue

	var (
		sitemapURLs                        map[string]bool
		parsed                             int
		invalidDeclared, truncatedDeclared []string
	)
	if a.pre != nil {
		sitemapURLs, parsed = a.pre.urls, a.pre.parsed
		invalidDeclared, truncatedDeclared = a.pre.invalidDeclared, a.pre.truncatedDeclared
	} else {
		sitemapURLs, parsed, invalidDeclared, truncatedDeclared = Discover(ctx, a.fetcher, result.Seed, result.Robots)
	}
```

  The rest of `Analyze` (from `for _, u := range invalidDeclared {` to the end) stays as is.

- [ ] **Step 4: Run.** `go test -race ./internal/analyze/sitemap ./internal/redirectcheck`
  Expected: `ok` for both (the existing analyzer tests prove `Discover` matches the previous behavior).

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/analyze/sitemap/sitemap.go internal/analyze/sitemap/sitemap_test.go
git commit -F - <<'EOF'
refactor(sitemap): extract Discover and accept a pre-fetched set via WithURLs

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-5: Runner sitemap pre-pass, Coverage sitemap fields, budget note

**Files:**
- Modify: `internal/crawler/types.go` (`Coverage`), `internal/runner/runner.go`
- Create: `internal/runner/sitemap.go`, `internal/runner/sitemap_test.go`

**Interfaces:**
- Consumes: `Engine.SetSeed/InScope/Normalize/RobotsFor` (W1-PROG-3), `sitemap.DiscoverAll`,
  `sitemap.WithURLs`, `sitemap.Option` (W1-PROG-4), wave 0 `RunOption`/`runSettings`.
- Produces:
  - `crawler.Coverage.SitemapTotal int json:"sitemap_total,omitempty"`,
    `crawler.Coverage.SitemapFetched int json:"sitemap_fetched,omitempty"` — `report.Coverage`
    is a `*crawler.Coverage`, so this is where the contract's "report.Coverage gains" lands.
  - `runner.RegistryOptions.SitemapOptions []sitemap.Option`
  - unexported `sitemapSet`, `discoverSitemap`, `budgetNote`, `groupThousands`.
  - `Run(ctx, cfg, seed, runOpts ...RunOption)` now fetches the sitemap once, before the crawl.

- [ ] **Step 1: Write the failing tests** `internal/runner/sitemap_test.go`:

```go
package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

// sitemapSite serves robots.txt declaring /sitemap.xml, a sitemap listing /, /a and /b, and a
// home page linking to /a, /b and /off (which is not in the sitemap). It counts how often the
// sitemap is fetched.
func sitemapSite(t *testing.T, sitemapHits *atomic.Int32) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", base)
		case "/sitemap.xml":
			sitemapHits.Add(1)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>%[1]s/</loc></url><url><loc>%[1]s/a</loc></url><url><loc>%[1]s/b</loc></url></urlset>`, base)
		case "/":
			fmt.Fprint(w, `<html><head><title>Home</title></head><body><a href="/a">a</a><a href="/b">b</a><a href="/off">off</a></body></html>`)
		case "/a", "/b", "/off":
			fmt.Fprintf(w, `<html><head><title>%s</title></head><body>page</body></html>`, r.URL.Path)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestRunFetchesSitemapOnce(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	rep, err := Run(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("sitemap.xml fetched %d times, want exactly 1 (pre-pass shared with the analyzer)", got)
	}
	if rep.Coverage == nil || rep.Coverage.SitemapTotal != 3 || rep.Coverage.SitemapFetched != 3 {
		t.Fatalf("Coverage = %+v, want sitemap_total 3 and sitemap_fetched 3", rep.Coverage)
	}
	found := false
	for _, is := range rep.Issues {
		if is.Code == "sitemap-coverage" {
			found = true
		}
	}
	if !found {
		t.Error("the sitemap analyzer should still report sitemap-coverage from the pre-fetched set")
	}
}

func TestRunBudgetNoteOnlyWhenBudgetBelowSitemap(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	for _, tc := range []struct {
		maxPages int
		want     bool
	}{{2, true}, {3, false}, {0, false}} {
		cfg := config.Default()
		cfg.Crawl.MaxPages = tc.maxPages
		rep, err := Run(context.Background(), cfg, ts.URL)
		if err != nil {
			t.Fatalf("Run(max_pages=%d): %v", tc.maxPages, err)
		}
		has := false
		for _, n := range rep.Notes {
			if strings.HasPrefix(n, "The page budget") {
				has = true
			}
		}
		if has != tc.want {
			t.Errorf("max_pages=%d: budget note present=%v, want %v (notes %q)", tc.maxPages, has, tc.want, rep.Notes)
		}
	}
}

func TestRunWithoutSitemapLeavesCoverageSitemapFieldsZero(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>hi</body></html>`)
	}))
	defer ts.Close()
	rep, err := Run(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Coverage.SitemapTotal != 0 || rep.Coverage.SitemapFetched != 0 {
		t.Errorf("Coverage = %+v, want zero sitemap fields without a sitemap", rep.Coverage)
	}
}

func TestBudgetNote(t *testing.T) {
	want := "The page budget (500) is below the sitemap total (1,240); this crawl covered at most 40% of the sitemap."
	if got := budgetNote(500, 1240); got != want {
		t.Errorf("budgetNote(500, 1240) = %q, want %q", got, want)
	}
	for _, tc := range [][2]int{{0, 1240}, {1240, 1240}, {2000, 1240}, {10, 0}} {
		if got := budgetNote(tc[0], tc[1]); got != "" {
			t.Errorf("budgetNote(%d, %d) = %q, want no note", tc[0], tc[1], got)
		}
	}
	if got := budgetNote(1, 1000); !strings.Contains(got, "at most 1%") {
		t.Errorf("budgetNote(1, 1000) = %q, want the percentage floored at 1%%", got)
	}
}

func TestGroupThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1240: "1,240", 1234567: "1,234,567", -1240: "-1,240"} {
		if got := groupThousands(n); got != want {
			t.Errorf("groupThousands(%d) = %q, want %q", n, got, want)
		}
	}
}
```

- [ ] **Step 2: Run and watch them fail.** `go test ./internal/runner -run 'TestRun(FetchesSitemapOnce|BudgetNote|WithoutSitemap)|TestBudgetNote|TestGroupThousands'`
  Expected: build failure, `undefined: budgetNote` and `rep.Coverage.SitemapTotal undefined`.

- [ ] **Step 3: Implement.**

  In `internal/crawler/types.go`, add to the end of `Coverage`:

```go
	// SitemapTotal is the number of in-scope URLs the site's sitemap lists, found by
	// runner.Run's pre-pass; SitemapFetched is how many of them this crawl fetched. Both are 0,
	// and omitted, when no sitemap was found. Set by runner.Run, not by Engine.Crawl.
	SitemapTotal   int `json:"sitemap_total,omitempty"`
	SitemapFetched int `json:"sitemap_fetched,omitempty"`
```

  Create `internal/runner/sitemap.go`:

```go
package runner

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/sitemap"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// sitemapSet is the outcome of the sitemap pre-pass: everything the sitemap declared (handed
// to the sitemap analyzer unchanged) and the in-scope subset, keyed by the crawl's own URL
// normalization, that progress and Coverage count against. Read-only once built.
type sitemapSet struct {
	all     sitemap.Discovery
	inScope map[string]bool
}

// discoverSitemap runs the pre-pass: robots.txt Sitemap: lines for the seed host, then the
// conventional paths, through fetcher (the governed analyzer fetcher, so it obeys the crawl's
// rate limit and robots.txt). It never fails: no sitemap is an empty set.
func discoverSitemap(ctx context.Context, engine *crawler.Engine, fetcher crawler.Fetcher, seed string) sitemapSet {
	robots := map[string]*crawler.RobotsData{}
	if rd := engine.RobotsFor(ctx, seed); rd != nil {
		robots[rd.Host] = rd
	}
	d := sitemap.DiscoverAll(ctx, fetcher, seed, robots)
	in := make(map[string]bool, len(d.URLs))
	for u := range d.URLs {
		if engine.InScope(u) {
			in[engine.Normalize(u)] = true
		}
	}
	return sitemapSet{all: d, inScope: in}
}

// found reports whether any sitemap parsed.
func (s sitemapSet) found() bool { return s.all.Parsed > 0 }

// match reports whether a recorded page (by requested or final URL) is an in-scope sitemap
// URL, returning the matched key so callers can count each sitemap URL once.
func (s sitemapSet) match(requested, final string, normalize func(string) string) (string, bool) {
	if len(s.inScope) == 0 {
		return "", false
	}
	for _, u := range []string{final, requested} {
		if u == "" {
			continue
		}
		if k := normalize(u); s.inScope[k] {
			return k, true
		}
	}
	return "", false
}

// fetchedCount counts the distinct in-scope sitemap URLs among pages.
func (s sitemapSet) fetchedCount(pages []*crawler.Page, normalize func(string) string) int {
	seen := map[string]bool{}
	for _, p := range pages {
		if p == nil {
			continue
		}
		if k, ok := s.match(p.RequestedURL, p.FinalURL, normalize); ok {
			seen[k] = true
		}
	}
	return len(seen)
}

// budgetNote is the Report.Notes entry for a page budget smaller than the sitemap, or "".
func budgetNote(maxPages, sitemapTotal int) string {
	if maxPages <= 0 || sitemapTotal <= 0 || maxPages >= sitemapTotal {
		return ""
	}
	pct := maxPages * 100 / sitemapTotal
	if pct < 1 {
		pct = 1
	}
	return fmt.Sprintf("The page budget (%s) is below the sitemap total (%s); this crawl covered at most %d%% of the sitemap.",
		groupThousands(maxPages), groupThousands(sitemapTotal), pct)
}

// groupThousands formats n with comma thousands separators (1240 -> "1,240").
func groupThousands(n int) string {
	s := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}
```

  In `internal/runner/runner.go`:

  1. Add a field to `RegistryOptions` (after `IgnoreExternalTagging`):

```go
	// SitemapOptions configure the sitemap analyzer. runner.Run passes sitemap.WithURLs here
	// so the analyzer reuses the pre-pass's sitemap instead of fetching it again.
	SitemapOptions []sitemap.Option
```

  2. In `BuildRegistry`, replace `r.Register(sitemap.New(fetcher))` with
     `r.Register(sitemap.New(fetcher, opts.SitemapOptions...))`.

  3. Replace the whole `Run` function (wave 0's version, whatever its option-applying lines
     look like) with:

```go
// Run performs a full crawl + analysis for the given config and seed, returning a Report.
// The seed argument overrides cfg.Seed when non-empty. runOpts add optional behavior
// (progress reporting, pause control) for callers that need it; the CLI passes none.
func Run(ctx context.Context, cfg config.Config, seed string, runOpts ...RunOption) (*report.Report, error) {
	settings := newRunSettings(runOpts)
	_ = settings // read by WithProgress (W1-PROG-6) and the pause options (W2-PAUSE-5)
	if seed == "" {
		seed = cfg.Seed
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	opts, err := cfg.ToOptions()
	if err != nil {
		return nil, err
	}

	fetcher, err := newFetcher(cfg, opts)
	if err != nil {
		return nil, err
	}
	if c, ok := fetcher.(io.Closer); ok {
		defer func() { _ = c.Close() }()
	}
	engine := crawler.New(opts, fetcher)
	// Fix the crawl's host scope now rather than inside Crawl: the sitemap pre-pass needs
	// InScope, and its robots.txt fetch must already know the seed host so credentials reach
	// that host (and only that host).
	seed, err = engine.SetSeed(seed)
	if err != nil {
		return nil, err
	}

	// Analyzers that fetch extra resources (sitemap.xml, llms.txt, the --specialized probes)
	// use a raw fetcher regardless of render mode. It's built fresh here rather than reusing
	// engine's fetcher, so — unlike that fetcher and the robots fetcher inside crawler.New —
	// it isn't restricted to the seed's own host by default. The sitemap analyzer fetches
	// whatever URL robots.txt's Sitemap: directive names, which is routinely a different host
	// (a CDN, a separate subdomain) with no FollowExternal needed to reach it, so Basic Auth,
	// the Cookie header, and custom headers must be restricted here explicitly. engine.Govern
	// then puts those fetches under the crawl's rate limiter and robots.txt policy, so a probe
	// can't hit a path the crawl was told to leave alone, or run unthrottled.
	analyzerFetcher := crawler.NewHTTPFetcher(opts)
	if seedURL, perr := url.Parse(seed); perr == nil {
		analyzerFetcher.RestrictCredentialsToHost(seedURL.Host, opts.AllowSubdomains)
	}
	governed := engine.Govern(analyzerFetcher)

	// Sitemap pre-pass: read the sitemap once, before the crawl, so progress can be measured
	// against it. The sitemap analyzer reuses the same set below. A missing or unreachable
	// sitemap is an empty set, never an error.
	sm := discoverSitemap(ctx, engine, governed, seed)

	result, err := engine.Crawl(ctx, seed)
	if err != nil {
		return nil, err
	}
	if sm.found() {
		result.Coverage.SitemapTotal = len(sm.inScope)
		result.Coverage.SitemapFetched = sm.fetchedCount(result.Pages, engine.Normalize)
	}

	reg := BuildRegistry(governed, RegistryOptions{
		Specialized:           cfg.Analyzers.Specialized,
		SecurityAudit:         cfg.Analyzers.SecurityAudit,
		IgnoreExternalTagging: cfg.Analyzers.IgnoreExternalTagging,
		SitemapOptions: []sitemap.Option{
			sitemap.WithURLs(sm.all.URLs, sm.all.Parsed, sm.all.InvalidDeclared, sm.all.TruncatedDeclared),
		},
	})
	analyzers, skipped := planAnalyzers(reg, cfg.Analyzers, cfg.Crawl.StripQuery)
	issues := analyze.Run(ctx, analyzers, result)

	rep := report.Build(result, issues)
	// Every analyzer has read what it needs from each page's body/DOM (analyze.Run, above),
	// and report.Build just finished the one remaining reader (the site-map tree's per-page
	// title, extracted from Doc). Nothing touches them after this point, so release them now
	// rather than holding a large crawl's raw bytes and parsed DOM trees in memory for the
	// rest of the process's life.
	result.ReleaseBodies()
	if len(skipped) > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf("strip_query is on, so query-dependent analyzers were skipped: %s", strings.Join(skipped, ", ")))
	}
	if cfg.Analyzers.SecurityAudit && cfg.Render == "headless" {
		rep.Notes = append(rep.Notes, "security audit: TLS and certificate checks read the handshake "+
			"behind each response, which headless rendering does not expose — those checks were skipped. "+
			"Cookie and header checks still ran. Re-run with --render raw for the transport-layer findings.")
	}
	if result.ThrottleEvents > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf("server returned HTTP 429/503: adaptive delay slowed the crawl %d time(s), down to %.3g req/s (disable with --adaptive-delay=false)", result.ThrottleEvents, result.FinalRate))
	}
	if !result.Coverage.Complete {
		rep.Notes = append(rep.Notes, coverageNote(result.Coverage))
	}
	if note := budgetNote(opts.MaxPages, result.Coverage.SitemapTotal); note != "" {
		rep.Notes = append(rep.Notes, note)
	}

	// The crawled site is rendered as a tab in the HTML report (report.Build attaches it). When
	// --sitemap is set, also write it out as a standalone, machine-readable sitemap.xml.
	if cfg.Output.SitemapPath != "" && rep.SiteMap != nil {
		if err := sitemapgen.WriteXMLFile(cfg.Output.SitemapPath, *rep.SiteMap); err != nil {
			return nil, fmt.Errorf("writing sitemap.xml: %w", err)
		}
		rep.Notes = append(rep.Notes, fmt.Sprintf("sitemap.xml written to %s (%d URLs)", cfg.Output.SitemapPath, len(rep.SiteMap.Entries)))
	}
	return rep, nil
}
```

  Keep wave 0's `newRunSettings` (it skips nil options, and `TestNewRunSettingsAppliesEveryOption`
  pins it).

- [ ] **Step 4: Run.** `go test -race ./internal/runner`
  Expected: `ok`. The existing `TestRunDoesNotLeakBasicAuthToSitemapHost`,
  `TestRunScopesCustomHeadersForAnalyzerFetches` and `TestRunAnalyzerProbesRespectRobots` pass
  unchanged: they pin that the pre-pass uses the restricted, governed fetcher and that
  `SetSeed` runs before the robots.txt fetch.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/types.go internal/runner/runner.go internal/runner/sitemap.go internal/runner/sitemap_test.go
git commit -F - <<'EOF'
feat(runner): read the sitemap once before the crawl and report sitemap coverage

Adds coverage.sitemap_total / sitemap_fetched and a note when the page budget is
below the sitemap total. The sitemap analyzer reuses the pre-fetched set.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-6: runner.Progress and WithProgress

**Files:**
- Create: `internal/runner/progress.go`, `internal/runner/progress_test.go`
- Modify: `internal/runner/options.go`, `internal/runner/runner.go`

**Interfaces:**
- Consumes: `crawler.Progress`, `Options.OnProgress` (W1-PROG-2), `sitemapSet` (W1-PROG-5).
- Produces (contract): `type Phase string`; `PhaseSitemap`, `PhaseCrawling`, `PhaseAnalyzing`;
  `type Progress struct { crawler.Progress; Phase; SitemapFound; SitemapTotal; SitemapFetched; OffSitemap; InSitemap }`;
  `func WithProgress(fn func(Progress)) RunOption`; `runSettings.progress func(Progress)`.

- [ ] **Step 1: Write the failing test** `internal/runner/progress_test.go`:

```go
package runner

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestRunWithProgressReportsPhasesAndSitemapCounts(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)

	var (
		mu     sync.Mutex
		events []Progress
	)
	rep, err := Run(context.Background(), config.Default(), ts.URL, WithProgress(func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 || events[0].Phase != PhaseSitemap {
		t.Fatalf("first event = %+v, want phase %q", events, PhaseSitemap)
	}
	order := map[Phase]int{PhaseSitemap: 0, PhaseCrawling: 1, PhaseAnalyzing: 2}
	pages := map[int]bool{}
	for i, ev := range events {
		if i > 0 && order[ev.Phase] < order[events[i-1].Phase] {
			t.Errorf("phase went backwards at event %d: %s after %s", i, ev.Phase, events[i-1].Phase)
		}
		if ev.Fetched > 0 {
			pages[ev.Fetched] = true
		}
		if strings.HasSuffix(ev.CurrentURL, "/off") && ev.InSitemap {
			t.Errorf("/off is not in the sitemap but was reported in_sitemap")
		}
		if strings.HasSuffix(ev.CurrentURL, "/a") && !ev.InSitemap {
			t.Errorf("/a is in the sitemap but was reported off it")
		}
	}
	if len(pages) != rep.PagesCrawled {
		t.Errorf("saw %d distinct Fetched values, want one per page (%d)", len(pages), rep.PagesCrawled)
	}
	last := events[len(events)-1]
	if last.Phase != PhaseAnalyzing || last.Fetched != rep.PagesCrawled {
		t.Errorf("last event = phase %s fetched %d, want analyzing / %d", last.Phase, last.Fetched, rep.PagesCrawled)
	}
	if !last.SitemapFound || last.SitemapTotal != 3 {
		t.Errorf("last event sitemap = found %v total %d, want true / 3", last.SitemapFound, last.SitemapTotal)
	}
	if last.SitemapFetched != rep.Coverage.SitemapFetched || last.SitemapFetched != 3 {
		t.Errorf("SitemapFetched = %d, Coverage says %d, want 3", last.SitemapFetched, rep.Coverage.SitemapFetched)
	}
	if last.OffSitemap != 1 {
		t.Errorf("OffSitemap = %d, want 1 (/off)", last.OffSitemap)
	}
}

func TestRunWithoutProgressOptionSendsNothing(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	// Nothing to assert beyond "no panic on the nil reporter": the CLI path.
	if _, err := Run(context.Background(), config.Default(), ts.URL); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
```

- [ ] **Step 2: Run and watch it fail.** `go test ./internal/runner -run TestRunWithProgress`
  Expected: build failure, `undefined: WithProgress`.

- [ ] **Step 3: Implement.**

  In `internal/runner/options.go`, replace `type runSettings struct{}` with:

```go
// runSettings collects what the RunOptions turned on. The zero value is the CLI's behavior.
type runSettings struct {
	// progress receives a Progress snapshot per recorded page and per phase change.
	progress func(Progress)
}

// WithProgress makes Run report progress to fn: one call per phase change and one per page
// the crawl records, serialized and in order. fn runs on crawl goroutines while the engine
// holds its locks, so it must return quickly and must not call a Control method.
func WithProgress(fn func(Progress)) RunOption {
	return func(s *runSettings) { s.progress = fn }
}
```

  Create `internal/runner/progress.go`:

```go
package runner

import (
	"sync"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Phase is the stage a Run is in.
type Phase string

const (
	PhaseSitemap   Phase = "sitemap"   // discovery pre-pass, before the crawl
	PhaseCrawling  Phase = "crawling"  // Engine.Crawl is running
	PhaseAnalyzing Phase = "analyzing" // analyzers and report building
)

// Progress is crawler.Progress plus the run phase and progress against the site's sitemap.
type Progress struct {
	crawler.Progress
	Phase Phase `json:"phase"`
	// SitemapFound is true when the pre-pass parsed a sitemap. SitemapTotal counts its
	// in-scope URLs; SitemapFetched counts the distinct ones fetched so far.
	SitemapFound   bool `json:"sitemap_found"`
	SitemapTotal   int  `json:"sitemap_total"`
	SitemapFetched int  `json:"sitemap_fetched"`
	// OffSitemap is Fetched - SitemapFetched: fetched pages the sitemap doesn't list.
	OffSitemap int `json:"off_sitemap"`
	// InSitemap reports whether CurrentURL (or where it redirected) is in the sitemap.
	InSitemap bool `json:"in_sitemap"`
}

// progressReporter turns engine events into runner.Progress. A nil reporter (no WithProgress)
// ignores every call, so Run needs no conditionals.
type progressReporter struct {
	fn func(Progress)

	mu        sync.Mutex // guards everything below; fn is called with it held to keep order
	phase     Phase
	last      crawler.Progress
	set       sitemapSet
	normalize func(string) string
	matched   map[string]bool
	inSitemap bool
}

func newProgressReporter(fn func(Progress)) *progressReporter {
	if fn == nil {
		return nil
	}
	return &progressReporter{fn: fn, phase: PhaseSitemap, matched: map[string]bool{}}
}

// setPhase records a phase change and reports it.
func (r *progressReporter) setPhase(p Phase) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = p
	r.emitLocked()
}

// setSitemap hands over the pre-pass result. Called once, before the crawl starts.
func (r *progressReporter) setSitemap(set sitemapSet, normalize func(string) string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set, r.normalize = set, normalize
}

// onEngine is the crawler.Options.OnProgress callback. A rise in Fetched marks a page event;
// anything else is a state event (pause, resume) that keeps the last page's sitemap flag.
func (r *progressReporter) onEngine(p crawler.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.Fetched > r.last.Fetched {
		r.inSitemap = false
		if key, ok := r.set.match(p.CurrentURL, p.FinalURL, r.normalize); ok {
			r.matched[key] = true
			r.inSitemap = true
		}
	}
	r.last = p
	r.emitLocked()
}

func (r *progressReporter) emitLocked() {
	fetched := len(r.matched)
	r.fn(Progress{
		Progress:       r.last,
		Phase:          r.phase,
		SitemapFound:   r.set.found(),
		SitemapTotal:   len(r.set.inScope),
		SitemapFetched: fetched,
		OffSitemap:     r.last.Fetched - fetched,
		InSitemap:      r.inSitemap,
	})
}
```

  In `internal/runner/runner.go` `Run`:

  1. Delete the line `_ = settings // read by WithProgress (W1-PROG-6) and the pause options (W2-PAUSE-5)`
     (settings is read below now). After `opts, err := cfg.ToOptions()` and its error check, add:

```go
	progress := newProgressReporter(settings.progress)
	if progress != nil {
		opts.OnProgress = progress.onEngine
	}
```

  2. Replace `sm := discoverSitemap(ctx, engine, governed, seed)` with:

```go
	progress.setPhase(PhaseSitemap)
	sm := discoverSitemap(ctx, engine, governed, seed)
	progress.setSitemap(sm, engine.Normalize)
	progress.setPhase(PhaseCrawling)
```

  3. Right after the `result, err := engine.Crawl(ctx, seed)` error check, add:

```go
	progress.setPhase(PhaseAnalyzing)
```

- [ ] **Step 4: Run.** `go test -race ./internal/runner`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/runner/options.go internal/runner/progress.go internal/runner/progress_test.go internal/runner/runner.go
git commit -F - <<'EOF'
feat(runner): add WithProgress with run phases and sitemap counts

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-6b: Save the tape in the report (`Coverage.Tape`, `Coverage.TapeOff`)

Coordinator decision: the tape is persisted, so a saved or reopened crawl shows the same tape
(notches included) as the live view, and the Crawls list can draw a real mini tape. This
supersedes the spec's "saved crawls rebuild a status-only tape" rule; the status-only tape
remains only as a fallback for reports saved before this change.

**Files:**
- Create: `internal/crawler/tape.go`, `internal/crawler/tape_test.go`
- Modify: `internal/crawler/types.go` (`Coverage`), `internal/runner/progress.go`,
  `internal/runner/runner.go`, `web/src/types/crawls.ts` (`Coverage`)
- Test: `internal/runner/progress_test.go`, `internal/runner/sitemap_test.go`

**Interfaces:**
- Consumes: `progressReporter` (W1-PROG-6), `sitemapSet.found` (W1-PROG-5), `progressStatus`'s
  first-hop rule (W1-PROG-2).
- Produces:
  - `func crawler.TapeChar(status int) byte` (`'2'`..`'5'`, `'x'` for 0 or outside 200-599)
  - `func crawler.TapePreview(tape string, limit int) string` (worst status wins per run, the
    same rule and run length as the UI's `buildTicks`), `const crawler.TapePreviewLen = 160`
  - `crawler.Coverage.Tape string json:"tape,omitempty"`,
    `crawler.Coverage.TapeOff string json:"tape_off,omitempty"`
  - `func (p runner.Progress) TapeOffChar() byte` (`'1'` when `SitemapFound && !InSitemap`)
  - `newProgressReporter` now always returns a reporter (fn may be nil) that records the tape;
    `Run` always sets `opts.OnProgress`.
  - TypeScript: `Coverage` gains `sitemap_total?`, `sitemap_fetched?`, `tape?`, `tape_off?`.

- [ ] **Step 1: Write the failing tests.** Create `internal/crawler/tape_test.go`:

```go
package crawler

import (
	"strings"
	"testing"
)

func TestTapeChar(t *testing.T) {
	for status, want := range map[int]byte{200: '2', 204: '2', 301: '3', 404: '4', 429: '4', 503: '5', 0: 'x', 99: 'x', 600: 'x'} {
		if got := TapeChar(status); got != want {
			t.Errorf("TapeChar(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestTapePreview(t *testing.T) {
	if got := TapePreview("2234", TapePreviewLen); got != "2234" {
		t.Errorf("a tape that fits = %q, want it unchanged", got)
	}
	if got := TapePreview("2222", 0); got != "" {
		t.Errorf("limit 0 = %q, want empty", got)
	}
	// Worst status wins inside a run: 5xx > failed fetch > 4xx > 3xx > 2xx.
	for in, want := range map[string]string{"2345x": "5", "234x": "x", "2343": "4", "2223": "3"} {
		if got := TapePreview(in, 1); got != want {
			t.Errorf("TapePreview(%q, 1) = %q, want %q", in, got, want)
		}
	}
	// 301 pages, at most 160 ticks: runs of 2, so 151 ticks, the last one the lone 5xx.
	got := TapePreview(strings.Repeat("2", 300)+"5", TapePreviewLen)
	if len(got) != 151 || got[150] != '5' || strings.Count(got, "5") != 1 {
		t.Errorf("TapePreview(300 x 2, then 5) = %d chars %q, want 151 ending in the single 5", len(got), got)
	}
}

// A very large crawl (100,000 pages) still previews in TapePreviewLen characters and keeps the
// one failure that would otherwise drown in green.
func TestTapePreviewLargeCrawl(t *testing.T) {
	in := []byte(strings.Repeat("2", 100_000))
	in[73_210] = '4'
	got := TapePreview(string(in), TapePreviewLen)
	if len(got) > TapePreviewLen {
		t.Fatalf("preview is %d chars, want at most %d", len(got), TapePreviewLen)
	}
	if strings.Count(got, "4") != 1 {
		t.Errorf("preview %q lost the single 4xx", got)
	}
}
```

  Append to `internal/runner/progress_test.go` (add
  `"github.com/Patience-dot-devl/gocrawl/internal/crawler"` to its imports):

```go
// TestRunSavesTheLiveTapeInCoverage: the tape Run saves is the one a WithProgress consumer
// builds live (one character per page event, crawler.TapeChar and Progress.TapeOffChar, the
// web server's rule), so a reopened crawl shows the same tape, notches included.
func TestRunSavesTheLiveTapeInCoverage(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)

	var (
		mu        sync.Mutex
		tape, off []byte
		lastPage  int
	)
	rep, err := Run(context.Background(), config.Default(), ts.URL, WithProgress(func(p Progress) {
		mu.Lock()
		defer mu.Unlock()
		if p.Fetched > lastPage {
			lastPage = p.Fetched
			tape = append(tape, crawler.TapeChar(p.Status))
			off = append(off, p.TapeOffChar())
		}
	}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	cov := rep.Coverage
	if cov.Tape != string(tape) || cov.TapeOff != string(off) {
		t.Errorf("saved tape %q / %q, live tape %q / %q: they must match", cov.Tape, cov.TapeOff, tape, off)
	}
	if len(cov.Tape) != rep.PagesCrawled || len(cov.TapeOff) != rep.PagesCrawled {
		t.Errorf("tape lengths %d / %d, want one character per page (%d)", len(cov.Tape), len(cov.TapeOff), rep.PagesCrawled)
	}
	if cov.Tape != "2222" || strings.Count(cov.TapeOff, "1") != 1 {
		t.Errorf("tape %q / %q, want 2222 with exactly one off-sitemap page (/off)", cov.Tape, cov.TapeOff)
	}
}

// The CLI and MCP pass no WithProgress; their reports carry the tape too.
func TestRunSavesTheTapeWithoutProgressOption(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	rep, err := Run(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Coverage.Tape != "2222" || len(rep.Coverage.TapeOff) != 4 {
		t.Errorf("tape %q / %q, want 2222 and four tape_off characters", rep.Coverage.Tape, rep.Coverage.TapeOff)
	}
}
```

  Append to `internal/runner/sitemap_test.go`:

```go
// A sitemap that only lists URLs outside the crawl scope (a www. host for an apex seed, a CDN
// mirror) is found but counts no in-scope pages: Coverage has no sitemap totals, and every
// fetched page is "not in the sitemap" on the tape.
func TestRunCountsOnlyInScopeSitemapURLs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://www.other.example/</loc></url><url><loc>https://www.other.example/a</loc></url></urlset>`)
		case "/":
			fmt.Fprint(w, `<html><head><title>Home</title></head><body>hi</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	rep, err := Run(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Coverage.SitemapTotal != 0 || rep.Coverage.SitemapFetched != 0 {
		t.Errorf("Coverage = %+v, want no in-scope sitemap URLs counted", rep.Coverage)
	}
	if rep.Coverage.Tape != "2" || rep.Coverage.TapeOff != "1" {
		t.Errorf("tape %q / %q, want 2 / 1 (a sitemap was found; the page is not in its in-scope part)", rep.Coverage.Tape, rep.Coverage.TapeOff)
	}
}
```

- [ ] **Step 2: Run and watch them fail.**
  `go test ./internal/crawler -run 'TestTape' ; go test ./internal/runner -run 'TestRunSaves|TestRunCountsOnlyInScope'`
  Expected: build failures, `undefined: TapeChar` and `rep.Coverage.Tape undefined`.

- [ ] **Step 3: Implement.**

  Create `internal/crawler/tape.go`:

```go
package crawler

// The crawl tape is one character per recorded page, in recording order. The web API streams
// it live, runner.Run saves it in Coverage.Tape / TapeOff, and crawl lists show a preview. All
// three use these helpers, so a saved tape matches the live one character for character.

// TapePreviewLen is the length of a crawl list's tape preview: one compact row in the UI
// (MAX_COMPACT_TICKS in web/src/components/tape/tapeModel.ts).
const TapePreviewLen = 160

// TapeChar is the tape character for a recorded page's Progress.Status: '2', '3', '4' or '5'
// for the status class, 'x' for a failed fetch (status 0) or anything outside 200-599.
func TapeChar(status int) byte {
	switch {
	case status >= 200 && status < 300:
		return '2'
	case status >= 300 && status < 400:
		return '3'
	case status >= 400 && status < 500:
		return '4'
	case status >= 500 && status < 600:
		return '5'
	default:
		return 'x'
	}
}

// tapeRank orders tape characters worst first, the rule the UI's Tape component uses when one
// tick stands for several pages: 5xx, then failed fetches, then 4xx, 3xx, 2xx.
func tapeRank(c byte) int {
	switch c {
	case '5':
		return 5
	case 'x':
		return 4
	case '4':
		return 3
	case '3':
		return 2
	case '2':
		return 1
	default:
		return 0
	}
}

// TapePreview shortens tape to at most limit characters for a crawl list. Each output
// character stands for a run of ceil(len/limit) consecutive pages and takes the worst status
// among them, so a single 5xx in a long crawl still shows. A tape that already fits is
// returned unchanged; limit <= 0 returns "".
func TapePreview(tape string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(tape) <= limit {
		return tape
	}
	per := (len(tape) + limit - 1) / limit
	out := make([]byte, 0, (len(tape)+per-1)/per)
	for start := 0; start < len(tape); start += per {
		end := min(start+per, len(tape))
		worst := tape[start]
		for i := start + 1; i < end; i++ {
			if tapeRank(tape[i]) > tapeRank(worst) {
				worst = tape[i]
			}
		}
		out = append(out, worst)
	}
	return string(out)
}
```

  In `internal/crawler/types.go`, add to the end of `Coverage` (after `SitemapFetched`):

```go
	// Tape is the crawl tape: one character per recorded page in recording order, '2'..'5' for
	// the status class or 'x' for a failed fetch (see TapeChar; a redirected page reports its
	// first hop). TapeOff has the same length: '1' where a sitemap was found and the page is
	// not in its in-scope part, else '0'. Set by runner.Run from the same progress events the
	// live web tape is built from. Omitted when no page was recorded.
	Tape    string `json:"tape,omitempty"`
	TapeOff string `json:"tape_off,omitempty"`
```

  In `internal/runner/progress.go`:

  1. Replace the doc comment `// progressReporter turns engine events into runner.Progress. A nil reporter (no WithProgress)`
     `// ignores every call, so Run needs no conditionals.` with:

```go
// progressReporter turns engine events into runner.Progress for WithProgress (fn) and records
// the crawl tape Run saves in Coverage. Run always creates one; fn is nil without WithProgress.
```

  2. In the struct, after `inSitemap bool`, add:

```go
	// tape and tapeOff grow by one character per page event (TapeChar, tapeOffChar).
	tape, tapeOff []byte
```

  3. Replace `newProgressReporter` with:

```go
func newProgressReporter(fn func(Progress)) *progressReporter {
	return &progressReporter{fn: fn, phase: PhaseSitemap, matched: map[string]bool{}}
}
```

  4. In `onEngine`, inside `if p.Fetched > r.last.Fetched { … }`, after the `match` block, add:

```go
		r.tape = append(r.tape, crawler.TapeChar(p.Status))
		r.tapeOff = append(r.tapeOff, tapeOffChar(r.set.found(), r.inSitemap))
```

  5. Make `emitLocked` start with:

```go
	if r.fn == nil {
		return
	}
```

  6. Append:

```go
// tapes returns the tape recorded so far.
func (r *progressReporter) tapes() (tape, off string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.tape), string(r.tapeOff)
}

// tapeOffChar is the tape_off character for one page: '1' when a sitemap was found and the
// page is not in it, '0' otherwise (without a sitemap no page is "off" it).
func tapeOffChar(sitemapFound, inSitemap bool) byte {
	if sitemapFound && !inSitemap {
		return '1'
	}
	return '0'
}

// TapeOffChar is the tape_off character for the page this event reports. The web server
// builds the live tape_off with it; Run builds Coverage.TapeOff with the same rule.
func (p Progress) TapeOffChar() byte {
	return tapeOffChar(p.SitemapFound, p.InSitemap)
}
```

  In `internal/runner/runner.go` `Run`:

  1. Replace

```go
	progress := newProgressReporter(settings.progress)
	if progress != nil {
		opts.OnProgress = progress.onEngine
	}
```

  with

```go
	// Always on: besides feeding WithProgress, the reporter records the crawl tape saved in
	// Coverage.Tape, so a saved report's tape matches the one the web UI showed live.
	progress := newProgressReporter(settings.progress)
	opts.OnProgress = progress.onEngine
```

  2. Directly after the `if sm.found() { … SitemapFetched … }` block, add:

```go
	result.Coverage.Tape, result.Coverage.TapeOff = progress.tapes()
```

  In `web/src/types/crawls.ts`, add to the `Coverage` interface:

```ts
  sitemap_total?: number
  sitemap_fetched?: number
  tape?: string
  tape_off?: string
```

- [ ] **Step 4: Run.** `go test -race ./internal/crawler ./internal/runner ./internal/report ./internal/store`
  and `cd web && npm run build && npm run lint`.
  Expected: `ok` for all four packages (golden JSON reports, if any, gain only the optional
  `tape` / `tape_off` keys; update them if a golden test says so); the web build passes.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git restore internal/webserver/webui/dist/index.html
git add internal/crawler/tape.go internal/crawler/tape_test.go internal/crawler/types.go internal/runner/progress.go internal/runner/progress_test.go internal/runner/sitemap_test.go internal/runner/runner.go web/src/types/crawls.ts
git commit -F - <<'MSG'
feat(runner): save the crawl tape in coverage.tape / tape_off

Every report now carries the per-page status tape and the not-in-sitemap marks, built
from the same progress events as the live web tape, so a reopened crawl shows the same
tape. crawler.TapePreview gives crawl lists a short worst-status-wins preview.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-PROG-7: runner.PreviewSitemap

**Files:**
- Modify: `internal/runner/sitemap.go`, `internal/runner/sitemap_test.go`

**Interfaces:**
- Produces (gap, used by `POST /api/sitemap-preview`):
  `type SitemapPreview struct { Found bool json:"found"; Total int json:"total"; Sources []string json:"sources"; ViaRobots bool json:"via_robots" }` and
  `func PreviewSitemap(ctx context.Context, cfg config.Config, seed string) (SitemapPreview, error)`.

- [ ] **Step 1: Write the failing test.** Append to `internal/runner/sitemap_test.go`:

```go
func TestPreviewSitemap(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)

	got, err := PreviewSitemap(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("PreviewSitemap: %v", err)
	}
	if !got.Found || got.Total != 3 || !got.ViaRobots {
		t.Errorf("preview = %+v, want found, 3 in scope, via robots.txt", got)
	}
	if len(got.Sources) != 1 || !strings.HasSuffix(got.Sources[0], "/sitemap.xml") {
		t.Errorf("Sources = %v, want the /sitemap.xml robots.txt names", got.Sources)
	}

	cfg := config.Default()
	cfg.Crawl.Exclude = []string{`/b$`}
	scoped, err := PreviewSitemap(context.Background(), cfg, ts.URL)
	if err != nil {
		t.Fatalf("PreviewSitemap with exclude: %v", err)
	}
	if scoped.Total != 2 {
		t.Errorf("Total with /b excluded = %d, want 2", scoped.Total)
	}
}

func TestPreviewSitemapWithoutSitemap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer ts.Close()
	got, err := PreviewSitemap(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("PreviewSitemap: %v", err)
	}
	if got.Found || got.Total != 0 || got.Sources == nil || len(got.Sources) != 0 {
		t.Errorf("preview = %+v, want not found with an empty (non-nil) source list", got)
	}
}

func TestPreviewSitemapRejectsBadRegex(t *testing.T) {
	cfg := config.Default()
	cfg.Crawl.Exclude = []string{"("}
	if _, err := PreviewSitemap(context.Background(), cfg, "https://example.com"); err == nil {
		t.Error("expected an error for an invalid exclude regex")
	}
}
```

- [ ] **Step 2: Run and watch it fail.** `go test ./internal/runner -run TestPreviewSitemap`
  Expected: build failure, `undefined: PreviewSitemap`.

- [ ] **Step 3: Implement.** Add `"net/url"` and
  `"github.com/Patience-dot-devl/gocrawl/internal/config"` to the imports of
  `internal/runner/sitemap.go`, then append:

```go
// SitemapPreview is what the sitemap pre-pass would find for a crawl, without crawling.
type SitemapPreview struct {
	Found     bool     `json:"found"`
	Total     int      `json:"total"`   // in-scope sitemap URLs
	Sources   []string `json:"sources"` // sitemap files that parsed, sorted; never null
	ViaRobots bool     `json:"via_robots"`
}

// PreviewSitemap runs only the sitemap discovery step for cfg and seed: the same robots.txt
// handling, User-Agent, rate limit and scope rules a crawl would apply. Errors are config
// errors (bad regex, bad seed); a missing sitemap is Found == false.
func PreviewSitemap(ctx context.Context, cfg config.Config, seed string) (SitemapPreview, error) {
	if seed == "" {
		seed = cfg.Seed
	}
	if err := cfg.Validate(); err != nil {
		return SitemapPreview{}, err
	}
	opts, err := cfg.ToOptions()
	if err != nil {
		return SitemapPreview{}, err
	}
	engine := crawler.New(opts, crawler.NewHTTPFetcher(opts))
	seed, err = engine.SetSeed(seed)
	if err != nil {
		return SitemapPreview{}, err
	}
	f := crawler.NewHTTPFetcher(opts)
	if u, perr := url.Parse(seed); perr == nil {
		f.RestrictCredentialsToHost(u.Host, opts.AllowSubdomains)
	}
	sm := discoverSitemap(ctx, engine, engine.Govern(f), seed)
	sources := sm.all.Sources
	if sources == nil {
		sources = []string{}
	}
	return SitemapPreview{Found: sm.found(), Total: len(sm.inScope), Sources: sources, ViaRobots: sm.all.ViaRobots}, nil
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/runner -run TestPreviewSitemap`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/runner/sitemap.go internal/runner/sitemap_test.go
git commit -F - <<'EOF'
feat(runner): add PreviewSitemap to run only the sitemap discovery step

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-8: Progress and tape in web jobs, `tape_from`, list coverage

**Files:**
- Create: `internal/webserver/progress.go`, `internal/webserver/progress_test.go`
- Modify: `internal/webserver/crawl_job.go` (`crawlJob`, `JobView`, `crawlListItem`,
  `jobListItem`; wave 0 put all four there), `internal/webserver/crawls.go`
  (`handleStartCrawl`, `handleGetCrawl`, `handleListCrawls`), `internal/store/store.go`,
  `internal/store/store_test.go`

**Interfaces:**
- Consumes: `runner.WithProgress`, `runner.Progress`, `Progress.TapeOffChar` (W1-PROG-6,
  W1-PROG-6b), `crawler.TapeChar`, `crawler.TapePreview`, `crawler.TapePreviewLen`
  (W1-PROG-6b); wave 0 `jobManager[T].with/start/list`.
- Produces:
  - `crawlJob.Progress *runner.Progress`, `crawlJob.Tape []byte`, `crawlJob.TapeOff []byte`
    (contract), `crawlJob.MaxPages int` (gap: needed for `progress.max_pages`).
  - `JobView.Progress *progressView json:"progress,omitempty"`;
    `type progressView struct { runner.Progress; Tape string json:"tape"; TapeOff string json:"tape_off"; TapeFrom int json:"tape_from"; MaxPages int json:"max_pages"; ActiveMS int64 json:"active_ms" }`
    (`active_ms` is a gap field the UI needs for "elapsed").
  - `GET /api/crawls/{id}?tape_from=N`: `400` for a negative or non-integer `N`.
  - `crawlListItem` gains `StatusClasses map[string]int json:"status_classes,omitempty"`,
    `SitemapTotal int json:"sitemap_total,omitempty"`, `SitemapFetched int json:"sitemap_fetched,omitempty"`,
    `TapePreview string json:"tape_preview,omitempty"` (gap: mini tapes and coverage in the
    Crawls list; the preview is at most 160 characters, worst status wins, so the list stays
    light and never carries a full tape).
  - `store.Entry` gains `ByStatus map[string]int json:"pages_by_status,omitempty"`,
    `SitemapTotal`, `SitemapFetched`, `TapePreview` (`omitempty`) (gap: same reason, for saved
    crawls; the preview comes from the report's `coverage.tape`).
  - `func (s *Server) crawlRunOptions(id string) []runner.RunOption`,
    `func (s *Server) recordProgress(id string, p runner.Progress)`,
    `func (s *Server) crawlView(id string, tapeFrom int) (JobView, bool)`.

Concurrency: `recordProgress` runs on crawl worker goroutines and mutates the job only inside
`s.crawls.with`. It always replaces `Data.Progress` with a fresh pointer (never writes through
it), so a shallow snapshot from `get`/`list` can keep reading the old value. The tape is copied
into the view inside `with`. `list()` snapshots read `Tape[:len]` outside the lock, which is
race-free because appends only write at indices `>= len` of the snapshot.

- [ ] **Step 1: Write the failing store test.** Add the crawler import
  (`"github.com/Patience-dot-devl/gocrawl/internal/crawler"`, and `"strings"` if the file
  doesn't import it yet) to `internal/store/store_test.go` and append:

```go
func TestListCarriesStatusCountsAndSitemapCoverage(t *testing.T) {
	s := New(t.TempDir())
	r := rep("https://example.com/", "2026-10-13T10:00:00Z", 3)
	r.Summary.ByStatus = map[string]int{"200": 2, "404": 1}
	r.Coverage = &crawler.Coverage{Complete: true, SitemapTotal: 5, SitemapFetched: 2, Tape: strings.Repeat("2", 300) + "5"}
	if _, err := s.Save(r); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := s.List("")
	if err != nil || len(entries) != 1 {
		t.Fatalf("List: %v entries, err %v", len(entries), err)
	}
	e := entries[0]
	if e.ByStatus["200"] != 2 || e.ByStatus["404"] != 1 {
		t.Errorf("ByStatus = %v", e.ByStatus)
	}
	if e.SitemapTotal != 5 || e.SitemapFetched != 2 {
		t.Errorf("sitemap = %d/%d, want 2/5", e.SitemapFetched, e.SitemapTotal)
	}
	if len(e.TapePreview) != 151 || !strings.HasSuffix(e.TapePreview, "5") {
		t.Errorf("TapePreview = %q, want 151 chars ending in the 5xx (crawler.TapePreview of the saved tape)", e.TapePreview)
	}
}
```

- [ ] **Step 2: Write the failing webserver test** `internal/webserver/progress_test.go`:

```go
package webserver

import (
	"net/http"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

func pageEvent(fetched, status int, inSitemap bool) runner.Progress {
	return runner.Progress{
		Progress:     crawler.Progress{Fetched: fetched, Status: status, CurrentURL: "https://example.com/p"},
		Phase:        runner.PhaseCrawling,
		SitemapFound: true,
		SitemapTotal: 10,
		InSitemap:    inSitemap,
	}
}

func TestRecordProgressGrowsTapeAndSlicesByTapeFrom(t *testing.T) {
	srv := newTestServer(t)
	job, ok := srv.crawls.start(crawlJob{Seed: "https://example.com/", MaxPages: 500}, func() {}, srv.maxRunning)
	if !ok {
		t.Fatal("could not start a job")
	}
	srv.recordProgress(job.ID, runner.Progress{Phase: runner.PhaseSitemap}) // no page yet
	srv.recordProgress(job.ID, pageEvent(1, 200, true))
	srv.recordProgress(job.ID, pageEvent(2, 301, false))
	phaseOnly := pageEvent(2, 301, false) // same Fetched: a state event adds no tick
	phaseOnly.Phase = runner.PhaseAnalyzing
	srv.recordProgress(job.ID, phaseOnly)
	srv.recordProgress(job.ID, pageEvent(3, 404, true))
	srv.recordProgress(job.ID, pageEvent(4, 0, true))

	_, full := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID, nil)
	prog, ok := full["progress"].(map[string]any)
	if !ok {
		t.Fatalf("no progress in %v", full)
	}
	if prog["tape"] != "234x" || prog["tape_off"] != "0100" || prog["tape_from"] != float64(0) {
		t.Errorf("tape=%v tape_off=%v tape_from=%v, want 234x / 0100 / 0", prog["tape"], prog["tape_off"], prog["tape_from"])
	}
	if prog["max_pages"] != float64(500) || prog["fetched"] != float64(4) || prog["sitemap_total"] != float64(10) {
		t.Errorf("max_pages=%v fetched=%v sitemap_total=%v", prog["max_pages"], prog["fetched"], prog["sitemap_total"])
	}

	_, tail := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID+"?tape_from=2", nil)
	tp := tail["progress"].(map[string]any)
	if tp["tape"] != "4x" || tp["tape_off"] != "00" || tp["tape_from"] != float64(2) {
		t.Errorf("tape_from=2: tape=%v tape_off=%v tape_from=%v", tp["tape"], tp["tape_off"], tp["tape_from"])
	}

	_, past := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID+"?tape_from=99", nil)
	pp := past["progress"].(map[string]any)
	if pp["tape"] != "" || pp["tape_from"] != float64(4) {
		t.Errorf("tape_from past the end: tape=%v tape_from=%v, want empty / 4", pp["tape"], pp["tape_from"])
	}

	for _, bad := range []string{"-1", "abc"} {
		rec, _ := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID+"?tape_from="+bad, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("tape_from=%s: status %d, want 400", bad, rec.Code)
		}
	}
}

func TestRecordProgressWithoutSitemapNeverNotches(t *testing.T) {
	srv := newTestServer(t)
	job, _ := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, srv.maxRunning)
	ev := pageEvent(1, 200, false)
	ev.SitemapFound = false
	srv.recordProgress(job.ID, ev)
	_, out := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID, nil)
	if got := out["progress"].(map[string]any)["tape_off"]; got != "0" {
		t.Errorf("tape_off = %v, want 0 when no sitemap was found", got)
	}
}

func TestCrawlProgressEndToEnd(t *testing.T) {
	srv := newTestServer(t)
	target := startCrawlTarget(t)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "max_pages": 1})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, body %v", rec.Code, started)
	}
	id, _ := started["id"].(string)
	job := pollUntilFinished(t, srv, id)
	prog, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("finished job has no progress: %v", job)
	}
	if prog["phase"] != "analyzing" || prog["tape"] != "2" || prog["fetched"] != float64(1) || prog["max_pages"] != float64(1) {
		t.Errorf("progress = %v", prog)
	}

	_, list := doJSON(t, srv, http.MethodGet, "/api/crawls", nil)
	items, _ := list["crawls"].([]any)
	if len(items) == 0 {
		t.Fatal("empty crawl list")
	}
	first, _ := items[0].(map[string]any)
	classes, _ := first["status_classes"].(map[string]any)
	if classes["2"] != float64(1) {
		t.Errorf("status_classes = %v, want {2: 1}", classes)
	}
	if first["tape_preview"] != "2" {
		t.Errorf("tape_preview = %v, want 2", first["tape_preview"])
	}
	if _, ok := first["tape"]; ok {
		t.Error("crawl list items must not carry the full tape")
	}
}

// After a server restart an in-memory job is gone. A poll for it (with tape_from, as the UI
// sends) must get a JSON 404, so the UI stops polling and says what happened, not a 400 or a
// hang.
func TestGetCrawlForgottenJobIs404(t *testing.T) {
	srv := newTestServer(t)
	rec, out := doJSON(t, srv, http.MethodGet, "/api/crawls/0123456789abcdef?tape_from=40", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
	if msg, _ := out["error"].(string); msg == "" {
		t.Errorf("body %v, want a JSON error", out)
	}
}
```

- [ ] **Step 3: Run and watch them fail.**
  `go test ./internal/store ./internal/webserver -run 'TestListCarries|TestRecordProgress|TestCrawlProgressEndToEnd|TestGetCrawlForgottenJob'`
  Expected: build failures, `e.ByStatus undefined` and `srv.recordProgress undefined`.

- [ ] **Step 4: Implement.**

  `internal/store/store.go`: add to `Entry` (after `BySeverity`):

```go
	// ByStatus, SitemapTotal and SitemapFetched let a crawl list draw a status tape and
	// sitemap coverage per crawl without loading the report again.
	ByStatus       map[string]int `json:"pages_by_status,omitempty"`
	SitemapTotal   int            `json:"sitemap_total,omitempty"`
	SitemapFetched int            `json:"sitemap_fetched,omitempty"`
	// TapePreview is crawler.TapePreview of the report's coverage.tape (at most 160
	// characters); empty for reports saved before tapes were stored.
	TapePreview string `json:"tape_preview,omitempty"`
```

  add the import `"github.com/Patience-dot-devl/gocrawl/internal/crawler"` to `store.go`, and
  replace the `return Entry{...}` in `entryFor` with:

```go
	e := Entry{
		ID:           id,
		Host:         hostOf(rep.Seed),
		Seed:         rep.Seed,
		FinishedAt:   rep.FinishedAt,
		PagesCrawled: rep.PagesCrawled,
		BySeverity:   rep.Summary.BySeverity,
		ByStatus:     rep.Summary.ByStatus,
		Path:         path,
	}
	if rep.Coverage != nil {
		e.SitemapTotal, e.SitemapFetched = rep.Coverage.SitemapTotal, rep.Coverage.SitemapFetched
		e.TapePreview = crawler.TapePreview(rep.Coverage.Tape, crawler.TapePreviewLen)
	}
	return e
```

  `internal/webserver/crawl_job.go`: make `crawlJob` read (keep any wave-0 fields not listed
  here, such as a `control` placeholder, as they are):

```go
type crawlJob struct {
	Seed    string
	Report  *report.Report
	StoreID string
	// MaxPages is the crawl's page budget (0 = unlimited), shown next to live progress.
	MaxPages int
	// Progress is the latest snapshot. recordProgress replaces the pointer on every event and
	// never writes through it, so a shallow Job snapshot may keep reading the old value.
	Progress *runner.Progress
	// Tape holds one status-class char per recorded page ('2','3','4','5','x'); TapeOff holds
	// '1' where that page is not in the sitemap. Appended only under the job manager's lock.
	Tape    []byte
	TapeOff []byte
}
```

  `internal/webserver/crawl_job.go`: add to `JobView` (last field):

```go
	Progress *progressView `json:"progress,omitempty"`
```

  add to `crawlListItem` (after `BySeverity`):

```go
	StatusClasses  map[string]int `json:"status_classes,omitempty"`
	SitemapTotal   int            `json:"sitemap_total,omitempty"`
	SitemapFetched int            `json:"sitemap_fetched,omitempty"`
	// TapePreview is a short (<= 160 chars) worst-status-wins tape for the list's mini tape.
	// The full tape is only in GET /api/crawls/{id}.
	TapePreview string `json:"tape_preview,omitempty"`
```

  and in `jobListItem`, right before `return item`:

```go
	item.StatusClasses, item.SitemapTotal, item.SitemapFetched, item.TapePreview = listProgress(j)
```

  Create `internal/webserver/progress.go`:

```go
package webserver

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// progressView is JobView.progress: the latest runner.Progress plus the tape from tape_from.
type progressView struct {
	runner.Progress
	Tape     string `json:"tape"`
	TapeOff  string `json:"tape_off"`
	TapeFrom int    `json:"tape_from"` // index the returned tape slice starts at
	MaxPages int    `json:"max_pages"`
	ActiveMS int64  `json:"active_ms"`
}

// crawlRunOptions are the runner options every web crawl gets.
func (s *Server) crawlRunOptions(id string) []runner.RunOption {
	return []runner.RunOption{
		runner.WithProgress(func(p runner.Progress) { s.recordProgress(id, p) }),
	}
}

// recordProgress stores the latest snapshot and grows the tape by one character per recorded
// page, with the same helpers runner.Run uses for the saved Coverage.Tape (crawler.TapeChar,
// Progress.TapeOffChar), so the live and saved tapes match. It runs on a crawl worker goroutine
// (engine locks held, see the lock order in the plan), so it does nothing but a few
// assignments under the job manager's lock.
func (s *Server) recordProgress(id string, p runner.Progress) {
	s.crawls.with(id, func(j *Job[crawlJob]) {
		snap := p
		j.Data.Progress = &snap
		if p.Fetched > len(j.Data.Tape) {
			j.Data.Tape = append(j.Data.Tape, crawler.TapeChar(p.Status))
			j.Data.TapeOff = append(j.Data.TapeOff, p.TapeOffChar())
		}
	})
}

// newProgressView builds the API view of j's progress. Call it inside s.crawls.with: it reads
// the tape recordProgress appends to.
func newProgressView(j *Job[crawlJob], tapeFrom int, now time.Time) *progressView {
	if j.Data.Progress == nil {
		return nil
	}
	if tapeFrom > len(j.Data.Tape) {
		tapeFrom = len(j.Data.Tape)
	}
	return &progressView{
		Progress: *j.Data.Progress,
		Tape:     string(j.Data.Tape[tapeFrom:]),
		TapeOff:  string(j.Data.TapeOff[tapeFrom:]),
		TapeFrom: tapeFrom,
		MaxPages: j.Data.MaxPages,
		ActiveMS: activeMS(j, now),
	}
}

// activeMS is how long the job has been running.
func activeMS(j *Job[crawlJob], now time.Time) int64 {
	end := now
	if !j.FinishedAt.IsZero() {
		end = j.FinishedAt
	}
	return end.Sub(j.StartedAt).Milliseconds()
}

// crawlView is the JobView for a live or finished in-memory job, with the tape sliced from
// tapeFrom. False when id is not an in-memory job.
func (s *Server) crawlView(id string, tapeFrom int) (JobView, bool) {
	var v JobView
	ok := s.crawls.with(id, func(j *Job[crawlJob]) {
		v = jobView(*j)
		v.Progress = newProgressView(j, tapeFrom, time.Now())
	})
	return v, ok
}

// parseTapeFrom reads the tape_from query parameter (default 0).
func parseTapeFrom(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("tape_from must be a non-negative integer, got %q", raw)
	}
	return n, nil
}

// classesFromTape counts tape characters per status class.
func classesFromTape(tape []byte) map[string]int {
	if len(tape) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, c := range tape {
		out[string(c)]++
	}
	return out
}

// classesFromByStatus folds a report's pages_by_status into status classes.
func classesFromByStatus(byStatus map[string]int) map[string]int {
	if len(byStatus) == 0 {
		return nil
	}
	out := map[string]int{}
	for code, n := range byStatus {
		status, _ := strconv.Atoi(code)
		out[string(crawler.TapeChar(status))] += n
	}
	return out
}

// listProgress returns the status classes, sitemap coverage and tape preview for a crawl list
// row. j is a list() snapshot: reading j.Data.Tape[:len] outside the lock is safe because later
// appends only write past that length.
func listProgress(j Job[crawlJob]) (classes map[string]int, sitemapTotal, sitemapFetched int, preview string) {
	switch {
	case len(j.Data.Tape) > 0:
		classes = classesFromTape(j.Data.Tape)
		preview = crawler.TapePreview(string(j.Data.Tape), crawler.TapePreviewLen)
	case j.Data.Report != nil:
		classes = classesFromByStatus(j.Data.Report.Summary.ByStatus)
		if j.Data.Report.Coverage != nil {
			preview = crawler.TapePreview(j.Data.Report.Coverage.Tape, crawler.TapePreviewLen)
		}
	}
	if rep := j.Data.Report; rep != nil && rep.Coverage != nil {
		return classes, rep.Coverage.SitemapTotal, rep.Coverage.SitemapFetched, preview
	}
	if p := j.Data.Progress; p != nil {
		return classes, p.SitemapTotal, p.SitemapFetched, preview
	}
	return classes, 0, 0, preview
}
```

  `internal/webserver/crawls.go`:

  1. In `handleStartCrawl`, change the `start` call to
     `s.crawls.start(crawlJob{Seed: seed, MaxPages: cfg.Crawl.MaxPages}, cancel, s.maxRunning)`
     and the run call to `runner.Run(ctx, cfg, seed, s.crawlRunOptions(job.ID)...)`.

  2. Replace the in-memory branch at the top of `handleGetCrawl` (the lines from
     `id := r.PathValue("id")` through the `return` after writing the in-memory job view) with:

```go
	id := r.PathValue("id")
	tapeFrom, err := parseTapeFrom(r.URL.Query().Get("tape_from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if view, ok := s.crawlView(id, tapeFrom); ok {
		writeJSON(w, http.StatusOK, view)
		return
	}
```

     The store fallback below it stays unchanged (a saved crawl has no `progress`).

  3. In `handleListCrawls`, in the `crawlListItem{...}` literal built for store entries, add:

```go
			StatusClasses:  classesFromByStatus(e.ByStatus),
			SitemapTotal:   e.SitemapTotal,
			SitemapFetched: e.SitemapFetched,
			TapePreview:    e.TapePreview,
```

- [ ] **Step 5: Run.** `go test -race ./internal/store ./internal/webserver`
  Expected: `ok` for both.

- [ ] **Step 6: `make check`.** Expected: exits 0.

- [ ] **Step 7: Commit.**

```sh
git add internal/store/store.go internal/store/store_test.go internal/webserver/crawl_job.go internal/webserver/crawls.go internal/webserver/progress.go internal/webserver/progress_test.go
git commit -F - <<'EOF'
feat(webserver): stream crawl progress and the status tape through GET /api/crawls/{id}

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-9: `POST /api/sitemap-preview`

**Files:**
- Modify: `internal/webserver/sitemap.go` (wave 0 stub `sitemapRoutes`)
- Create: `internal/webserver/sitemap_test.go`

**Interfaces:**
- Consumes: `runner.PreviewSitemap`, `runner.SitemapPreview` (W1-PROG-7), `requireJSON`,
  `crawlrequest.Params`.
- Produces: `POST /api/sitemap-preview`, body `{url, include, exclude, allow_subdomains}`,
  `200 {found, total, sources, via_robots}`; `400` bad URL/regex/body; `415` non-JSON;
  `403` cross-origin (guard).

- [ ] **Step 1: Write the failing test** `internal/webserver/sitemap_test.go`:

```go
package webserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sitemapTarget(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", base)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>%[1]s/</loc></url><url><loc>%[1]s/a</loc></url><url><loc>%[1]s/blog/x</loc></url></urlset>`, base)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestSitemapPreview(t *testing.T) {
	srv := newTestServer(t)
	target := sitemapTarget(t)

	rec, out := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": target.URL})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %v", rec.Code, out)
	}
	if out["found"] != true || out["total"] != float64(3) || out["via_robots"] != true {
		t.Errorf("preview = %v, want found, 3, via robots.txt", out)
	}
	if sources, _ := out["sources"].([]any); len(sources) != 1 {
		t.Errorf("sources = %v, want one", out["sources"])
	}

	_, scoped := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": target.URL, "exclude": []string{"/blog/"}})
	if scoped["total"] != float64(2) {
		t.Errorf("total with /blog/ excluded = %v, want 2", scoped["total"])
	}
}

func TestSitemapPreviewNoSitemap(t *testing.T) {
	srv := newTestServer(t)
	target := startCrawlTarget(t) // HTML for every path: no parseable sitemap
	rec, out := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": target.URL})
	if rec.Code != http.StatusOK || out["found"] != false || out["total"] != float64(0) {
		t.Errorf("status %d, preview %v, want 200 not found", rec.Code, out)
	}
}

func TestSitemapPreviewRejectsBadInput(t *testing.T) {
	srv := newTestServer(t)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing url: status %d, want 400", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": "https://example.com", "exclude": []string{"("}}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad regex: status %d, want 400", rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sitemap-preview", strings.NewReader(`{"url":"https://example.com"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: status %d, want 415", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/sitemap-preview", strings.NewReader(`{"url":"https://example.com"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status %d, want 403", rec.Code)
	}
}
```

- [ ] **Step 2: Run and watch it fail.** `go test ./internal/webserver -run TestSitemapPreview`
  Expected: `FAIL` — the route is not registered yet, so the first request falls through to the
  frontend handler (status 200 with HTML or 405), and `doJSON` fails decoding the body.

- [ ] **Step 3: Implement.** Replace the wave 0 stub in `internal/webserver/sitemap.go` with:

```go
package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// sitemapPreviewTimeout bounds one preview: robots.txt plus a sitemap index two levels deep.
const sitemapPreviewTimeout = 20 * time.Second

// sitemapPreviewRequest is the POST /api/sitemap-preview body: the scope fields of a crawl.
type sitemapPreviewRequest struct {
	URL             string   `json:"url"`
	Include         []string `json:"include,omitempty"`
	Exclude         []string `json:"exclude,omitempty"`
	AllowSubdomains bool     `json:"allow_subdomains,omitempty"`
}

func (s *Server) sitemapRoutes() {
	s.mux.HandleFunc("POST /api/sitemap-preview", s.handleSitemapPreview)
}

// handleSitemapPreview runs only the crawl's sitemap discovery step, so the New crawl form can
// show how many in-scope pages the sitemap lists before anything is crawled.
func (s *Server) handleSitemapPreview(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var req sitemapPreviewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	subdomains := req.AllowSubdomains
	cfg, seed, err := crawlrequest.Params{
		URL:        req.URL,
		Include:    req.Include,
		Exclude:    req.Exclude,
		Subdomains: &subdomains,
	}.ToConfig()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), sitemapPreviewTimeout)
	defer cancel()
	preview, err := runner.PreviewSitemap(ctx, cfg, seed)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/webserver -run TestSitemapPreview`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/webserver/sitemap.go internal/webserver/sitemap_test.go
git commit -F - <<'EOF'
feat(webserver): add POST /api/sitemap-preview

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W1-PROG-10: Docs and changelog for the progress backend

**Files:**
- Modify: `docs/web.md`, `docs/output.md`, `docs/architecture.md`, `docs/README.md`
  (generated), `CHANGELOG.md`

- [ ] **Step 1: `docs/web.md`.** Set frontmatter `updated: 2026-10-13`. Delete the paragraph
  that starts "There is no live per-page progress in this version". In the API table, replace
  the `GET /api/crawls` and `GET /api/crawls/{id}` rows and add the preview row:

```markdown
| `GET /api/crawls` | List every job (in-memory, this process) merged with the store's saved history, newest first. Each item also carries `status_classes` (pages per status class: `2`, `3`, `4`, `5`, `x`), `tape_preview` (the crawl tape shortened to at most 160 characters, each standing for a run of pages and showing the worst status among them) and, when known, `sitemap_total` / `sitemap_fetched`. List items never carry the full tape. |
| `GET /api/crawls/{id}` | One job's status, its live `progress` (see [Live progress](#live-progress)) and, once finished, its full `Report` (see [Output reference](output.md)). `?tape_from=N` returns only the tape characters from index `N`; a negative or non-numeric `N` is a `400`. Falls back to the store for an id that isn't a live job; saved crawls have no `progress`, but their report's `coverage.tape` / `tape_off` hold the same tape. An id that is neither (for example a crawl that was running when the server restarted) is a `404`. |
| `POST /api/sitemap-preview` | Find a site's sitemap without crawling. JSON body `{url, include, exclude, allow_subdomains}`; returns `{found, total, sources, via_robots}`, where `total` counts only sitemap URLs inside that scope. Respects robots.txt and sends the crawl's default User-Agent. |
```

  Then add this section after the API table's "Job status is one of…" line:

```markdown
## Live progress

While a crawl runs (and afterwards, until the server restarts) the job view carries a
`progress` object. The UI polls it every 1.5 seconds, passing `tape_from` so each poll only
transfers the new tape characters.

| Field | Description |
| --- | --- |
| `phase` | `sitemap` (reading robots.txt and the sitemap before the crawl), `crawling`, or `analyzing`. |
| `fetched` / `queued` | Pages recorded so far, and URLs waiting to be fetched. |
| `current_url`, `final_url`, `status`, `duration_ms` | The page recorded last. `status` is the first redirect hop's status when the fetch was redirected, and `0` when it failed. |
| `rate` | The request-rate limit in effect after any adaptive backoff (`0` = unlimited). |
| `throttled` | HTTP 429 and 503 responses in the last 60 seconds. |
| `sitemap_found`, `sitemap_total`, `sitemap_fetched`, `off_sitemap`, `in_sitemap` | Progress against the site's sitemap: in-scope sitemap URLs, how many of them were fetched, fetched pages the sitemap doesn't list, and whether `current_url` is in it. |
| `max_pages` | The page budget (`0` = unlimited). |
| `active_ms` | How long the crawl has been running. |
| `tape`, `tape_off`, `tape_from` | One character per recorded page, in fetch order. `tape` holds the status class (`2`, `3`, `4`, `5`, or `x` for a failed fetch); `tape_off` holds `1` for a page the sitemap doesn't list. `tape_from` is the index the returned slice starts at. |

Before every crawl, CLI and MCP included, gocrawl reads the site's sitemap once: robots.txt
`Sitemap:` lines, then `/sitemap.xml` and `/sitemap_index.xml`, following indexes. Only URLs
inside the crawl scope count toward `sitemap_total`. The `sitemap` analyzer reuses the same
set instead of fetching it again.
```

- [ ] **Step 2: `docs/output.md`.** Set `updated: 2026-10-13`. Add four rows to the Coverage
  table, after the `max_pages` / `max_depth` row:

```markdown
| `sitemap_total` | int | In-scope URLs the site's sitemap lists, read once before the crawl. Omitted when no sitemap was found. |
| `sitemap_fetched` | int | How many of those sitemap URLs this crawl fetched. Omitted when 0. |
| `tape` | string | The crawl tape: one character per page in fetch order, `2`, `3`, `4`, `5` for the status class (a redirected page counts as its first hop, so it reads `3`) or `x` for a failed fetch. The web UI draws it; it is the same tape the live view showed. |
| `tape_off` | string | Same length as `tape`: `1` where a sitemap was found and the page is not in its in-scope part, `0` otherwise. |
```

  and append this paragraph to the end of the Coverage section (before "### Example JSON report"):

```markdown
When the page budget is below `sitemap_total`, `notes` says so up front, for example: "The page
budget (500) is below the sitemap total (1,240); this crawl covered at most 40% of the sitemap."
```

- [ ] **Step 3: `docs/architecture.md`.** Set `updated: 2026-10-13`. Add a bullet to "The crawl
  engine" list, before the "Stopping early" bullet:

```markdown
- `Options.OnProgress`, when set, is called after each page is recorded with a
  [`crawler.Progress`](../internal/crawler/progress.go) snapshot: pages recorded, queue length,
  the page just recorded, the rate limit in effect, and 429/503 responses in the last minute.
  Calls are serialized in recording order and run while the engine holds its locks, so the
  callback must return quickly and never call back into the engine.
```

  and add this paragraph right after "The engine produces a `crawler.Result` …" paragraph:

```markdown
Before the crawl, [`runner.Run`](../internal/runner/runner.go) runs a sitemap pre-pass:
`sitemap.DiscoverAll` on the governed analyzer fetcher, filtered through `Engine.InScope`. The
`sitemap` analyzer gets the result through `sitemap.WithURLs`, so the sitemap is fetched once
per crawl, and `Coverage.SitemapTotal` / `SitemapFetched` come from it. `runner.WithProgress`
wraps the engine callback into a `runner.Progress` that adds the run phase and the sitemap
counts; the web server uses it for live progress.
```

- [ ] **Step 4: `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`, add one line:

```markdown
- Live crawl progress in the web API: `GET /api/crawls/{id}` returns a `progress` object (phase, pages against the site's sitemap, rate, recent 429/503s, and a per-page status tape sliced with `?tape_from=N`), and `POST /api/sitemap-preview` finds a site's sitemap before crawling. Every crawl now reads the sitemap once up front, reports `coverage.sitemap_total` / `sitemap_fetched`, saves the per-page tape in `coverage.tape` / `tape_off`, and adds a note when the page budget is below the sitemap total.
```

- [ ] **Step 5: Regenerate and check.** `make docs && make check`
  Expected: `make docs` rewrites `depends_on` / `sources` and the `docs/README.md` index;
  `make check` exits 0 (the repocheck link and frontmatter tests pass).

- [ ] **Step 6: Commit.**

```sh
git add docs/web.md docs/output.md docs/architecture.md docs/README.md CHANGELOG.md
git commit -F - <<'EOF'
docs: document live progress, the sitemap pre-pass and sitemap coverage

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

### Track: settings backend

#### Task W1-SET-1: Bearer token in config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/example.go`, plus its two byte-identical copies `configs/example.yaml`
  and the example-config block in `docs/configuration.md` (`TestExampleYAMLCopiesAreInSync`)
- Modify: `internal/render/render.go`
- Test: `internal/config/headers_test.go`, `internal/config/config_test.go`

**Interfaces:**
- Consumes: `crawler.ParseHeaderLines(lines []string) (http.Header, error)`.
- Produces: `CrawlConfig.BearerToken string \`mapstructure:"bearer_token"\``; config key
  `crawl.bearer_token`; env `GOCRAWL_CRAWL_BEARER_TOKEN`; `Config.ToOptions()` sets
  `Options.Headers["Authorization"] = "Bearer <token>"` and errors (naming `bearer_token`,
  never the token) when combined with `basic_auth` or any custom `Authorization` header.

- [ ] **Step 1: Write the failing tests.** Append to `internal/config/headers_test.go`:

```go
// A bearer token becomes an Authorization header on the custom-headers path, so it inherits
// that path's host restriction (crawler.HTTPFetcher.authHostAllowed). A pasted "Bearer "
// prefix is accepted rather than doubled.
func TestToOptionsBearerTokenSetsAuthorization(t *testing.T) {
	for _, token := range []string{"tok-123", "  tok-123  ", "Bearer tok-123", "bearer tok-123"} {
		c := Default()
		c.Crawl.BearerToken = token
		o, err := c.ToOptions()
		if err != nil {
			t.Fatalf("ToOptions(%q): %v", token, err)
		}
		if got := o.Headers.Get("Authorization"); got != "Bearer tok-123" {
			t.Errorf("BearerToken %q: Authorization = %q, want %q", token, got, "Bearer tok-123")
		}
	}
}

func TestToOptionsBearerTokenKeepsOtherHeaders(t *testing.T) {
	c := Default()
	c.Crawl.BearerToken = "tok"
	c.Crawl.Headers = []string{"X-Store: uk"}
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if o.Headers.Get("X-Store") != "uk" || o.Headers.Get("Authorization") != "Bearer tok" {
		t.Errorf("Headers = %v, want X-Store and the bearer Authorization", o.Headers)
	}
}

// Basic Auth and a custom Authorization header set the same header as the bearer token, so
// the combination is an error. The error must never echo the token: it reaches the CLI, MCP
// tool errors, and web API responses.
func TestToOptionsBearerTokenConflicts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeFile(t, filepath.Join(home, ".gocrawl", "headers", "auth.headers"), "Authorization: Basic eDp5\n")
	const token = "very-secret-token"
	cases := []struct {
		name string
		set  func(*Config)
	}{
		{"basic_auth", func(c *Config) { c.Crawl.BasicAuth = "alice:pw" }},
		{"inline Authorization header", func(c *Config) { c.Crawl.Headers = []string{"Authorization: Token abc"} }},
		{"Authorization from a header profile", func(c *Config) { c.Crawl.HeaderProfile = "auth" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			c.Crawl.BearerToken = token
			tc.set(&c)
			_, err := c.ToOptions()
			if err == nil {
				t.Fatal("ToOptions succeeded, want a conflict error")
			}
			if !strings.Contains(err.Error(), "bearer_token") {
				t.Errorf("error %q doesn't name bearer_token", err)
			}
			if strings.Contains(err.Error(), token) {
				t.Errorf("error %q leaks the token", err)
			}
		})
	}
}

func TestToOptionsBearerTokenRejectsHeaderInjection(t *testing.T) {
	c := Default()
	c.Crawl.BearerToken = "zq9-secret\r\nX-Evil: 1"
	_, err := c.ToOptions()
	if err == nil {
		t.Fatal("ToOptions accepted a token containing CR/LF")
	}
	if strings.Contains(err.Error(), "X-Evil") || strings.Contains(err.Error(), "zq9-secret") {
		t.Errorf("error %q echoes the token", err)
	}
}
```

Append to `internal/config/config_test.go`:

```go
func TestLoadReadsBearerTokenFromEnv(t *testing.T) {
	t.Setenv("GOCRAWL_CRAWL_BEARER_TOKEN", "env-token")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Crawl.BearerToken != "env-token" {
		t.Errorf("Crawl.BearerToken = %q, want env-token", cfg.Crawl.BearerToken)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/config/` — expected FAIL at build:
  `c.Crawl.BearerToken undefined (type CrawlConfig has no field or method BearerToken)`.

- [ ] **Step 3: Implement.** In `internal/config/config.go`:

  1. Add `"errors"` to the import block.
  2. In `CrawlConfig`, directly after the `HeaderProfile` line, add:

```go
	// BearerToken is sent as "Authorization: Bearer <token>" through the custom-headers path,
	// so it gets exactly the host restriction Headers get. It can't be combined with
	// BasicAuth or a custom Authorization header, which set the same header.
	BearerToken string `mapstructure:"bearer_token"`
```

  3. In `setDefaults`, after `v.SetDefault("crawl.header_profile", d.Crawl.HeaderProfile)`, add:

```go
	v.SetDefault("crawl.bearer_token", d.Crawl.BearerToken)
```

  4. In `ToOptions`, replace

```go
	if headers.Get("Authorization") != "" && o.BasicAuthUser != "" {
		return o, fmt.Errorf("a custom Authorization header can't be combined with basic_auth, which sets the same header")
	}
	o.Headers = headers
```

  with

```go
	if headers.Get("Authorization") != "" && o.BasicAuthUser != "" {
		return o, fmt.Errorf("a custom Authorization header can't be combined with basic_auth, which sets the same header")
	}
	headers, err = c.Crawl.withBearerToken(headers, o.BasicAuthUser != "")
	if err != nil {
		return o, err
	}
	o.Headers = headers
```

  5. Add after `loadHeaders`:

```go
// withBearerToken adds BearerToken, when set, to headers as "Authorization: Bearer <token>".
// A pasted "Bearer " prefix is dropped rather than doubled. The token never appears in an
// error: these errors reach the CLI, MCP tool results and web API responses.
func (c CrawlConfig) withBearerToken(headers http.Header, hasBasicAuth bool) (http.Header, error) {
	token := strings.TrimSpace(c.BearerToken)
	if token == "" {
		return headers, nil
	}
	if hasBasicAuth {
		return nil, errors.New("bearer_token can't be combined with basic_auth: both set the Authorization header")
	}
	if headers.Get("Authorization") != "" {
		return nil, errors.New("bearer_token can't be combined with a custom Authorization header (from headers, header_file or header_profile)")
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[len("bearer "):])
	}
	parsed, err := crawler.ParseHeaderLines([]string{"Authorization: Bearer " + token})
	if err != nil {
		return nil, errors.New("bearer_token contains characters not allowed in an HTTP header")
	}
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Authorization", parsed.Get("Authorization"))
	return headers, nil
}
```

  In `internal/config/example.go`, directly after the line `  header_profile: ""`, insert:

```yaml
  # Token sent as "Authorization: Bearer <token>", scoped and leak-guarded the same way as
  # basic_auth, and not supported with render: "headless". Can't be combined with basic_auth
  # or a custom Authorization header. Prefer GOCRAWL_CRAWL_BEARER_TOKEN or --bearer-token to
  # writing the token into this file.
  bearer_token: ""
```

  Insert the same five lines, byte for byte, after `  header_profile: ""` in
  `configs/example.yaml` and in the fenced YAML block under "## Example config file" in
  `docs/configuration.md`: `TestExampleYAMLCopiesAreInSync` requires all three copies to be
  identical.

  In `internal/render/render.go` (`NewHeadlessFetcher`), change the custom-headers error to:

```go
		return nil, fmt.Errorf("custom headers and bearer tokens (--header / --header-file / --header-profile / --bearer-token) are not supported with --render headless (Chromium can't scope them to the target host, so they would be sent to every third-party resource the page loads); use raw mode instead")
```

  Run `gofmt -w internal/config internal/render`.

- [ ] **Step 4: Run.** `go test ./internal/config/ ./internal/render/` — expected PASS
  (including `TestExampleYAMLCoversEveryOption`, which needs the new `bearer_token:` key, and
  `TestExampleYAMLCopiesAreInSync`, which needs the two copies updated).
  Then `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/config/config.go internal/config/example.go configs/example.yaml docs/configuration.md internal/config/headers_test.go internal/config/config_test.go internal/render/render.go
git commit -F - <<'MSG'
feat(config): add crawl.bearer_token sent as a host-scoped Authorization header

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-SET-2: New crawl parameters in `crawlrequest.Params` (and MCP)

**Files:**
- Modify: `internal/crawlrequest/crawlrequest.go`
- Test: `internal/crawlrequest/crawlrequest_test.go`, `internal/mcpserver/tools_test.go`

**Interfaces:**
- Consumes: `CrawlConfig.BearerToken` (W1-SET-1).
- Produces (contract names): `Params.StripQuery bool`, `Params.AdaptiveDelay *bool`,
  `Params.FollowNofollow bool`, `Params.Timeout string`, `Params.BearerToken string`, mapped in
  `ToConfig`. MCP `crawl` input schema gains the same properties (it is `crawlrequest.Params`).

- [ ] **Step 1: Write the failing tests.** In `internal/crawlrequest/crawlrequest_test.go`
  change the import block to

```go
import (
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)
```

  and append:

```go
func TestToConfig_MapsCrawlBehaviorParams(t *testing.T) {
	adaptive := false
	cfg, _, err := Params{
		URL:            "https://example.com",
		StripQuery:     true,
		AdaptiveDelay:  &adaptive,
		FollowNofollow: true,
		Timeout:        " 7s ",
		BearerToken:    " tok-123 ",
	}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if !cfg.Crawl.StripQuery {
		t.Error("StripQuery = false, want true")
	}
	if cfg.Crawl.AdaptiveDelay {
		t.Error("AdaptiveDelay = true, want false (explicit override)")
	}
	if !cfg.Crawl.FollowNofollow {
		t.Error("FollowNofollow = false, want true")
	}
	if cfg.Crawl.Timeout != 7*time.Second {
		t.Errorf("Timeout = %v, want 7s", cfg.Crawl.Timeout)
	}
	if cfg.Crawl.BearerToken != "tok-123" {
		t.Errorf("BearerToken = %q, want tok-123 (trimmed)", cfg.Crawl.BearerToken)
	}
}

func TestToConfig_CrawlBehaviorDefaults(t *testing.T) {
	cfg, _, err := Params{URL: "https://example.com"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	d := config.Default()
	if !cfg.Crawl.AdaptiveDelay || cfg.Crawl.AdaptiveDelay != d.Crawl.AdaptiveDelay {
		t.Error("AdaptiveDelay should default to the config default (true)")
	}
	if cfg.Crawl.Timeout != d.Crawl.Timeout {
		t.Errorf("Timeout = %v, want the default %v", cfg.Crawl.Timeout, d.Crawl.Timeout)
	}
	if cfg.Crawl.StripQuery || cfg.Crawl.FollowNofollow || cfg.Crawl.BearerToken != "" {
		t.Errorf("unexpected non-default: strip_query=%v follow_nofollow=%v bearer set=%v",
			cfg.Crawl.StripQuery, cfg.Crawl.FollowNofollow, cfg.Crawl.BearerToken != "")
	}
}

func TestToConfig_InvalidTimeout(t *testing.T) {
	for _, v := range []string{"soon", "-1s", "0s"} {
		if _, _, err := (Params{URL: "https://example.com", Timeout: v}).ToConfig(); err == nil {
			t.Errorf("timeout %q: expected an error", v)
		}
	}
}
```

  Append to `internal/mcpserver/tools_test.go`:

```go
// CrawlInput is crawlrequest.Params, so new crawl parameters reach MCP through the generated
// input schema. login_session is web-only and must not.
func TestCrawlToolSchemaListsCrawlParams(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "crawl" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"strip_query", "adaptive_delay", "follow_nofollow", "timeout", "bearer_token"} {
			if _, ok := schema.Properties[name]; !ok {
				t.Errorf("crawl input schema has no %q property", name)
			}
		}
		if _, ok := schema.Properties["login_session"]; ok {
			t.Error("crawl input schema exposes login_session, which is web-only")
		}
		return
	}
	t.Fatal("no crawl tool listed")
}
```

- [ ] **Step 2: Run them.** `go test ./internal/crawlrequest/ ./internal/mcpserver/` — expected
  FAIL: crawlrequest at build (`unknown field StripQuery in struct literal of type Params`),
  mcpserver with `crawl input schema has no "strip_query" property` (and the other four).

- [ ] **Step 3: Implement.** In `internal/crawlrequest/crawlrequest.go`, in `Params`, after the
  `Exclude` field add:

```go
	StripQuery     bool   `json:"strip_query,omitempty" jsonschema:"Ignore query strings, treating ?a=1 and ?a=2 as one URL; this skips the query-dependent utm/tracking analysis (default false)"`
	AdaptiveDelay  *bool  `json:"adaptive_delay,omitempty" jsonschema:"Slow the crawl automatically when the server answers HTTP 429/503, honoring Retry-After (default true)"`
	FollowNofollow bool   `json:"follow_nofollow,omitempty" jsonschema:"Also follow links marked rel=nofollow (default false)"`
	Timeout        string `json:"timeout,omitempty" jsonschema:"Per-request timeout as a Go duration string, e.g. '10s' (default 15s)"`
```

  and after the `BasicAuth` field add:

```go
	BearerToken string `json:"bearer_token,omitempty" jsonschema:"Token sent as 'Authorization: Bearer <token>', only to the seed host (plus subdomains when subdomains is set); raw render mode only. Can't be combined with basic_auth or a custom Authorization header"`
```

  In `ToConfig`, directly after the `MaxDuration` block, add:

```go
	if t := strings.TrimSpace(p.Timeout); t != "" {
		d, derr := time.ParseDuration(t)
		if derr != nil {
			return config.Config{}, "", fmt.Errorf("timeout %q: %w", p.Timeout, derr)
		}
		if d <= 0 {
			return config.Config{}, "", fmt.Errorf("timeout %q: must be greater than zero", p.Timeout)
		}
		cfg.Crawl.Timeout = d
	}
	cfg.Crawl.StripQuery = p.StripQuery
	cfg.Crawl.FollowNofollow = p.FollowNofollow
	if p.AdaptiveDelay != nil {
		cfg.Crawl.AdaptiveDelay = *p.AdaptiveDelay
	}
```

  and after `cfg.Crawl.BasicAuth = p.BasicAuth` add:

```go
	cfg.Crawl.BearerToken = strings.TrimSpace(p.BearerToken)
```

  Run `gofmt -w internal/crawlrequest`.

- [ ] **Step 4: Run.** `go test ./internal/crawlrequest/ ./internal/mcpserver/` — expected PASS.
  `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/crawlrequest/crawlrequest.go internal/crawlrequest/crawlrequest_test.go internal/mcpserver/tools_test.go
git commit -F - <<'MSG'
feat(crawlrequest): accept strip_query, adaptive_delay, follow_nofollow, timeout, bearer_token

The MCP crawl tool shares Params, so its input schema gains the same fields.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-SET-3: `--bearer-token` on the CLI and in interactive mode

**Files:**
- Modify: `cmd/gocrawl/crawl.go`, `cmd/gocrawl/interactive.go`
- Test: `cmd/gocrawl/crawl_test.go`

**Interfaces:**
- Consumes: `CrawlConfig.BearerToken` (W1-SET-1).
- Produces: CLI flag `--bearer-token string`; interactive "Bearer token" field.

- [ ] **Step 1: Write the failing tests.** In `cmd/gocrawl/crawl_test.go` add `"sync"` to the
  import block and append:

```go
// A bearer token goes to the seed host only: a crawl that follows a link to another host
// (--external) must not send it there.
func TestCrawlBearerTokenOnlyReachesSeedHost(t *testing.T) {
	var mu sync.Mutex
	var seedAuth, otherAuth string
	otherHit := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		otherHit = true
		otherAuth = r.Header.Get("Authorization")
		mu.Unlock()
		fmt.Fprint(w, `<html><head><title>Other</title></head><body>other</body></html>`)
	}))
	defer other.Close()
	// Same IP, different host name: credential scoping compares host names without the
	// port, so a second 127.0.0.1 server would count as the seed host.
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)
	seed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			mu.Lock()
			seedAuth = r.Header.Get("Authorization")
			mu.Unlock()
		}
		fmt.Fprintf(w, `<html><head><title>Home</title></head><body><a href="%s/page">other</a></body></html>`, otherURL)
	}))
	defer seed.Close()

	cmd := newCrawlCmd()
	for name, value := range map[string]string{
		"format":         "json",
		"out":            filepath.Join(t.TempDir(), "report.json"),
		"bearer-token":   "tok-123",
		"external":       "true",
		"respect-robots": "false",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	cmd.SetContext(context.Background())
	if err := runCrawl(cmd, []string{seed.URL}); err != nil {
		t.Fatalf("runCrawl: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seedAuth != "Bearer tok-123" {
		t.Errorf("seed Authorization = %q, want %q", seedAuth, "Bearer tok-123")
	}
	if !otherHit {
		t.Fatal("the crawl never reached the second host, so the test proves nothing")
	}
	if otherAuth != "" {
		t.Errorf("second host got Authorization %q, want none", otherAuth)
	}
}

func TestCrawlBearerTokenConflictsWithBasicAuth(t *testing.T) {
	cmd := newCrawlCmd()
	for name, value := range map[string]string{
		"out":          filepath.Join(t.TempDir(), "report.json"),
		"bearer-token": "tok-secret",
		"basic-auth":   "alice:pw",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	cmd.SetContext(context.Background())
	err := runCrawl(cmd, []string{"http://127.0.0.1:1/"})
	if err == nil {
		t.Fatal("runCrawl succeeded with both --bearer-token and --basic-auth")
	}
	if !strings.Contains(err.Error(), "bearer_token") {
		t.Errorf("error %q doesn't name bearer_token", err)
	}
	if strings.Contains(err.Error(), "tok-secret") {
		t.Errorf("error %q leaks the token", err)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./cmd/gocrawl/ -run Bearer` — expected FAIL:
  `flag accessed but not defined: bearer-token` (from `cmd.Flags().Set`).

- [ ] **Step 3: Implement.** In `cmd/gocrawl/crawl.go`, after the `header-profile` flag:

```go
	f.String("bearer-token", "", `token sent as "Authorization: Bearer <token>", only to the seed host like --cookie (a flag value lands in shell history; GOCRAWL_CRAWL_BEARER_TOKEN doesn't)`)
```

  In `applyFlagOverrides`, after the `header-profile` block:

```go
	if f.Changed("bearer-token") {
		cfg.Crawl.BearerToken, _ = f.GetString("bearer-token")
	}
```

  In `cmd/gocrawl/interactive.go`: in the `var (...)` block, after `cookie = cfg.Crawl.Cookie`
  add `bearerToken = cfg.Crawl.BearerToken` (gofmt aligns it). In the auth group, after the
  Cookie input, add:

```go
			huh.NewInput().
				Title("Bearer token").
				Description("Sent as \"Authorization: Bearer <token>\", only to the crawled host. Can't be combined with Basic Auth. Raw mode only. Leave blank otherwise.").
				EchoMode(huh.EchoModePassword).
				Value(&bearerToken),
```

  After `cfg.Crawl.Cookie = strings.TrimSpace(cookie)` add:

```go
	cfg.Crawl.BearerToken = strings.TrimSpace(bearerToken)
```

  Run `gofmt -w cmd/gocrawl`.

- [ ] **Step 4: Run.** `go test ./cmd/gocrawl/` — expected PASS. Manual check of the
  interactive field: `go run ./cmd/gocrawl` in a terminal, walk to the auth group, confirm
  the "Bearer token" field shows masked input, Ctrl-C out. `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add cmd/gocrawl/crawl.go cmd/gocrawl/interactive.go cmd/gocrawl/crawl_test.go
git commit -F - <<'MSG'
feat(cli): add --bearer-token and a bearer-token field to the interactive menu

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-SET-4: Header profile list / read / save / delete in config

**Files:**
- Create: `internal/config/profiles.go`
- Modify: `internal/config/config.go` (`HeaderProfilePath` error)
- Test: `internal/config/profiles_test.go` (new)

**Interfaces:**
- Consumes: `HeaderProfileDir()`, `HeaderProfilePath(name)`, `headerProfileName`,
  `crawler.ParseHeaderLines`, `atomicfile.Write(path, perm, write)`.
- Produces (contract): `ListHeaderProfiles() ([]string, error)`,
  `ReadHeaderProfile(name string) (string, error)`, `SaveHeaderProfile(name, body string) error`,
  `DeleteHeaderProfile(name string) error`. Gap filled: `var ErrInvalidHeaderProfile error`
  (bad name or body; the API maps it to 400) and missing profiles wrap `fs.ErrNotExist` (404).

- [ ] **Step 1: Write the failing tests.** Create `internal/config/profiles_test.go`:

```go
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestHeaderProfilesSaveListReadDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	names, err := ListHeaderProfiles()
	if err != nil {
		t.Fatalf("ListHeaderProfiles on a fresh home: %v", err)
	}
	if names == nil || len(names) != 0 {
		t.Fatalf("names = %#v, want an empty, non-nil list", names)
	}

	if err := SaveHeaderProfile("uk", "# UK store\r\nSignature: abc\r\nSignature-Agent: https://shopify.com"); err != nil {
		t.Fatalf("SaveHeaderProfile(uk): %v", err)
	}
	if err := SaveHeaderProfile("de", "X-Token: 1\n"); err != nil {
		t.Fatalf("SaveHeaderProfile(de): %v", err)
	}

	names, err = ListHeaderProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"de", "uk"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}

	body, err := ReadHeaderProfile("uk")
	if err != nil {
		t.Fatal(err)
	}
	if want := "# UK store\nSignature: abc\nSignature-Agent: https://shopify.com\n"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}

	if runtime.GOOS != "windows" {
		path, _ := HeaderProfilePath("uk")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("profile mode = %o, want 600", perm)
		}
		dirInfo, err := os.Stat(HeaderProfileDir())
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Errorf("profile dir mode = %o, want 700", perm)
		}
	}

	if err := DeleteHeaderProfile("uk"); err != nil {
		t.Fatal(err)
	}
	names, _ = ListHeaderProfiles()
	if want := []string{"de"}; !reflect.DeepEqual(names, want) {
		t.Errorf("after delete names = %v, want %v", names, want)
	}
	if _, err := ReadHeaderProfile("uk"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadHeaderProfile(deleted) err = %v, want fs.ErrNotExist", err)
	}
	if err := DeleteHeaderProfile("uk"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("DeleteHeaderProfile(deleted) err = %v, want fs.ErrNotExist", err)
	}
}

func TestSaveHeaderProfileValidates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cases := []struct{ label, name, body string }{
		{"traversal", "../evil", "X-Token: 1"},
		{"hidden name", ".hidden", "X-Token: 1"},
		{"no colon", "uk", "just some text"},
		{"reserved header", "uk", "Cookie: a=b"},
		{"no headers", "uk", "# only a comment\n\n"},
		{"control character", "uk", "X-Token: a\x00b"},
	}
	for _, tc := range cases {
		if err := SaveHeaderProfile(tc.name, tc.body); !errors.Is(err, ErrInvalidHeaderProfile) {
			t.Errorf("%s: err = %v, want ErrInvalidHeaderProfile", tc.label, err)
		}
	}
	if names, _ := ListHeaderProfiles(); len(names) != 0 {
		t.Errorf("invalid saves wrote profiles: %v", names)
	}
}

func TestHeaderProfileFuncsRejectBadNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := ReadHeaderProfile("../x"); !errors.Is(err, ErrInvalidHeaderProfile) {
		t.Errorf("ReadHeaderProfile(../x) err = %v, want ErrInvalidHeaderProfile", err)
	}
	if err := DeleteHeaderProfile("a/b"); !errors.Is(err, ErrInvalidHeaderProfile) {
		t.Errorf("DeleteHeaderProfile(a/b) err = %v, want ErrInvalidHeaderProfile", err)
	}
}

func TestListHeaderProfilesSkipsOtherEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".gocrawl", "headers")
	writeFile(t, filepath.Join(dir, "uk.headers"), "X-A: 1\n")
	writeFile(t, filepath.Join(dir, "notes.txt"), "x")
	writeFile(t, filepath.Join(dir, ".hidden.headers"), "X-A: 1\n")
	if err := os.MkdirAll(filepath.Join(dir, "sub.headers"), 0o700); err != nil {
		t.Fatal(err)
	}
	names, err := ListHeaderProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"uk"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

// A profile saved through the API is one a crawl can use by name.
func TestSavedHeaderProfileFeedsToOptions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHeaderProfile("uk", "Signature: abc\n"); err != nil {
		t.Fatal(err)
	}
	c := Default()
	c.Crawl.HeaderProfile = "uk"
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if got := o.Headers.Get("Signature"); got != "abc" {
		t.Errorf("Signature = %q, want abc", got)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/config/ -run 'HeaderProfile'` — expected FAIL
  at build: `undefined: ListHeaderProfiles`, `undefined: ErrInvalidHeaderProfile`, …

- [ ] **Step 3: Implement.** Create `internal/config/profiles.go`:

```go
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/atomicfile"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// ErrInvalidHeaderProfile marks a header profile name or body that can't be used. The web
// API maps it to 400; a profile that doesn't exist wraps fs.ErrNotExist instead.
var ErrInvalidHeaderProfile = errors.New("invalid header profile")

const headerProfileExt = ".headers"

// ListHeaderProfiles returns the names of the profiles in HeaderProfileDir, sorted. A missing
// directory is not an error: no profile has been saved yet.
func ListHeaderProfiles() ([]string, error) {
	names := []string{}
	entries, err := os.ReadDir(HeaderProfileDir())
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name, ok := strings.CutSuffix(e.Name(), headerProfileExt)
		if !ok || !headerProfileName.MatchString(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// ReadHeaderProfile returns a profile's file content.
func ReadHeaderProfile(name string) (string, error) {
	path, err := HeaderProfilePath(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("header profile %q: %w", name, fs.ErrNotExist)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveHeaderProfile validates body with crawler.ParseHeaderLines and writes it atomically to
// the profile's file with mode 0600, in a 0700 directory: profiles hold access secrets.
// Windows line endings are normalized and a final newline is added.
func SaveHeaderProfile(name, body string) error {
	path, err := HeaderProfilePath(name)
	if err != nil {
		return err
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	h, err := crawler.ParseHeaderLines(strings.Split(body, "\n"))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidHeaderProfile, err)
	}
	if len(h) == 0 {
		return fmt.Errorf(`%w: no "Name: value" lines`, ErrInvalidHeaderProfile)
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.Write(path, 0o600, func(w io.Writer) error {
		_, err := io.WriteString(w, body)
		return err
	})
}

// DeleteHeaderProfile removes a profile's file.
func DeleteHeaderProfile(name string) error {
	path, err := HeaderProfilePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("header profile %q: %w", name, fs.ErrNotExist)
		}
		return err
	}
	return nil
}
```

  In `internal/config/config.go`, change the error in `HeaderProfilePath` to:

```go
		return "", fmt.Errorf("%w: name %q (letters, digits, '.', '_' and '-' only, not starting with '.')", ErrInvalidHeaderProfile, name)
```

- [ ] **Step 4: Run.** `go test ./internal/config/` — expected PASS (the existing
  `TestHeaderProfilePathRejectsTraversal` still passes: it only checks for an error).
  `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/config/profiles.go internal/config/profiles_test.go internal/config/config.go
git commit -F - <<'MSG'
feat(config): list, read, save and delete header profiles

Profiles are validated with crawler.ParseHeaderLines and written atomically with mode 0600.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-SET-5: Reject bearer-token conflicts with 400 on `POST /api/crawls`

**Files:**
- Modify: `internal/webserver/crawls.go` (`handleStartCrawl`)
- Test: `internal/webserver/crawl_auth_test.go` (new)

**Interfaces:**
- Consumes: `crawlrequest.Params.ToConfig()`, `config.Config.ToOptions()`,
  `config.SaveHeaderProfile` (test only).
- Produces: `POST /api/crawls` returns 400 (no job) for any config `ToOptions` rejects,
  including bearer token + Basic Auth / seed userinfo / custom Authorization header.

- [ ] **Step 1: Write the failing tests.** Create `internal/webserver/crawl_auth_test.go`:

```go
package webserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestStartCrawlRejectsBearerTokenConflicts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := config.SaveHeaderProfile("auth", "Authorization: Basic eDp5\n"); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t)
	const token = "tok-secret"
	cases := map[string]map[string]any{
		"basic_auth":     {"url": "http://127.0.0.1:1/", "bearer_token": token, "basic_auth": "alice:pw"},
		"seed userinfo":  {"url": "http://alice:pw@127.0.0.1:1/", "bearer_token": token},
		"inline header":  {"url": "http://127.0.0.1:1/", "bearer_token": token, "headers": []string{"Authorization: Token abc"}},
		"header profile": {"url": "http://127.0.0.1:1/", "bearer_token": token, "header_profile": "auth"},
	}
	for name, body := range cases {
		rec, out := doJSON(t, srv, http.MethodPost, "/api/crawls", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", name, rec.Code, rec.Body)
			continue
		}
		if msg, _ := out["error"].(string); !strings.Contains(msg, "bearer_token") {
			t.Errorf("%s: error %q doesn't name bearer_token", name, msg)
		}
		if strings.Contains(rec.Body.String(), token) {
			t.Errorf("%s: response leaks the token: %s", name, rec.Body)
		}
	}
	if n := len(srv.crawls.list()); n != 0 {
		t.Errorf("%d crawl(s) started despite the conflicts", n)
	}
}

// End to end: the token reaches the seed as a Bearer Authorization header.
func TestStartCrawlSendsBearerTokenToSeed(t *testing.T) {
	var mu sync.Mutex
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			mu.Lock()
			got = r.Header.Get("Authorization")
			mu.Unlock()
		}
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>ok</body></html>`)
	}))
	t.Cleanup(target.Close)

	srv := newTestServer(t)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "bearer_token": "tok-123"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %s)", rec.Code, rec.Body)
	}
	job := pollUntilFinished(t, srv, started["id"].(string))
	if job["status"] != "done" {
		t.Fatalf("job = %v, want done", job)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer tok-123")
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/webserver/ -run 'BearerToken'` — expected FAIL:
  `basic_auth: status = 202, want 400` (and the other three cases), plus
  `4 crawl(s) started despite the conflicts`. (`TestStartCrawlSendsBearerTokenToSeed` already
  passes; it pins the end-to-end path.)

- [ ] **Step 3: Implement.** In `internal/webserver/crawls.go`, `handleStartCrawl`, directly
  after the block

```go
	cfg, seed, err := req.ToConfig()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
```

  insert:

```go
	// Compile the options now, so a configuration the crawl would reject (a bearer token next
	// to Basic Auth, a bad regex, a missing header profile) is a 400 here rather than a job
	// that fails a moment later.
	if _, err := cfg.ToOptions(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
```

- [ ] **Step 4: Run.** `go test ./internal/webserver/` — expected PASS. `make check` — PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/webserver/crawls.go internal/webserver/crawl_auth_test.go
git commit -F - <<'MSG'
feat(webserver): reject invalid crawl options, such as bearer-token conflicts, with 400

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-SET-6: GUI settings (`GET`/`PUT /api/settings`, `gui.json`)

**Files:**
- Modify: `internal/webserver/settings.go` (extend the wave-0 file: keep its
  `WithGUISettingsPath`, fill in the empty `settingsRoutes`)
- Modify: `cmd/gocrawl/serve.go`
- Test: `internal/webserver/settings_test.go` (new)

**Interfaces:**
- Consumes: `Server.guiSettingsPath`, `WithGUISettingsPath` (both declared by wave 0 in
  `settings.go`; do not redeclare), `Server.store.Root()`, `requireJSON`, `writeJSON`,
  `writeError`, `atomicfile.Write`, `crawlrequest.Params.ToConfig`.
- Produces (contract): `type GUISettings struct { Theme string; CrawlDefaults crawlrequest.Params }`,
  routes `GET`/`PUT /api/settings`.
  Gap filled: `func DefaultGUISettingsPath() string` (`~/.gocrawl/gui.json`), response field
  `store_dir`, `PUT` 503 when the server has no settings path, empty theme means `dark`.

- [ ] **Step 1: Write the failing tests.** Create `internal/webserver/settings_test.go`:

```go
package webserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

func newSettingsServer(t *testing.T) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gui.json")
	return New(store.New(t.TempDir()), WithGUISettingsPath(path)), path
}

// settingsRaw sends a request with full control over Content-Type and Origin, for the guard
// and content-type checks that doJSON can't express.
func settingsRaw(t *testing.T, srv *Server, method, path, contentType, origin, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "localhost:8080"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestGetSettingsDefaultsWhenFileMissing(t *testing.T) {
	srv, _ := newSettingsServer(t)
	rec, out := doJSON(t, srv, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body)
	}
	if out["theme"] != "dark" {
		t.Errorf("theme = %v, want dark", out["theme"])
	}
	if _, ok := out["crawl_defaults"].(map[string]any); !ok {
		t.Errorf("crawl_defaults = %v, want an object", out["crawl_defaults"])
	}
	if out["store_dir"] != srv.store.Root() {
		t.Errorf("store_dir = %v, want %q", out["store_dir"], srv.store.Root())
	}
}

func TestPutSettingsPersistsAndStripsSecrets(t *testing.T) {
	srv, path := newSettingsServer(t)
	secrets := []string{"tok-secret", "basic-secret", "cookie-secret", "header-secret", "proxy-secret", "url-secret"}
	body := map[string]any{
		"theme": "light",
		"crawl_defaults": map[string]any{
			"url":            "https://alice:url-secret@example.com/",
			"max_pages":      50,
			"timeout":        "9s",
			"header_profile": "uk",
			"bearer_token":   "tok-secret",
			"basic_auth":     "alice:basic-secret",
			"cookie":         "sid=cookie-secret",
			"headers":        []string{"X-Key: header-secret"},
			"proxy":          "http://pu:proxy-secret@proxy.example:8080",
			"proxies":        []string{"socks5://pu:proxy-secret@proxy2.example:1080"},
		},
	}
	rec, out := doJSON(t, srv, http.MethodPut, "/api/settings", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body)
	}
	if out["theme"] != "light" {
		t.Errorf("theme = %v, want light", out["theme"])
	}
	defaults, _ := out["crawl_defaults"].(map[string]any)
	if defaults["max_pages"] != float64(50) || defaults["timeout"] != "9s" || defaults["header_profile"] != "uk" {
		t.Errorf("crawl_defaults lost non-secret values: %v", defaults)
	}
	if defaults["url"] != "https://example.com/" {
		t.Errorf("url = %v, want the userinfo stripped", defaults["url"])
	}
	if defaults["proxy"] != "http://proxy.example:8080" {
		t.Errorf("proxy = %v, want the userinfo stripped", defaults["proxy"])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range secrets {
		if strings.Contains(string(data), s) {
			t.Errorf("gui.json holds %q:\n%s", s, data)
		}
		if strings.Contains(rec.Body.String(), s) {
			t.Errorf("PUT response echoes %q", s)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("gui.json mode = %o, want 600", perm)
		}
	}

	// A new server reading the same file (the next launch) sees the saved settings.
	again := New(store.New(t.TempDir()), WithGUISettingsPath(path))
	_, reloaded := doJSON(t, again, http.MethodGet, "/api/settings", nil)
	if reloaded["theme"] != "light" {
		t.Errorf("reloaded theme = %v, want light", reloaded["theme"])
	}
	if d, _ := reloaded["crawl_defaults"].(map[string]any); d["max_pages"] != float64(50) {
		t.Errorf("reloaded crawl_defaults = %v, want max_pages 50", d)
	}
}

func TestPutSettingsValidates(t *testing.T) {
	srv, path := newSettingsServer(t)
	cases := map[string]map[string]any{
		"unknown theme":    {"theme": "neon", "crawl_defaults": map[string]any{}},
		"bad timeout":      {"theme": "dark", "crawl_defaults": map[string]any{"timeout": "soon"}},
		"bad max_duration": {"theme": "dark", "crawl_defaults": map[string]any{"max_duration": "later"}},
	}
	for name, body := range cases {
		if rec, _ := doJSON(t, srv, http.MethodPut, "/api/settings", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("gui.json was written despite invalid input (stat err %v)", err)
	}
}

func TestPutSettingsEmptyThemeMeansDark(t *testing.T) {
	srv, _ := newSettingsServer(t)
	rec, out := doJSON(t, srv, http.MethodPut, "/api/settings", map[string]any{"crawl_defaults": map[string]any{}})
	if rec.Code != http.StatusOK || out["theme"] != "dark" {
		t.Errorf("status %d, theme %v; want 200 and dark", rec.Code, out["theme"])
	}
}

func TestPutSettingsGuards(t *testing.T) {
	srv, path := newSettingsServer(t)
	if rec := settingsRaw(t, srv, http.MethodPut, "/api/settings", "text/plain", "", `{"theme":"light"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: status = %d, want 415", rec.Code)
	}
	if rec := settingsRaw(t, srv, http.MethodPut, "/api/settings", "application/json", "http://evil.example", `{"theme":"light"}`); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status = %d, want 403", rec.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("gui.json was written by a rejected request")
	}
}

// A hand-edited gui.json never feeds a secret back to the UI.
func TestGetSettingsStripsHandEditedSecrets(t *testing.T) {
	srv, path := newSettingsServer(t)
	if err := os.WriteFile(path, []byte(`{"theme":"system","crawl_defaults":{"bearer_token":"tok-secret","cookie":"sid=1","max_pages":7}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, out := doJSON(t, srv, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusOK || out["theme"] != "system" {
		t.Fatalf("status %d, theme %v", rec.Code, out["theme"])
	}
	if strings.Contains(rec.Body.String(), "tok-secret") || strings.Contains(rec.Body.String(), "sid=1") {
		t.Errorf("GET returned a secret: %s", rec.Body)
	}
	if d, _ := out["crawl_defaults"].(map[string]any); d["max_pages"] != float64(7) {
		t.Errorf("crawl_defaults = %v, want max_pages 7", d)
	}
}

func TestGetSettingsReportsCorruptFile(t *testing.T) {
	srv, path := newSettingsServer(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, out := doJSON(t, srv, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, path) {
		t.Errorf("error %q doesn't name the file", msg)
	}
}

func TestPutSettingsWithoutPathIsUnavailable(t *testing.T) {
	srv := newTestServer(t)
	if rec, _ := doJSON(t, srv, http.MethodPut, "/api/settings", map[string]any{"theme": "light"}); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/webserver/ -run Settings` — expected FAIL:
  wave 0's `settingsRoutes` registers nothing, so `GET /api/settings` falls through to the
  frontend handler (HTML, not JSON) and `doJSON` fails to decode the body;
  `TestPutSettingsWithoutPathIsUnavailable` gets the frontend handler's status instead of 503.

- [ ] **Step 3: Implement.** Replace the wave-0 `internal/webserver/settings.go` with the file
  below. It keeps wave 0's `WithGUISettingsPath` exactly as it was (same name, signature and
  behavior; only its doc comment grows) and adds everything else:

```go
package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/atomicfile"
	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
)

// GUISettings is what the web UI keeps between launches, in gui.json. The UI is served from a
// different port on every desktop launch, so nothing may live in localStorage. It never holds
// a secret: see stripSecrets.
type GUISettings struct {
	Theme         string              `json:"theme"` // "dark" (default) | "light" | "system"
	CrawlDefaults crawlrequest.Params `json:"crawl_defaults"`
}

// settingsView is the GET/PUT /api/settings response: the stored settings plus read-only facts
// about this server. PUT ignores StoreDir.
type settingsView struct {
	GUISettings
	StoreDir string `json:"store_dir"`
}

const (
	defaultTheme    = "dark"
	maxSettingsBody = 1 << 20
)

var validThemes = map[string]bool{"dark": true, "light": true, "system": true}

// WithGUISettingsPath sets the file GET and PUT /api/settings read and write (gocrawl serve
// passes ~/.gocrawl/gui.json, see DefaultGUISettingsPath). An empty path, the default, means
// settings are not stored: GET returns the defaults and PUT answers 503. Declared by wave 0.
func WithGUISettingsPath(path string) Option {
	return func(s *Server) { s.guiSettingsPath = path }
}

// DefaultGUISettingsPath is ~/.gocrawl/gui.json, next to the crawl store and header profiles.
func DefaultGUISettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".gocrawl", "gui.json")
	}
	return filepath.Join(home, ".gocrawl", "gui.json")
}

func (s *Server) settingsRoutes() {
	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	settings, err := loadGUISettings(s.guiSettingsPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.settingsView(settings))
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	if s.guiSettingsPath == "" {
		writeError(w, http.StatusServiceUnavailable, errors.New("this server was started without a settings file, so settings can't be saved"))
		return
	}
	var in GUISettings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSettingsBody)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	if in.Theme == "" {
		in.Theme = defaultTheme
	}
	if !validThemes[in.Theme] {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown theme %q (want dark, light or system)", in.Theme))
		return
	}
	in.CrawlDefaults = stripSecrets(in.CrawlDefaults)
	if err := validateCrawlDefaults(in.CrawlDefaults); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("crawl_defaults: %w", err))
		return
	}
	if err := saveGUISettings(s.guiSettingsPath, in); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("saving settings: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, s.settingsView(in))
}

func (s *Server) settingsView(g GUISettings) settingsView {
	return settingsView{GUISettings: g, StoreDir: s.store.Root()}
}

func defaultGUISettings() GUISettings {
	return GUISettings{Theme: defaultTheme}
}

// loadGUISettings reads path; a missing file (or no path) gives the defaults. Secrets are
// stripped on load too, so a hand-edited file never feeds one back to the UI.
func loadGUISettings(path string) (GUISettings, error) {
	if path == "" {
		return defaultGUISettings(), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultGUISettings(), nil
	}
	if err != nil {
		return GUISettings{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var g GUISettings
	if err := json.Unmarshal(data, &g); err != nil {
		return GUISettings{}, fmt.Errorf("reading %s: %w (save the settings again to replace the file)", path, err)
	}
	if !validThemes[g.Theme] {
		g.Theme = defaultTheme
	}
	g.CrawlDefaults = stripSecrets(g.CrawlDefaults)
	return g, nil
}

func saveGUISettings(path string, g GUISettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.Write(path, 0o600, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(g)
	})
}

// stripSecrets drops every credential a Params can carry: the bearer token, Basic Auth, the
// cookie, inline headers (treated as credentials everywhere else too; header profiles are the
// way to keep headers), and userinfo embedded in the seed or proxy URLs.
func stripSecrets(p crawlrequest.Params) crawlrequest.Params {
	p.BearerToken = ""
	p.BasicAuth = ""
	p.Cookie = ""
	p.Headers = nil
	p.URL = stripUserinfo(p.URL)
	p.Proxy = stripUserinfo(p.Proxy)
	if p.Proxies != nil {
		proxies := make([]string, 0, len(p.Proxies))
		for _, px := range p.Proxies {
			if clean := stripUserinfo(px); clean != "" {
				proxies = append(proxies, clean)
			}
		}
		p.Proxies = proxies
	}
	return p
}

// stripUserinfo removes user:pass@ from a URL, with or without a scheme. A value that has an
// '@' but doesn't parse is dropped rather than risk storing a credential.
func stripUserinfo(raw string) string {
	if !strings.Contains(raw, "@") {
		return raw
	}
	hasScheme := strings.Contains(raw, "://")
	candidate := raw
	if !hasScheme {
		candidate = "http://" + raw
	}
	u, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	if u.User == nil {
		return raw
	}
	u.User = nil
	out := u.String()
	if !hasScheme {
		out = strings.TrimPrefix(out, "http://")
	}
	return out
}

// validateCrawlDefaults runs the same mapping a crawl start would, so a default the form can't
// use (an unparseable timeout, say) is refused when saved, not when the next crawl starts.
func validateCrawlDefaults(p crawlrequest.Params) error {
	if strings.TrimSpace(p.URL) == "" {
		p.URL = "https://example.invalid"
	}
	_, _, err := p.ToConfig()
	return err
}
```

  In `cmd/gocrawl/serve.go`, `runServe`, replace `var opts []webserver.Option` with:

```go
	opts := []webserver.Option{webserver.WithGUISettingsPath(webserver.DefaultGUISettingsPath())}
```

- [ ] **Step 4: Run.** `go test ./internal/webserver/ ./cmd/gocrawl/` — expected PASS. Manual:
  `go build -o /tmp/gocrawl-dev ./cmd/gocrawl && HOME="$(mktemp -d)" sh -c '/tmp/gocrawl-dev serve & sleep 1; curl -s localhost:8080/api/settings; curl -s -X PUT -H "Content-Type: application/json" -d "{\"theme\":\"light\",\"crawl_defaults\":{\"bearer_token\":\"x\",\"max_pages\":9}}" localhost:8080/api/settings; cat "$HOME/.gocrawl/gui.json"; kill %1'`
  — expected: GET shows `"theme":"dark"` and `store_dir`; `gui.json` has `"theme": "light"`,
  `"max_pages": 9`, no `bearer_token` value. `make check` — PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/webserver/settings.go internal/webserver/settings_test.go cmd/gocrawl/serve.go
git commit -F - <<'MSG'
feat(webserver): GET/PUT /api/settings stored in ~/.gocrawl/gui.json without secrets

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-SET-7: Header-profile routes

**Files:**
- Modify: `internal/webserver/profiles.go` (replace the wave-0 stub)
- Test: `internal/webserver/profiles_test.go` (new)

**Interfaces:**
- Consumes: `config.ListHeaderProfiles`, `ReadHeaderProfile`, `SaveHeaderProfile`,
  `DeleteHeaderProfile`, `config.ErrInvalidHeaderProfile`, `requireJSON`.
- Produces (contract): `GET /api/header-profiles` -> `{profiles: string[]}`;
  `GET /api/header-profiles/{name}` -> `{name, body}`; `PUT` JSON `{body}` -> 204;
  `DELETE` -> 204. Errors: 400 bad name/body, 404 unknown, 415 non-JSON PUT.

- [ ] **Step 1: Write the failing tests.** Create `internal/webserver/profiles_test.go`:

```go
package webserver

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestHeaderProfileRoutes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := newTestServer(t)

	rec, out := doJSON(t, srv, http.MethodGet, "/api/header-profiles", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status = %d (body %s)", rec.Code, rec.Body)
	}
	if list, ok := out["profiles"].([]any); !ok || len(list) != 0 {
		t.Fatalf("profiles = %#v, want an empty list", out["profiles"])
	}

	if rec, _ := doJSON(t, srv, http.MethodPut, "/api/header-profiles/uk", map[string]string{"body": "Signature: abc\n"}); rec.Code != http.StatusNoContent {
		t.Fatalf("put: status = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	_, out = doJSON(t, srv, http.MethodGet, "/api/header-profiles", nil)
	if got := out["profiles"]; !reflect.DeepEqual(got, []any{"uk"}) {
		t.Errorf("profiles = %v, want [uk]", got)
	}
	rec, out = doJSON(t, srv, http.MethodGet, "/api/header-profiles/uk", nil)
	if rec.Code != http.StatusOK || out["name"] != "uk" || out["body"] != "Signature: abc\n" {
		t.Errorf("get: status %d, body %v", rec.Code, out)
	}

	rec, out = doJSON(t, srv, http.MethodPut, "/api/header-profiles/uk", map[string]string{"body": "not a header"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid body: status = %d, want 400", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "Name: value") {
		t.Errorf("invalid body: error %q doesn't explain the format", msg)
	}
	if _, out = doJSON(t, srv, http.MethodGet, "/api/header-profiles/uk", nil); out["body"] != "Signature: abc\n" {
		t.Errorf("a rejected save changed the profile: %v", out["body"])
	}

	if rec, _ := doJSON(t, srv, http.MethodDelete, "/api/header-profiles/uk", nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodDelete, "/api/header-profiles/uk", nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete again: status = %d, want 404", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodGet, "/api/header-profiles/uk", nil); rec.Code != http.StatusNotFound {
		t.Errorf("get deleted: status = %d, want 404", rec.Code)
	}
}

func TestHeaderProfileRoutesRejectBadNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := newTestServer(t)
	for _, name := range []string{".hidden", "bad%20name"} {
		if rec, _ := doJSON(t, srv, http.MethodPut, "/api/header-profiles/"+name, map[string]string{"body": "X-A: 1"}); rec.Code != http.StatusBadRequest {
			t.Errorf("put %q: status = %d, want 400", name, rec.Code)
		}
		if rec, _ := doJSON(t, srv, http.MethodGet, "/api/header-profiles/"+name, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("get %q: status = %d, want 400", name, rec.Code)
		}
	}
	// An encoded slash must never reach the file system as a path separator.
	rec := settingsRaw(t, srv, http.MethodPut, "/api/header-profiles/a%2Fb", "application/json", "", `{"body":"X-A: 1"}`)
	if rec.Code < 400 {
		t.Errorf("put a%%2Fb: status = %d, want an error", rec.Code)
	}
	if names, _ := config.ListHeaderProfiles(); len(names) != 0 {
		t.Errorf("bad names created profiles: %v", names)
	}
}

func TestHeaderProfileRoutesGuards(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := config.SaveHeaderProfile("uk", "X-A: 1\n"); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t)
	if rec := settingsRaw(t, srv, http.MethodPut, "/api/header-profiles/uk", "text/plain", "", `{"body":"X-B: 2"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain put: status = %d, want 415", rec.Code)
	}
	if rec := settingsRaw(t, srv, http.MethodDelete, "/api/header-profiles/uk", "", "http://evil.example", ""); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin delete: status = %d, want 403", rec.Code)
	}
	if body, err := config.ReadHeaderProfile("uk"); err != nil || body != "X-A: 1\n" {
		t.Errorf("profile changed by a rejected request: %q, %v", body, err)
	}
}
```

  (`settingsRaw` comes from `settings_test.go`, W1-SET-6.)

- [ ] **Step 2: Run them.** `go test ./internal/webserver/ -run HeaderProfile` — expected
  FAIL: `decode response body "<!doctype html>…"` (the routes don't exist yet, so the frontend
  handler answers).

- [ ] **Step 3: Implement.** Replace `internal/webserver/profiles.go` with:

```go
package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

// headerProfileView is GET /api/header-profiles/{name}. The body holds secrets; it is only
// served to the loopback, same-origin UI the guard admits, so the editor can show it.
type headerProfileView struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

const maxProfileBody = 64 << 10

func (s *Server) profileRoutes() {
	s.mux.HandleFunc("GET /api/header-profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/header-profiles/{name}", s.handleGetProfile)
	s.mux.HandleFunc("PUT /api/header-profiles/{name}", s.handlePutProfile)
	s.mux.HandleFunc("DELETE /api/header-profiles/{name}", s.handleDeleteProfile)
}

func (s *Server) handleListProfiles(w http.ResponseWriter, _ *http.Request) {
	names, err := config.ListHeaderProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": names})
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	body, err := config.ReadHeaderProfile(name)
	if err != nil {
		writeError(w, profileErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, headerProfileView{Name: name, Body: body})
}

func (s *Server) handlePutProfile(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxProfileBody)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	if err := config.SaveHeaderProfile(r.PathValue("name"), in.Body); err != nil {
		writeError(w, profileErrorStatus(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	if err := config.DeleteHeaderProfile(r.PathValue("name")); err != nil {
		writeError(w, profileErrorStatus(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func profileErrorStatus(err error) int {
	switch {
	case errors.Is(err, config.ErrInvalidHeaderProfile):
		return http.StatusBadRequest
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
```

- [ ] **Step 4: Run.** `go test ./internal/webserver/` — expected PASS. `make check` — PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/webserver/profiles.go internal/webserver/profiles_test.go
git commit -F - <<'MSG'
feat(webserver): list, read, save and delete header profiles over the API

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-SET-8: Docs and changelog for the settings backend

**Files:**
- Modify: `docs/configuration.md`, `docs/mcp.md`, `docs/web.md`, `CHANGELOG.md`
- Generated: `docs/README.md` and frontmatter `depends_on` / `sources` via `make docs`

**Interfaces:** none (docs only).

- [ ] **Step 1: Check what the docs-drift tests require first.** `go test ./internal/repocheck/ ./internal/report/`
  — expected PASS before the edit (a baseline; these tests fail if a link or frontmatter
  breaks below).

- [ ] **Step 2: Edit `docs/configuration.md`.**
  1. Frontmatter: `updated: 2026-10-13`.
  2. Options table, after the `crawl.header_profile` row, add:

```markdown
| `crawl.bearer_token` | `--bearer-token` | string | *(none)* | Token sent as `Authorization: Bearer <token>`, scoped like `basic_auth`. Can't be combined with `basic_auth` or a custom `Authorization` header. See [Bearer tokens](#bearer-tokens). |
```

  3. Environment table, after the `crawl.header_profile` row, add
     `| \`crawl.bearer_token\` | \`GOCRAWL_CRAWL_BEARER_TOKEN\` |`.
  4. In "Custom request headers", after the paragraph starting **Profiles are how agents and
     the web UI use secrets.**, add: "The web UI's Settings view lists, creates, edits and
     deletes profiles; it validates each one with the same parser a crawl uses and writes the
     file with mode `0600`."
  5. Before `## Selecting analyzers`, add:

~~~markdown
### Bearer tokens

Staging sites and APIs behind an OAuth-style gateway often expect
`Authorization: Bearer <token>`. Pass the token with `--bearer-token`, `crawl.bearer_token`, or
`GOCRAWL_CRAWL_BEARER_TOKEN` (the environment keeps it out of shell history and `ps`):

```sh
GOCRAWL_CRAWL_BEARER_TOKEN="$STAGING_TOKEN" gocrawl crawl https://staging.example.com
```

The token travels the custom-headers path, so it gets exactly the same scoping: only the seed
host (plus subdomains under `--subdomains`), never over an https → http downgrade, re-checked on
every redirect hop, and not supported with `--render headless`. A pasted `Bearer ` prefix is
accepted. Combining it with `--basic-auth` or a custom `Authorization` header (inline, from a
header file, or from a profile) is an error, because all of them set the same header; the error
never repeats the token. The MCP `crawl` tool and the web API take it as `bearer_token`; the web
UI never saves it.
~~~

- [ ] **Step 3: Edit `docs/mcp.md`.** Frontmatter `updated: 2026-10-13`. In the `crawl` input
  table, after the `exclude` row add:

```markdown
| `follow_nofollow` | bool | `false` | Also follow links marked `rel="nofollow"`. |
| `strip_query` | bool | `false` | Ignore query strings (treat `?a=1` and `?a=2` as one URL); skips the query-dependent `utm` / `tracking` analysis. |
| `adaptive_delay` | bool | `true` | Slow the crawl automatically on HTTP 429/503, honoring `Retry-After`. |
| `timeout` | string | `15s` | Per-request timeout as a Go duration string. |
```

  and after the `cookie` row add:

```markdown
| `bearer_token` | string | *(none)* | Sent as `Authorization: Bearer <token>`, only to the seed host; raw render mode only. Can't be combined with `basic_auth` or a custom `Authorization` header (see [Bearer tokens](configuration.md#bearer-tokens)). |
```

- [ ] **Step 4: Edit `docs/web.md`.** Frontmatter `updated: 2026-10-13`. In the API table,
  after the export row add:

```markdown
| `GET /api/settings` | The UI's saved settings: `theme` (`dark`, `light` or `system`), `crawl_defaults` (a crawl request body that pre-fills the New crawl form), and the read-only `store_dir`. |
| `PUT /api/settings` | Save `theme` and `crawl_defaults` (JSON). Written atomically to `~/.gocrawl/gui.json` with mode `0600`. `bearer_token`, `basic_auth`, `cookie`, inline `headers`, and user:password in the URL or proxy URLs are stripped before saving. `400` for an unknown theme or an unusable default. |
| `GET /api/header-profiles` | `{"profiles": [...]}`: the names in `~/.gocrawl/headers/`. |
| `GET /api/header-profiles/{name}` | `{"name", "body"}` of one profile. `404` when it doesn't exist, `400` for an invalid name. |
| `PUT /api/header-profiles/{name}` | Create or replace a profile from JSON `{"body": "Name: value\n…"}`, validated like `--header-file`. `204`; `400` with the parser's message when invalid. |
| `DELETE /api/header-profiles/{name}` | Delete a profile. `204`, or `404`. |
```

  In "Notes", replace the bullet starting "The web API builds its config from" with: "The web
  API builds each crawl's config from `config.Default()` plus the request body — like the MCP
  tool, it does not read a YAML config file or `GOCRAWL_*` environment variables. The UI's own
  defaults live in `~/.gocrawl/gui.json` and are applied by the form, not the server. A start
  request whose options can't compile (a bad regex, a bearer token next to Basic Auth, a
  missing header profile) is a `400`."

- [ ] **Step 5: Edit `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`, add one line:

```markdown
- Crawl requests (web API and MCP `crawl` tool) accept `strip_query`, `adaptive_delay`, `follow_nofollow`, `timeout` and `bearer_token`; the CLI gains `--bearer-token` (`crawl.bearer_token`), and the web API gains `GET`/`PUT /api/settings` (stored in `~/.gocrawl/gui.json`, never with secrets) and `/api/header-profiles` to manage header profiles.
```

- [ ] **Step 6: Regenerate and run.** `make docs`, then `make check` — expected PASS.

- [ ] **Step 7: Commit.**

```sh
git add docs/configuration.md docs/mcp.md docs/web.md docs/README.md CHANGELOG.md
git commit -F - <<'MSG'
docs: document bearer tokens, new crawl parameters, settings and header-profile API

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

### Track: login backend

#### Task W1-LOGIN-1: Browser lookup and the chromedp login window (`internal/render`)

**Files:**
- Create: `internal/render/login.go`
- Test: `internal/render/login_test.go` (new)

**Interfaces:**
- Consumes: chromedp v0.19.1 (`NewExecAllocator`, `DefaultExecAllocatorOptions`, `ExecPath`,
  `UserDataDir`, `VisibleWindow`, `NewContext`, `Do`, `Navigate`, `Func`, `FromContext`,
  `BrowserEvents`, `ErrNoDisplay`), cdproto `storage.GetCookies`, `target.TargetDestroyed`;
  `quietErrorf` from `render.go`.
- Produces: `var ErrNoBrowser, ErrNoDisplay, ErrLoginWindowClosed error`;
  `func FindBrowser() (string, error)`; `func LoginAvailable() error`;
  `type LoginWindow`; `func OpenLoginWindow(ctx context.Context, rawURL string) (*LoginWindow, error)`;
  `(*LoginWindow).Cookies(ctx) ([]*http.Cookie, error)`, `Done() <-chan struct{}`, `Close() error`
  — which satisfies the webserver's `loginWindow` interface.

Browser detection: chromedp's own lookup (`findExecPath`, what headless render relies on) is
unexported, so `FindBrowser` lists the same locations per OS, minus `headless_shell` (no
window) and plus Microsoft Edge on macOS/Linux, and the login window passes the found path
with `chromedp.ExecPath` so detection and launch always agree.

- [ ] **Step 1: Write the failing tests.** Create `internal/render/login_test.go`:

```go
package render

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
)

func TestBrowserLocationsCoverEveryDesktopOS(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		if len(browserLocations(goos)) == 0 {
			t.Errorf("no browser locations for %s", goos)
		}
	}
	if !slices.Contains(browserLocations("darwin"), "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome") {
		t.Error("darwin locations miss Google Chrome")
	}
	if !slices.Contains(browserLocations("windows"), "msedge") {
		t.Error("windows locations miss Edge")
	}
	if slices.Contains(browserLocations("linux"), "headless_shell") {
		t.Error("linux locations include headless_shell, which can't show a login window")
	}
}

func TestLoginAvailableReportsMissingBrowser(t *testing.T) {
	if _, err := FindBrowser(); err != nil {
		if !errors.Is(err, ErrNoBrowser) {
			t.Fatalf("FindBrowser err = %v, want ErrNoBrowser", err)
		}
		if err := LoginAvailable(); !errors.Is(err, ErrNoBrowser) {
			t.Errorf("LoginAvailable = %v, want ErrNoBrowser", err)
		}
	}
}

// httpCookies keeps the value (the point of a login capture, unlike convertCookies, which
// drops it) and CDP's domain spelling: a leading dot marks a domain cookie.
func TestHTTPCookiesKeepsValuesAndDomainForm(t *testing.T) {
	expires := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	got := httpCookies([]*network.Cookie{
		{Name: "sid", Value: "abc", Domain: "shop.example.com", Path: "/", HTTPOnly: true, Secure: true, Session: true, Expires: -1},
		nil,
		{Name: "pref", Value: "1", Domain: ".example.com", Path: "/", Expires: float64(expires.Unix())},
	})
	if len(got) != 2 {
		t.Fatalf("got %d cookies, want 2", len(got))
	}
	sid, pref := got[0], got[1]
	if sid.Name != "sid" || sid.Value != "abc" || sid.Domain != "shop.example.com" || !sid.HttpOnly || !sid.Secure || !sid.Expires.IsZero() {
		t.Errorf("sid = %+v", sid)
	}
	if pref.Domain != ".example.com" || !pref.Expires.Equal(expires) {
		t.Errorf("pref = %+v, want domain .example.com expiring %v", pref, expires)
	}
}

// Real browser, skipped without one like the headless render tests. It runs the window
// headless (visible=false) so `go test` never pops a window up; visibility only changes the
// allocator flags.
func TestLoginWindowReadsHTTPOnlyCookiesAndRemovesProfile(t *testing.T) {
	if _, err := FindBrowser(); err != nil {
		t.Skip("no Chromium-class browser available")
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s3cret", Path: "/", HttpOnly: true})
		fmt.Fprint(w, `<html><body>signed in</body></html>`)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	w, err := openLoginWindow(ctx, ts.URL, false)
	if err != nil {
		t.Skipf("could not launch the browser: %v", err)
	}
	dir := w.profileDir
	if _, err := os.Stat(dir); err != nil {
		_ = w.Close()
		t.Fatalf("profile dir %s missing while the window is open: %v", dir, err)
	}

	cookies, err := w.Cookies(ctx)
	if err != nil {
		_ = w.Close()
		t.Fatalf("Cookies: %v", err)
	}
	var sid *http.Cookie
	for _, c := range cookies {
		if c.Name == "sid" {
			sid = c
		}
	}
	if sid == nil || sid.Value != "s3cret" || !sid.HttpOnly || sid.Domain != "127.0.0.1" {
		t.Errorf("sid cookie = %+v, want the HttpOnly value s3cret for 127.0.0.1", sid)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-w.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Done not closed after Close")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("profile dir %s still exists after Close (stat err %v)", dir, err)
	}
	if _, err := w.Cookies(ctx); !errors.Is(err, ErrLoginWindowClosed) {
		t.Errorf("Cookies after Close err = %v, want ErrLoginWindowClosed", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/render/ -run 'Browser|Login|HTTPCookies'` —
  expected FAIL at build: `undefined: browserLocations`, `undefined: openLoginWindow`, …

- [ ] **Step 3: Implement.** Create `internal/render/login.go`:

```go
package render

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

var (
	// ErrNoBrowser means no Chromium-class browser is installed where gocrawl looks.
	ErrNoBrowser = errors.New("no Chromium-class browser found (install Google Chrome, Chromium or Microsoft Edge)")
	// ErrNoDisplay means a visible window can't open: no X11 or Wayland display.
	ErrNoDisplay = errors.New("no display for a browser window (DISPLAY and WAYLAND_DISPLAY are both unset)")
	// ErrLoginWindowClosed is returned by Cookies once the window is gone.
	ErrLoginWindowClosed = errors.New("the browser window was closed")
)

// browserLocations lists, per OS, the browser names and paths FindBrowser tries in order:
// chromedp's own list (what headless render launches through), minus headless_shell, which
// has no window, plus Microsoft Edge where chromedp only looks for it on Windows.
func browserLocations(goos string) []string {
	switch goos {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "windows":
		return []string{
			"chrome",
			"chrome.exe",
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Chromium\Application\chrome.exe`),
			"msedge",
			"msedge.exe",
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
	default:
		return []string{
			"chromium",
			"chromium-browser",
			"google-chrome",
			"google-chrome-stable",
			"google-chrome-beta",
			"google-chrome-unstable",
			"/usr/bin/google-chrome",
			"/usr/local/bin/chrome",
			"/snap/bin/chromium",
			"chrome",
			"microsoft-edge",
			"microsoft-edge-stable",
		}
	}
}

// FindBrowser returns the path of the first Chromium-class browser it finds, or ErrNoBrowser.
func FindBrowser() (string, error) {
	for _, p := range browserLocations(runtime.GOOS) {
		if found, err := exec.LookPath(p); err == nil {
			return found, nil
		}
	}
	return "", ErrNoBrowser
}

// LoginAvailable reports why OpenLoginWindow can't work on this machine, or nil when it can.
func LoginAvailable() error {
	if _, err := FindBrowser(); err != nil {
		return err
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" &&
		os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return ErrNoDisplay
	}
	return nil
}

// LoginWindow is a browser window opened for a person to log in to a site by hand, in a
// throwaway profile directory that Close deletes. gocrawl only reads its cookie jar.
type LoginWindow struct {
	profileDir  string
	allocCancel context.CancelFunc
	tabCtx      context.Context
	tabCancel   context.CancelFunc

	done      chan struct{}
	doneOnce  sync.Once
	closeOnce sync.Once
	closeErr  error
}

// OpenLoginWindow starts a visible browser on rawURL. ctx bounds only the launch and the first
// navigation: the window stays open until Close, the person closes it, or the process exits.
func OpenLoginWindow(ctx context.Context, rawURL string) (*LoginWindow, error) {
	if err := LoginAvailable(); err != nil {
		return nil, err
	}
	return openLoginWindow(ctx, rawURL, true)
}

func openLoginWindow(ctx context.Context, rawURL string, visible bool) (*LoginWindow, error) {
	path, err := FindBrowser()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "gocrawl-login-")
	if err != nil {
		return nil, fmt.Errorf("creating a temporary browser profile: %w", err)
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(path), chromedp.UserDataDir(dir))
	if visible {
		opts = append(opts, chromedp.VisibleWindow)
	}
	// The browser must outlive the HTTP request that opened it, so its contexts hang off
	// Background, not ctx.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(quietErrorf))
	w := &LoginWindow{
		profileDir:  dir,
		allocCancel: allocCancel,
		tabCtx:      tabCtx,
		tabCancel:   tabCancel,
		done:        make(chan struct{}),
	}

	// The first action starts the browser and ties it to the context it runs on, so it runs
	// on tabCtx itself; ctx only bounds how long we wait for it.
	started := make(chan error, 1)
	go func() { started <- chromedp.Do(tabCtx) }()
	select {
	case err := <-started:
		if err != nil {
			_ = w.Close()
			if errors.Is(err, chromedp.ErrNoDisplay) {
				return nil, ErrNoDisplay
			}
			return nil, fmt.Errorf("launching the browser: %w", err)
		}
	case <-ctx.Done():
		_ = w.Close()
		return nil, fmt.Errorf("launching the browser: %w", ctx.Err())
	}

	c := chromedp.FromContext(tabCtx)
	go w.watch(c.Browser.LostConnection, c.Target.TargetID)

	navCtx, navCancel := context.WithCancel(tabCtx)
	defer navCancel()
	stop := context.AfterFunc(ctx, navCancel)
	defer stop()
	if err := chromedp.Do(navCtx, chromedp.Navigate(rawURL)); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("opening %s: %w", rawURL, err)
	}
	return w, nil
}

// watch closes done when the person quits the browser (lost connection) or closes the tab
// (Target.targetDestroyed for our target; on macOS the browser keeps running windowless).
func (w *LoginWindow) watch(lost <-chan struct{}, id target.ID) {
	gone := make(chan struct{})
	go func() {
		for ev, err := range chromedp.BrowserEvents(w.tabCtx, target.TargetDestroyed) {
			if err != nil {
				return
			}
			if ev.TargetID == id {
				close(gone)
				return
			}
		}
	}()
	select {
	case <-lost:
	case <-gone:
	case <-w.tabCtx.Done():
	}
	w.doneOnce.Do(func() { close(w.done) })
}

// Done is closed once the window is gone, by Close or by the person.
func (w *LoginWindow) Done() <-chan struct{} { return w.done }

// Cookies reads the browser's whole cookie jar over CDP, HttpOnly cookies and values
// included. Filtering to the site is the caller's job.
func (w *LoginWindow) Cookies(ctx context.Context) ([]*http.Cookie, error) {
	select {
	case <-w.done:
		return nil, ErrLoginWindowClosed
	default:
	}
	callCtx, cancel := context.WithTimeout(w.tabCtx, 15*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	var jar []*network.Cookie
	err := chromedp.Do(callCtx, chromedp.Func(func(c context.Context, t *chromedp.Target) error {
		res, err := cdp.Call(c, t, storage.GetCookies, storage.GetCookiesParams{})
		if err != nil {
			return err
		}
		jar = res.Cookies
		return nil
	}))
	if err != nil {
		select {
		case <-w.done:
			return nil, ErrLoginWindowClosed
		default:
		}
		return nil, fmt.Errorf("reading the browser's cookies: %w", err)
	}
	return httpCookies(jar), nil
}

// Close stops the browser and deletes the throwaway profile. Safe to call more than once.
func (w *LoginWindow) Close() error {
	w.closeOnce.Do(func() {
		w.tabCancel()
		w.allocCancel()
		w.doneOnce.Do(func() { close(w.done) })
		w.closeErr = removeProfileDir(w.profileDir)
	})
	return w.closeErr
}

// removeProfileDir retries for a few seconds: Chromium's helper processes can hold files in
// the profile briefly after the main process exits.
func removeProfileDir(dir string) error {
	var err error
	for range 50 {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("removing the temporary browser profile %s: %w", dir, err)
}

// httpCookies converts CDP cookies to net/http ones, keeping the value and CDP's domain
// spelling (leading dot = domain cookie, none = host-only). CDP reports a session cookie's
// expiry as -1; it maps to the zero time.
func httpCookies(jar []*network.Cookie) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(jar))
	for _, c := range jar {
		if c == nil {
			continue
		}
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}
		if !c.Session && c.Expires > 0 {
			hc.Expires = time.Unix(int64(c.Expires), 0).UTC()
		}
		out = append(out, hc)
	}
	return out
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/render/` — expected PASS (the real-browser test
  runs where Chrome/Chromium/Edge is installed, else it reports SKIP). Manual check of the
  visible path and the "person closes the window" signal (not covered by the headless test):

```sh
cat > /tmp/loginwin_test.go <<'GO'
package render

import (
	"context"
	"testing"
	"time"
)

func TestManualLoginWindow(t *testing.T) {
	w, err := OpenLoginWindow(context.Background(), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("close the window now")
	select {
	case <-w.Done():
		t.Log("Done fired")
	case <-time.After(60 * time.Second):
		t.Fatal("Done never fired")
	}
	_ = w.Close()
}
GO
cp /tmp/loginwin_test.go internal/render/zz_manual_test.go && go test ./internal/render/ -run TestManualLoginWindow -v; rm internal/render/zz_manual_test.go
```

  Expected: a normal browser window opens on example.com without an "automated software"
  banner; closing it prints `Done fired`. Then `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/render/login.go internal/render/login_test.go
git commit -F - <<'MSG'
feat(render): visible login window with a throwaway profile and CDP cookie read

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-LOGIN-2: `loginManager` sessions (states, TTL, cookie filtering)

**Files:**
- Modify: `internal/webserver/login.go` (replace the wave-0 stub; keep `loginRoutes` empty)
- Test: `internal/webserver/login_test.go` (new; also holds the fakes used by later tasks)

**Interfaces:**
- Consumes: `render.OpenLoginWindow`, `formatTime`.
- Produces (contract): `type loginBrowser interface { Open(ctx, url) (loginWindow, error) }`,
  `type loginWindow interface { Cookies(ctx) ([]*http.Cookie, error); Done() <-chan struct{}; Close() error }`,
  states `waiting`/`captured`/`closed`/`expired`, view `{id, state, host, cookie_names[], cookie_count, expires_at?}`.
  Internal: `newLoginManager() *loginManager` (same signature as the stub),
  `(m) open(ctx, rawURL, host) (loginView, error)`, `get(id) (loginView, bool)`,
  `capture(ctx, id) (loginView, error)`, `remove(id) bool`,
  `cookieHeader(id) (header, host string, err error)`, `closeAll()`,
  `cookiesForHost(jar, host, now) []*http.Cookie`; errors `errLoginNotFound` (404),
  `errLoginLimit` (429), `*loginStateError` (409).

- [ ] **Step 1: Write the failing tests.** Create `internal/webserver/login_test.go`:

```go
package webserver

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"
)

// fakeLoginWindow stands in for a browser window. userCloses simulates the person closing
// it: Done fires without Close being called.
type fakeLoginWindow struct {
	mu      sync.Mutex
	cookies []*http.Cookie
	closed  bool
	done    chan struct{}
	once    sync.Once
}

func (w *fakeLoginWindow) Cookies(context.Context) ([]*http.Cookie, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, errors.New("window closed")
	}
	return w.cookies, nil
}

func (w *fakeLoginWindow) Done() <-chan struct{} { return w.done }

func (w *fakeLoginWindow) Close() error {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.once.Do(func() { close(w.done) })
	return nil
}

func (w *fakeLoginWindow) userCloses() { w.once.Do(func() { close(w.done) }) }

func (w *fakeLoginWindow) setCookies(c ...*http.Cookie) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cookies = c
}

func (w *fakeLoginWindow) isClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// fakeLoginBrowser records every Open. next seeds each new window's cookies; err makes Open
// fail; gate, when set, holds Open until closed; available is what Available reports.
type fakeLoginBrowser struct {
	mu        sync.Mutex
	opened    []string
	windows   []*fakeLoginWindow
	next      []*http.Cookie
	err       error
	gate      chan struct{}
	available error
}

func (b *fakeLoginBrowser) Open(ctx context.Context, url string) (loginWindow, error) {
	b.mu.Lock()
	gate := b.gate
	b.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return nil, b.err
	}
	w := &fakeLoginWindow{cookies: b.next, done: make(chan struct{})}
	b.opened = append(b.opened, url)
	b.windows = append(b.windows, w)
	return w, nil
}

func (b *fakeLoginBrowser) Available() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.available
}

func (b *fakeLoginBrowser) window(t *testing.T, i int) *fakeLoginWindow {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if i >= len(b.windows) {
		t.Fatalf("window %d was never opened (%d opened)", i, len(b.windows))
	}
	return b.windows[i]
}

func (b *fakeLoginBrowser) openedURLs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.opened...)
}

func newTestLoginManager(b loginBrowser) *loginManager {
	m := newLoginManager()
	m.browser = b
	return m
}

func waitForLogin(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The capture keeps exactly the cookies a browser would send to the seed host.
func TestCookiesForHostFollowsBrowserDomainRules(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	jar := []*http.Cookie{
		{Name: "host_only", Value: "1", Domain: "shop.example.com"},
		{Name: "parent_domain", Value: "2", Domain: ".example.com"},
		{Name: "same_domain_dot", Value: "3", Domain: ".shop.example.com"},
		{Name: "sibling", Value: "4", Domain: "accounts.example.com"},
		{Name: "child_host_only", Value: "5", Domain: "eu.shop.example.com"},
		{Name: "other_site", Value: "6", Domain: ".evil.com"},
		{Name: "suffix_trick", Value: "7", Domain: "evilshop.example.com"},
		{Name: "partial_label", Value: "8", Domain: ".hop.example.com"},
		{Name: "expired", Value: "9", Domain: "shop.example.com", Expires: now.Add(-time.Minute)},
		{Name: "future", Value: "10", Domain: "shop.example.com", Expires: now.Add(time.Hour)},
		{Name: "no_domain", Value: "11"},
		nil,
	}
	var got []string
	for _, c := range cookiesForHost(jar, "Shop.Example.com", now) {
		got = append(got, c.Name)
	}
	want := []string{"host_only", "parent_domain", "same_domain_dot", "future"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
}

func TestLoginManagerCaptureKeepsSeedCookiesAndClosesWindow(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/account", "shop.example.com")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if v.State != "waiting" || v.Host != "shop.example.com" || v.CookieCount != 0 {
		t.Fatalf("open view = %+v", v)
	}
	win := b.window(t, 0)
	win.setCookies(
		&http.Cookie{Name: "sid", Value: "s3cret", Domain: "shop.example.com", Expires: time.Now().Add(2 * time.Hour)},
		&http.Cookie{Name: "pref", Value: "x", Domain: ".example.com", Expires: time.Now().Add(time.Hour)},
		&http.Cookie{Name: "idp", Value: "y", Domain: "login.idp.example"},
	)
	v, err = m.capture(ctx, v.ID)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if v.State != "captured" || v.CookieCount != 2 || !reflect.DeepEqual(v.CookieNames, []string{"pref", "sid"}) {
		t.Errorf("captured view = %+v", v)
	}
	if v.ExpiresAt == "" {
		t.Error("ExpiresAt empty, want the earliest cookie expiry")
	}
	if !win.isClosed() {
		t.Error("window still open after capture")
	}
	header, host, err := m.cookieHeader(v.ID)
	if err != nil {
		t.Fatalf("cookieHeader: %v", err)
	}
	if header != "sid=s3cret; pref=x" || host != "shop.example.com" {
		t.Errorf("cookieHeader = %q for %q", header, host)
	}
}

func TestLoginManagerCaptureWithoutSeedCookiesKeepsWaiting(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	b.window(t, 0).setCookies(&http.Cookie{Name: "idp", Value: "y", Domain: "login.idp.example"})
	_, err = m.capture(ctx, v.ID)
	var se *loginStateError
	if !errors.As(err, &se) {
		t.Fatalf("capture err = %v, want a loginStateError", err)
	}
	if got, _ := m.get(v.ID); got.State != "waiting" {
		t.Errorf("state = %s, want waiting", got.State)
	}
	if b.window(t, 0).isClosed() {
		t.Error("window closed although nothing was captured")
	}
}

func TestLoginManagerUserClosingWindowEndsSession(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	win := b.window(t, 0)
	win.userCloses()
	waitForLogin(t, "state closed", func() bool { got, _ := m.get(v.ID); return got.State == "closed" })
	waitForLogin(t, "profile cleanup (Close)", win.isClosed)
	var se *loginStateError
	if _, err := m.capture(ctx, v.ID); !errors.As(err, &se) {
		t.Errorf("capture after close err = %v, want a loginStateError", err)
	}
	if _, _, err := m.cookieHeader(v.ID); !errors.As(err, &se) {
		t.Errorf("cookieHeader after close err = %v, want a loginStateError", err)
	}
}

func TestLoginManagerExpiresSessions(t *testing.T) {
	b := &fakeLoginBrowser{next: []*http.Cookie{{Name: "sid", Value: "1", Domain: "shop.example.com"}}}
	m := newTestLoginManager(b)
	m.ttl = 200 * time.Millisecond
	ctx := context.Background()
	waiting, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	captured, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.capture(ctx, captured.ID); err != nil {
		t.Fatal(err)
	}
	waitForLogin(t, "both sessions expired", func() bool {
		a, _ := m.get(waiting.ID)
		c, _ := m.get(captured.ID)
		return a.State == "expired" && c.State == "expired"
	})
	if !b.window(t, 0).isClosed() {
		t.Error("expired waiting session left its window open")
	}
	if got, _ := m.get(captured.ID); got.CookieCount != 0 {
		t.Errorf("expired session still holds %d cookies", got.CookieCount)
	}
	var se *loginStateError
	if _, _, err := m.cookieHeader(captured.ID); !errors.As(err, &se) {
		t.Errorf("cookieHeader after expiry err = %v, want a loginStateError", err)
	}
}

func TestLoginManagerCapsOpenWindows(t *testing.T) {
	m := newTestLoginManager(&fakeLoginBrowser{})
	ctx := context.Background()
	for i := 0; i < maxOpenLoginWindows; i++ {
		if _, err := m.open(ctx, "https://shop.example.com/", "shop.example.com"); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	if _, err := m.open(ctx, "https://shop.example.com/", "shop.example.com"); !errors.Is(err, errLoginLimit) {
		t.Errorf("open past the cap err = %v, want errLoginLimit", err)
	}
}

// A session cancelled while its browser is still starting must not leave a window behind.
func TestLoginManagerRemoveWhileOpening(t *testing.T) {
	b := &fakeLoginBrowser{gate: make(chan struct{})}
	m := newTestLoginManager(b)
	type result struct {
		v   loginView
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := m.open(context.Background(), "https://shop.example.com/", "shop.example.com")
		done <- result{v, err}
	}()
	var id string
	waitForLogin(t, "session registered", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		for k := range m.sessions {
			id = k
		}
		return id != ""
	})
	if !m.remove(id) {
		t.Fatal("remove returned false for a registered session")
	}
	close(b.gate)
	r := <-done
	if !errors.Is(r.err, errLoginNotFound) {
		t.Errorf("open err = %v, want errLoginNotFound", r.err)
	}
	waitForLogin(t, "late window closed", func() bool { return b.window(t, 0).isClosed() })
}

func TestLoginManagerCloseAll(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	m.closeAll()
	if !b.window(t, 0).isClosed() {
		t.Error("closeAll left a window open")
	}
	if _, ok := m.get(v.ID); ok {
		t.Error("closeAll kept the session")
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/webserver/ -run 'Login|CookiesForHost'` —
  expected FAIL at build: `undefined: cookiesForHost`, `m.open undefined (type *loginManager has no field or method open)`, …

- [ ] **Step 3: Implement.** Replace `internal/webserver/login.go` with:

```go
package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpguts"

	"github.com/Patience-dot-devl/gocrawl/internal/render"
)

// loginBrowser opens a browser window for a person to log in to a site by hand. ctx bounds
// only the launch and first navigation; the window lives until Close, the person closes it, or
// the process exits. Tests use a fake; production uses chromedpLoginBrowser.
type loginBrowser interface {
	Open(ctx context.Context, url string) (loginWindow, error)
}

// loginWindow is one open login window. Cookies returns the whole cookie jar, values and
// HttpOnly cookies included, with CDP's domain spelling (leading dot = domain cookie). Done is
// closed when the window goes away, by Close or by the person. Close is idempotent and
// removes the window's throwaway profile.
type loginWindow interface {
	Cookies(ctx context.Context) ([]*http.Cookie, error)
	Done() <-chan struct{}
	Close() error
}

type loginState string

const (
	loginWaiting  loginState = "waiting"
	loginCaptured loginState = "captured"
	loginClosed   loginState = "closed"
	loginExpired  loginState = "expired"
)

const (
	// loginSessionTTL ends every session an hour after it starts, captured or not.
	loginSessionTTL = time.Hour
	// maxOpenLoginWindows caps browser windows waiting for a login at once (429 beyond).
	maxOpenLoginWindows = 2
)

var (
	errLoginNotFound = errors.New("no login session with that id")
	errLoginLimit    = fmt.Errorf("%d login windows are already open; finish or cancel one first", maxOpenLoginWindows)
)

// loginStateError is a request the session's state doesn't allow (HTTP 409).
type loginStateError struct{ msg string }

func (e *loginStateError) Error() string { return e.msg }

func loginStateErr(st loginState) error {
	switch st {
	case loginCaptured:
		return &loginStateError{"this login session was already captured"}
	case loginClosed:
		return &loginStateError{"the browser window was closed before the login was captured; start a new login"}
	case loginExpired:
		return &loginStateError{"this login session expired; start a new login"}
	default:
		return &loginStateError{"the login was not captured yet; log in in the browser window, then press I'm logged in"}
	}
}

// loginSession is one browser login. Cookies live only here, in memory, and are only ever
// returned as a Cookie header for a crawl of the same host.
type loginSession struct {
	id      string
	host    string
	state   loginState
	window  loginWindow    // set while waiting; nil before Open returns and once ended
	cookies []*http.Cookie // set once captured; dropped on expiry
	timer   *time.Timer
	endedAt time.Time
}

func (s *loginSession) end() {
	s.endedAt = time.Now()
	if s.timer != nil {
		s.timer.Stop()
	}
}

// loginView is the API representation of a session: never a cookie value.
type loginView struct {
	ID          string   `json:"id"`
	State       string   `json:"state"`
	Host        string   `json:"host"`
	CookieNames []string `json:"cookie_names"`
	CookieCount int      `json:"cookie_count"`
	ExpiresAt   string   `json:"expires_at,omitempty"` // earliest cookie expiry; omitted when all are session cookies
}

func (s *loginSession) view() loginView {
	v := loginView{ID: s.id, State: string(s.state), Host: s.host, CookieNames: []string{}, CookieCount: len(s.cookies)}
	seen := map[string]bool{}
	var earliest time.Time
	for _, c := range s.cookies {
		if !seen[c.Name] {
			seen[c.Name] = true
			v.CookieNames = append(v.CookieNames, c.Name)
		}
		if !c.Expires.IsZero() && (earliest.IsZero() || c.Expires.Before(earliest)) {
			earliest = c.Expires
		}
	}
	sort.Strings(v.CookieNames)
	v.ExpiresAt = formatTime(earliest)
	return v
}

// loginManager holds every login session for the life of the process.
type loginManager struct {
	mu       sync.Mutex
	browser  loginBrowser
	sessions map[string]*loginSession
	ttl      time.Duration
}

func newLoginManager() *loginManager {
	return &loginManager{
		browser:  chromedpLoginBrowser{},
		sessions: make(map[string]*loginSession),
		ttl:      loginSessionTTL,
	}
}

// newLoginID is 128 random bits: a session id is the only thing standing between a crawl
// request and someone's logged-in cookies.
func newLoginID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// open registers a waiting session, then opens the browser without holding the lock.
func (m *loginManager) open(ctx context.Context, rawURL, host string) (loginView, error) {
	m.mu.Lock()
	m.pruneLocked(time.Now())
	waiting := 0
	for _, s := range m.sessions {
		if s.state == loginWaiting {
			waiting++
		}
	}
	if waiting >= maxOpenLoginWindows {
		m.mu.Unlock()
		return loginView{}, errLoginLimit
	}
	s := &loginSession{id: newLoginID(), host: host, state: loginWaiting}
	m.sessions[s.id] = s
	browser, ttl := m.browser, m.ttl
	m.mu.Unlock()

	win, err := browser.Open(ctx, rawURL)

	m.mu.Lock()
	if err != nil {
		delete(m.sessions, s.id)
		m.mu.Unlock()
		return loginView{}, err
	}
	if m.sessions[s.id] != s {
		// Removed (DELETE, or closeAll at shutdown) while the browser was starting.
		m.mu.Unlock()
		_ = win.Close()
		return loginView{}, errLoginNotFound
	}
	s.window = win
	s.timer = time.AfterFunc(ttl, func() { m.expire(s.id) })
	v := s.view()
	m.mu.Unlock()
	go m.watch(s.id, win)
	return v, nil
}

// watch moves a waiting session to closed when the person closes its window, and cleans up
// the window's profile. Windows the manager closes itself are detached first, so they no-op.
func (m *loginManager) watch(id string, win loginWindow) {
	<-win.Done()
	m.mu.Lock()
	s, ok := m.sessions[id]
	userClosed := ok && s.window == win && s.state == loginWaiting
	if userClosed {
		s.state = loginClosed
		s.window = nil
		s.end()
	}
	m.mu.Unlock()
	if userClosed {
		_ = win.Close()
	}
}

func (m *loginManager) get(id string) (loginView, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return loginView{}, false
	}
	return s.view(), true
}

// capture reads the window's cookies, keeps the ones for the session's host, and closes the
// window. With no cookie for the host yet, the session stays waiting so the person can finish.
func (m *loginManager) capture(ctx context.Context, id string) (loginView, error) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return loginView{}, errLoginNotFound
	}
	if s.state != loginWaiting {
		st := s.state
		m.mu.Unlock()
		return loginView{}, loginStateErr(st)
	}
	win, host := s.window, s.host
	m.mu.Unlock()
	if win == nil {
		return loginView{}, &loginStateError{"the browser window is still opening; try again in a moment"}
	}

	jar, err := win.Cookies(ctx)
	if err != nil {
		select {
		case <-win.Done():
			return loginView{}, loginStateErr(loginClosed)
		default:
		}
		return loginView{}, err
	}
	kept := cookiesForHost(jar, host, time.Now())
	if len(kept) == 0 {
		return loginView{}, &loginStateError{fmt.Sprintf("the browser has no cookies for %s yet; finish logging in there, then try again", host)}
	}

	m.mu.Lock()
	if m.sessions[id] != s {
		m.mu.Unlock()
		return loginView{}, errLoginNotFound
	}
	if s.state != loginWaiting || s.window != win {
		st := s.state
		m.mu.Unlock()
		return loginView{}, loginStateErr(st)
	}
	s.state = loginCaptured
	s.cookies = kept
	s.window = nil
	v := s.view()
	m.mu.Unlock()
	_ = win.Close()
	return v, nil
}

// remove ends and forgets a session, closing its window.
func (m *loginManager) remove(id string) bool {
	m.mu.Lock()
	s, ok := m.sessions[id]
	var win loginWindow
	if ok {
		delete(m.sessions, id)
		s.end()
		s.cookies = nil
		win, s.window = s.window, nil
	}
	m.mu.Unlock()
	if win != nil {
		_ = win.Close()
	}
	return ok
}

// expire runs from the session's timer: it drops the cookies and closes any open window. The
// session stays listed as expired (until pruned) so the UI can say what happened.
func (m *loginManager) expire(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok || (s.state != loginWaiting && s.state != loginCaptured) {
		m.mu.Unlock()
		return
	}
	win := s.window
	s.state = loginExpired
	s.window = nil
	s.cookies = nil
	s.endedAt = time.Now()
	m.mu.Unlock()
	if win != nil {
		_ = win.Close()
	}
}

// pruneLocked forgets ended sessions once they have been ended for a TTL.
func (m *loginManager) pruneLocked(now time.Time) {
	for id, s := range m.sessions {
		if (s.state == loginClosed || s.state == loginExpired) && now.Sub(s.endedAt) > m.ttl {
			delete(m.sessions, id)
		}
	}
}

// cookieHeader returns a captured session's cookies as a Cookie header value, and the host
// they belong to.
func (m *loginManager) cookieHeader(id string) (header, host string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return "", "", errLoginNotFound
	}
	if s.state != loginCaptured {
		return "", "", loginStateErr(s.state)
	}
	parts := make([]string, 0, len(s.cookies))
	for _, c := range cookiesForHost(s.cookies, s.host, time.Now()) {
		// Skip anything that isn't a valid name=value pair rather than sanitizing it:
		// net/http's sanitizer logs the bytes it drops, and these are secrets.
		if !httpguts.ValidHeaderFieldName(c.Name) || !httpguts.ValidHeaderFieldValue(c.Value) || strings.Contains(c.Value, ";") {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	if len(parts) == 0 {
		return "", "", &loginStateError{"the captured cookies have expired; log in again"}
	}
	return strings.Join(parts, "; "), s.host, nil
}

// closeAll closes every window and forgets every session (process shutdown).
func (m *loginManager) closeAll() {
	m.mu.Lock()
	var wins []loginWindow
	for id, s := range m.sessions {
		if s.window != nil {
			wins = append(wins, s.window)
		}
		s.end()
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	for _, w := range wins {
		_ = w.Close()
	}
}

// cookiesForHost keeps the unexpired cookies a browser would send to host: a host-only cookie
// (CDP domain without a leading dot) only to that exact host, a domain cookie (".example.com")
// to that domain and its subdomains.
func cookiesForHost(jar []*http.Cookie, host string, now time.Time) []*http.Cookie {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	var out []*http.Cookie
	for _, c := range jar {
		if c == nil || c.Name == "" {
			continue
		}
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		if cookieDomainMatches(c.Domain, host) {
			out = append(out, c)
		}
	}
	return out
}

func cookieDomainMatches(domain, host string) bool {
	d := strings.ToLower(domain)
	if d == "" {
		return false
	}
	if rest, ok := strings.CutPrefix(d, "."); ok {
		return rest != "" && (host == rest || strings.HasSuffix(host, "."+rest))
	}
	return host == d
}

// chromedpLoginBrowser opens real windows through internal/render.
type chromedpLoginBrowser struct{}

func (chromedpLoginBrowser) Open(ctx context.Context, url string) (loginWindow, error) {
	w, err := render.OpenLoginWindow(ctx, url)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Server) loginRoutes() {}
```

  `golang.org/x/net` is already a direct dependency (crawler uses `httpguts`); no go.mod change.

- [ ] **Step 4: Run.** `go test -race ./internal/webserver/` — expected PASS. `make check` — PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/webserver/login.go internal/webserver/login_test.go
git commit -F - <<'MSG'
feat(webserver): login session manager with capture, expiry and seed-host cookie filtering

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-LOGIN-3: Login-session routes, `WithLoginBrowser`, `Server.Close`

**Files:**
- Modify: `internal/webserver/login.go`, `cmd/gocrawl/serve.go`
- Test: `internal/webserver/login_routes_test.go` (new)

**Interfaces:**
- Consumes: `loginManager` (W1-LOGIN-2), `render.ErrNoBrowser`, `render.LoginAvailable`,
  `crawler.SanitizeSeed`, `requireJSON`.
- Produces (contract): `func WithLoginBrowser(b loginBrowser) Option`; routes
  `POST /api/login-sessions` `{url}` -> 202 view; `GET /api/login-sessions/{id}`;
  `POST /api/login-sessions/{id}/capture`; `DELETE /api/login-sessions/{id}` -> 204;
  `GET /api/login-sessions/available` -> `{available, reason?}`. Gap filled:
  `func (s *Server) Close()` (closes windows on shutdown), optional `loginProber` interface
  (`Available() error`), `loginErrStatus(err) int`.

- [ ] **Step 1: Write the failing tests.** Create `internal/webserver/login_routes_test.go`:

```go
package webserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/render"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

func newLoginServer(t *testing.T, b *fakeLoginBrowser) *Server {
	t.Helper()
	srv := New(store.New(t.TempDir()), WithLoginBrowser(b))
	t.Cleanup(srv.Close)
	return srv
}

func loginRaw(t *testing.T, srv *Server, method, path, contentType, origin, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "localhost:8080"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestLoginAvailability(t *testing.T) {
	_, out := doJSON(t, newLoginServer(t, &fakeLoginBrowser{}), http.MethodGet, "/api/login-sessions/available", nil)
	if out["available"] != true || out["reason"] != nil {
		t.Errorf("available = %v", out)
	}
	_, out = doJSON(t, newLoginServer(t, &fakeLoginBrowser{available: render.ErrNoBrowser}), http.MethodGet, "/api/login-sessions/available", nil)
	if out["available"] != false {
		t.Errorf("available = %v, want false", out["available"])
	}
	if reason, _ := out["reason"].(string); !strings.Contains(reason, "Chromium") {
		t.Errorf("reason = %q, want it to mention Chromium", reason)
	}
}

func TestLoginSessionFlow(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "shop.example.com/account"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start: status = %d (body %s)", rec.Code, rec.Body)
	}
	if started["state"] != "waiting" || started["host"] != "shop.example.com" {
		t.Errorf("start view = %v", started)
	}
	if got := b.openedURLs(); !reflect.DeepEqual(got, []string{"https://shop.example.com/account"}) {
		t.Errorf("opened %v, want the https seed URL (is WithLoginBrowser applied after New builds s.logins?)", got)
	}
	id, _ := started["id"].(string)

	b.window(t, 0).setCookies(
		&http.Cookie{Name: "sid", Value: "s3cret-value", Domain: "shop.example.com"},
		&http.Cookie{Name: "idp", Value: "other-secret", Domain: "idp.example"},
	)
	rec, captured := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("capture: status = %d (body %s)", rec.Code, rec.Body)
	}
	if captured["state"] != "captured" || captured["cookie_count"] != float64(1) || !reflect.DeepEqual(captured["cookie_names"], []any{"sid"}) {
		t.Errorf("captured view = %v", captured)
	}
	if strings.Contains(rec.Body.String(), "s3cret-value") || strings.Contains(rec.Body.String(), "other-secret") {
		t.Errorf("capture response leaks a cookie value: %s", rec.Body)
	}

	rec, got := doJSON(t, srv, http.MethodGet, "/api/login-sessions/"+id, nil)
	if rec.Code != http.StatusOK || got["state"] != "captured" || strings.Contains(rec.Body.String(), "s3cret-value") {
		t.Errorf("get: status %d, body %s", rec.Code, rec.Body)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil); rec.Code != http.StatusConflict {
		t.Errorf("second capture: status = %d, want 409", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodDelete, "/api/login-sessions/"+id, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodGet, "/api/login-sessions/"+id, nil); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: status = %d, want 404", rec.Code)
	}
}

func TestStartLoginValidatesAndStripsCredentials(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	for _, bad := range []string{"", "ftp://example.com/", "https://"} {
		if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": bad}); rec.Code != http.StatusBadRequest {
			t.Errorf("url %q: status = %d, want 400", bad, rec.Code)
		}
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://alice:pw@shop.example.com/"}); rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if got := b.openedURLs(); !reflect.DeepEqual(got, []string{"https://shop.example.com/"}) {
		t.Errorf("opened %v, want only the credential-free URL", got)
	}
}

func TestStartLoginGuards(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	if rec := loginRaw(t, srv, http.MethodPost, "/api/login-sessions", "text/plain", "", `{"url":"https://shop.example.com"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: status = %d, want 415", rec.Code)
	}
	if rec := loginRaw(t, srv, http.MethodPost, "/api/login-sessions", "application/json", "http://evil.example", `{"url":"https://shop.example.com"}`); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status = %d, want 403", rec.Code)
	}
	if got := b.openedURLs(); len(got) != 0 {
		t.Errorf("rejected requests opened windows: %v", got)
	}
}

func TestStartLoginWhenBrowserFails(t *testing.T) {
	rec, out := doJSON(t, newLoginServer(t, &fakeLoginBrowser{err: render.ErrNoBrowser}), http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no browser: status = %d, want 503", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(strings.ToLower(msg), "no chromium-class browser found") {
		t.Errorf("no browser: error = %q", msg)
	}
	rec, out = doJSON(t, newLoginServer(t, &fakeLoginBrowser{err: errors.New("boom")}), http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("launch failure: status = %d, want 503", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "boom") {
		t.Errorf("launch failure: error = %q", msg)
	}
}

func TestLoginSessionWindowClosedByUser(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	_, started := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"})
	id, _ := started["id"].(string)
	b.window(t, 0).userCloses()
	waitForLogin(t, "state closed", func() bool {
		_, got := doJSON(t, srv, http.MethodGet, "/api/login-sessions/"+id, nil)
		return got["state"] == "closed"
	})
	rec, out := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("capture: status = %d, want 409", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "closed") {
		t.Errorf("capture: error = %q, want it to say the window was closed", msg)
	}
}

func TestLoginRoutesUnknownSession(t *testing.T) {
	srv := newLoginServer(t, &fakeLoginBrowser{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/login-sessions/nope"},
		{http.MethodPost, "/api/login-sessions/nope/capture"},
		{http.MethodDelete, "/api/login-sessions/nope"},
	} {
		if rec, _ := doJSON(t, srv, tc.method, tc.path, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}

func TestLoginWindowLimitReturns429(t *testing.T) {
	srv := newLoginServer(t, &fakeLoginBrowser{})
	for i := 0; i < maxOpenLoginWindows; i++ {
		if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"}); rec.Code != http.StatusAccepted {
			t.Fatalf("start %d: status = %d", i, rec.Code)
		}
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("start past the cap: status = %d, want 429", rec.Code)
	}
}

func TestServerCloseClosesLoginWindows(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := New(store.New(t.TempDir()), WithLoginBrowser(b))
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"}); rec.Code != http.StatusAccepted {
		t.Fatalf("start: status = %d", rec.Code)
	}
	srv.Close()
	if !b.window(t, 0).isClosed() {
		t.Error("Close left a login window open")
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/webserver/ -run 'Login|ServerClose'` —
  expected FAIL at build: `undefined: WithLoginBrowser`, `srv.Close undefined`.

- [ ] **Step 3: Implement.** In `internal/webserver/login.go`:
  1. Add imports `"encoding/json"`, `"net/url"`, and
     `"github.com/Patience-dot-devl/gocrawl/internal/crawler"`.
  2. Replace `func (s *Server) loginRoutes() {}` with:

```go
// loginProber is implemented by a loginBrowser that can tell, without opening a window,
// whether Open can work here. A browser that doesn't implement it is assumed available.
type loginProber interface {
	Available() error
}

func (chromedpLoginBrowser) Available() error { return render.LoginAvailable() }

// WithLoginBrowser replaces the Chromium launcher behind /api/login-sessions; tests pass a fake.
func WithLoginBrowser(b loginBrowser) Option {
	return func(s *Server) {
		if s.logins == nil {
			s.logins = newLoginManager()
		}
		s.logins.mu.Lock()
		s.logins.browser = b
		s.logins.mu.Unlock()
	}
}

// Close releases what the server holds outside its own memory: open login windows and their
// throwaway browser profiles. gocrawl serve calls it on shutdown.
func (s *Server) Close() {
	s.logins.closeAll()
}

const (
	maxLoginBody        = 16 << 10
	loginOpenTimeout    = 45 * time.Second
	loginCaptureTimeout = 30 * time.Second
)

type loginAvailability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (s *Server) loginRoutes() {
	s.mux.HandleFunc("GET /api/login-sessions/available", s.handleLoginAvailable)
	s.mux.HandleFunc("POST /api/login-sessions", s.handleStartLogin)
	s.mux.HandleFunc("GET /api/login-sessions/{id}", s.handleGetLogin)
	s.mux.HandleFunc("POST /api/login-sessions/{id}/capture", s.handleCaptureLogin)
	s.mux.HandleFunc("DELETE /api/login-sessions/{id}", s.handleDeleteLogin)
}

func (s *Server) handleLoginAvailable(w http.ResponseWriter, _ *http.Request) {
	s.logins.mu.Lock()
	browser := s.logins.browser
	s.logins.mu.Unlock()
	out := loginAvailability{Available: true}
	if p, ok := browser.(loginProber); ok {
		if err := p.Available(); err != nil {
			out = loginAvailability{Available: false, Reason: err.Error()}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleStartLogin(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var in struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	target, host, err := loginTarget(in.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), loginOpenTimeout)
	defer cancel()
	v, err := s.logins.open(ctx, target, host)
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, v)
	case errors.Is(err, errLoginLimit):
		writeError(w, http.StatusTooManyRequests, err)
	case errors.Is(err, render.ErrNoBrowser):
		writeError(w, http.StatusServiceUnavailable, errors.New("no Chromium-class browser found; install Google Chrome, Chromium or Microsoft Edge to log in with a browser"))
	case errors.Is(err, errLoginNotFound):
		writeError(w, http.StatusConflict, errors.New("the login was cancelled while the browser was starting"))
	default:
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("could not open the login browser: %w", err))
	}
}

func (s *Server) handleGetLogin(w http.ResponseWriter, r *http.Request) {
	v, ok := s.logins.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errLoginNotFound)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleCaptureLogin(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), loginCaptureTimeout)
	defer cancel()
	v, err := s.logins.capture(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, loginErrStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleDeleteLogin(w http.ResponseWriter, r *http.Request) {
	if !s.logins.remove(r.PathValue("id")) {
		writeError(w, http.StatusNotFound, errLoginNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func loginErrStatus(err error) int {
	var stateErr *loginStateError
	switch {
	case errors.Is(err, errLoginNotFound):
		return http.StatusNotFound
	case errors.As(err, &stateErr):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// loginTarget normalizes the URL to open like a crawl seed: https:// by default, and any
// user:password stripped so it never reaches the browser's address bar or history.
func loginTarget(raw string) (target, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("url is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	raw, _, _ = crawler.SanitizeSeed(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", "", fmt.Errorf("url %q must be an http or https URL with a host", raw)
	}
	return u.String(), strings.ToLower(u.Hostname()), nil
}
```

  In `cmd/gocrawl/serve.go`, `runServe`, replace
  `srv := newHTTPServer(addr, webserver.New(st, opts...).Handler())` with:

```go
	web := webserver.New(st, opts...)
	// Closes login browser windows (and deletes their temporary profiles) on Ctrl-C.
	defer web.Close()
	srv := newHTTPServer(addr, web.Handler())
```

- [ ] **Step 4: Run.** `go test -race ./internal/webserver/ ./cmd/gocrawl/` — expected PASS.
  Manual (needs Chrome): `go build -o /tmp/gocrawl-dev ./cmd/gocrawl && /tmp/gocrawl-dev serve`
  in one terminal; in another:
  `curl -s localhost:8080/api/login-sessions/available` (expect `{"available":true}`),
  `curl -s -H 'Content-Type: application/json' -d '{"url":"https://github.com/login"}' localhost:8080/api/login-sessions`
  (a window opens; expect `"state":"waiting"`), log in, then
  `curl -s -X POST localhost:8080/api/login-sessions/<id>/capture` (window closes; expect
  cookie names and a count, no values). Start another session and Ctrl-C the server: its window
  closes and `ls "$TMPDIR" | grep gocrawl-login-` prints nothing. `make check` — PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/webserver/login.go internal/webserver/login_routes_test.go cmd/gocrawl/serve.go
git commit -F - <<'MSG'
feat(webserver): /api/login-sessions routes for interactive browser login

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-LOGIN-4: `login_session` on `POST /api/crawls`

**Files:**
- Modify: `internal/webserver/crawls.go`, `internal/webserver/login.go`
- Test: `internal/webserver/crawl_login_test.go` (new)

**Interfaces:**
- Consumes: `loginManager.cookieHeader`, `config.Config`.
- Produces (contract): `startCrawlRequest.LoginSession string \`json:"login_session,omitempty"\``
  (web-only). 400 with `cookie`, with `render: headless`, or when the session's host isn't the
  seed host; 404 unknown session; 409 not captured / closed / expired. Resolved into
  `cfg.Crawl.Cookie`. Internal: `(s *Server) applyLoginSession(cfg *config.Config, seed, id, cookie string) error`,
  `*loginRequestError` (400).

- [ ] **Step 1: Write the failing tests.** Create `internal/webserver/crawl_login_test.go`:

```go
package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// captureTestLogin runs a login through the fake browser to "captured" and returns its id.
func captureTestLogin(t *testing.T, srv *Server, b *fakeLoginBrowser, rawURL string, cookies ...*http.Cookie) string {
	t.Helper()
	b.mu.Lock()
	b.next = cookies
	b.mu.Unlock()
	rec, started := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": rawURL})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start login: status = %d (body %s)", rec.Code, rec.Body)
	}
	id, _ := started["id"].(string)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil); rec.Code != http.StatusOK {
		t.Fatalf("capture: status = %d (body %s)", rec.Code, rec.Body)
	}
	return id
}

func TestStartCrawlWithLoginSessionSendsCookiesToSeed(t *testing.T) {
	var mu sync.Mutex
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			mu.Lock()
			got = r.Header.Get("Cookie")
			mu.Unlock()
		}
		fmt.Fprint(w, `<html><head><title>Members</title></head><body>hi</body></html>`)
	}))
	t.Cleanup(target.Close)

	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	id := captureTestLogin(t, srv, b, target.URL,
		&http.Cookie{Name: "sid", Value: "abc123", Domain: "127.0.0.1"},
		&http.Cookie{Name: "theme", Value: "dark", Domain: "127.0.0.1"},
	)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "login_session": id})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start crawl: status = %d (body %s)", rec.Code, rec.Body)
	}
	job := pollUntilFinished(t, srv, started["id"].(string))
	if job["status"] != "done" {
		t.Fatalf("job = %v, want done", job)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != "sid=abc123; theme=dark" {
		t.Errorf("Cookie = %q, want %q", got, "sid=abc123; theme=dark")
	}
	if raw, _ := json.Marshal(job); strings.Contains(string(raw), "abc123") {
		t.Error("the job view exposes a cookie value")
	}
}

func TestStartCrawlLoginSessionErrors(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	captured := captureTestLogin(t, srv, b, "https://shop.example.com/",
		&http.Cookie{Name: "sid", Value: "1", Domain: "shop.example.com"})
	rec, waiting := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com/"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("second login: status = %d", rec.Code)
	}
	waitingID, _ := waiting["id"].(string)

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"combined with cookie", map[string]any{"url": "https://shop.example.com/", "login_session": captured, "cookie": "a=b"}, http.StatusBadRequest},
		{"unknown session", map[string]any{"url": "https://shop.example.com/", "login_session": "nope"}, http.StatusNotFound},
		{"not captured yet", map[string]any{"url": "https://shop.example.com/", "login_session": waitingID}, http.StatusConflict},
		{"other host", map[string]any{"url": "https://other.example.com/", "login_session": captured}, http.StatusBadRequest},
		{"headless render", map[string]any{"url": "https://shop.example.com/", "login_session": captured, "render": "headless"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", tc.body); rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (body %s)", tc.name, rec.Code, tc.want, rec.Body)
		}
	}
	if n := len(srv.crawls.list()); n != 0 {
		t.Errorf("%d crawl(s) started despite the errors", n)
	}
}

// An internationalized host: the user types the Unicode form, Chromium stores cookie domains
// in the ASCII (punycode) form. Both sides compare in ASCII, so the capture finds the cookie
// and the crawl accepts the session for the Unicode seed.
func TestLoginSessionMatchesIDNHosts(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	id := captureTestLogin(t, srv, b, "https://bücher.example/",
		&http.Cookie{Name: "sid", Value: "1", Domain: "xn--bcher-kva.example"})
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": "https://bücher.example/", "login_session": id, "max_pages": 1})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start crawl: status = %d, want 202 (body %s)", rec.Code, rec.Body)
	}
	jobID := started["id"].(string)
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+jobID+"/cancel", nil)
	pollUntilFinished(t, srv, jobID)
}
```

- [ ] **Step 2: Run them.** `go test ./internal/webserver/ -run 'LoginSession'` — expected
  FAIL: `Cookie = "", want "sid=abc123; theme=dark"`, and `combined with cookie: status = 202, want 400`
  (the unknown `login_session` field is ignored today), plus `5 crawl(s) started`, and
  `TestLoginSessionMatchesIDNHosts` fails in `captureTestLogin` with a 409 (no cookie matches the
  Unicode session host yet).

- [ ] **Step 3: Implement.** In `internal/webserver/crawls.go`:
  1. Add to `startCrawlRequest`:

```go
	// LoginSession names a captured browser login (POST /api/login-sessions); its cookies
	// become the crawl's Cookie header. Web-only: not in crawlrequest.Params, so the MCP
	// tool never sees it.
	LoginSession string `json:"login_session,omitempty"`
```

  2. In `handleStartCrawl`, directly after the `req.ToConfig()` error block (and before the
     `cfg.ToOptions()` validation block if the settings track already added it), insert:

```go
	if req.LoginSession != "" {
		if err := s.applyLoginSession(&cfg, seed, req.LoginSession, req.Cookie); err != nil {
			writeError(w, loginErrStatus(err), err)
			return
		}
	}
```

  In `internal/webserver/login.go`, add the imports
  `"github.com/Patience-dot-devl/gocrawl/internal/config"` and `"golang.org/x/net/idna"`
  (already a direct dependency), change the last line of `loginTarget` from
  `return u.String(), strings.ToLower(u.Hostname()), nil` to
  `return u.String(), asciiHost(u.Hostname()), nil`, add a case to `loginErrStatus` before
  `default`:

```go
	case errors.As(err, &reqErr):
		return http.StatusBadRequest
```

  with `var reqErr *loginRequestError` declared next to `stateErr`, and append:

```go
// asciiHost is the lower-case ASCII (punycode) form of a host name, the form browsers store
// cookie domains in, so "bücher.example" and "xn--bcher-kva.example" compare equal. A host the
// IDNA lookup rules reject (an underscore, say) is compared as typed, lower-cased.
func asciiHost(host string) string {
	if a, err := idna.Lookup.ToASCII(host); err == nil && a != "" {
		return strings.ToLower(a)
	}
	return strings.ToLower(host)
}

// loginRequestError is a crawl request that can't use its login session as asked (HTTP 400).
type loginRequestError struct{ msg string }

func (e *loginRequestError) Error() string { return e.msg }

// applyLoginSession turns a captured login into the crawl's Cookie header. From there the
// existing cookie path applies: sent only to the seed host (plus subdomains when asked), never
// over an https -> http downgrade, and refused by headless rendering.
func (s *Server) applyLoginSession(cfg *config.Config, seed, id, cookie string) error {
	if strings.TrimSpace(cookie) != "" {
		return &loginRequestError{"login_session and cookie can't be combined: the login session supplies the Cookie header"}
	}
	if cfg.Render == "headless" {
		return &loginRequestError{`login_session needs render "raw": headless Chromium can't keep the session cookie on the seed host`}
	}
	header, host, err := s.logins.cookieHeader(id)
	if err != nil {
		return err
	}
	u, err := url.Parse(seed)
	if err != nil || asciiHost(u.Hostname()) != host {
		return &loginRequestError{fmt.Sprintf("this login session is for %s, not for the crawl's seed host; log in again for that site", host)}
	}
	cfg.Crawl.Cookie = header
	return nil
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/webserver/` — expected PASS. `make check` — PASS.

- [ ] **Step 5: Commit.**

```sh
git add internal/webserver/crawls.go internal/webserver/login.go internal/webserver/crawl_login_test.go
git commit -F - <<'MSG'
feat(webserver): crawl with a captured browser login via login_session

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W1-LOGIN-5: Docs and changelog for the login backend

**Files:**
- Modify: `docs/web.md`, `CHANGELOG.md`; generated `docs/README.md` via `make docs`

**Interfaces:** none.

- [ ] **Step 1: Baseline.** `go test ./internal/repocheck/` — expected PASS.

- [ ] **Step 2: Edit `docs/web.md`.** Frontmatter `updated: 2026-10-13`. Append to the
  `POST /api/crawls` row's description: "Optional `login_session` (web only, not MCP) names a
  captured browser login whose cookies become the crawl's `Cookie` header; `400` together with
  `cookie` or `render: headless` or for another host, `404` unknown, `409` not captured." Add
  API rows after the export row:

```markdown
| `GET /api/login-sessions/available` | `{"available": bool, "reason"?: string}`: whether a Chromium-class browser (and, on Linux, a display) is there to log in with. |
| `POST /api/login-sessions` | JSON `{"url"}`: open a visible browser window on that URL in a throwaway profile. `202` with the session; `429` when two login windows are already open; `503` "no Chromium-class browser found". |
| `GET /api/login-sessions/{id}` | `{"id", "state", "host", "cookie_names", "cookie_count", "expires_at"?}`. `state` is `waiting`, `captured`, `closed` (window closed before capture) or `expired` (after one hour). Never cookie values. |
| `POST /api/login-sessions/{id}/capture` | Read the browser's cookies (HttpOnly included), keep those a browser would send to the session's host, close the window and delete its profile. `409` when the window is gone, already captured, or has no cookie for the host yet (the window then stays open). |
| `DELETE /api/login-sessions/{id}` | Close the window and forget the session and its cookies. `204`. |
```

  Add a section before `## Notes`:

```markdown
## Logging in with a browser

For sites behind a login form (staging, member areas, a Shopify password page), the UI can
borrow a real browser session: gocrawl opens a normal Chrome, Chromium or Edge window on the
site in a fresh, temporary profile, you log in there (SSO, two-factor and CAPTCHAs work), and
gocrawl reads that window's cookies when you say you're done. It keeps only the cookies a
browser would send to the site's host, closes the window, and deletes the profile.

- Cookies stay in the server's memory: never written to disk, never returned by the API
  (only names, the count and the earliest expiry), dropped after one hour or when
  `gocrawl serve` stops.
- The crawl sends them as its `Cookie` header with the same scoping as `--cookie`: the seed
  host only (plus subdomains with `subdomains`), raw rendering only.
- The form adds the exclude pattern `(?i)(log|sign)-?out`, so the crawl doesn't follow a
  logout link and end the session. It stays visible in the exclude field; remove it if you need
  those pages.
- Web UI only; the MCP `crawl` tool has no equivalent. Some identity providers (Google among
  them) may refuse to sign in from a browser that is being remote-controlled; use a cookie or
  a bearer token for those.
```

- [ ] **Step 3: Edit `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`:

```markdown
- Web API: interactive browser login. `POST /api/login-sessions` opens a visible Chromium window in a throwaway profile, `…/capture` keeps the seed host's cookies (HttpOnly included) in memory for an hour, and `POST /api/crawls` accepts `login_session` to crawl with them.
```

- [ ] **Step 4: Regenerate and run.** `make docs`, then `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -F - <<'MSG'
docs: document interactive browser login and the login-session API

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

### Track: compare and import backend

#### Task W1-CMP-1: `store.ResolveStored`, which never reads a file path

`store.Resolve` loads any JSON file named by an existing path, because "a real file on disk
wins". That behavior is correct for the CLI. It is a local-file read for a network API. The web
API uses a variant that only looks inside the store.

**Files:**
- Modify: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `(*Store).latest`, `(*Store).pathForID`, `(*Store).Load`, test helpers `rep`, `mustSave`, `writeJSON`
- Produces: `func (s *Store) ResolveStored(ref string) (*report.Report, string, error)`

- [ ] **Step 1: Write the failing test.** Append to `internal/store/store_test.go`:

```go
func TestResolveStoredNeverReadsFilePaths(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "store"))
	id := mustSave(t, s, rep("https://example.com/", "2026-10-13T10:00:00Z", 3))

	outside := filepath.Join(dir, "outside.json")
	if err := writeJSON(outside, rep("https://elsewhere.example/", "2026-10-13T11:00:00Z", 1)); err != nil {
		t.Fatal(err)
	}

	// The CLI form keeps loading files.
	if _, _, err := s.Resolve(outside); err != nil {
		t.Fatalf("Resolve(file path) = %v, want the file loaded", err)
	}
	// The store-only form must not.
	if got, _, err := s.ResolveStored(outside); err == nil {
		t.Fatalf("ResolveStored(file path) loaded %q from outside the store", got.Seed)
	}
	for _, ref := range []string{"latest", "example.com", id} {
		got, gotID, err := s.ResolveStored(ref)
		if err != nil {
			t.Fatalf("ResolveStored(%q): %v", ref, err)
		}
		if got.PagesCrawled != 3 || gotID != id {
			t.Errorf("ResolveStored(%q) = %d pages, id %q; want 3, %q", ref, got.PagesCrawled, gotID, id)
		}
	}
	for _, ref := range []string{"", "nope.example", "example.com/19000101T000000Z", "../outside"} {
		if _, _, err := s.ResolveStored(ref); err == nil {
			t.Errorf("ResolveStored(%q) succeeded, want an error", ref)
		}
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/store -run TestResolveStoredNeverReadsFilePaths`
  Expected: build failure, `s.ResolveStored undefined (type *Store has no field or method ResolveStored)`.

- [ ] **Step 3: Implement.** In `internal/store/store.go`, replace the whole `Resolve` function with:

```go
// Resolve turns a user-supplied reference into a report. A ref may be:
//   - a path to an existing JSON file (loaded directly, not from the store);
//   - "latest" — the newest crawl in the store;
//   - a bare host like "example.com" — that host's newest crawl;
//   - a crawl ID "<host>/<timestamp>".
//
// It returns the resolved report and the ID it resolved to (empty for a direct file path).
func (s *Store) Resolve(ref string) (*report.Report, string, error) {
	if ref == "" {
		return nil, "", fmt.Errorf("empty crawl reference")
	}
	// A real file on disk wins, so explicit paths always work regardless of the store.
	if fi, err := os.Stat(ref); err == nil && !fi.IsDir() {
		rep, rerr := readReport(ref)
		return rep, "", rerr
	}
	return s.ResolveStored(ref)
}

// ResolveStored resolves ref the way Resolve does, but only inside the store: "latest", a
// bare host, or a crawl ID. It never reads a file path. Callers that take refs from the
// network (the web API's compare endpoint) use it, so a request cannot make the server read
// an arbitrary JSON file from disk.
func (s *Store) ResolveStored(ref string) (*report.Report, string, error) {
	if ref == "" {
		return nil, "", fmt.Errorf("empty crawl reference")
	}
	if strings.EqualFold(ref, "latest") {
		return s.latest("")
	}
	// A "<host>/<timestamp>" ID addresses a stored file directly.
	if strings.Contains(ref, "/") {
		path, perr := s.pathForID(ref)
		if perr != nil {
			return nil, "", perr
		}
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			rep, rerr := s.Load(ref)
			return rep, ref, rerr
		}
		return nil, "", fmt.Errorf("no stored crawl with ID %q (looked in %s)", ref, s.root)
	}
	// Otherwise treat it as a host name and take that site's newest crawl.
	return s.latest(ref)
}
```

- [ ] **Step 4: Run.** `go test ./internal/store` Expected: `ok` (the new test and the existing
  `TestResolve` and `TestResolveFilePathWins` all pass).

- [ ] **Step 5: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(store): add ResolveStored, which never reads a file path" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W1-CMP-2: `GET /api/compare`

**Files:**
- Modify: `internal/webserver/compare.go` (replaces the wave-0 stub)
- Test: `internal/webserver/compare_test.go` (new)

**Interfaces:**
- Consumes: `(*store.Store).ResolveStored`, `diff.Compare(base, current *report.Report) *diff.Diff`,
  `s.crawls.get`, `writeJSON`, `writeError`
- Produces: `func (s *Server) compareRoutes()` registers `GET /api/compare?base=<ref>&current=<ref>` → `200 diff.Diff`.
  A ref can be a crawl-job ID from this process, a stored crawl ID, a bare host, or `latest`.
  Errors: `400` when a ref is missing; `404` when a ref does not resolve or names a failed job;
  `409` when a ref names a job that is still running or paused. Unexported helpers:
  `errCompareUnfinished`, `(*Server).resolveCompareRef(ref string) (*report.Report, error)`,
  `writeCompareRefError(w, side string, err error)`.

Why job IDs also count: `GET /api/crawls` lists jobs from this process under their job ID,
including saved ones. `store.Resolve` cannot resolve a job ID, so the Crawls list's Compare
checkboxes would get a 404 for any crawl started in this session.

- [ ] **Step 1: Write the failing test.** Create `internal/webserver/compare_test.go`:

```go
package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

// compareFixture builds a minimal report of example.com: the given issues, plus a site map
// listing the given pages (diff.Compare takes page sets from the site map).
func compareFixture(started string, issues []analyze.Issue, pages ...string) *report.Report {
	rep := &report.Report{
		Seed:         "https://example.com/",
		StartedAt:    started,
		FinishedAt:   started,
		PagesCrawled: len(pages),
		Issues:       issues,
		Summary: report.Summary{
			BySeverity: map[string]int{},
			ByAnalyzer: map[string]int{},
			ByStatus:   map[string]int{"200": len(pages)},
		},
		SiteMap: &sitemapgen.Map{Seed: "https://example.com/"},
	}
	for _, is := range issues {
		rep.Summary.BySeverity[string(is.Severity)]++
		rep.Summary.ByAnalyzer[is.Analyzer]++
	}
	for _, p := range pages {
		rep.SiteMap.Entries = append(rep.SiteMap.Entries, sitemapgen.Entry{Loc: p})
	}
	return rep
}

func compareIssue(code, pageURL string) analyze.Issue {
	return analyze.Issue{Analyzer: "seo", URL: pageURL, Severity: analyze.Error, Code: code, Message: code}
}

// saveComparePair stores an older and a newer crawl of example.com and returns their IDs. The
// newer one fixes missing-title, adds missing-description, drops /b and adds /c.
func saveComparePair(t *testing.T, srv *Server) (baseID, currentID string) {
	t.Helper()
	base := compareFixture("2026-10-01T10:00:00Z",
		[]analyze.Issue{compareIssue("missing-title", "https://example.com/a")},
		"https://example.com/a", "https://example.com/b")
	current := compareFixture("2026-10-08T10:00:00Z",
		[]analyze.Issue{compareIssue("missing-description", "https://example.com/a")},
		"https://example.com/a", "https://example.com/c")
	var err error
	if baseID, err = srv.store.Save(base); err != nil {
		t.Fatalf("save base: %v", err)
	}
	if currentID, err = srv.store.Save(current); err != nil {
		t.Fatalf("save current: %v", err)
	}
	return baseID, currentID
}

func compareURL(base, current string) string {
	q := url.Values{}
	if base != "" {
		q.Set("base", base)
	}
	if current != "" {
		q.Set("current", current)
	}
	return "/api/compare?" + q.Encode()
}

func assertCompareCodes(t *testing.T, label string, list any, want ...string) {
	t.Helper()
	items, _ := list.([]any)
	got := []string{}
	for _, it := range items {
		code, _ := it.(map[string]any)["code"].(string)
		got = append(got, code)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s codes = %v, want %v", label, got, want)
	}
}

func TestCompareDiffsTwoStoredCrawls(t *testing.T) {
	srv := newTestServer(t)
	baseID, currentID := saveComparePair(t, srv)

	// The current side can be an ID, a bare host, or "latest", as on the CLI.
	for _, current := range []string{currentID, "example.com", "latest"} {
		rec, out := doJSON(t, srv, http.MethodGet, compareURL(baseID, current), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("current=%q: status = %d, body = %s", current, rec.Code, rec.Body)
		}
		issues, _ := out["issues"].(map[string]any)
		assertCompareCodes(t, "new", issues["new"], "missing-description")
		assertCompareCodes(t, "resolved", issues["resolved"], "missing-title")
		pages, _ := out["pages"].(map[string]any)
		if got := fmt.Sprint(pages["added"]); got != "[https://example.com/c]" {
			t.Errorf("added = %s", got)
		}
		if got := fmt.Sprint(pages["removed"]); got != "[https://example.com/b]" {
			t.Errorf("removed = %s", got)
		}
		summary, _ := out["summary"].(map[string]any)
		if _, ok := summary["new_by_severity"].(map[string]any); !ok {
			t.Errorf("summary.new_by_severity missing: %v", summary)
		}
	}
}

func TestCompareRequiresBothRefs(t *testing.T) {
	srv := newTestServer(t)
	baseID, _ := saveComparePair(t, srv)
	for _, tc := range [][2]string{{"", ""}, {baseID, ""}, {"", baseID}} {
		rec, out := doJSON(t, srv, http.MethodGet, compareURL(tc[0], tc[1]), nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("base=%q current=%q: status = %d, want 400", tc[0], tc[1], rec.Code)
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Errorf("base=%q current=%q: no error message", tc[0], tc[1])
		}
	}
}

func TestCompareUnknownRefIs404(t *testing.T) {
	srv := newTestServer(t)
	_, currentID := saveComparePair(t, srv)
	for _, ref := range []string{"nope.example", "example.com/20990101T000000Z", "0123456789abcdef"} {
		rec, _ := doJSON(t, srv, http.MethodGet, compareURL(ref, currentID), nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("base=%q: status = %d, want 404", ref, rec.Code)
		}
	}
}

func TestCompareNeverReadsFilePaths(t *testing.T) {
	srv := newTestServer(t)
	_, currentID := saveComparePair(t, srv)
	outside := filepath.Join(t.TempDir(), "report.json")
	data, err := json.Marshal(compareFixture("2026-01-01T00:00:00Z", nil, "https://example.com/x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, data, 0o600); err != nil {
		t.Fatal(err)
	}
	rec, _ := doJSON(t, srv, http.MethodGet, compareURL(outside, currentID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("file-path ref: status = %d, want 404", rec.Code)
	}
}

func TestCompareUsesCrawlJobsFromThisProcess(t *testing.T) {
	srv := newTestServer(t)
	baseID, _ := saveComparePair(t, srv)

	job, ok := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, DefaultMaxRunning)
	if !ok {
		t.Fatal("could not start a fake crawl job")
	}
	rec, _ := doJSON(t, srv, http.MethodGet, compareURL(baseID, job.ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("running job: status = %d, want 409", rec.Code)
	}

	srv.crawls.finish(job.ID, nil, func(d *crawlJob) {
		d.Report = compareFixture("2026-10-09T10:00:00Z", nil, "https://example.com/a")
	})
	rec, out := doJSON(t, srv, http.MethodGet, compareURL(baseID, job.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("finished job: status = %d, body = %s", rec.Code, rec.Body)
	}
	issues, _ := out["issues"].(map[string]any)
	assertCompareCodes(t, "resolved", issues["resolved"], "missing-title")

	failed, ok := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, DefaultMaxRunning)
	if !ok {
		t.Fatal("could not start a second fake crawl job")
	}
	srv.crawls.finish(failed.ID, errors.New("boom"), nil)
	rec, _ = doJSON(t, srv, http.MethodGet, compareURL(baseID, failed.ID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("failed job: status = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/webserver -run TestCompare`
  Expected: FAIL. With no route, `GET /api/compare` falls through to the frontend handler. The
  tests fail with `decode response body "<!doctype html>…"` or with `status = 200, want 400`.

- [ ] **Step 3: Implement.** Replace `internal/webserver/compare.go` with:

```go
package webserver

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Patience-dot-devl/gocrawl/internal/diff"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// errCompareUnfinished marks a compare ref that names a crawl job still in progress, which
// has no report to compare yet.
var errCompareUnfinished = errors.New("crawl has not finished yet")

func (s *Server) compareRoutes() {
	s.mux.HandleFunc("GET /api/compare", s.handleCompare)
}

// handleCompare diffs two crawls, base being the earlier one, with the same diff.Compare the
// `gocrawl compare` command prints.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	baseRef, currentRef := q.Get("base"), q.Get("current")
	if baseRef == "" || currentRef == "" {
		writeError(w, http.StatusBadRequest, errors.New(`both "base" and "current" are required: a crawl ID, a host such as example.com, or "latest"`))
		return
	}
	base, err := s.resolveCompareRef(baseRef)
	if err != nil {
		writeCompareRefError(w, "base", err)
		return
	}
	current, err := s.resolveCompareRef(currentRef)
	if err != nil {
		writeCompareRefError(w, "current", err)
		return
	}
	writeJSON(w, http.StatusOK, diff.Compare(base, current))
}

// resolveCompareRef finds the report a compare ref names. A crawl-job ID from this process
// wins, because GET /api/crawls lists those jobs under their job ID. Otherwise the ref goes
// to the store: a crawl ID, a bare host, or "latest". File paths are deliberately not
// accepted (store.ResolveStored).
func (s *Server) resolveCompareRef(ref string) (*report.Report, error) {
	if job, ok := s.crawls.get(ref); ok {
		if job.Data.Report != nil {
			return job.Data.Report, nil
		}
		if job.Status == StatusRunning || job.Status == StatusPaused {
			return nil, fmt.Errorf("crawl %q: %w", ref, errCompareUnfinished)
		}
		return nil, fmt.Errorf("crawl %q has no report (status %s)", ref, job.Status)
	}
	rep, _, err := s.store.ResolveStored(ref)
	return rep, err
}

func writeCompareRefError(w http.ResponseWriter, side string, err error) {
	status := http.StatusNotFound
	if errors.Is(err, errCompareUnfinished) {
		status = http.StatusConflict
	}
	writeError(w, status, fmt.Errorf("%s: %w", side, err))
}
```

  Locking: `s.crawls.get` returns a snapshot taken under the job mutex. The `*report.Report`
  it holds is never mutated after `finish`, so reading it outside the lock is race-free.

- [ ] **Step 4: Run.** `go test -race ./internal/webserver -run TestCompare` Expected: `ok`.

- [ ] **Step 5: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/webserver/compare.go internal/webserver/compare_test.go
git commit -m "feat(webserver): add GET /api/compare over stored crawls and live jobs" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W1-CMP-3: `POST /api/reports/import`

**Files:**
- Modify: `internal/webserver/import.go` (replaces the wave-0 stub)
- Test: `internal/webserver/import_test.go` (new)

**Interfaces:**
- Consumes: `parseMultipart(w, r, maxBytes int64) bool`, `(*store.Store).Save(*report.Report) (string, error)`
- Produces: `func (s *Server) importRoutes()` registers `POST /api/reports/import`. The route takes
  multipart field `file` and returns `201 {"id": "<host>/<timestamp>"}`. Errors: `400` for a
  missing file, bad JSON, no seed, or a non-http(s) seed; `413` over the limit; `415` for a
  request that is not multipart; `403` cross-origin (from the guard). Unexported:
  `var maxImportBytes int64 = 64 << 20` (a var, so tests can lower it),
  `validateImportedSeed(seed string) error`, `fillImportedDefaults(rep *report.Report)`.

- [ ] **Step 1: Write the failing test.** Create `internal/webserver/import_test.go`:

```go
package webserver

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const importFixture = `{"seed":"https://example.com/","started_at":"2026-10-13T10:00:00Z",` +
	`"finished_at":"2026-10-13T10:01:00Z","pages_crawled":2,` +
	`"summary":{"by_severity":{"error":1},"by_analyzer":{"seo":1},"pages_by_status":{"200":2}},` +
	`"issues":[{"analyzer":"seo","url":"https://example.com/","severity":"error","code":"missing-title","message":"Page has no title"}]}`

// postImportFile sends body as the multipart "file" part of POST /api/reports/import through
// the full handler, guard included. An empty fileName sends a form with no file part.
func postImportFile(t *testing.T, srv *Server, fileName string, body []byte, origin string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if fileName == "" {
		if err := mw.WriteField("note", "no file here"); err != nil {
			t.Fatal(err)
		}
	} else {
		fw, err := mw.CreateFormFile("file", fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reports/import", &buf)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response %q: %v", rec.Body.String(), err)
		}
	}
	return rec, out
}

func storedCount(t *testing.T, srv *Server) int {
	t.Helper()
	entries, err := srv.store.List("")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestImportReportSavesIt(t *testing.T) {
	srv := newTestServer(t)
	rec, out := postImportFile(t, srv, "report.json", []byte(importFixture), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	id, _ := out["id"].(string)
	if id != "example.com/20261013T100000Z" {
		t.Fatalf("id = %q", id)
	}

	rec, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+url.PathEscape(id), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET imported crawl: status = %d", rec.Code)
	}
	rep, _ := job["report"].(map[string]any)
	if rep["seed"] != "https://example.com/" {
		t.Fatalf("imported report seed = %v", rep["seed"])
	}

	_, list := doJSON(t, srv, http.MethodGet, "/api/crawls", nil)
	crawls, _ := list["crawls"].([]any)
	found := false
	for _, c := range crawls {
		if c.(map[string]any)["id"] == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("imported crawl %q missing from GET /api/crawls: %v", id, crawls)
	}
}

func TestImportReportRejectsInvalidFiles(t *testing.T) {
	srv := newTestServer(t)
	for name, body := range map[string]string{
		"not JSON":          "this is not json",
		"JSON array":        "[1,2]",
		"no seed":           `{"pages_crawled":3}`,
		"null":              "null",
		"non-http seed":     `{"seed":"file:///etc/passwd"}`,
		"seed with no host": `{"seed":"https://"}`,
	} {
		rec, out := postImportFile(t, srv, "report.json", []byte(body), "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Errorf("%s: no error message", name)
		}
	}
	if n := storedCount(t, srv); n != 0 {
		t.Fatalf("%d report(s) saved from invalid uploads", n)
	}
}

func TestImportReportRequiresFileField(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := postImportFile(t, srv, "", nil, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestImportReportRejectsOversizedUpload(t *testing.T) {
	prev := maxImportBytes
	maxImportBytes = 1024
	t.Cleanup(func() { maxImportBytes = prev })

	srv := newTestServer(t)
	big := importFixture + strings.Repeat(" ", 4096)
	rec, _ := postImportFile(t, srv, "report.json", []byte(big), "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if n := storedCount(t, srv); n != 0 {
		t.Fatalf("%d report(s) saved from an oversized upload", n)
	}
}

func TestImportReportRequiresMultipart(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/reports/import", map[string]any{"seed": "https://example.com/"})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestImportReportRejectsCrossOrigin(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := postImportFile(t, srv, "report.json", []byte(importFixture), "http://evil.example")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if n := storedCount(t, srv); n != 0 {
		t.Fatalf("%d report(s) saved despite the cross-origin rejection", n)
	}
}

func TestImportReportFillsEmptyCollections(t *testing.T) {
	srv := newTestServer(t)
	rec, out := postImportFile(t, srv, "minimal.json", []byte(`{"seed":"https://example.com/","started_at":"2026-10-13T10:00:00Z"}`), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	_, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+url.PathEscape(out["id"].(string)), nil)
	rep, _ := job["report"].(map[string]any)
	summary, _ := rep["summary"].(map[string]any)
	for _, key := range []string{"by_severity", "by_analyzer", "pages_by_status"} {
		if _, ok := summary[key].(map[string]any); !ok {
			t.Errorf("summary.%s = %v, want an object (the UI reads it without null checks)", key, summary[key])
		}
	}
	if issues, ok := rep["issues"].([]any); !ok || len(issues) != 0 {
		t.Errorf("issues = %v, want []", rep["issues"])
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/webserver -run TestImportReport`
  Expected: build failure, `undefined: maxImportBytes`.

- [ ] **Step 3: Implement.** Replace `internal/webserver/import.go` with:

```go
package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// maxImportBytes caps the POST /api/reports/import request body. A report for a large site
// (every issue plus the site map) runs to tens of megabytes, so the cap is generous but
// bounded. It is a var so tests can lower it.
var maxImportBytes int64 = 64 << 20

func (s *Server) importRoutes() {
	s.mux.HandleFunc("POST /api/reports/import", s.handleImportReport)
}

// handleImportReport saves an uploaded gocrawl JSON report to the store. This is the web
// equivalent of `gocrawl render`. The imported crawl then appears in the Crawls list and
// works with export and compare.
func (s *Server) handleImportReport(w http.ResponseWriter, r *http.Request) {
	if !parseMultipart(w, r, maxImportBytes) {
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New(`missing report file: send it as the multipart field "file"`))
		return
	}
	defer func() { _ = file.Close() }()

	var rep report.Report
	if err := json.NewDecoder(file).Decode(&rep); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("not a gocrawl JSON report: %w", err))
		return
	}
	if err := validateImportedSeed(rep.Seed); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fillImportedDefaults(&rep)

	id, err := s.store.Save(&rep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("saving the imported report: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// validateImportedSeed requires an http(s) seed URL with a host. The store files a report
// under its seed's host, and every report gocrawl writes has such a seed.
func validateImportedSeed(seed string) error {
	if strings.TrimSpace(seed) == "" {
		return errors.New("the file has no seed URL; only JSON reports written by gocrawl can be imported")
	}
	u, err := url.Parse(seed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("the report's seed %q is not an http(s) URL", seed)
	}
	return nil
}

// fillImportedDefaults gives an imported report the non-nil collections that a report built by
// report.Build always has. The UI can then read the summary maps and the issue list without
// null checks.
func fillImportedDefaults(rep *report.Report) {
	if rep.Issues == nil {
		rep.Issues = []analyze.Issue{}
	}
	if rep.Summary.BySeverity == nil {
		rep.Summary.BySeverity = map[string]int{}
	}
	if rep.Summary.ByAnalyzer == nil {
		rep.Summary.ByAnalyzer = map[string]int{}
	}
	if rep.Summary.ByStatus == nil {
		rep.Summary.ByStatus = map[string]int{}
	}
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/webserver -run TestImportReport` Expected: `ok`.

- [ ] **Step 5: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/webserver/import.go internal/webserver/import_test.go
git commit -m "feat(webserver): add POST /api/reports/import to save an uploaded JSON report" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W1-CMP-4: docs and changelog for compare and import

**Files:**
- Modify: `docs/web.md`, `CHANGELOG.md`

**Interfaces:** none (documentation only).

- [ ] **Step 1: Update `docs/web.md`.** Set `updated: 2026-10-13` in the frontmatter. Add these
  two rows to the API table, after the `/api/crawls/{id}/export` row:

```markdown
| `GET /api/compare?base=<ref>&current=<ref>` | Diff two crawls, `base` being the earlier one. The response is the same JSON as `gocrawl compare --format json`: new, resolved and persisting issues, summary deltas, and added and removed pages. A ref is a crawl ID from `GET /api/crawls` (a live job or a saved crawl), a host such as `example.com` (its newest saved crawl), or `latest`. File paths are not accepted. `400` when a ref is missing, `404` when one does not resolve, `409` when one names a crawl that is still running. |
| `POST /api/reports/import` | Save a gocrawl JSON report to the store, the web equivalent of `gocrawl render`. Multipart form with the report in the field `file`, up to 64 MB. Returns `201` with `{"id": "<host>/<timestamp>"}`; the crawl then appears in the crawl list and works with export and compare. `400` when the file is not a report or its `seed` is not an http(s) URL, `413` when it is too large, `415` when the request is not `multipart/form-data`. An import with the same host and start time as a saved crawl replaces it. |
```

  In the "Exposure" section, change the sentence "and `POST /api/crawls` insists on
  `Content-Type: application/json` (`415` otherwise)" to: "and `POST /api/crawls` insists on
  `Content-Type: application/json` and upload routes on `multipart/form-data` (`415`
  otherwise)".

- [ ] **Step 2: Add the changelog line.** Under `## [Unreleased]`, `### Added` in `CHANGELOG.md`:

```markdown
- Web API: `GET /api/compare` diffs two crawls (crawl IDs, a host, or `latest`, as
  `gocrawl compare` takes them), and `POST /api/reports/import` saves an uploaded JSON report
  to the crawl store.
```

- [ ] **Step 3: Regenerate the docs metadata.** `make docs` Expected: exits 0 and updates the
  `docs/README.md` index (the `updated` date).

- [ ] **Step 4: Run the checker.** `make check` Expected: exits 0, including the docs-drift tests.

- [ ] **Step 5: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -m "docs: document the compare and report-import endpoints" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Track: redirects backend

#### Task W1-RED-1: `OnRow`, JSON tags, and a shared fetcher constructor in `redirectcheck`

**Files:**
- Create: `internal/redirectcheck/fetcher.go`
- Modify: `internal/redirectcheck/types.go`, `internal/redirectcheck/parse.go`, `internal/redirectcheck/run.go`, `cmd/gocrawl/checkredirects.go`
- Test: `internal/redirectcheck/run_test.go` (append), `internal/redirectcheck/json_test.go` (new), `internal/redirectcheck/fetcher_test.go` (new)

**Interfaces:**
- Produces:
  - `RunOptions.OnRow func(done, total int)`. It is called once per rule, including skipped
    rows. Calls are serialized under a mutex in `Run`, and `done` goes 1, 2, …, `total`.
  - Snake-case JSON tags on `RowResult` (`scope, source_status, source_final_url,
    source_matches_target, target_status, original_in_sitemap, target_in_sitemap, verdict,
    notes`; `notes` has `omitempty`) and on `Rule` (`original, target, redirect_type,
    redirect_style, priority, match_query_strings, ignore_trailing_slash, ignore_protocol,
    disable_if_page_exists, note`).
  - `const DefaultTimeout = 15 * time.Second`, `const DefaultUserAgent = "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)"`,
    `func NewFetcher(timeout time.Duration, userAgent string) *crawler.HTTPFetcher`.
- Consumes: `crawler.NewHTTPFetcher(crawler.Options)`.

How `Run` runs rows concurrently: a goroutine per in-scope rule writes `results[i]`, bounded by
a semaphore. Skipped rows are written on the calling goroutine. `OnRow` must be safe from both
places. A `sync.Mutex` held around the `done++` and the callback gives one ordered sequence.
The callback never calls back into `Run`, so holding the lock while it runs cannot deadlock.

- [ ] **Step 1: Write the failing tests.**

  Append to `internal/redirectcheck/run_test.go` (and add `"sync"` to its imports):

```go
func TestRunReportsEveryRowThroughOnRow(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
		"https://example.com/old":         {StatusCode: 200, FinalURL: "https://example.com/a", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/a", Status: 301}}},
		"https://example.com/a":           {StatusCode: 200, FinalURL: "https://example.com/a"},
	}}
	inScope := redirectcheck.Rule{Original: "/old", Target: "/a", IgnoreTrailingSlash: "TRUE", IgnoreProtocol: "TRUE"}
	rules := []redirectcheck.Rule{
		inScope, inScope, inScope, inScope,
		{Original: "/gone", Target: "https://other-site.com/x"},   // external: skipped
		{Original: "/blog/(?P<slug>.*)", Target: "/news/{slug}"}, // dynamic: skipped
	}

	var mu sync.Mutex
	var seen []int
	totals := map[int]bool{}
	_, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{
		Domain: "example.com", Fetcher: ff, Concurrency: 3,
		OnRow: func(done, total int) {
			mu.Lock()
			defer mu.Unlock()
			seen = append(seen, done)
			totals[total] = true
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != len(rules) {
		t.Fatalf("OnRow called %d times, want %d", len(seen), len(rules))
	}
	for i, d := range seen {
		if d != i+1 {
			t.Fatalf("OnRow done sequence = %v, want 1..%d in order", seen, len(rules))
		}
	}
	if len(totals) != 1 || !totals[len(rules)] {
		t.Fatalf("OnRow totals = %v, want only %d", totals, len(rules))
	}
}

func TestRunWithoutOnRowStillWorks(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
	}}
	rules := []redirectcheck.Rule{{Original: "/gone", Target: "https://other-site.com/x"}}
	if _, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{Domain: "example.com", Fetcher: ff}); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
```

  Create `internal/redirectcheck/json_test.go`:

```go
package redirectcheck_test

import (
	"encoding/json"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestRowResultJSONUsesSnakeCase(t *testing.T) {
	b, err := json.Marshal(redirectcheck.RowResult{
		Scope: redirectcheck.ScopeInScope, SourceStatus: 301, SourceFinalURL: "https://example.com/a",
		SourceMatchesTarget: true, TargetStatus: 200, OriginalInSitemap: true, TargetInSitemap: true,
		Verdict: redirectcheck.VerdictOK, Notes: []string{"n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"scope":"in-scope","source_status":301,"source_final_url":"https://example.com/a",` +
		`"source_matches_target":true,"target_status":200,"original_in_sitemap":true,` +
		`"target_in_sitemap":true,"verdict":"ok","notes":["n"]}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}

	b, err = json.Marshal(redirectcheck.RowResult{Verdict: redirectcheck.VerdictSkippedExternal, Scope: redirectcheck.ScopeExternal})
	if err != nil {
		t.Fatal(err)
	}
	want = `{"scope":"external","source_status":0,"source_final_url":"","source_matches_target":false,` +
		`"target_status":0,"original_in_sitemap":false,"target_in_sitemap":false,"verdict":"skipped-external"}`
	if string(b) != want {
		t.Fatalf("notes must be omitted when empty\ngot  %s\nwant %s", b, want)
	}
}

func TestRuleJSONUsesSnakeCase(t *testing.T) {
	b, err := json.Marshal(redirectcheck.Rule{
		Original: "/old", Target: "/new", RedirectType: "301", RedirectStyle: "STANDARD", Priority: "1",
		MatchQueryStrings: "FALSE", IgnoreTrailingSlash: "TRUE", IgnoreProtocol: "TRUE",
		DisableIfPageExists: "FALSE", Note: "moved",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"original":"/old","target":"/new","redirect_type":"301","redirect_style":"STANDARD",` +
		`"priority":"1","match_query_strings":"FALSE","ignore_trailing_slash":"TRUE",` +
		`"ignore_protocol":"TRUE","disable_if_page_exists":"FALSE","note":"moved"}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
}
```

  Create `internal/redirectcheck/fetcher_test.go`:

```go
package redirectcheck_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestNewFetcherSendsUserAgentAndRecordsRedirects(t *testing.T) {
	var mu sync.Mutex
	var agents []string
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents = append(agents, r.UserAgent())
		mu.Unlock()
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents = append(agents, r.UserAgent())
		mu.Unlock()
		fmt.Fprint(w, "ok")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	f := redirectcheck.NewFetcher(5*time.Second, "redirect-test/1.0")
	page, err := f.Fetch(context.Background(), ts.URL+"/old")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if page.StatusCode != http.StatusOK || page.FinalURL != ts.URL+"/new" || len(page.Redirects) != 1 {
		t.Fatalf("page = status %d, final %q, %d redirect(s)", page.StatusCode, page.FinalURL, len(page.Redirects))
	}
	mu.Lock()
	defer mu.Unlock()
	for _, ua := range agents {
		if ua != "redirect-test/1.0" {
			t.Fatalf("User-Agent = %q, want redirect-test/1.0", ua)
		}
	}
	if redirectcheck.DefaultTimeout != 15*time.Second || redirectcheck.DefaultUserAgent == "" {
		t.Fatalf("defaults changed: %v %q", redirectcheck.DefaultTimeout, redirectcheck.DefaultUserAgent)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/redirectcheck`
  Expected: build failure, `unknown field OnRow in struct literal of type redirectcheck.RunOptions`
  and `undefined: redirectcheck.NewFetcher`.

- [ ] **Step 3: Implement.**

  In `internal/redirectcheck/types.go`, replace the `RowResult` struct with:

```go
// RowResult is what checking one rule against the live site produced. The JSON names are what
// the web API returns for each row.
type RowResult struct {
	Scope               Scope    `json:"scope"`
	SourceStatus        int      `json:"source_status"`
	SourceFinalURL      string   `json:"source_final_url"`
	SourceMatchesTarget bool     `json:"source_matches_target"`
	TargetStatus        int      `json:"target_status"`
	OriginalInSitemap   bool     `json:"original_in_sitemap"`
	TargetInSitemap     bool     `json:"target_in_sitemap"`
	Verdict             Verdict  `json:"verdict"`
	Notes               []string `json:"notes,omitempty"`
}
```

  In `internal/redirectcheck/parse.go`, replace the `Rule` struct with:

```go
// Rule is one row of the redirect-rule CSV. Columns are kept as raw strings so they can be
// echoed back verbatim in the output report.
type Rule struct {
	Original            string `json:"original"`
	Target              string `json:"target"`
	RedirectType        string `json:"redirect_type"`
	RedirectStyle       string `json:"redirect_style"`
	Priority            string `json:"priority"`
	MatchQueryStrings   string `json:"match_query_strings"`
	IgnoreTrailingSlash string `json:"ignore_trailing_slash"`
	IgnoreProtocol      string `json:"ignore_protocol"`
	DisableIfPageExists string `json:"disable_if_page_exists"`
	Note                string `json:"note"`
}
```

  Create `internal/redirectcheck/fetcher.go`:

```go
package redirectcheck

import (
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// DefaultTimeout is the per-request timeout of a redirect check.
const DefaultTimeout = 15 * time.Second

// DefaultUserAgent is the User-Agent a redirect check sends unless told otherwise.
const DefaultUserAgent = "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)"

// NewFetcher builds the HTTP fetcher a redirect check runs on. It follows up to 10 redirects,
// recording each hop, and reads at most 5 MiB per body. It has no crawl scope or credentials:
// a redirect check fetches exactly the URLs in the rules and the sitemap. The CLI
// (`gocrawl check-redirects`) and the web API share it so the two behave the same.
func NewFetcher(timeout time.Duration, userAgent string) *crawler.HTTPFetcher {
	return crawler.NewHTTPFetcher(crawler.Options{
		Timeout:      timeout,
		UserAgent:    userAgent,
		MaxRedirects: 10,
		MaxBodyBytes: 5 << 20,
	})
}
```

  Replace `internal/redirectcheck/run.go` with:

```go
package redirectcheck

import (
	"context"
	"sync"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// RunOptions configures a check-redirects run.
type RunOptions struct {
	Domain        string
	SitemapURL    string
	Fetcher       crawler.Fetcher
	Concurrency   int
	RatePerSecond float64
	// AdaptiveDelay automatically slows requests (halving requests-per-second, honoring any
	// Retry-After header) when the site responds with HTTP 429/503, the same way the crawl
	// engine does. Without it, a fixed concurrency/rate that's too aggressive for the target
	// can trip bot-mitigation (e.g. Cloudflare) partway through a run.
	AdaptiveDelay bool
	// Stats, if non-nil, is filled in with adaptive-delay activity once Run returns.
	Stats *RunStats
	// OnRow, if non-nil, is called once for every rule as its row completes. The arguments are
	// the number of rows done so far and the total number of rules. Calls are serialized
	// (never concurrent), and done rises by one per call, ending at total. Rows skipped
	// without a fetch (external, dynamic pattern, unclassifiable) count too. OnRow is not
	// called when Run fails before checking rows (no sitemap). It runs on a worker goroutine
	// and holds up the other workers' progress reports, so keep it short.
	OnRow func(done, total int)
}

// RunStats reports adaptive-delay activity for a Run call.
type RunStats struct {
	ThrottleEvents int
	FinalRate      float64
}

// Run classifies every rule, fetches the site's sitemap once, then checks each in-scope rule
// concurrently (bounded by opts.Concurrency and opts.RatePerSecond). Every fetch — sitemap
// discovery included — goes through a shared AdaptiveLimiter, so a run that starts too
// aggressively for the target site backs itself off instead of hammering it into a
// bot-mitigation block. Results are returned in the same order as rules. Rows classified
// external or dynamic-pattern are not fetched.
func Run(ctx context.Context, rules []Rule, opts RunOptions) ([]RowResult, error) {
	limiter := crawler.NewAdaptiveLimiter(opts.RatePerSecond, opts.AdaptiveDelay)
	fetcher := &crawler.AdaptiveFetcher{Fetcher: opts.Fetcher, Limiter: limiter}

	sitemapURLs, err := DiscoverSitemap(ctx, fetcher, opts.Domain, opts.SitemapURL)
	if err != nil {
		return nil, err
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	// rowDone reports one finished row. Skipped rows finish on this goroutine, checked rows on
	// worker goroutines; the mutex makes the count and the callback one ordered sequence.
	var progressMu sync.Mutex
	done, total := 0, len(rules)
	rowDone := func() {
		if opts.OnRow == nil {
			return
		}
		progressMu.Lock()
		defer progressMu.Unlock()
		done++
		opts.OnRow(done, total)
	}

	results := make([]RowResult, len(rules))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, rule := range rules {
		scope, err := Classify(rule, opts.Domain)
		if err != nil {
			results[i] = RowResult{Verdict: VerdictError, Notes: []string{"could not classify rule: " + err.Error()}}
			rowDone()
			continue
		}
		if scope == ScopeExternal {
			results[i] = RowResult{Scope: ScopeExternal, Verdict: VerdictSkippedExternal}
			rowDone()
			continue
		}
		if scope == ScopeDynamic {
			results[i] = RowResult{Scope: ScopeDynamic, Verdict: VerdictSkippedDynamic}
			rowDone()
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(i int, rule Rule) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = CheckRule(ctx, fetcher, opts.Domain, rule, sitemapURLs)
			rowDone()
		}(i, rule)
	}
	wg.Wait()

	if opts.Stats != nil {
		opts.Stats.ThrottleEvents = limiter.ThrottleCount()
		opts.Stats.FinalRate = limiter.CurrentRate()
	}
	return results, nil
}
```

  In `cmd/gocrawl/checkredirects.go`:
  1. Change the two flag defaults:
     `f.Duration("timeout", redirectcheck.DefaultTimeout, "per-request timeout")` and
     `f.String("user-agent", redirectcheck.DefaultUserAgent, "User-Agent header")`.
  2. Replace the `fetcher := crawler.NewHTTPFetcher(crawler.Options{…})` block with
     `fetcher := redirectcheck.NewFetcher(timeout, userAgent)`.
  3. Remove the `"time"` and `".../internal/crawler"` imports, which are now unused.

- [ ] **Step 4: Run.** `go test -race ./internal/redirectcheck ./cmd/gocrawl -run 'Run|JSON|NewFetcher|CheckRedirects'`
  Expected: `ok` for both packages.

- [ ] **Step 5: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/redirectcheck/types.go internal/redirectcheck/parse.go internal/redirectcheck/run.go internal/redirectcheck/fetcher.go internal/redirectcheck/run_test.go internal/redirectcheck/json_test.go internal/redirectcheck/fetcher_test.go cmd/gocrawl/checkredirects.go
git commit -m "feat(redirectcheck): report row progress, add JSON tags and a shared fetcher" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W1-RED-2: redirect-check jobs: start, get, list

**Files:**
- Modify: `internal/webserver/redirects.go` (replaces the wave-0 placeholder)
- Test: `internal/webserver/redirects_test.go` (new)

**Interfaces:**
- Consumes: `jobManager[redirectJob]` (`start`, `get`, `list`, `with`, `finish`), `parseMultipart`,
  `redirectcheck.ParseCSV`, `redirectcheck.Run`, `redirectcheck.NewFetcher`, `formatTime`.
- Produces:
  - `type redirectJob struct { Domain string; FileName string; Total, Done int; Rules []redirectcheck.Rule; Rows []redirectcheck.RowResult }`.
    This is the contract shape plus `FileName`.
  - `POST /api/redirect-checks`. Multipart: `file` (CSV, at most 10 MB), `domain` (required; a
    bare host, or a URL whose host is taken), `sitemap_url` (optional http(s) URL),
    `concurrency` (1..16, default 4), `rate` (≥ 0, default 0 = unlimited), `adaptive_delay`
    (bool, default true). Returns `202` with the job view. `400` for invalid input or a bad
    CSV header, `413`, `415`, `429` when 2 checks are already running, `403` cross-origin.
  - `GET /api/redirect-checks/{id}` returns the job view. `rows` appears only when status is
    `done`. `404` for an unknown id.
  - `GET /api/redirect-checks` returns `{"checks": [view without rows]}`, newest first. This
    route is a **contract addition** that feeds the Redirects view's session list.
  - The view is JSON `{id, domain, file_name?, status, error?, started_at?, finished_at?, done, total, rows?}`.
    Each row is a `Rule` and a `RowResult` flattened into one object.
  - Unexported: `maxRunningRedirectChecks = 2`, `maxRedirectConcurrency = 16`,
    `var maxRedirectCSVBytes int64 = 10 << 20`, `var newRedirectFetcher func() crawler.Fetcher`,
    `normalizeRedirectDomain(raw string) (string, error)`.

Locking: `OnRow` writes `Data.Done` only through `s.redirects.with`, under the job mutex.
`Data.Rows` is set once, inside `finish`'s `apply`, under the same mutex. `get` and `list`
return snapshots copied under the lock. `Rows` and `Rules` are never written again after
`finish`, so views read the snapshot's slices outside the lock without a race. The job's
`context.CancelFunc` is stored by `start`. The run goroutine also calls it from `defer`, so the
context is always released.

Cancel during sitemap discovery: `redirectcheck.Run` then fails with a "could not fetch a
sitemap" error. The goroutine clears `err` whenever `ctx.Err() != nil`, so `finish` records
`canceled` and not `error`. W1-RED-3 tests this.

- [ ] **Step 1: Write the failing tests.** Create `internal/webserver/redirects_test.go`:

```go
package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

// redirectTestOrigin is the domain the tests check. hostRewriteFetcher sends its requests to
// an httptest site, because redirectcheck always builds https://<domain>/... URLs and an
// httptest URL has a port that the domain scope rules would reject.
const redirectTestOrigin = "https://example.com"

const redirectCSVHeader = `"Original URL","Redirect to","Redirect type","Redirect style","Priority",` +
	`"Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"` + "\n"

const redirectCSV = redirectCSVHeader +
	"/old,/a,301,STANDARD,1,FALSE,TRUE,TRUE,FALSE,moved\n" +
	"/gone,https://other.example/x,301,STANDARD,2,FALSE,TRUE,TRUE,FALSE,\n"

const redirectTestSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
</urlset>`

// redirectTestSite serves a small site: a sitemap listing /a, a 301 from /old to /a, and /a.
// A non-nil gate holds every request until the test ends or the request is canceled, which
// keeps a check "running" for the cap, cancel and export tests. Tests must not close the
// gate themselves.
func redirectTestSite(t *testing.T, gate chan struct{}) *httptest.Server {
	t.Helper()
	wait := func(r *http.Request) {
		if gate == nil {
			return
		}
		select {
		case <-gate:
		case <-r.Context().Done():
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		wait(r)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, redirectTestSitemap)
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		wait(r)
		http.Redirect(w, r, "/a", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		wait(r)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>A</title></head><body>a</body></html>")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	if gate != nil {
		// Cleanups run last-in first-out: release blocked handlers before ts.Close waits on them.
		t.Cleanup(func() { close(gate) })
	}
	return ts
}

// hostRewriteFetcher sends requests for redirectTestOrigin to the httptest site through the
// real redirectcheck fetcher and rewrites the URLs on the way back.
type hostRewriteFetcher struct {
	base  string
	inner crawler.Fetcher
}

func (f hostRewriteFetcher) Fetch(ctx context.Context, rawURL string) (*crawler.Page, error) {
	page, err := f.inner.Fetch(ctx, strings.Replace(rawURL, redirectTestOrigin, f.base, 1))
	if page != nil {
		back := func(s string) string { return strings.Replace(s, f.base, redirectTestOrigin, 1) }
		page.RequestedURL = back(page.RequestedURL)
		page.FinalURL = back(page.FinalURL)
		for i := range page.Redirects {
			page.Redirects[i].From = back(page.Redirects[i].From)
			page.Redirects[i].To = back(page.Redirects[i].To)
		}
	}
	return page, err
}

// useRedirectTestSite points new redirect-check jobs at site until the test ends. The handler
// reads newRedirectFetcher synchronously, so swapping it here never races a job goroutine.
func useRedirectTestSite(t *testing.T, site *httptest.Server) {
	t.Helper()
	prev := newRedirectFetcher
	newRedirectFetcher = func() crawler.Fetcher {
		return hostRewriteFetcher{base: site.URL, inner: redirectcheck.NewFetcher(5*time.Second, "gocrawl-test")}
	}
	t.Cleanup(func() { newRedirectFetcher = prev })
}

// postRedirectCheckForm sends POST /api/redirect-checks as multipart through the full handler.
// An empty csvName sends no file part.
func postRedirectCheckForm(t *testing.T, srv *Server, fields map[string]string, csvName, csv, origin string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if csvName != "" {
		fw, err := mw.CreateFormFile("file", csvName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(csv)); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/redirect-checks", &buf)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response %q: %v", rec.Body.String(), err)
		}
	}
	return rec, out
}

// pollRedirectCheck polls GET /api/redirect-checks/{id} until the status is not "running".
func pollRedirectCheck(t *testing.T, srv *Server, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rec, out := doJSON(t, srv, http.MethodGet, "/api/redirect-checks/"+id, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("poll: status = %d, body = %s", rec.Code, rec.Body)
		}
		if out["status"] != "running" {
			return out
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("redirect check %s did not finish within the test deadline", id)
	return nil
}

func startRedirectCheckForTest(t *testing.T, srv *Server, fields map[string]string) string {
	t.Helper()
	rec, out := postRedirectCheckForm(t, srv, fields, "redirects.csv", redirectCSV, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start: status = %d, body = %s", rec.Code, rec.Body)
	}
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("start: no id in %v", out)
	}
	return id
}

func TestRedirectCheckRunsToCompletion(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)

	rec, started := postRedirectCheckForm(t, srv, map[string]string{
		"domain": "https://Example.com/", "concurrency": "2", "rate": "0", "adaptive_delay": "false",
	}, "redirects.csv", redirectCSV, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if started["status"] != "running" || started["total"] != float64(2) || started["done"] != float64(0) {
		t.Fatalf("start view = %v", started)
	}
	if started["domain"] != "example.com" || started["file_name"] != "redirects.csv" {
		t.Fatalf("start view domain/file = %v / %v", started["domain"], started["file_name"])
	}
	if _, ok := started["rows"]; ok {
		t.Fatalf("a running check must not carry rows: %v", started)
	}

	job := pollRedirectCheck(t, srv, started["id"].(string))
	if job["status"] != "done" {
		t.Fatalf("status = %v, error = %v", job["status"], job["error"])
	}
	if job["done"] != float64(2) || job["total"] != float64(2) {
		t.Fatalf("done/total = %v/%v, want 2/2", job["done"], job["total"])
	}
	rows, _ := job["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", job["rows"])
	}
	first := rows[0].(map[string]any)
	if first["original"] != "/old" || first["note"] != "moved" || first["verdict"] != "ok" ||
		first["source_final_url"] != "https://example.com/a" || first["target_status"] != float64(200) ||
		first["target_in_sitemap"] != true || first["scope"] != "in-scope" {
		t.Errorf("row 0 = %v", first)
	}
	second := rows[1].(map[string]any)
	if second["verdict"] != "skipped-external" || second["scope"] != "external" {
		t.Errorf("row 1 = %v", second)
	}
}

func TestListRedirectChecks(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)

	rec, out := doJSON(t, srv, http.MethodGet, "/api/redirect-checks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty list: status = %d", rec.Code)
	}
	if checks, ok := out["checks"].([]any); !ok || len(checks) != 0 {
		t.Fatalf("empty list = %v, want checks: []", out)
	}

	id := startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	pollRedirectCheck(t, srv, id)
	_, out = doJSON(t, srv, http.MethodGet, "/api/redirect-checks", nil)
	checks, _ := out["checks"].([]any)
	if len(checks) != 1 {
		t.Fatalf("checks = %v", out["checks"])
	}
	c := checks[0].(map[string]any)
	if c["id"] != id || c["status"] != "done" || c["domain"] != "example.com" {
		t.Fatalf("list item = %v", c)
	}
	if _, ok := c["rows"]; ok {
		t.Fatal("list items must not carry rows")
	}
}

func TestRedirectCheckWithoutSitemapEndsInError(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)
	id := startRedirectCheckForTest(t, srv, map[string]string{
		"domain": "example.com", "sitemap_url": "https://example.com/missing.xml",
	})
	job := pollRedirectCheck(t, srv, id)
	if job["status"] != "error" {
		t.Fatalf("status = %v, want error", job["status"])
	}
	if msg, _ := job["error"].(string); !strings.Contains(msg, "sitemap") {
		t.Fatalf("error = %q, want it to explain the sitemap was not found", msg)
	}
	if _, ok := job["rows"]; ok {
		t.Fatal("a failed check must not carry rows")
	}
}

func TestStartRedirectCheckRejectsBadInput(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)
	badHeader := "\"Original URL\",\"Redirect to\"\n/a,/b\n"
	cases := []struct {
		name    string
		fields  map[string]string
		csvName string
		csv     string
	}{
		{"missing domain", map[string]string{}, "r.csv", redirectCSV},
		{"domain with a path", map[string]string{"domain": "example.com/blog"}, "r.csv", redirectCSV},
		{"concurrency zero", map[string]string{"domain": "example.com", "concurrency": "0"}, "r.csv", redirectCSV},
		{"concurrency too high", map[string]string{"domain": "example.com", "concurrency": "99"}, "r.csv", redirectCSV},
		{"concurrency not a number", map[string]string{"domain": "example.com", "concurrency": "abc"}, "r.csv", redirectCSV},
		{"negative rate", map[string]string{"domain": "example.com", "rate": "-1"}, "r.csv", redirectCSV},
		{"rate NaN", map[string]string{"domain": "example.com", "rate": "NaN"}, "r.csv", redirectCSV},
		{"rate not a number", map[string]string{"domain": "example.com", "rate": "fast"}, "r.csv", redirectCSV},
		{"adaptive delay not a bool", map[string]string{"domain": "example.com", "adaptive_delay": "maybe"}, "r.csv", redirectCSV},
		{"sitemap url not http", map[string]string{"domain": "example.com", "sitemap_url": "ftp://example.com/s.xml"}, "r.csv", redirectCSV},
		{"missing file", map[string]string{"domain": "example.com"}, "", ""},
		{"wrong CSV header", map[string]string{"domain": "example.com"}, "r.csv", badHeader},
		{"header but no rules", map[string]string{"domain": "example.com"}, "r.csv", redirectCSVHeader},
	}
	for _, tc := range cases {
		rec, out := postRedirectCheckForm(t, srv, tc.fields, tc.csvName, tc.csv, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", tc.name, rec.Code, rec.Body)
			continue
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Errorf("%s: no error message", tc.name)
		}
	}
	if n := len(srv.redirects.list()); n != 0 {
		t.Fatalf("%d job(s) started from invalid requests", n)
	}
}

func TestNormalizeRedirectDomain(t *testing.T) {
	for in, want := range map[string]string{
		"example.com":                   "example.com",
		"  Example.COM ":                "example.com",
		"example.com/":                  "example.com",
		"https://example.com/":          "example.com",
		"http://shop.example.com/a?b=c": "shop.example.com",
	} {
		got, err := normalizeRedirectDomain(in)
		if err != nil || got != want {
			t.Errorf("normalizeRedirectDomain(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "   ", "example.com/blog", "https://", "exa mple.com"} {
		if got, err := normalizeRedirectDomain(in); err == nil {
			t.Errorf("normalizeRedirectDomain(%q) = %q, want an error", in, got)
		}
	}
}

func TestStartRedirectCheckGuards(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)

	rec, _ := postRedirectCheckForm(t, srv, map[string]string{"domain": "example.com"}, "r.csv", redirectCSV, "http://evil.example")
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status = %d, want 403", rec.Code)
	}
	rec, _ = doJSON(t, srv, http.MethodPost, "/api/redirect-checks", map[string]any{"domain": "example.com"})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("JSON body: status = %d, want 415", rec.Code)
	}

	prev := maxRedirectCSVBytes
	maxRedirectCSVBytes = 256
	t.Cleanup(func() { maxRedirectCSVBytes = prev })
	big := redirectCSV + strings.Repeat("/x,/a,301,STANDARD,1,FALSE,TRUE,TRUE,FALSE,\n", 100)
	rec, _ = postRedirectCheckForm(t, srv, map[string]string{"domain": "example.com"}, "r.csv", big, "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized: status = %d, want 413", rec.Code)
	}
	if n := len(srv.redirects.list()); n != 0 {
		t.Fatalf("%d job(s) started despite the guards", n)
	}
}

func TestRedirectCheckCapReturns429(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, make(chan struct{})))
	srv := newTestServer(t)
	for i := 0; i < maxRunningRedirectChecks; i++ {
		startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	}
	rec, out := postRedirectCheckForm(t, srv, map[string]string{"domain": "example.com"}, "r.csv", redirectCSV, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third check: status = %d, want 429", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "redirect checks already running") {
		t.Fatalf("error = %q", msg)
	}
}

func TestGetUnknownRedirectCheckIs404(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodGet, "/api/redirect-checks/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/webserver -run 'RedirectCheck|NormalizeRedirectDomain'`
  Expected: build failure, `undefined: newRedirectFetcher`, `undefined: normalizeRedirectDomain`,
  `undefined: maxRedirectCSVBytes`, `undefined: maxRunningRedirectChecks`.

- [ ] **Step 3: Implement.** Replace `internal/webserver/redirects.go` with:

```go
package webserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

const (
	// maxRunningRedirectChecks caps concurrent redirect-check jobs, separately from crawls.
	maxRunningRedirectChecks = 2
	// maxRedirectConcurrency bounds the concurrency field, so one request cannot open hundreds
	// of connections to the checked site.
	maxRedirectConcurrency     = 16
	defaultRedirectConcurrency = 4
)

// maxRedirectCSVBytes caps the POST /api/redirect-checks request body. It is a var so tests can
// lower it.
var maxRedirectCSVBytes int64 = 10 << 20

// newRedirectFetcher builds the fetcher a redirect-check job uses, the same one `gocrawl
// check-redirects` uses. Tests replace it to point jobs at an httptest site.
var newRedirectFetcher = func() crawler.Fetcher {
	return redirectcheck.NewFetcher(redirectcheck.DefaultTimeout, redirectcheck.DefaultUserAgent)
}

// redirectJob is the kind-specific data of a redirect-check job. Done is updated under the job
// lock while the check runs. Rows is set once, when the check finishes without an error, and is
// index-aligned with Rules.
type redirectJob struct {
	Domain   string
	FileName string
	Total    int
	Done     int
	Rules    []redirectcheck.Rule
	Rows     []redirectcheck.RowResult
}

// redirectRowView is one row of the API response: the CSV columns and the check result,
// flattened into one JSON object.
type redirectRowView struct {
	redirectcheck.Rule
	redirectcheck.RowResult
}

// redirectJobView is the API representation of a redirect-check job.
type redirectJobView struct {
	ID         string            `json:"id"`
	Domain     string            `json:"domain"`
	FileName   string            `json:"file_name,omitempty"`
	Status     string            `json:"status"`
	Err        string            `json:"error,omitempty"`
	StartedAt  string            `json:"started_at,omitempty"`
	FinishedAt string            `json:"finished_at,omitempty"`
	Done       int               `json:"done"`
	Total      int               `json:"total"`
	Rows       []redirectRowView `json:"rows,omitempty"`
}

// redirectJobViewOf builds the view from a snapshot. Rows are included only when asked for and
// only for a check that finished normally. A canceled check keeps none, because its unchecked
// rows would read as fetch failures.
func redirectJobViewOf(j Job[redirectJob], withRows bool) redirectJobView {
	v := redirectJobView{
		ID:         j.ID,
		Domain:     j.Data.Domain,
		FileName:   j.Data.FileName,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Done:       j.Data.Done,
		Total:      j.Data.Total,
	}
	if withRows && j.Status == StatusDone && len(j.Data.Rows) == len(j.Data.Rules) {
		v.Rows = make([]redirectRowView, len(j.Data.Rows))
		for i := range j.Data.Rows {
			v.Rows[i] = redirectRowView{Rule: j.Data.Rules[i], RowResult: j.Data.Rows[i]}
		}
	}
	return v
}

func (s *Server) redirectRoutes() {
	s.mux.HandleFunc("GET /api/redirect-checks", s.handleListRedirectChecks)
	s.mux.HandleFunc("POST /api/redirect-checks", s.handleStartRedirectCheck)
	s.mux.HandleFunc("GET /api/redirect-checks/{id}", s.handleGetRedirectCheck)
}

// redirectCheckParams are the validated non-file form fields of POST /api/redirect-checks.
type redirectCheckParams struct {
	domain      string
	sitemapURL  string
	concurrency int
	rate        float64
	adaptive    bool
}

func parseRedirectCheckForm(r *http.Request) (redirectCheckParams, error) {
	p := redirectCheckParams{concurrency: defaultRedirectConcurrency, adaptive: true}
	domain, err := normalizeRedirectDomain(r.FormValue("domain"))
	if err != nil {
		return p, err
	}
	p.domain = domain
	if raw := strings.TrimSpace(r.FormValue("sitemap_url")); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return p, fmt.Errorf("sitemap_url %q is not an http(s) URL", raw)
		}
		p.sitemapURL = raw
	}
	if raw := strings.TrimSpace(r.FormValue("concurrency")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxRedirectConcurrency {
			return p, fmt.Errorf("concurrency must be a whole number from 1 to %d", maxRedirectConcurrency)
		}
		p.concurrency = n
	}
	if raw := strings.TrimSpace(r.FormValue("rate")); raw != "" {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return p, errors.New("rate must be a number of requests per second, or 0 for unlimited")
		}
		p.rate = f
	}
	if raw := strings.TrimSpace(r.FormValue("adaptive_delay")); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return p, errors.New("adaptive_delay must be true or false")
		}
		p.adaptive = b
	}
	return p, nil
}

// normalizeRedirectDomain turns what a user types into the bare domain redirectcheck expects:
// it lowercases the input, takes the host when given a URL, and drops a trailing slash. A
// value with a path is rejected, because redirectcheck builds https://<domain>/... URLs.
func normalizeRedirectDomain(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	if d == "" {
		return "", errors.New("domain is required, for example example.com")
	}
	if strings.Contains(d, "://") {
		u, err := url.Parse(d)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("domain %q is not a host name; use the bare domain, for example example.com", raw)
		}
		d = u.Host
	}
	d = strings.TrimSuffix(d, "/")
	if d == "" || strings.ContainsAny(d, "/?# \t") {
		return "", fmt.Errorf("domain %q is not a host name; use the bare domain, for example example.com", raw)
	}
	return d, nil
}

func (s *Server) handleStartRedirectCheck(w http.ResponseWriter, r *http.Request) {
	if !parseMultipart(w, r, maxRedirectCSVBytes) {
		return
	}
	params, err := parseRedirectCheckForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New(`missing CSV file: send it as the multipart field "file"`))
		return
	}
	defer func() { _ = file.Close() }()
	// Parse up front, so a wrong export format is a 400 here and not a failed job later.
	rules, err := redirectcheck.ParseCSV(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parsing the CSV: %w", err))
		return
	}
	if len(rules) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("the CSV has a header but no redirect rules"))
		return
	}

	fetcher := newRedirectFetcher()
	ctx, cancel := context.WithCancel(context.Background())
	job, ok := s.redirects.start(redirectJob{
		Domain:   params.domain,
		FileName: header.Filename,
		Total:    len(rules),
		Rules:    rules,
	}, cancel, maxRunningRedirectChecks)
	if !ok {
		cancel()
		writeError(w, http.StatusTooManyRequests, fmt.Errorf("%d redirect checks already running; wait for one to finish or cancel it", maxRunningRedirectChecks))
		return
	}

	go s.runRedirectCheck(ctx, cancel, job.ID, rules, redirectcheck.RunOptions{
		Domain:        params.domain,
		SitemapURL:    params.sitemapURL,
		Fetcher:       fetcher,
		Concurrency:   params.concurrency,
		RatePerSecond: params.rate,
		AdaptiveDelay: params.adaptive,
	})

	writeJSON(w, http.StatusAccepted, redirectJobViewOf(job, false))
}

// runRedirectCheck runs one check to completion on its own goroutine and records the outcome.
func (s *Server) runRedirectCheck(ctx context.Context, cancel context.CancelFunc, id string, rules []redirectcheck.Rule, opts redirectcheck.RunOptions) {
	defer cancel()
	opts.OnRow = func(done, _ int) {
		s.redirects.with(id, func(j *Job[redirectJob]) { j.Data.Done = done })
	}
	rows, err := redirectcheck.Run(ctx, rules, opts)
	if err != nil && ctx.Err() != nil {
		// A cancel during sitemap discovery surfaces as "could not fetch a sitemap". Clear it
		// so finish records the job as canceled, which is what the user asked for.
		err = nil
	}
	s.redirects.finish(id, err, func(d *redirectJob) {
		if err == nil {
			d.Rows = rows
		}
	})
}

func (s *Server) handleListRedirectChecks(w http.ResponseWriter, _ *http.Request) {
	jobs := s.redirects.list()
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].StartedAt.After(jobs[k].StartedAt) })
	views := make([]redirectJobView, 0, len(jobs))
	for _, j := range jobs {
		views = append(views, redirectJobViewOf(j, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": views})
}

func (s *Server) handleGetRedirectCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.redirects.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("no redirect check with id %q", id))
		return
	}
	writeJSON(w, http.StatusOK, redirectJobViewOf(job, true))
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/webserver -run 'RedirectCheck|NormalizeRedirectDomain'` Expected: `ok`.

- [ ] **Step 5: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/webserver/redirects.go internal/webserver/redirects_test.go
git commit -m "feat(webserver): run redirect-rule checks as background jobs" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W1-RED-3: cancel and CSV export for redirect checks

**Files:**
- Modify: `internal/webserver/redirects.go`
- Test: `internal/webserver/redirects_test.go` (append)

**Interfaces:**
- Consumes: `s.redirects.get`, `s.redirects.cancel`, `redirectcheck.WriteCSV(w io.Writer, rules []Rule, results []RowResult) error`
- Produces: `POST /api/redirect-checks/{id}/cancel` returns `204`, `404` for an unknown id, and
  `409` when the check is not running. `GET /api/redirect-checks/{id}/export` returns
  `200 text/csv` as an attachment named `redirect-check-<domain>.csv`, `404` for an unknown id,
  and `409` unless the status is `done`.

- [ ] **Step 1: Write the failing tests.** Append to `internal/webserver/redirects_test.go`
  (and add `"encoding/csv"` to its imports):

```go
func TestRedirectCheckExportCSV(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)
	id := startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	if job := pollRedirectCheck(t, srv, id); job["status"] != "done" {
		t.Fatalf("status = %v, error = %v", job["status"], job["error"])
	}

	req := httptest.NewRequest(http.MethodGet, "/api/redirect-checks/"+id+"/export", nil)
	req.Host = "localhost:8080"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="redirect-check-example.com.csv"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	records, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil {
		t.Fatalf("export is not valid CSV: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d CSV records, want header + 2 rows", len(records))
	}
	header := records[0]
	if header[len(header)-2] != "verdict" || header[len(header)-1] != "notes" {
		t.Errorf("header = %v", header)
	}
	if records[1][0] != "/old" || records[1][len(header)-2] != "ok" {
		t.Errorf("row 1 = %v", records[1])
	}
	if records[2][len(header)-2] != "skipped-external" {
		t.Errorf("row 2 = %v", records[2])
	}
}

func TestRedirectCheckExportBeforeDoneIs409(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, make(chan struct{})))
	srv := newTestServer(t)
	id := startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	rec, _ := doJSON(t, srv, http.MethodGet, "/api/redirect-checks/"+id+"/export", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	rec, _ = doJSON(t, srv, http.MethodGet, "/api/redirect-checks/nope/export", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: status = %d, want 404", rec.Code)
	}
}

func TestCancelRedirectCheck(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, make(chan struct{})))
	srv := newTestServer(t)
	id := startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})

	// The gated site holds the sitemap fetch, so this cancel lands during sitemap discovery:
	// the job must end canceled, not as a "could not fetch a sitemap" error.
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/redirect-checks/"+id+"/cancel", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cancel: status = %d, want 204", rec.Code)
	}
	job := pollRedirectCheck(t, srv, id)
	if job["status"] != "canceled" {
		t.Fatalf("status = %v (error %v), want canceled", job["status"], job["error"])
	}
	if _, ok := job["rows"]; ok {
		t.Fatal("a canceled check must not carry rows")
	}

	rec, _ = doJSON(t, srv, http.MethodGet, "/api/redirect-checks/"+id+"/export", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("export after cancel: status = %d, want 409", rec.Code)
	}
	rec, _ = doJSON(t, srv, http.MethodPost, "/api/redirect-checks/"+id+"/cancel", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second cancel: status = %d, want 409", rec.Code)
	}
	rec, _ = doJSON(t, srv, http.MethodPost, "/api/redirect-checks/nope/cancel", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown cancel: status = %d, want 404", rec.Code)
	}

	// A canceled check frees its slot under the cap of 2.
	for i := 0; i < maxRunningRedirectChecks; i++ {
		startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	}
}

func TestCancelRedirectCheckRejectsCrossOrigin(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, make(chan struct{})))
	srv := newTestServer(t)
	id := startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/redirect-checks/"+id+"/cancel", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if job, _ := srv.redirects.get(id); job.Status != StatusRunning {
		t.Fatalf("status = %s after a rejected cancel, want running", job.Status)
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/webserver -run 'RedirectCheckExport|CancelRedirectCheck'`
  Expected: FAIL. The cancel and export paths do not match any `/api/redirect-checks` route,
  so they fall through to the frontend handler, and the tests report wrong statuses
  (for example `cancel: status = 200, want 204` or `405`) or an HTML decode error.

- [ ] **Step 3: Implement.** In `internal/webserver/redirects.go`, replace `redirectRoutes` with:

```go
func (s *Server) redirectRoutes() {
	s.mux.HandleFunc("GET /api/redirect-checks", s.handleListRedirectChecks)
	s.mux.HandleFunc("POST /api/redirect-checks", s.handleStartRedirectCheck)
	s.mux.HandleFunc("GET /api/redirect-checks/{id}", s.handleGetRedirectCheck)
	s.mux.HandleFunc("POST /api/redirect-checks/{id}/cancel", s.handleCancelRedirectCheck)
	s.mux.HandleFunc("GET /api/redirect-checks/{id}/export", s.handleExportRedirectCheck)
}
```

  and append:

```go
func (s *Server) handleCancelRedirectCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.redirects.get(id); !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("no redirect check with id %q", id))
		return
	}
	if !s.redirects.cancel(id) {
		writeError(w, http.StatusConflict, fmt.Errorf("redirect check %q is not running", id))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleExportRedirectCheck writes the input CSV back with the verdict columns appended, the
// same file `gocrawl check-redirects --output` writes.
func (s *Server) handleExportRedirectCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.redirects.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("no redirect check with id %q", id))
		return
	}
	if job.Status != StatusDone {
		writeError(w, http.StatusConflict, fmt.Errorf("redirect check %q is %s; only a finished check can be exported", id, job.Status))
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="redirect-check-%s.csv"`, job.Data.Domain))
	// The status line is already sent once WriteCSV starts writing, so a failure partway
	// through (a client that went away) has no better response to give.
	_ = redirectcheck.WriteCSV(w, job.Data.Rules, job.Data.Rows)
}
```

  Locking: the cancel handler runs `get` and then `cancel`, two separate locked calls. If the
  job finishes between them, `cancel` returns false and the handler answers `409`. That is the
  correct answer for a check that is no longer running.

- [ ] **Step 4: Run.** `go test -race ./internal/webserver` Expected: `ok`.

- [ ] **Step 5: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/webserver/redirects.go internal/webserver/redirects_test.go
git commit -m "feat(webserver): cancel and export redirect-check jobs" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W1-RED-4: docs and changelog for redirect-check jobs

**Files:**
- Modify: `docs/web.md`, `docs/redirect-check.md`, `CHANGELOG.md`

**Interfaces:** none.

- [ ] **Step 1: Update `docs/web.md`.** Set `updated: 2026-10-13`. Add these rows to the end of
  the API table:

```markdown
| `GET /api/redirect-checks` | List the redirect checks started in this process, newest first, without their rows: `{"checks": [...]}`. |
| `POST /api/redirect-checks` | Start a redirect-rule check, the same check as [`gocrawl check-redirects`](redirect-check.md). Multipart form: `file` (the HubSpot-format CSV, up to 10 MB), `domain` (required), and optionally `sitemap_url`, `concurrency` (1 to 16, default 4), `rate` (requests per second, default 0 = unlimited) and `adaptive_delay` (default `true`). The CSV is parsed before the job starts, so a wrong header is a `400` and not a failed job. Returns `202` with the job. `413` over 10 MB, `415` when the request is not multipart, `429` when 2 checks are already running. |
| `GET /api/redirect-checks/{id}` | One check: `status`, `done` and `total` rules, and, once the status is `done`, `rows`. Each row is the CSV columns (`original`, `target`, …, `note`) plus the result (`scope`, `source_status`, `source_final_url`, `source_matches_target`, `target_status`, `original_in_sitemap`, `target_in_sitemap`, `verdict`, `notes`). When no sitemap is found, the status is `error` with the reason. |
| `POST /api/redirect-checks/{id}/cancel` | Stop a running check. A canceled check keeps no rows. `409` when it is not running. |
| `GET /api/redirect-checks/{id}/export` | Download the result CSV: the input columns plus the verdict columns. `409` until the check is `done`. |
```

  In the "Exposure" list, after the crawl-cap bullet, add:
  "- At most 2 redirect checks run at once; a third `POST /api/redirect-checks` returns `429`."
  In "Notes", add: "- Redirect-check results live in memory only, like unsaved crawls."

- [ ] **Step 2: Update `docs/redirect-check.md`.** Set `updated: 2026-10-13`. Append:

```markdown
## In the web UI

`gocrawl serve` runs the same check from the browser: open **Redirects**, choose the CSV, enter
the domain, and select **Check redirects**. The check runs as a background job with a progress
bar. When it finishes, the results table can be filtered by verdict and scope, and **Export
CSV** downloads the same file `--output` writes. The flags map to form fields: `--sitemap-url`,
`--rate`, `--concurrency` and `--adaptive-delay`. `--timeout` and `--user-agent` keep their
defaults. See the [web UI guide](web.md) for the API behind it.
```

- [ ] **Step 3: Add the changelog line.** Under `## [Unreleased]`, `### Added`:

```markdown
- Web API: redirect-rule checks run as background jobs under `/api/redirect-checks`: upload
  the CSV, poll progress, cancel, and export the result CSV. It is the same check as
  `gocrawl check-redirects`.
```

- [ ] **Step 4: Regenerate.** `make docs` Expected: exits 0. `docs/redirect-check.md` gains
  `depends_on: ["web.md"]`, and the index picks up both dates.

- [ ] **Step 5: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add docs/web.md docs/redirect-check.md docs/README.md CHANGELOG.md
git commit -m "docs: document redirect-check jobs in the web API" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

## Wave 2: UIs and the pause backend

### Track: progress UI


Manual-check fixture used by the frontend tasks (run once in a spare terminal; it serves a
300-page site whose sitemap lists 280 of them, plus one broken link):

```sh
mkdir -p /tmp/gocrawl-fixture && cd /tmp/gocrawl-fixture
{ echo '<html><head><title>Home</title></head><body><a href="/missing.html">missing</a>'
  for i in $(seq 1 300); do echo "<a href=\"/p$i.html\">p$i</a>"; done
  echo '</body></html>'; } > index.html
for i in $(seq 1 300); do echo "<html><head><title>P$i</title></head><body>page $i</body></html>" > "p$i.html"; done
{ echo '<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'
  for i in $(seq 1 280); do echo "<url><loc>http://127.0.0.1:8099/p$i.html</loc></url>"; done
  echo '</urlset>'; } > sitemap.xml
printf 'User-agent: *\nAllow: /\nSitemap: http://127.0.0.1:8099/sitemap.xml\n' > robots.txt
python3 -m http.server 8099 --bind 127.0.0.1
```

And the app: `make web-build && go run ./cmd/gocrawl serve`, then open http://127.0.0.1:8080.

#### Task W2-PROG-UI-1: Progress types, API calls and tape helpers

**Files:**
- Create: `web/src/types/progress.ts`, `web/src/api/progress.ts`,
  `web/src/components/progress/tapeText.ts`
- Modify: `web/src/types/crawls.ts` (one line), `web/src/crawlStatus.ts` (one entry)

**Interfaces:**
- Consumes: `request` (api/client.ts), `JobView`, `CrawlListItem` (types/crawls.ts),
  `StatusClass`, `STATUS_CLASSES`, `STATUS_LABEL`, `statusClass` (`web/src/status.ts`, visual
  system), `STATUS_PILL` (`web/src/crawlStatus.ts`).
- Produces: `ProgressView`, `JobWithProgress`, `CrawlListItemWithProgress`, `Phase`,
  `PauseReason`, `SitemapPreview`, `SitemapPreviewRequest`; `getCrawlProgress(id, tapeFrom)`,
  `previewSitemap(body, signal?)`; tape helpers listed in `tapeText.ts`.

- [ ] **Step 1: Edit `web/src/types/crawls.ts` and `web/src/crawlStatus.ts`.** Add `'paused'`
  to `CrawlStatus` (the `Coverage` fields `sitemap_total`, `sitemap_fetched`, `tape`,
  `tape_off` already came with W1-PROG-6b):

```ts
export type CrawlStatus = 'running' | 'paused' | 'done' | 'error' | 'canceled'
```

  `STATUS_PILL` in `web/src/crawlStatus.ts` is a `Record<CrawlStatus, …>`, so `tsc` now asks
  for the new entry. Add it after `running`:

```ts
  paused: { phase: 'paused', label: 'Paused' },
```

- [ ] **Step 2: Create `web/src/types/progress.ts`:**

```ts
// Mirrors internal/webserver progressView (runner.Progress + tape fields) and the sitemap
// preview response. Keep in sync with the Go side by hand.
import type { CrawlListItem, JobView } from './crawls'

export type Phase = 'sitemap' | 'crawling' | 'analyzing'
export type PauseReason = 'user' | 'rate_limited'

export interface ProgressView {
  fetched: number
  queued: number
  current_url: string
  final_url?: string
  status: number
  duration_ms: number
  rate: number
  throttled: number
  paused: boolean
  pause_reason?: PauseReason
  retry_after_seconds?: number
  phase: Phase
  sitemap_found: boolean
  sitemap_total: number
  sitemap_fetched: number
  off_sitemap: number
  in_sitemap: boolean
  tape: string
  tape_off: string
  tape_from: number
  max_pages: number
  active_ms: number
}

export type JobWithProgress = JobView & { progress?: ProgressView }

export type CrawlListItemWithProgress = CrawlListItem & {
  status_classes?: Record<string, number>
  sitemap_total?: number
  sitemap_fetched?: number
  /** At most 160 tape characters, worst status per run (crawler.TapePreview). */
  tape_preview?: string
}

export interface SitemapPreviewRequest {
  url: string
  include?: string[]
  exclude?: string[]
  allow_subdomains?: boolean
}

export interface SitemapPreview {
  found: boolean
  total: number
  sources: string[]
  via_robots: boolean
}
```

- [ ] **Step 3: Create `web/src/api/progress.ts`:**

```ts
import { request } from './client'
import type { JobWithProgress, SitemapPreview, SitemapPreviewRequest } from '../types/progress'

/** One poll of a crawl: tape characters from tapeFrom on, plus the latest snapshot. */
export function getCrawlProgress(id: string, tapeFrom: number): Promise<JobWithProgress> {
  return request(`/api/crawls/${encodeURIComponent(id)}?tape_from=${tapeFrom}`)
}

/** Runs only the sitemap discovery step for a crawl scope. */
export function previewSitemap(body: SitemapPreviewRequest, signal?: AbortSignal): Promise<SitemapPreview> {
  return request('/api/sitemap-preview', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  })
}
```

- [ ] **Step 4: Create `web/src/components/progress/tapeText.ts`:**

```ts
// Helpers for the crawl tape and the counts around it. components/tape/Tape.tsx draws the
// tape; this module builds its inputs and the text that mirrors every count it shows. Status
// classes, their order and their labels come from web/src/status.ts (visual system), so the
// legend, StatusCode and these counts all use one vocabulary.
import { STATUS_CLASSES, STATUS_LABEL, statusClass, type StatusClass } from '../../status'

export type TapeClass = StatusClass
export type ClassCounts = Record<TapeClass, number>

export const CLASS_ORDER: TapeClass[] = STATUS_CLASSES

/** Legend labels, one per status class. */
export const CLASS_LABEL: Record<TapeClass, string> = STATUS_LABEL

/** Singular and plural nouns for the tape's text summary (as tapeModel.describeTape words them). */
const SUMMARY_NOUN: Record<TapeClass, [string, string]> = {
  '2': ['OK', 'OK'],
  '3': ['redirected', 'redirected'],
  '4': ['not found or blocked', 'not found or blocked'],
  '5': ['server error', 'server errors'],
  x: ['without a response', 'without a response'],
}

/** The server's page budget when the form leaves max pages empty (config.Default). */
export const DEFAULT_MAX_PAGES = 500

export function emptyCounts(): ClassCounts {
  return { '2': 0, '3': 0, '4': 0, '5': 0, x: 0 }
}

export const classOf: (status: number) => TapeClass = statusClass

function isClass(c: string): c is TapeClass {
  return c === '2' || c === '3' || c === '4' || c === '5' || c === 'x'
}

export function countClasses(tape: string): ClassCounts {
  const counts = emptyCounts()
  for (const ch of tape) if (isClass(ch)) counts[ch] += 1
  return counts
}

/** Counts from a list item's status_classes ("2" -> n). */
export function countsFromRecord(rec?: Record<string, number>): ClassCounts {
  const counts = emptyCounts()
  if (!rec) return counts
  for (const [k, n] of Object.entries(rec)) if (isClass(k)) counts[k] += n
  return counts
}

/** Counts from a report's pages_by_status ("404" -> n). */
export function countsFromStatuses(byStatus?: Record<string, number>): ClassCounts {
  const counts = emptyCounts()
  if (!byStatus) return counts
  for (const [code, n] of Object.entries(byStatus)) counts[classOf(Number(code))] += n
  return counts
}

export function totalOf(counts: ClassCounts): number {
  return CLASS_ORDER.reduce((n, c) => n + counts[c], 0)
}

/** A status-only tape in class order (fallback for reports saved before coverage.tape existed). */
export function tapeFromCounts(counts: ClassCounts): string {
  return CLASS_ORDER.map((c) => c.repeat(counts[c])).join('')
}

/** A status-only tape scaled to at most maxTicks; every non-empty class keeps one tick. */
export function scaleTape(counts: ClassCounts, maxTicks: number): string {
  const total = totalOf(counts)
  if (total <= maxTicks) return tapeFromCounts(counts)
  const scaled = emptyCounts()
  for (const c of CLASS_ORDER) {
    if (counts[c] > 0) scaled[c] = Math.max(1, Math.round((counts[c] / total) * maxTicks))
  }
  return tapeFromCounts(scaled)
}

export function formatCount(n: number): string {
  return n.toLocaleString('en-US')
}

export function percentOf(part: number, whole: number): number {
  return whole > 0 ? Math.floor((part * 100) / whole) : 0
}

/** "At most N%" of the sitemap a budget can cover; never shows 0%. */
export function budgetPercent(maxPages: number, sitemapTotal: number): number {
  return Math.max(1, percentOf(maxPages, sitemapTotal))
}

export function formatElapsed(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const ss = String(total % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

export function formatRate(rate: number): string {
  if (rate >= 10) return rate.toFixed(0)
  if (rate >= 1) return rate.toFixed(1)
  return String(Number(rate.toPrecision(2)))
}

export function formatDuration(ms: number): string {
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`
}

/** Half the current rate, as a form value ('' when unlimited). */
export function halfRate(rate: number): string {
  return rate > 0 ? String(Number((rate / 2).toPrecision(2))) : ''
}

export interface Outline {
  notFetched: number
  pastBudget: number
}

/**
 * Outline ticks after the fetched ones: sitemap pages the budget still allows (not fetched yet,
 * or after the crawl, not reached by links) and sitemap pages past the page budget. Without a
 * sitemap there are none: the counts fall back to the page budget as text instead.
 */
export function outlineTicks(fetched: number, sitemapTotal: number, sitemapFetched: number, maxPages: number): Outline {
  if (sitemapTotal <= 0) return { notFetched: 0, pastBudget: 0 }
  const remaining = Math.max(0, sitemapTotal - sitemapFetched)
  if (maxPages <= 0) return { notFetched: remaining, pastBudget: 0 }
  const notFetched = Math.min(remaining, Math.max(0, maxPages - fetched))
  return { notFetched, pastBudget: remaining - notFetched }
}

export interface SummaryInput {
  counts: ClassCounts
  fetched: number
  sitemapTotal: number
  sitemapFetched: number
  offSitemap: number
  notFetched: number
  pastBudget: number
  live: boolean
}

function plural(n: number, one: string, many: string): string {
  return n === 1 ? one : many
}

/** The tape's text alternative: every count the tape shows, in words. */
export function tapeSummary(o: SummaryInput): string {
  const head =
    o.sitemapTotal > 0
      ? `${formatCount(o.sitemapFetched)} of ${formatCount(o.sitemapTotal)} sitemap pages fetched`
      : `${formatCount(o.fetched)} ${plural(o.fetched, 'page', 'pages')} fetched`
  const classes = CLASS_ORDER.filter((c) => o.counts[c] > 0).map((c) => {
    const [one, many] = SUMMARY_NOUN[c]
    return `${formatCount(o.counts[c])} ${plural(o.counts[c], one, many)}`
  })
  const parts = [classes.length > 0 ? `${head}: ${classes.join(', ')}.` : `${head}.`]
  if (o.offSitemap > 0) {
    parts.push(`${formatCount(o.offSitemap)} fetched ${plural(o.offSitemap, 'page is', 'pages are')} not in the sitemap.`)
  }
  if (o.notFetched > 0) {
    parts.push(
      `${formatCount(o.notFetched)} sitemap ${plural(o.notFetched, 'page', 'pages')} ${o.live ? 'not fetched yet' : 'not reached by links'}.`,
    )
  }
  if (o.pastBudget > 0) {
    parts.push(`${formatCount(o.pastBudget)} sitemap ${plural(o.pastBudget, 'page is', 'pages are')} past the page budget.`)
  }
  return parts.join(' ')
}

/** "1,020 of 1,240 (82%)" for a crawl list row, or an em dash without sitemap data. */
export function sitemapCoverageText(total?: number, fetched?: number): string {
  if (!total) return '—'
  const f = fetched ?? 0
  return `${formatCount(f)} of ${formatCount(total)} (${percentOf(f, total)}%)`
}

export function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}

/** Path and query when url is on the seed's host, else the full URL. */
export function pathOf(url: string, seed: string): string {
  try {
    const u = new URL(url)
    return u.host === new URL(seed).host ? `${u.pathname}${u.search}` : url
  } catch {
    return url
  }
}

/** Last path segment of a URL ("sitemap_index.xml"). */
export function fileName(url: string): string {
  try {
    const parts = new URL(url).pathname.split('/')
    return parts[parts.length - 1] || url
  } catch {
    return url
  }
}
```

- [ ] **Step 5: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: `tsc -b` and `vite build` succeed; oxlint and the contrast check report no errors.
  (The new modules are not imported yet, so no behavior changes.)

- [ ] **Step 6: Manual check.** `make web-build && go run ./cmd/gocrawl serve`; the UI loads and
  starting a crawl of `http://127.0.0.1:8099/` still works exactly as before. Then
  `curl -s 'http://127.0.0.1:8080/api/crawls/<id>?tape_from=0' | head -c 600` shows a
  `progress` object whose fields match `ProgressView`.

- [ ] **Step 7: `make check`.** Expected: exits 0.

- [ ] **Step 8: Commit.**

```sh
git restore internal/webserver/webui/dist/index.html
git add web/src/types/crawls.ts web/src/crawlStatus.ts web/src/types/progress.ts web/src/api/progress.ts web/src/components/progress/tapeText.ts
git commit -F - <<'EOF'
feat(web): add progress types, API calls and tape helpers

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PROG-UI-2: Running-crawl view

**Files:**
- Create: `web/src/components/progress/useCrawlPoll.ts`, `RunningCrawl.tsx`, `RunPhasePill.tsx`,
  `progress.css`
- Modify: `web/src/components/CrawlReport.tsx`

**Interfaces:**
- Consumes: `getCrawlProgress` (W2-PROG-UI-1), tape helpers, `Tape`, `TapeLegend`, `Notice`,
  `StatusCode`, `Button`, `PhasePill` (`components/ui/PhasePill.tsx`, visual system), `cancelCrawl`.
- Produces: `useCrawlPoll(id): CrawlPoll` (`job`, `progress`, `tape`, `tapeOff`, `recent`,
  `receivedAt`, `error`, `refresh`), `RecentFetch`, `isLive(status)`, `POLL_MS`;
  `<RunningCrawl job progress tape tapeOff recent receivedAt controls notice? />`;
  `<RunPhasePill phase />` (maps the run phase onto the visual system's `PhasePill`; no second
  pill component or pill CSS).

- [ ] **Step 1: Create `web/src/components/progress/useCrawlPoll.ts`:**

```ts
import { useCallback, useEffect, useRef, useState } from 'react'
import { getCrawlProgress } from '../../api/progress'
import type { JobWithProgress, ProgressView } from '../../types/progress'

export const POLL_MS = 1500
const RECENT_MAX = 8

/** One row of the "Fetching now" table, taken from a progress snapshot. */
export interface RecentFetch {
  fetched: number
  url: string
  status: number
  durationMs: number
  inSitemap: boolean
}

export interface CrawlPoll {
  job: JobWithProgress | null
  progress: ProgressView | null
  tape: string
  tapeOff: string
  recent: RecentFetch[]
  /** Date.now() when the latest poll landed; lets the elapsed clock tick between polls. */
  receivedAt: number
  error: string
  /** Poll right away (after pause, resume or stop) instead of waiting for the next tick. */
  refresh: () => void
}

export function isLive(status: string | undefined): boolean {
  return status === 'running' || status === 'paused'
}

/**
 * Polls GET /api/crawls/{id} every POLL_MS while the crawl is running or paused. The tape is
 * accumulated client-side: each poll asks only for characters from the current length on
 * (tape_from), and the server says where its slice starts, so a shorter server tape (a restart)
 * truncates instead of duplicating.
 */
export function useCrawlPoll(id: string): CrawlPoll {
  const [job, setJob] = useState<JobWithProgress | null>(null)
  const [tape, setTape] = useState({ tape: '', off: '' })
  const [recent, setRecent] = useState<RecentFetch[]>([])
  const [receivedAt, setReceivedAt] = useState(0)
  const [error, setError] = useState('')
  const [nonce, setNonce] = useState(0)
  const tapeRef = useRef({ tape: '', off: '' })

  useEffect(() => {
    let stopped = false
    let timer: number | undefined
    async function poll() {
      try {
        const j = await getCrawlProgress(id, tapeRef.current.tape.length)
        if (stopped) return
        const p = j.progress
        if (p) {
          const cur = tapeRef.current
          const keep = Math.min(p.tape_from, cur.tape.length)
          const next = { tape: cur.tape.slice(0, keep) + p.tape, off: cur.off.slice(0, keep) + p.tape_off }
          tapeRef.current = next
          setTape(next)
          if (p.fetched > 0 && p.current_url) {
            const entry: RecentFetch = {
              fetched: p.fetched,
              url: p.current_url,
              status: p.status,
              durationMs: p.duration_ms,
              inSitemap: p.in_sitemap,
            }
            setRecent((prev) =>
              prev.length > 0 && prev[0].fetched === entry.fetched ? prev : [entry, ...prev].slice(0, RECENT_MAX),
            )
          }
        }
        setJob(j)
        setReceivedAt(Date.now())
        setError('')
        if (isLive(j.status)) timer = window.setTimeout(poll, POLL_MS)
      } catch (err) {
        if (!stopped) setError(String(err))
      }
    }
    void poll()
    return () => {
      stopped = true
      window.clearTimeout(timer)
    }
  }, [id, nonce])

  const refresh = useCallback(() => setNonce((n) => n + 1), [])
  return { job, progress: job?.progress ?? null, tape: tape.tape, tapeOff: tape.off, recent, receivedAt, error, refresh }
}
```

- [ ] **Step 2: Create `web/src/components/progress/RunPhasePill.tsx`** (a thin mapping onto
  the visual system's `PhasePill`, which already draws the green pulsing, yellow and violet
  dots and honors `prefers-reduced-motion`):

```tsx
import PhasePill, { type Phase as PillPhase } from '../ui/PhasePill'
import type { Phase } from '../../types/progress'

const PILL: Record<Phase, { phase: PillPhase; label: string }> = {
  sitemap: { phase: 'analyzing', label: 'Reading sitemap' },
  crawling: { phase: 'crawling', label: 'Crawling' },
  analyzing: { phase: 'analyzing', label: 'Analyzing' },
}

/** Green pulsing dot while crawling, violet while reading the sitemap or analyzing. */
export default function RunPhasePill({ phase }: { phase?: Phase }) {
  const p = PILL[phase ?? 'sitemap']
  return <PhasePill phase={p.phase}>{p.label}</PhasePill>
}
```

- [ ] **Step 3: Create `web/src/components/progress/progress.css`:**

```css
/* Running crawl, finished tape, sitemap preview line. Colors come from styles/tokens.css. */
.running-crawl,
.finished-tape {
  display: grid;
  gap: 16px;
}
.rc-head {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}
.rc-head h1 {
  font-size: 24px;
  margin: 0;
  font-weight: 700;
}
.rc-actions {
  margin-left: auto;
  display: flex;
  gap: 10px;
}
.rc-sub {
  color: var(--ink-2);
  margin: 0;
}
.rc-stats {
  display: flex;
  gap: 32px;
  flex-wrap: wrap;
}
.rc-stat b {
  display: block;
  font-size: 28px;
  font-weight: 700;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}
.rc-stat span {
  font-size: 13px;
  color: var(--ink-2);
}
.rc-stat b.rc-warn {
  color: var(--s3);
}
.rc-classes {
  list-style: none;
  display: flex;
  gap: 18px;
  flex-wrap: wrap;
  padding: 0;
  margin: 0;
  font-size: 14px;
  color: var(--ink-2);
}
.rc-classes b {
  color: var(--ink);
  font-variant-numeric: tabular-nums;
}
.rc-swatch {
  display: inline-block;
  width: 4px;
  height: 12px;
  margin-right: 6px;
  vertical-align: -1px;
  background: var(--s2);
}
.rc-class-3 .rc-swatch {
  background: var(--s3);
}
.rc-class-4 .rc-swatch {
  background: var(--s4);
}
.rc-class-5 .rc-swatch {
  background: var(--s5);
}
.rc-class-x .rc-swatch {
  background: var(--ink-2);
}
.rc-h2 {
  font-size: 16px;
  margin: 0;
}
.rc-recent {
  width: 100%;
  border-collapse: collapse;
  background: var(--surface);
  border: 1px solid var(--rule);
  font-size: 14px;
}
.rc-recent th {
  text-align: left;
  font-weight: 600;
  font-size: 13px;
  color: var(--ink-2);
  padding: 6px 12px;
}
.rc-recent td {
  padding: 6px 12px;
  border-top: 1px solid var(--rule);
}
.rc-url {
  font-family: var(--font-mono);
  font-size: 13.5px;
  overflow-wrap: anywhere;
}
.rc-off {
  font-family: inherit;
  font-size: 12px;
  color: var(--ink-2);
}
.rc-recent .rc-ms {
  width: 80px;
  text-align: right;
  color: var(--ink-2);
  font-variant-numeric: tabular-nums;
}
.rc-failed {
  color: var(--s5);
  font-weight: 600;
}
.ft-coverage,
.sp-line {
  margin: 0;
  color: var(--ink-2);
}
.ft-coverage b {
  color: var(--ink);
}
.sp-wrap {
  display: grid;
  gap: 8px;
}
.mini-tape-empty {
  color: var(--ink-2);
}
```

- [ ] **Step 4: Create `web/src/components/progress/RunningCrawl.tsx`:**

```tsx
import { useEffect, useState, type ReactNode } from 'react'
import Notice from '../ui/Notice'
import StatusCode from '../ui/StatusCode'
import Tape from '../tape/Tape'
import TapeLegend from '../tape/TapeLegend'
import RunPhasePill from './RunPhasePill'
import type { RecentFetch } from './useCrawlPoll'
import type { JobWithProgress, ProgressView } from '../../types/progress'
import {
  CLASS_LABEL,
  CLASS_ORDER,
  budgetPercent,
  countClasses,
  formatCount,
  formatDuration,
  formatElapsed,
  formatRate,
  hostOf,
  outlineTicks,
  pathOf,
  tapeSummary,
} from './tapeText'
import './progress.css'

interface Props {
  job: JobWithProgress
  progress: ProgressView | null
  tape: string
  tapeOff: string
  recent: RecentFetch[]
  receivedAt: number
  /** Header buttons (Stop crawl now; Pause / Resume crawl / Stop crawl from wave 3). */
  controls: ReactNode
  /** Shown under the header, e.g. the auto-pause explanation. */
  notice?: ReactNode
}

export default function RunningCrawl({ job, progress, tape, tapeOff, recent, receivedAt, controls, notice }: Props) {
  const paused = job.status === 'paused'
  const ticking = !paused && progress?.phase === 'crawling'
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!ticking) return
    const t = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(t)
  }, [ticking])

  const fetched = progress?.fetched ?? 0
  const sitemapFound = progress?.sitemap_found ?? false
  const sitemapTotal = progress?.sitemap_total ?? 0
  const sitemapFetched = progress?.sitemap_fetched ?? 0
  const offSitemap = sitemapFound ? (progress?.off_sitemap ?? 0) : 0
  const maxPages = progress?.max_pages ?? 0
  const rate = progress?.rate ?? 0
  const throttled = progress?.throttled ?? 0
  const counts = countClasses(tape)
  const { notFetched, pastBudget } = outlineTicks(fetched, sitemapTotal, sitemapFetched, maxPages)
  const elapsed = (progress?.active_ms ?? 0) + (ticking && receivedAt > 0 ? Math.max(0, now - receivedAt) : 0)
  const budgetBelow = maxPages > 0 && sitemapTotal > maxPages
  const summary = tapeSummary({ counts, fetched, sitemapTotal, sitemapFetched, offSitemap, notFetched, pastBudget, live: true })

  return (
    <section className="running-crawl">
      <header className="rc-head">
        <h1>{hostOf(job.seed)}</h1>
        <RunPhasePill phase={progress?.phase} />
        <div className="rc-actions">{controls}</div>
      </header>
      {notice}
      {budgetBelow && (
        <Notice tone="warning">
          <p>
            The page budget ({formatCount(maxPages)}) is below the sitemap total ({formatCount(sitemapTotal)}). This crawl will
            cover at most {budgetPercent(maxPages, sitemapTotal)}% of the sitemap.
          </p>
        </Notice>
      )}
      {progress?.phase === 'sitemap' && <p className="rc-sub">Reading robots.txt and the sitemap before the crawl starts…</p>}
      <Tape
        tape={tape}
        tapeOff={sitemapFound ? tapeOff : undefined}
        notFetched={notFetched}
        pastBudget={pastBudget}
        summary={summary}
        live
      />
      <TapeLegend sitemap={sitemapFound} />
      <div className="rc-stats">
        {sitemapTotal > 0 ? (
          <Stat value={formatCount(sitemapFetched)} label={`of ${formatCount(sitemapTotal)} sitemap pages fetched`} />
        ) : (
          <Stat
            value={formatCount(fetched)}
            label={maxPages > 0 ? `pages fetched of a ${formatCount(maxPages)}-page budget` : 'pages fetched'}
          />
        )}
        {sitemapFound && <Stat value={formatCount(offSitemap)} label="fetched pages not in the sitemap" />}
        <Stat value={formatCount(progress?.queued ?? 0)} label="in queue" />
        <Stat value={formatElapsed(elapsed)} label="elapsed (pauses don't count)" />
        <Stat value={rate > 0 ? `${formatRate(rate)}/s` : 'Unlimited'} label={paused ? 'request rate when paused' : 'request rate'} />
        {throttled > 0 && <Stat value={formatCount(throttled)} label="429 or 503 responses in the last minute" warn />}
      </div>
      <ul className="rc-classes" aria-label="Pages by status class">
        {CLASS_ORDER.map((c) => (
          <li key={c} className={`rc-class-${c}`}>
            <span className="rc-swatch" aria-hidden="true" />
            {CLASS_LABEL[c]} <b>{formatCount(counts[c])}</b>
          </li>
        ))}
      </ul>
      <h2 className="rc-h2">Fetching now</h2>
      {recent.length === 0 ? (
        <p className="rc-sub">
          {progress?.phase === 'sitemap' ? 'Pages appear here once the crawl starts.' : 'Waiting for the first page…'}
        </p>
      ) : (
        <table className="rc-recent">
          <thead>
            <tr>
              <th scope="col">Status</th>
              <th scope="col">URL</th>
              <th scope="col" className="rc-ms">
                Time
              </th>
            </tr>
          </thead>
          <tbody>
            {recent.map((r) => (
              <tr key={r.fetched}>
                <td>{r.status > 0 ? <StatusCode code={r.status} /> : <span className="rc-failed">Failed</span>}</td>
                <td className="rc-url" title={r.url}>
                  {pathOf(r.url, job.seed)}
                  {sitemapFound && !r.inSitemap && <span className="rc-off"> · not in the sitemap</span>}
                </td>
                <td className="rc-ms">{formatDuration(r.durationMs)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}

function Stat({ value, label, warn = false }: { value: string; label: string; warn?: boolean }) {
  return (
    <div className="rc-stat">
      <b className={warn ? 'rc-warn' : undefined}>{value}</b>
      <span>{label}</span>
    </div>
  )
}
```

- [ ] **Step 5: Wire it into `web/src/components/CrawlReport.tsx`.** Make these edits to the
  visual-system version of the file:

  1. Imports: add

```tsx
import RunningCrawl from './progress/RunningCrawl'
import { useCrawlPoll } from './progress/useCrawlPoll'
```

     (W1-VIS-7 already imports `Button` and `Notice`), and remove `getCrawl` from the
     `../api/crawls` import (keep `cancelCrawl`, `exportURL`, `listExplanations`).

  2. Replace `const [job, setJob] = useState<JobView | null>(null)` with

```tsx
  const { job, progress, tape, tapeOff, recent, receivedAt, error: pollError } = useCrawlPoll(id)
```

  3. Delete the polling `useEffect` (the one that calls `getCrawl(id)` and re-arms
     `window.setTimeout(poll, POLL_MS)`), the `timer` ref, the `POLL_MS` constant, the `now`
     state and the `useEffect` that ticks `setNow` every second, and `formatElapsed` if nothing
     else uses it. `RunningCrawl` owns all of that now.

  4. Replace the visual system's `if (error) return <Notice tone="error">{error}</Notice>` with
     (a poll that fails, typically a 404 after the server restarted, says what happened and
     where to look):

```tsx
  if (error) return <Notice tone="error">{error}</Notice>
  if (pollError)
    return (
      <Notice tone="error">
        <p>
          Could not load this crawl: {pollError}. If gocrawl was restarted while it ran, an unsaved crawl is gone; saved
          crawls are listed under <a href={href({ view: 'crawls' })}>Crawls</a>.
        </p>
      </Notice>
    )
```

     and add `import { href } from '../routes'` to the imports.

  5. Directly after the visual system's `if (!job) return <p className="muted">Loading…</p>`,
     add:

```tsx
  function stopCrawl() {
    if (!window.confirm('Stop this crawl? The report keeps the pages fetched so far.')) return
    cancelCrawl(id).catch((e) => setError(String(e)))
  }

  if (job.status === 'running' || job.status === 'paused') {
    return (
      <RunningCrawl
        job={job}
        progress={progress}
        tape={tape}
        tapeOff={tapeOff}
        recent={recent}
        receivedAt={receivedAt}
        controls={
          <Button type="button" variant="stop" onClick={stopCrawl}>
            Stop crawl
          </Button>
        }
      />
    )
  }
```

  6. In the header JSX below, delete the two blocks guarded by `job.status === 'running'`: the
     **Stop crawl** `Button` inside `cr-actions` and the `cr-stats` block with the elapsed
     counter. That state never reaches this render any more. Remove any import the build now
     reports as unused (for example `JobView`).

- [ ] **Step 6: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: success, no oxlint errors, contrast check passes.

- [ ] **Step 7: Manual check.** With the fixture on :8099 and
  `make web-build && go run ./cmd/gocrawl serve`: start a crawl of `http://127.0.0.1:8099/`
  with Rate limit `5` and Max pages `100`. Expect: the pill reads "Reading sitemap" briefly,
  then "Crawling" with a pulsing green dot; the budget notice says the budget (100) is below the
  sitemap total (280) and "at most 35%"; ticks drop into the tape about five per second, with
  slate outline ticks for the 99 remaining-budget pages and amber outline ticks past the budget;
  "of 280 sitemap pages fetched" rises; "fetched pages not in the sitemap" counts the home page
  and `/missing.html`; the elapsed clock ticks each second; "request rate" reads `5.0/s`;
  "Fetching now" lists up to 8 recent URLs with a `404` for `/missing.html`. In DevTools →
  Network, every poll after the first carries `tape_from` equal to the tape length so far.
  **Stop crawl** asks for confirmation and the view switches to the finished report.

- [ ] **Step 8: `make check`.** Expected: exits 0.

- [ ] **Step 9: Commit.**

```sh
git restore internal/webserver/webui/dist/index.html
git add web/src/components/progress/useCrawlPoll.ts web/src/components/progress/RunPhasePill.tsx web/src/components/progress/progress.css web/src/components/progress/RunningCrawl.tsx web/src/components/CrawlReport.tsx
git commit -F - <<'EOF'
feat(web): live running-crawl view with tape, counts and recent fetches

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PROG-UI-3: Tape and sitemap coverage on the finished view

**Files:**
- Create: `web/src/components/progress/FinishedTape.tsx`
- Modify: `web/src/components/CrawlReport.tsx`

**Interfaces:**
- Consumes: `Report` (types/crawls.ts), tape helpers, `Tape`, `TapeLegend` (`finished` names
  the slate outline "Not reached by links", as the spec's finished view asks).
- Produces: `<FinishedTape report tape tapeOff sitemapFound />`.

- [ ] **Step 1: Create `web/src/components/progress/FinishedTape.tsx`:**

```tsx
import Tape from '../tape/Tape'
import TapeLegend from '../tape/TapeLegend'
import type { Report } from '../../types/crawls'
import {
  countClasses,
  countsFromStatuses,
  formatCount,
  outlineTicks,
  percentOf,
  tapeFromCounts,
  tapeSummary,
} from './tapeText'
import './progress.css'

interface Props {
  report: Report
  /** The tape streamed while this server ran the crawl; '' for a saved crawl. */
  tape: string
  tapeOff: string
  sitemapFound: boolean
}

export default function FinishedTape({ report, tape, tapeOff, sitemapFound }: Props) {
  const cov = report.coverage
  const sitemapTotal = cov?.sitemap_total ?? 0
  const sitemapFetched = cov?.sitemap_fetched ?? 0
  const pages = report.pages_crawled
  // Every report saved by this version carries the tape (coverage.tape / tape_off, built from
  // the same events as the live tape), so a crawl reopened after a restart looks the same as it
  // did live. The tape streamed by this server is the fallback for a report without one; a
  // report saved before tapes were stored gets a status-only tape in class order.
  const saved = cov?.tape ?? ''
  const ordered = saved !== '' ? saved : tape.length === pages ? tape : ''
  const orderedOff = saved !== '' ? (cov?.tape_off ?? '') : tapeOff
  const hasSitemap = sitemapFound || sitemapTotal > 0 || orderedOff.includes('1')
  const shown = ordered !== '' ? ordered : tapeFromCounts(countsFromStatuses(report.summary.pages_by_status))
  const outline =
    ordered !== ''
      ? outlineTicks(pages, sitemapTotal, sitemapFetched, cov?.max_pages ?? 0)
      : { notFetched: 0, pastBudget: 0 }
  const offSitemap = sitemapTotal > 0 ? Math.max(0, pages - sitemapFetched) : 0
  const summary = tapeSummary({
    counts: countClasses(shown),
    fetched: pages,
    sitemapTotal,
    sitemapFetched,
    offSitemap,
    notFetched: outline.notFetched,
    pastBudget: outline.pastBudget,
    live: false,
  })

  return (
    <section className="finished-tape">
      <Tape
        tape={shown}
        tapeOff={ordered !== '' && hasSitemap ? orderedOff : undefined}
        notFetched={outline.notFetched}
        pastBudget={outline.pastBudget}
        summary={summary}
      />
      <TapeLegend sitemap={ordered !== '' && hasSitemap} finished />
      <p className="ft-coverage">
        {sitemapTotal > 0 ? (
          <>
            Fetched <b>{formatCount(sitemapFetched)}</b> of <b>{formatCount(sitemapTotal)}</b> sitemap pages (
            {percentOf(sitemapFetched, sitemapTotal)}%).
            {offSitemap > 0 && (
              <>
                {' '}
                {formatCount(offSitemap)} fetched {offSitemap === 1 ? 'page is' : 'pages are'} not in the sitemap.
              </>
            )}
          </>
        ) : (
          <>
            {formatCount(pages)} {pages === 1 ? 'page' : 'pages'} fetched. No sitemap coverage for this crawl.
          </>
        )}
      </p>
    </section>
  )
}
```

- [ ] **Step 2: Wire it in.** In `web/src/components/CrawlReport.tsx` add
  `import FinishedTape from './progress/FinishedTape'`, and make it the first child of the
  `{report && (<> … </>)}` fragment, above the coverage banner:

```tsx
          <FinishedTape report={report} tape={tape} tapeOff={tapeOff} sitemapFound={progress?.sitemap_found ?? false} />
```

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: success.

- [ ] **Step 4: Manual check.** Let a crawl of `http://127.0.0.1:8099/` (Max pages 100) finish.
  Expect the tape at the top of the report with notches on off-sitemap pages, slate outline
  ticks now described as "not reached by links" in the tape's accessible summary (inspect the
  `aria-label`), amber ticks past the budget, and the line "Fetched 98 of 280 sitemap pages
  (35%). 2 fetched pages are not in the sitemap." (exact numbers depend on the crawl). Restart
  `gocrawl serve`, open the same crawl from Crawls (start it with "Save" on): the tape is the
  same, in fetch order with the same notches and outline ticks, because it comes from the
  saved report's `coverage.tape` / `tape_off`. Then import (or drop into the store) a JSON
  report saved before this change, without `coverage.tape`: it shows a status-only tape (green
  block, then one orange tick) and the coverage line.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git restore internal/webserver/webui/dist/index.html
git add web/src/components/progress/FinishedTape.tsx web/src/components/CrawlReport.tsx
git commit -F - <<'EOF'
feat(web): show the crawl tape and sitemap coverage on finished reports

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PROG-UI-4: Sitemap preview line and "Raise budget" in the New crawl form

**Files:**
- Create: `web/src/components/progress/SitemapPreviewLine.tsx`
- Modify: `web/src/components/CrawlForm.tsx`

**Interfaces:**
- Consumes: `previewSitemap` (W2-PROG-UI-1), `Notice`, `Button`.
- Produces: `<SitemapPreviewLine url include exclude subdomains maxPages onRaiseBudget />`.

- [ ] **Step 1: Create `web/src/components/progress/SitemapPreviewLine.tsx`:**

```tsx
import { useEffect, useState } from 'react'
import Button from '../ui/Button'
import Notice from '../ui/Notice'
import { previewSitemap } from '../../api/progress'
import type { SitemapPreview } from '../../types/progress'
import { budgetPercent, fileName, formatCount } from './tapeText'
import './progress.css'

const DEBOUNCE_MS = 600

interface Props {
  url: string
  include: string[]
  exclude: string[]
  subdomains: boolean
  /** Effective page budget; 0 = unlimited. */
  maxPages: number
  onRaiseBudget: (pages: number) => void
}

type Lookup = { key: string; kind: 'done'; preview: SitemapPreview } | { key: string; kind: 'error'; message: string }

/** Skips the lookup while the field can't be a site yet ("exa", "https://"). */
function looksLikeSite(u: string): boolean {
  return /^(https?:\/\/)?(localhost|[^\s/.]+\.[^\s/]+)/i.test(u)
}

export default function SitemapPreviewLine({ url, include, exclude, subdomains, maxPages, onRaiseBudget }: Props) {
  const target = url.trim()
  // One string for every input the lookup depends on: a stable effect dependency, and a tag
  // that lets a late answer for old inputs be ignored.
  const key = JSON.stringify([target, include, exclude, subdomains])
  const [lookup, setLookup] = useState<Lookup | null>(null)

  useEffect(() => {
    const [u, inc, exc, sub] = JSON.parse(key) as [string, string[], string[], boolean]
    if (!looksLikeSite(u)) return
    const ctrl = new AbortController()
    const timer = window.setTimeout(() => {
      previewSitemap({ url: u, include: inc, exclude: exc, allow_subdomains: sub }, ctrl.signal)
        .then((preview) => setLookup({ key, kind: 'done', preview }))
        .catch((err) => {
          if (!ctrl.signal.aborted) setLookup({ key, kind: 'error', message: String(err) })
        })
    }, DEBOUNCE_MS)
    return () => {
      window.clearTimeout(timer)
      ctrl.abort()
    }
  }, [key])

  if (!looksLikeSite(target)) return null
  const current = lookup?.key === key ? lookup : null
  if (!current) {
    return (
      <p className="sp-line" aria-live="polite">
        Looking for a sitemap…
      </p>
    )
  }
  if (current.kind === 'error') {
    return (
      <p className="sp-line" aria-live="polite">
        Couldn't check for a sitemap: {current.message}
      </p>
    )
  }
  const { preview } = current
  if (!preview.found) {
    return (
      <p className="sp-line" aria-live="polite">
        No sitemap found. Progress will count pages against the page budget instead.
      </p>
    )
  }
  const source = preview.sources.length > 0 ? fileName(preview.sources[0]) : 'a sitemap'
  const budgetLow = maxPages > 0 && preview.total > maxPages
  return (
    <div className="sp-wrap">
      <p className="sp-line" aria-live="polite">
        Found {source}
        {preview.via_robots ? ' via robots.txt' : ''}: {formatCount(preview.total)} {preview.total === 1 ? 'page' : 'pages'} in
        scope
      </p>
      {budgetLow && (
        <Notice
          tone="warning"
          action={
            <Button type="button" variant="secondary" onClick={() => onRaiseBudget(preview.total)}>
              Raise budget to {formatCount(preview.total)}
            </Button>
          }
        >
          <p>
            The page budget ({formatCount(maxPages)}) is below the sitemap total ({formatCount(preview.total)}). This crawl would
            cover at most {budgetPercent(maxPages, preview.total)}% of the sitemap.
          </p>
        </Notice>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Wire it in.** In `web/src/components/CrawlForm.tsx` add the imports

```tsx
import SitemapPreviewLine from './progress/SitemapPreviewLine'
import { DEFAULT_MAX_PAGES } from './progress/tapeText'
```

  and insert directly after the **Site URL** `</label>` (the `cf-field cf-url` label from
  W1-VIS-6):

```tsx
      <SitemapPreviewLine
        url={url}
        include={splitList(include)}
        exclude={splitList(exclude)}
        subdomains={subdomains}
        maxPages={maxPages.trim() === '' ? DEFAULT_MAX_PAGES : Number(maxPages)}
        onRaiseBudget={(pages) => setMaxPages(String(pages))}
      />
```

  (`splitList`, `include`, `exclude`, `subdomains`, `maxPages` and `setMaxPages` already exist in
  the visual system's form. W2-SET-3 later moves the form state into `FormValues` and carries
  this element over with `values.*` names; it is written out there.)

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: success.

- [ ] **Step 4: Manual check.** On New crawl, type `http://127.0.0.1:8099/`. After about half a
  second: "Found sitemap.xml via robots.txt: 280 pages in scope". Leave Max pages empty: no
  notice (default budget 500). Set Max pages to `100`: a warning notice with **Raise budget to
  280**; clicking it sets Max pages to 280 and the notice disappears. Add `p1\d\d` to Exclude:
  the count drops to 180. Type `https://example.com`: "No sitemap found…" (or the real count if
  it gains one). While typing `http://127.0.0.1:80`, DevTools → Network shows only one
  `sitemap-preview` request after typing stops, and cancelled earlier ones.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git restore internal/webserver/webui/dist/index.html
git add web/src/components/progress/SitemapPreviewLine.tsx web/src/components/CrawlForm.tsx
git commit -F - <<'EOF'
feat(web): preview the sitemap in the New crawl form and offer to raise the budget

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PROG-UI-5: Mini tapes and sitemap coverage in the Crawls list

**Files:**
- Create: `web/src/components/progress/MiniTape.tsx`
- Modify: `web/src/components/History.tsx`

**Interfaces:**
- Consumes: `CrawlListItemWithProgress`, `tape_preview`/`status_classes`/`sitemap_total`/
  `sitemap_fetched` (W1-PROG-8), `Tape` (`compact`).
- Produces: `<MiniTape item />`.

- [ ] **Step 1: Create `web/src/components/progress/MiniTape.tsx`:**

```tsx
import Tape from '../tape/Tape'
import type { CrawlListItemWithProgress } from '../../types/progress'
import { CLASS_LABEL, CLASS_ORDER, countsFromRecord, formatCount, scaleTape, totalOf } from './tapeText'
import './progress.css'

const MINI_TICKS = 60

/**
 * A small tape for one crawl list row. The server sends tape_preview (at most 160 ticks, worst
 * status per run, in fetch order); reports saved before tapes were stored fall back to a
 * status-only tape scaled from status_classes.
 */
export default function MiniTape({ item }: { item: CrawlListItemWithProgress }) {
  const counts = countsFromRecord(item.status_classes)
  const total = totalOf(counts)
  if (total === 0 && !item.tape_preview) return <span className="mini-tape-empty">—</span>
  const summary = `${formatCount(total)} pages by status: ${CLASS_ORDER.filter((c) => counts[c] > 0)
    .map((c) => `${formatCount(counts[c])} ${CLASS_LABEL[c].toLowerCase()}`)
    .join(', ')}`
  return <Tape tape={item.tape_preview || scaleTape(counts, MINI_TICKS)} summary={summary} compact />
}
```

- [ ] **Step 2: Edit `web/src/components/History.tsx`.**

  1. Imports: add

```tsx
import MiniTape from './progress/MiniTape'
import { percentOf, sitemapCoverageText } from './progress/tapeText'
import type { CrawlListItemWithProgress } from '../types/progress'
```

     and drop the `CrawlListItem` type import if it becomes unused.

  2. Change the item type everywhere it appears in this file from `CrawlListItem` to
     `CrawlListItemWithProgress` (the `historySortValue` parameter and
     `useState<CrawlListItemWithProgress[]>([])`). `listCrawls()` results are assignable because
     the extra fields are optional.

  3. In `historySortValue`, add before `default:`:

```tsx
    case 'sitemap':
      return item.sitemap_total ? percentOf(item.sitemap_fetched ?? 0, item.sitemap_total) : -1
```

  4. In `COLUMNS`, insert after the `pages_crawled` entry:

```tsx
  { key: 'sitemap', label: 'Sitemap coverage' },
```

  5. In the header row, after the mapped `COLUMNS` cells, add
     `<th scope="col">Pages by status</th>` (not sortable, so no `hist-sortable` class).

  6. In each body row, after the `pages_crawled` cell add

```tsx
              <td>{sitemapCoverageText(item.sitemap_total, item.sitemap_fetched)}</td>
```

     and as the last cell add

```tsx
              <td className="mini-tape-cell">
                <MiniTape item={item} />
              </td>
```

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: success.

- [ ] **Step 4: Manual check.** Open Crawls after the fixture crawls above. Each row shows a
  small tape in fetch order (mostly green, one orange tick for `/missing.html`) and "98 of 280
  (35%)" in Sitemap coverage; sorting by Sitemap coverage orders rows by percentage; a crawl of
  a site without a sitemap shows "—". A crawl that is still running shows a growing mini tape on
  reload. In DevTools → Network, `GET /api/crawls` items carry `tape_preview` (at most 160
  characters) and no `tape`. Saved crawls from before this change show "—" coverage and a
  status-only tape built from `status_classes`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git restore internal/webserver/webui/dist/index.html
git add web/src/components/progress/MiniTape.tsx web/src/components/History.tsx
git commit -F - <<'EOF'
feat(web): mini status tapes and sitemap coverage in the Crawls list

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PROG-UI-6: Docs and changelog for the progress UI

**Files:**
- Modify: `docs/web.md`, `docs/README.md` (generated), `CHANGELOG.md`

- [ ] **Step 1: `docs/web.md`.** Set `updated: 2026-10-13`. In "What the UI does", add these
  sentences (merge with the visual-system track's wording; keep each view's bullet):
  - New crawl: "Under the URL, a line reports the site's sitemap as you type (for example
    "Found sitemap.xml via robots.txt: 1,240 pages in scope"), and when the page budget is
    smaller, a notice offers **Raise budget to 1,240**."
  - Crawl (running): "The header shows the host, a phase pill (Reading sitemap, Crawling,
    Analyzing) and **Stop crawl**. Below it: a notice when the budget is below the sitemap
    total, the crawl tape (one tick per page, colored and sized by status class, notched when
    the page isn't in the sitemap, with outline ticks for sitemap pages not fetched yet or past
    the budget), counts (sitemap pages fetched of the total, fetched pages not in the sitemap,
    queue, elapsed time, request rate), status-class totals, and a "Fetching now" table of the
    last eight pages with status and timing."
  - Crawl (finished): "The tape sits at the top of the report, followed by the sitemap
    coverage line. The report saves the tape (`coverage.tape` / `tape_off`), so a crawl reopened
    later, even after a server restart, shows the same tape; reports saved before this version
    get a status-only tape."
  - Crawls: "Each row shows a small tape (a 160-tick preview, worst status per tick) and its
    sitemap coverage."

- [ ] **Step 2: `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`, add:

```markdown
- The web UI shows a running crawl live: the crawl tape, pages fetched against the sitemap, queue, elapsed time, request rate and the last pages fetched. The New crawl form previews the sitemap and offers to raise a page budget below it, finished reports open with the tape and sitemap coverage, and the Crawls list shows a small tape and sitemap coverage per crawl.
```

- [ ] **Step 3: Regenerate and check.** `make docs && make check`
  Expected: both succeed.

- [ ] **Step 4: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -F - <<'EOF'
docs(web): describe the live crawl view, sitemap preview and crawl tapes

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

### Track: pause backend

#### Task W2-PAUSE-1: A pausable frontier

**Files:**
- Modify: `internal/crawler/frontier.go`
- Test: `internal/crawler/frontier_test.go`

**Interfaces:**
- Produces: `func (f *frontier) setPaused(p bool)`, `func (f *frontier) pushFront(t task)`;
  `next` blocks while paused, still returns `false` on cancellation or exhaustion.

- [ ] **Step 1: Write the failing tests.** Append to `internal/crawler/frontier_test.go`:

```go
func TestFrontierPausedBlocksNextUntilResumed(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	f.setPaused(true)
	got := make(chan task, 1)
	go func() {
		if tk, ok := f.next(context.Background()); ok {
			got <- tk
		}
	}()
	select {
	case tk := <-got:
		t.Fatalf("next returned %q while paused", tk.url)
	case <-time.After(100 * time.Millisecond):
	}
	f.setPaused(false)
	select {
	case tk := <-got:
		if tk.url != "a" {
			t.Fatalf("next = %q, want a", tk.url)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("next did not wake when the frontier resumed")
	}
}

func TestFrontierPausedStillWakesOnCancel(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	f.setPaused(true)
	ctx, cancel := context.WithCancel(context.Background())
	stop := f.watchCancellation(ctx)
	defer stop()
	done := make(chan bool, 1)
	go func() {
		_, ok := f.next(ctx)
		done <- ok
	}()
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("next returned a task after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not wake a paused next()")
	}
}

func TestFrontierPausedWithNothingPendingIsExhausted(t *testing.T) {
	f := newFrontier()
	f.setPaused(true)
	done := make(chan bool, 1)
	go func() {
		_, ok := f.next(context.Background())
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("next returned a task from an empty frontier")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a paused, empty frontier must still report exhaustion")
	}
}

func TestFrontierPushFrontRequeuesAtHead(t *testing.T) {
	f := newFrontier()
	ctx := context.Background()
	f.push(task{url: "a"})
	f.push(task{url: "b"})
	a, _ := f.next(ctx)
	f.pushFront(a) // the worker hands the task back...
	f.taskDone()   // ...then ends its turn for it
	for _, want := range []string{"a", "b"} {
		got, ok := f.next(ctx)
		if !ok || got.url != want {
			t.Fatalf("next = %+v ok=%v, want %q", got, ok, want)
		}
		f.taskDone()
	}
	if _, ok := f.next(ctx); ok {
		t.Fatal("expected exhaustion after both tasks")
	}
}
```

- [ ] **Step 2: Run and watch them fail.** `go test ./internal/crawler -run TestFrontier`
  Expected: build failure, `f.setPaused undefined`.

- [ ] **Step 3: Implement** in `internal/crawler/frontier.go`. Add the field to `frontier`:

```go
	// paused makes next hold back queued tasks (Engine.Pause). Cancellation and exhaustion
	// still end next, so a paused crawl can be stopped and a crawl with nothing left ends.
	paused bool
```

  Replace `next` with:

```go
// next blocks until a task is available, ctx is done, or the frontier is exhausted (nothing
// queued and nothing pending), returning ok=false in the latter two cases. While paused it
// hands out nothing but keeps waiting. Checking ctx first on every wake means a cancellation
// abandons any remaining queued work immediately rather than draining it.
func (f *frontier) next(ctx context.Context) (task, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for {
		if ctx.Err() != nil {
			return task{}, false
		}
		// pending counts queued tasks too, so pending == 0 means the queue is empty.
		if f.pending == 0 {
			return task{}, false
		}
		if !f.paused && len(f.queue) > 0 {
			t := f.queue[0]
			f.queue = f.queue[1:]
			return t, true
		}
		f.cond.Wait()
	}
}
```

  Add after `queued`:

```go
// setPaused holds back (true) or releases (false) queued tasks and wakes every waiter.
func (f *frontier) setPaused(p bool) {
	f.mu.Lock()
	f.paused = p
	f.mu.Unlock()
	f.cond.Broadcast()
}

// pushFront puts a task a worker took but did not process back at the head of the queue and
// marks it pending again. The worker must still call taskDone for its original turn; pending
// never touches zero in between because pushFront increments first.
func (f *frontier) pushFront(t task) {
	f.mu.Lock()
	f.pending++
	f.queue = append([]task{t}, f.queue...)
	f.mu.Unlock()
	f.cond.Signal()
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/crawler -run 'TestFrontier|TestEngine'`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/frontier.go internal/crawler/frontier_test.go
git commit -F - <<'EOF'
feat(crawler): let the frontier hold back work while paused

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PAUSE-2: activeClock, a MaxDuration timer that stops while paused

**Files:**
- Create: `internal/crawler/clock.go`, `internal/crawler/clock_test.go`

**Interfaces:**
- Produces: `newActiveClock(limit time.Duration, onLimit func()) *activeClock` with
  `start()`, `stop()`, `halt()`, `elapsed() time.Duration`.

- [ ] **Step 1: Write the failing test** `internal/crawler/clock_test.go`:

```go
package crawler

import (
	"testing"
	"time"
)

func TestActiveClockExcludesStoppedTime(t *testing.T) {
	c := newActiveClock(0, nil)
	c.start()
	time.Sleep(20 * time.Millisecond)
	c.stop()
	frozen := c.elapsed()
	time.Sleep(400 * time.Millisecond)
	if got := c.elapsed(); got != frozen {
		t.Fatalf("elapsed moved from %v to %v while stopped", frozen, got)
	}
	c.start()
	time.Sleep(20 * time.Millisecond)
	c.halt()
	got := c.elapsed()
	if got < 40*time.Millisecond {
		t.Errorf("elapsed = %v, want at least the 40ms of running time", got)
	}
	if got >= 400*time.Millisecond {
		t.Errorf("elapsed = %v, want the 400ms stop excluded", got)
	}
}

func TestActiveClockFiresOnlyOnActiveTime(t *testing.T) {
	fired := make(chan struct{})
	c := newActiveClock(200*time.Millisecond, func() { close(fired) })
	c.start()
	time.Sleep(20 * time.Millisecond)
	c.stop()
	select {
	case <-fired:
		t.Fatal("a stopped clock fired")
	case <-time.After(400 * time.Millisecond):
	}
	c.start()
	select {
	case <-fired:
	case <-time.After(3 * time.Second):
		t.Fatal("clock did not fire once the remaining active time ran out")
	}
	c.halt()
}

func TestActiveClockHaltIsFinal(t *testing.T) {
	c := newActiveClock(0, nil)
	c.halt()
	c.start()
	time.Sleep(20 * time.Millisecond)
	if got := c.elapsed(); got != 0 {
		t.Errorf("elapsed after halt then start = %v, want 0", got)
	}
}
```

- [ ] **Step 2: Run and watch it fail.** `go test ./internal/crawler -run TestActiveClock`
  Expected: build failure, `undefined: newActiveClock`.

- [ ] **Step 3: Implement** `internal/crawler/clock.go`:

```go
package crawler

import (
	"sync"
	"time"
)

// activeClock measures a crawl's active time (wall-clock time minus time spent paused) and
// calls onLimit once that reaches limit (0 = no limit). It replaces the context.WithTimeout
// MaxDuration used to be, which kept counting while a crawl sat paused.
//
// onLimit runs on a timer goroutine and must not take c.mu (Engine passes a context cancel).
type activeClock struct {
	limit   time.Duration
	onLimit func()

	mu     sync.Mutex
	acc    time.Duration // active time banked by earlier runs
	since  time.Time     // start of the current run; zero while stopped
	timer  *time.Timer
	halted bool
}

func newActiveClock(limit time.Duration, onLimit func()) *activeClock {
	return &activeClock{limit: limit, onLimit: onLimit}
}

// start begins (or resumes) counting. A no-op while running or after halt.
func (c *activeClock) start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.halted || !c.since.IsZero() {
		return
	}
	c.since = time.Now()
	if c.limit > 0 && c.onLimit != nil {
		remaining := c.limit - c.acc
		if remaining < 0 {
			remaining = 0
		}
		c.timer = time.AfterFunc(remaining, c.onLimit)
	}
}

// stop pauses counting and disarms the limit timer.
func (c *activeClock) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *activeClock) stopLocked() {
	if c.since.IsZero() {
		return
	}
	c.acc += time.Since(c.since)
	c.since = time.Time{}
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

// halt stops the clock for good; elapsed keeps returning the final value.
func (c *activeClock) halt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
	c.halted = true
}

// elapsed reports the active time so far.
func (c *activeClock) elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := c.acc
	if !c.since.IsZero() {
		d += time.Since(c.since)
	}
	return d
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/crawler -run TestActiveClock -count=3`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/clock.go internal/crawler/clock_test.go
git commit -F - <<'EOF'
feat(crawler): add an active-time clock that stops while a crawl is paused

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PAUSE-3: Engine.Pause / Resume, MaxDuration in active time

**Files:**
- Create: `internal/crawler/pause.go`, `internal/crawler/pause_test.go`
- Modify: `internal/crawler/engine.go`, `internal/crawler/progress.go`,
  `internal/crawler/adaptive.go`, `internal/crawler/types.go` (`MaxDuration` doc comment)

**Interfaces:**
- Consumes: `frontier.setPaused/pushFront` (W2-PAUSE-1), `activeClock` (W2-PAUSE-2).
- Produces:
  - `func (e *Engine) Pause()`, `func (e *Engine) Resume(rate float64)` (contract; rate 0 keeps
    the current rate)
  - `func (e *Engine) Paused() bool`, `func (e *Engine) ActiveTime() time.Duration` (gaps,
    needed by `runner.Control`)
  - `const PauseReasonUser = "user"`, `PauseReasonRateLimited = "rate_limited"`
  - `func (a *AdaptiveLimiter) SetRate(r float64)`, `func (a *AdaptiveLimiter) ResetRecent()`
  - unexported `pauseState`, `pauseWith`, `emitState`, `beginRun`, `endRun`, `errMaxDuration`.

- [ ] **Step 1: Write the failing tests** `internal/crawler/pause_test.go`:

```go
package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// hubSite serves a home page linking to n leaf pages and counts leaf fetches.
func hubSite(t *testing.T, n int, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var b strings.Builder
			b.WriteString("<html><head><title>Hub</title></head><body>")
			for i := range n {
				fmt.Fprintf(&b, `<a href="/p%d">p%d</a>`, i, i)
			}
			b.WriteString("</body></html>")
			fmt.Fprint(w, b.String())
		case "/robots.txt":
			http.NotFound(w, r)
		default:
			hits.Add(1)
			fmt.Fprint(w, "<html><head><title>Leaf</title></head><body>leaf</body></html>")
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// startPausable starts a Concurrency-1 crawl and returns once the hub page is recorded.
func startPausable(t *testing.T, ts *httptest.Server, ctx context.Context, onProgress func(Progress)) (*Engine, chan *Result) {
	t.Helper()
	first := make(chan struct{}, 1)
	opts := DefaultOptions()
	opts.Concurrency = 1
	opts.OnProgress = func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
		if p.Fetched == 1 {
			select {
			case first <- struct{}{}:
			default:
			}
		}
	}
	e := New(opts, NewHTTPFetcher(opts))
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(ctx, ts.URL)
		done <- r
	}()
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("the hub page was never recorded")
	}
	return e, done
}

func TestEnginePauseStopsNewFetchesAndResumeFinishes(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 20, &hits)
	var (
		mu     sync.Mutex
		events []Progress
	)
	e, done := startPausable(t, ts, context.Background(), func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	})

	e.Pause()
	if !e.Paused() {
		t.Fatal("Paused() = false right after Pause()")
	}
	time.Sleep(100 * time.Millisecond) // a fetch already past the frontier may still land
	before := hits.Load()
	time.Sleep(300 * time.Millisecond)
	if after := hits.Load(); after != before {
		t.Fatalf("%d leaf pages fetched while paused", after-before)
	}
	if before > 1 {
		t.Errorf("%d leaf pages fetched after Pause, want at most the one already in flight", before)
	}
	select {
	case <-done:
		t.Fatal("the crawl returned while paused")
	default:
	}

	e.Resume(0)
	var r *Result
	select {
	case r = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the crawl did not finish after Resume")
	}
	if len(r.Pages) != 21 || hits.Load() != 20 || !r.Coverage.Complete {
		t.Errorf("pages=%d hits=%d complete=%v, want 21 / 20 / true", len(r.Pages), hits.Load(), r.Coverage.Complete)
	}

	mu.Lock()
	defer mu.Unlock()
	sawPause, sawResume := false, false
	for _, ev := range events {
		if ev.Paused && ev.PauseReason == PauseReasonUser {
			sawPause = true
		}
		if sawPause && !ev.Paused {
			sawResume = true
		}
	}
	if !sawPause || !sawResume {
		t.Errorf("progress events: saw pause %v, saw resume %v; want both", sawPause, sawResume)
	}
}

func TestEngineCancelWhilePausedReturnsPartialResult(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 20, &hits)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e, done := startPausable(t, ts, ctx, nil)
	e.Pause()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case r := <-done:
		if r == nil || !r.Coverage.Interrupted || len(r.Pages) < 1 || len(r.Pages) > 2 {
			t.Fatalf("result after cancel while paused: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not wake the paused crawl")
	}
}

func TestEngineMaxDurationExcludesPausedTime(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 1, &hits)
	opts := DefaultOptions()
	opts.MaxDuration = 250 * time.Millisecond
	e := New(opts, NewHTTPFetcher(opts))
	e.Pause() // queued: the crawl starts paused
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(context.Background(), ts.URL)
		done <- r
	}()
	time.Sleep(600 * time.Millisecond) // well past MaxDuration, all of it paused
	e.Resume(0)
	select {
	case r := <-done:
		if r.Coverage.DurationLimitReached || !r.Coverage.Complete || len(r.Pages) != 2 {
			t.Fatalf("coverage=%+v pages=%d, want a complete 2-page crawl", r.Coverage, len(r.Pages))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the crawl did not finish after Resume")
	}
	if at := e.ActiveTime(); at >= 250*time.Millisecond {
		t.Errorf("ActiveTime = %v, want the 600ms pause excluded", at)
	}
}

func TestEnginePauseAfterCrawlIsNoOp(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	opts := DefaultOptions()
	opts.MaxDepth = 1
	e := New(opts, NewHTTPFetcher(opts))
	if _, err := e.Crawl(context.Background(), ts.URL); err != nil {
		t.Fatalf("crawl: %v", err)
	}
	e.Pause()
	if e.Paused() {
		t.Error("Pause after the crawl ended should do nothing")
	}
	e.Resume(5) // must not block or panic
}

func TestEngineResumeAppliesNewRate(t *testing.T) {
	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	e.Pause()
	e.Resume(2.5)
	if got := e.limiter.EffectiveRate(); got != 2.5 {
		t.Errorf("EffectiveRate after Resume(2.5) = %v, want 2.5", got)
	}
	e.Pause()
	e.Resume(0)
	if got := e.limiter.EffectiveRate(); got != 2.5 {
		t.Errorf("Resume(0) changed the rate to %v, want it kept at 2.5", got)
	}
}
```

  Append to `internal/crawler/adaptive_test.go`:

```go
func TestAdaptiveLimiterSetRateAndResetRecent(t *testing.T) {
	l := NewAdaptiveLimiter(0, true)
	l.OnResponse(http.StatusTooManyRequests, nil)
	l.SetRate(0.25)
	if got := l.EffectiveRate(); got != 0.25 {
		t.Fatalf("EffectiveRate = %v, want 0.25", got)
	}
	if got := float64(l.limiter.Limit()); got != 0.25 {
		t.Fatalf("limit = %v, want 0.25", got)
	}
	l.SetRate(0) // ignored
	if got := l.EffectiveRate(); got != 0.25 {
		t.Fatalf("SetRate(0) changed the rate to %v", got)
	}
	l.ResetRecent()
	if got := l.RecentThrottled(); got != 0 {
		t.Fatalf("RecentThrottled after ResetRecent = %d, want 0", got)
	}
}
```

- [ ] **Step 2: Run and watch them fail.**
  `go test ./internal/crawler -run 'TestEngine(Pause|Cancel|MaxDurationExcludes|Resume)|TestAdaptiveLimiterSetRate'`
  Expected: build failure, `e.Pause undefined`.

- [ ] **Step 3: Implement.**

  Append to `internal/crawler/adaptive.go`:

```go
// SetRate replaces the current rate (an operator resuming at a chosen speed). r <= 0 is
// ignored. Later 429/503s halve from the new rate.
func (a *AdaptiveLimiter) SetRate(r float64) {
	if r <= 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.curRate = r
	a.limiter.SetLimit(rate.Limit(r))
}

// ResetRecent forgets the recent 429/503 window, so a resumed crawl gets a fresh minute before
// auto-pause can trigger again.
func (a *AdaptiveLimiter) ResetRecent() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.recent = nil
}
```

  Create `internal/crawler/pause.go`:

```go
package crawler

import (
	"errors"
	"sync"
	"time"
)

// Pause reasons reported in Progress.PauseReason.
const (
	PauseReasonUser        = "user"
	PauseReasonRateLimited = "rate_limited"
)

// errMaxDuration is the crawl context's cancellation cause when MaxDuration of active time
// runs out; Crawl uses it to tell the duration limit apart from an outside cancellation.
var errMaxDuration = errors.New("crawl reached its max duration")

// pauseState is the engine's pause bookkeeping. Lock order: ps.mu may take frontier.mu,
// activeClock.mu and limiter.mu; nothing that holds those takes ps.mu, and ps.mu is always
// released before a progress event is emitted.
type pauseState struct {
	mu         sync.Mutex
	paused     bool
	reason     string
	retryAfter int
	fr         *frontier    // the running crawl's frontier; nil outside Crawl
	clock      *activeClock // the latest crawl's clock; kept after Crawl for ActiveTime
	running    bool         // Crawl is between beginRun and endRun
	finished   bool         // Crawl has returned; Pause and Resume do nothing from then on
}

// Pause stops handing out new fetches. Requests already in flight finish and are recorded.
// Called before Crawl, the crawl starts paused; called after Crawl returned, it does nothing.
// Cancelling the crawl's context still ends a paused crawl.
func (e *Engine) Pause() { e.pauseWith(PauseReasonUser, 0) }

// pauseWith pauses for reason and reports whether this call paused the crawl.
func (e *Engine) pauseWith(reason string, retryAfter int) bool {
	e.ps.mu.Lock()
	if e.ps.finished || e.ps.paused {
		e.ps.mu.Unlock()
		return false
	}
	e.ps.paused, e.ps.reason, e.ps.retryAfter = true, reason, retryAfter
	if e.ps.fr != nil {
		e.ps.fr.setPaused(true)
	}
	if e.ps.running {
		e.ps.clock.stop()
	}
	e.ps.mu.Unlock()
	e.logf("crawl paused (%s)", reason)
	e.emitState()
	return true
}

// Resume continues a paused crawl. rate > 0 replaces the request rate; 0 keeps it. It also
// clears the recent-429 window so auto-pause doesn't fire again on the old responses.
func (e *Engine) Resume(rate float64) {
	e.ps.mu.Lock()
	if e.ps.finished || !e.ps.paused {
		e.ps.mu.Unlock()
		return
	}
	e.ps.paused, e.ps.reason, e.ps.retryAfter = false, "", 0
	if rate > 0 {
		e.limiter.SetRate(rate)
	}
	e.limiter.ResetRecent()
	if e.ps.fr != nil {
		e.ps.fr.setPaused(false)
	}
	if e.ps.running {
		e.ps.clock.start()
	}
	e.ps.mu.Unlock()
	e.logf("crawl resumed")
	e.emitState()
}

// Paused reports whether the crawl is paused (or queued to start paused).
func (e *Engine) Paused() bool {
	e.ps.mu.Lock()
	defer e.ps.mu.Unlock()
	return e.ps.paused
}

// ActiveTime reports how long the current or last crawl has been running, excluding pauses.
func (e *Engine) ActiveTime() time.Duration {
	e.ps.mu.Lock()
	c := e.ps.clock
	e.ps.mu.Unlock()
	if c == nil {
		return 0
	}
	return c.elapsed()
}

// beginRun attaches a starting crawl's frontier and clock, applying a pause queued earlier.
func (e *Engine) beginRun(fr *frontier, clock *activeClock) {
	e.ps.mu.Lock()
	defer e.ps.mu.Unlock()
	e.ps.fr, e.ps.clock, e.ps.running, e.ps.finished = fr, clock, true, false
	if e.ps.paused {
		fr.setPaused(true)
	} else {
		clock.start()
	}
}

// endRun detaches the finished crawl and reports whether it ended while paused (all queued
// work done or the context canceled), in which case the caller emits a final state event.
func (e *Engine) endRun() (wasPaused bool) {
	e.ps.mu.Lock()
	defer e.ps.mu.Unlock()
	wasPaused = e.ps.paused
	e.ps.clock.halt()
	e.ps.fr, e.ps.running, e.ps.finished = nil, false, true
	e.ps.paused, e.ps.reason, e.ps.retryAfter = false, "", 0
	return wasPaused
}

// isPaused is Paused for the hot path.
func (e *Engine) isPaused() bool { return e.Paused() }
```

  In `internal/crawler/progress.go`, replace `fillLive` with the version that adds the pause
  fields, and add `emitState`:

```go
// fillLive copies the fields that change between pages: rate limit, recent throttling, and
// pause state. Takes limiter.mu and ps.mu in turn, never together.
func (e *Engine) fillLive(p *Progress) {
	p.Rate = e.limiter.EffectiveRate()
	p.Throttled = e.limiter.RecentThrottled()
	e.ps.mu.Lock()
	p.Paused, p.PauseReason, p.RetryAfterSeconds = e.ps.paused, e.ps.reason, e.ps.retryAfter
	e.ps.mu.Unlock()
}

// emitState re-sends the last page snapshot with fresh live fields after a pause or resume.
// Fetched does not change, so consumers add no tape tick. Callers must not hold ps.mu.
func (e *Engine) emitState() {
	if e.opts.OnProgress == nil {
		return
	}
	e.progMu.Lock()
	defer e.progMu.Unlock()
	p := e.lastProgress
	e.fillLive(&p)
	e.lastProgress = p
	e.opts.OnProgress(p)
}
```

  In `internal/crawler/engine.go`:

  1. Add a field to `Engine` after `lastProgress Progress`:

```go

	// ps is the pause state shared by Pause/Resume and the running crawl.
	ps pauseState
```

  2. Replace the top of `Crawl`

```go
	if e.opts.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.opts.MaxDuration)
		defer cancel()
	}
```

  with

```go
	// MaxDuration counts active time only: the clock stops while the crawl is paused and
	// cancels the crawl with errMaxDuration once the budget is used up.
	ctx, cancelRun := context.WithCancelCause(ctx)
	defer cancelRun(nil)
	clock := newActiveClock(e.opts.MaxDuration, func() { cancelRun(errMaxDuration) })
```

  3. Right after `defer stopWatching()`, add:

```go
	e.beginRun(fr, clock)
```

  4. In `process`, replace

```go
		if err := e.limiter.Wait(ctx); err != nil {
			return
		}
```

  with

```go
		if err := e.limiter.Wait(ctx); err != nil {
			return
		}
		if e.isPaused() {
			// Paused while this task waited for the rate limiter: hand it back so it is the
			// first fetch after Resume, instead of sending one more request to a server that
			// may just have asked us to stop.
			fr.pushFront(t)
			return
		}
```

  5. Replace

```go
	wg.Wait()
```

  with

```go
	wg.Wait()
	if e.endRun() {
		e.emitState() // the crawl ended while paused: report it as no longer paused
	}
```

  6. Replace

```go
	durationLimitReached := errors.Is(ctx.Err(), context.DeadlineExceeded)
```

  with

```go
	durationLimitReached := errors.Is(context.Cause(ctx), errMaxDuration)
```

  In `internal/crawler/types.go`, replace the `MaxDuration` comment with:

```go
	// MaxDuration bounds the crawl's active time: wall-clock time minus time spent paused.
	// Zero means unlimited. When it elapses, the crawl stops early and still returns
	// everything fetched so far as a partial result (see Coverage.DurationLimitReached), the
	// same way a canceled context (e.g. an operator's Ctrl-C) does.
```

- [ ] **Step 4: Run.** `go test -race ./internal/crawler -count=2`
  Expected: `ok`, including the existing `TestEngineMaxDurationStopsCrawlEarly` and
  `TestEngineCanceledContextReturnsPartialResultNotError`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/pause.go internal/crawler/pause_test.go internal/crawler/engine.go internal/crawler/progress.go internal/crawler/adaptive.go internal/crawler/adaptive_test.go internal/crawler/types.go
git commit -F - <<'EOF'
feat(crawler): pause and resume a crawl; max duration counts active time only

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PAUSE-4: Auto-pause on sustained rate limiting (`Options.PauseOnThrottle`)

**Files:**
- Modify: `internal/crawler/types.go` (`Options`), `internal/crawler/pause.go`,
  `internal/crawler/engine.go`, `internal/crawler/adaptive.go`
- Test: `internal/crawler/pause_test.go`

**Interfaces:**
- Produces: `Options.PauseOnThrottle bool` (contract); unexported `autoPauseThreshold = 10`,
  `autoPauseMinBackoffs = 2`, `(*Engine).maybeAutoPause(status int)`,
  `AdaptiveLimiter.debounce` (defaults to `backoffDebounce`; tests set it to 0).

- [ ] **Step 1: Write the failing tests.** Append to `internal/crawler/pause_test.go`:

```go
func TestMaybeAutoPauseNeedsOptionBackoffsAndThreshold(t *testing.T) {
	burst := func(e *Engine, n int) {
		for range n {
			e.limiter.OnResponse(http.StatusTooManyRequests, nil)
		}
	}

	off := DefaultOptions()
	e := New(off, NewHTTPFetcher(off))
	e.limiter.debounce = 0
	burst(e, 10)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused without PauseOnThrottle")
	}

	on := DefaultOptions()
	on.PauseOnThrottle = true
	noBackoff := on
	noBackoff.AdaptiveDelay = false
	e = New(noBackoff, NewHTTPFetcher(noBackoff))
	burst(e, 10)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused although adaptive delay never lowered the rate")
	}

	e = New(on, NewHTTPFetcher(on)) // default 2s debounce: ten rapid 429s are one backoff
	burst(e, 10)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused after a single backoff")
	}

	e = New(on, NewHTTPFetcher(on))
	e.limiter.debounce = 0
	burst(e, 9)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused below the threshold of 10")
	}
	burst(e, 1)
	e.maybeAutoPause(http.StatusOK)
	if e.Paused() {
		t.Error("a 200 must not trigger auto-pause")
	}
	e.maybeAutoPause(http.StatusTooManyRequests)
	if !e.Paused() || e.ps.reason != PauseReasonRateLimited {
		t.Errorf("paused=%v reason=%q, want paused for %q", e.Paused(), e.ps.reason, PauseReasonRateLimited)
	}
}

// TestEngineAutoPausesOnSustained429 runs a real crawl against a server that answers 429 to
// everything but the home page, and expects the crawl to pause itself, report the server's
// Retry-After, stop fetching, and still end cleanly when canceled.
func TestEngineAutoPausesOnSustained429(t *testing.T) {
	var leafHits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var b strings.Builder
			b.WriteString("<html><body>")
			for i := range 40 {
				fmt.Fprintf(&b, `<a href="/p%d">p%d</a>`, i, i)
			}
			b.WriteString("</body></html>")
			fmt.Fprint(w, b.String())
		case "/robots.txt":
			http.NotFound(w, r)
		default:
			// Only the tenth refusal names a long wait: earlier ones would otherwise slow the
			// test to one request every two minutes.
			if leafHits.Add(1) >= 10 {
				w.Header().Set("Retry-After", "120")
			}
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer ts.Close()

	opts := DefaultOptions()
	opts.Concurrency = 1
	opts.RatePerSecond = 4000 // nine halvings from here still leave ~8 req/s
	opts.PauseOnThrottle = true
	paused := make(chan Progress, 1)
	opts.OnProgress = func(p Progress) {
		if p.Paused {
			select {
			case paused <- p:
			default:
			}
		}
	}
	e := New(opts, NewHTTPFetcher(opts))
	e.limiter.debounce = 0 // back off on every 429, not once per 2s

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(ctx, ts.URL)
		done <- r
	}()

	var p Progress
	select {
	case p = <-paused:
	case <-time.After(10 * time.Second):
		t.Fatal("the crawl never paused itself")
	}
	if p.PauseReason != PauseReasonRateLimited || p.RetryAfterSeconds != 120 || p.Throttled < 10 {
		t.Errorf("pause event = %+v, want reason rate_limited, Retry-After 120, >= 10 throttled", p)
	}
	atPause := leafHits.Load()
	time.Sleep(300 * time.Millisecond)
	if got := leafHits.Load(); got != atPause {
		t.Errorf("%d more requests after the auto-pause", got-atPause)
	}

	cancel()
	select {
	case r := <-done:
		if !r.Coverage.Interrupted || len(r.Pages) != int(atPause)+1 {
			t.Errorf("interrupted=%v pages=%d, want true / %d", r.Coverage.Interrupted, len(r.Pages), atPause+1)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not end the auto-paused crawl")
	}
}
```

- [ ] **Step 2: Run and watch them fail.**
  `go test ./internal/crawler -run 'TestMaybeAutoPause|TestEngineAutoPauses'`
  Expected: build failure, `e.limiter.debounce undefined` and `e.maybeAutoPause undefined`.

- [ ] **Step 3: Implement.**

  `internal/crawler/adaptive.go`: add the field to `AdaptiveLimiter` (in the `mu` group):

```go
	// debounce is backoffDebounce; a field so engine tests can turn it off.
	debounce time.Duration
```

  set it in `NewAdaptiveLimiter`'s literal (`debounce: backoffDebounce,`), and in
  `OnResponse` replace `now.Sub(a.lastAdjust) < backoffDebounce` with
  `now.Sub(a.lastAdjust) < a.debounce`.

  `internal/crawler/types.go`: add to `Options` after `MaxDuration`:

```go
	// PauseOnThrottle makes the engine pause itself when the server keeps rate limiting it:
	// once adaptive delay has lowered the rate at least twice, ten or more 429/503 responses
	// within a minute pause the crawl (Progress.PauseReason "rate_limited", with the latest
	// Retry-After). Only useful when someone can call Resume: the web UI turns it on, the CLI
	// and MCP leave it off.
	PauseOnThrottle bool
```

  `internal/crawler/pause.go`: add

```go
// Auto-pause tuning (Options.PauseOnThrottle).
const (
	autoPauseThreshold   = 10 // 429/503 responses within throttleWindow
	autoPauseMinBackoffs = 2  // adaptive-delay adjustments that must already have happened
)

// maybeAutoPause pauses the crawl when PauseOnThrottle is set and the server keeps answering
// 429/503 even after adaptive delay backed off. Called by a worker after each response; the
// worker holds no engine lock.
func (e *Engine) maybeAutoPause(status int) {
	if !e.opts.PauseOnThrottle {
		return
	}
	if status != http.StatusTooManyRequests && status != http.StatusServiceUnavailable {
		return
	}
	if e.limiter.ThrottleCount() < autoPauseMinBackoffs {
		return
	}
	n := e.limiter.RecentThrottled()
	if n < autoPauseThreshold {
		return
	}
	if e.pauseWith(PauseReasonRateLimited, e.limiter.LastRetryAfter()) {
		e.logf("%d HTTP 429/503 responses in the last minute after %d backoffs — pausing the crawl", n, e.limiter.ThrottleCount())
	}
}
```

  and add `"net/http"` to its imports.

  `internal/crawler/engine.go`, in `process`, directly after the `if adjusted, next := e.limiter.OnResponse(...) { ... }` block, add:

```go
		e.maybeAutoPause(page.StatusCode)
```

- [ ] **Step 4: Run.** `go test -race ./internal/crawler -count=2`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/crawler/adaptive.go internal/crawler/types.go internal/crawler/pause.go internal/crawler/engine.go internal/crawler/pause_test.go
git commit -F - <<'EOF'
feat(crawler): pause automatically after sustained 429/503 responses

Opt-in through Options.PauseOnThrottle: ten or more 429/503s within a minute,
after adaptive delay has backed off at least twice, pause the crawl and record
the server's Retry-After.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PAUSE-5: runner.Control, WithControl, WithPauseOnThrottle

**Files:**
- Create: `internal/runner/control.go`, `internal/runner/control_test.go`
- Modify: `internal/runner/options.go`, `internal/runner/runner.go`

**Interfaces:**
- Consumes: `Engine.Pause/Resume/Paused/ActiveTime` (W2-PAUSE-3), `Options.PauseOnThrottle`.
- Produces (contract): `type Control struct{ /* unexported */ }`, `func NewControl() *Control`,
  `func (c *Control) Pause() bool`, `func (c *Control) Resume(rate float64) bool`,
  `func WithControl(c *Control) RunOption`, `func WithPauseOnThrottle() RunOption`.
  Gap: `func (c *Control) Active() time.Duration` (lock-free, for `progress.active_ms`).

- [ ] **Step 1: Write the failing tests** `internal/runner/control_test.go`:

```go
package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

func newEngine() *crawler.Engine {
	opts := crawler.DefaultOptions()
	return crawler.New(opts, crawler.NewHTTPFetcher(opts))
}

func TestControlQueuesPauseBeforeTheCrawlPhase(t *testing.T) {
	c := NewControl()
	if !c.Pause() {
		t.Fatal("Pause before the crawl phase should be queued and return true")
	}
	e := newEngine()
	c.attach(e)
	if !e.Paused() {
		t.Fatal("a queued pause must reach the engine when the crawl phase starts")
	}
	if !c.Resume(0) || e.Paused() {
		t.Fatal("Resume during the crawl phase should resume the engine")
	}
	c.detach()
	if c.Pause() || c.Resume(0) {
		t.Error("Pause/Resume after the crawl phase must return false")
	}
}

func TestControlResumeBeforeCrawlCancelsQueuedPause(t *testing.T) {
	c := NewControl()
	c.Pause()
	c.Resume(0)
	e := newEngine()
	c.attach(e)
	if e.Paused() {
		t.Error("a pause cancelled before the crawl started must not reach the engine")
	}
	c.detach()
}

func TestControlNilIsSafe(t *testing.T) {
	var c *Control
	c.attach(newEngine())
	c.detach()
}

func TestWithPauseOnThrottleSetsTheSetting(t *testing.T) {
	var s runSettings
	WithPauseOnThrottle()(&s)
	if !s.pauseOnThrottle {
		t.Error("WithPauseOnThrottle did not turn the setting on")
	}
}

func TestRunWithControlPausesAndResumes(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	ctl := NewControl()
	ctl.Pause() // queued: the crawl phase starts paused

	var rep *report.Report
	done := make(chan error, 1)
	go func() {
		r, err := Run(context.Background(), config.Default(), ts.URL, WithControl(ctl))
		rep = r
		done <- err
	}()
	time.Sleep(300 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Run finished while its crawl was paused")
	default:
	}
	if !ctl.Resume(0) {
		t.Fatal("Resume during the crawl phase returned false")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not finish after Resume")
	}
	if rep.PagesCrawled != 4 {
		t.Errorf("PagesCrawled = %d, want 4", rep.PagesCrawled)
	}
	if ctl.Pause() {
		t.Error("Pause after Run returned should be false")
	}
	if ctl.Active() <= 0 || ctl.Active() >= 300*time.Millisecond {
		t.Errorf("Active = %v, want the crawl's active time without the 300ms pause", ctl.Active())
	}
}

// A pause queued during the sitemap phase, then a cancel while the crawl still sits paused (the
// web UI's Pause followed by Stop crawl before anything was fetched): Run must return its
// partial report promptly instead of waiting for a Resume that never comes.
func TestRunCanceledWhileQueuedPauseHolds(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	ctl := NewControl()
	ctl.Pause()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, config.Default(), ts.URL, WithControl(ctl))
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v (a canceled crawl returns a partial report, not an error)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel while its crawl was paused")
	}
	if ctl.Pause() || ctl.Resume(0) {
		t.Error("Pause/Resume after Run returned must be false")
	}
}
```

- [ ] **Step 2: Run and watch them fail.** `go test ./internal/runner -run 'TestControl|TestWithPauseOnThrottle|TestRunWithControl|TestRunCanceledWhileQueuedPause'`
  Expected: build failure, `undefined: NewControl`.

- [ ] **Step 3: Implement.**

  `internal/runner/options.go`: add the fields to `runSettings`

```go
	// control forwards Pause/Resume to the engine during the crawl phase.
	control *Control
	// pauseOnThrottle sets crawler.Options.PauseOnThrottle.
	pauseOnThrottle bool
```

  and append

```go
// WithControl lets the caller pause and resume this Run's crawl through c.
func WithControl(c *Control) RunOption {
	return func(s *runSettings) { s.control = c }
}

// WithPauseOnThrottle makes the crawl pause itself after sustained 429/503 responses (see
// crawler.Options.PauseOnThrottle). Only for callers that can resume it, like the web UI.
func WithPauseOnThrottle() RunOption {
	return func(s *runSettings) { s.pauseOnThrottle = true }
}
```

  Create `internal/runner/control.go`:

```go
package runner

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Control pauses and resumes the crawl inside a Run started with WithControl. Run hides the
// engine, so Control forwards to it while the crawl phase runs. Calls before the crawl phase
// are queued (a pause made during the sitemap pre-pass takes effect when crawling starts);
// calls after it ends do nothing and return false.
//
// Lock order: Control.mu is taken before the engine's locks (Pause and Resume emit a progress
// event while it is held). Nothing on the progress path takes Control.mu, and Active takes no
// lock at all, so a caller may call Active while holding its own job lock, but must not call
// Pause or Resume while holding a lock its progress callback takes.
type Control struct {
	mu      sync.Mutex
	engine  *crawler.Engine // set during the crawl phase
	pending bool            // pause requested before the crawl phase began
	ended   bool

	live   atomic.Pointer[crawler.Engine] // mirrors engine for the lock-free Active
	frozen atomic.Int64                   // active time in ns, fixed when the crawl phase ends
}

// NewControl returns a Control to pass to WithControl.
func NewControl() *Control { return &Control{} }

// Pause pauses the crawl, or queues the pause if crawling hasn't started. It returns false
// once the crawl phase is over.
func (c *Control) Pause() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return false
	}
	if c.engine == nil {
		c.pending = true
		return true
	}
	c.engine.Pause()
	return c.engine.Paused()
}

// Resume resumes a paused crawl at rate requests per second (0 keeps the current rate), or
// cancels a queued pause. It returns false once the crawl phase is over.
func (c *Control) Resume(rate float64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return false
	}
	if c.engine == nil {
		c.pending = false
		return true
	}
	c.engine.Resume(rate)
	return true
}

// Active reports the crawl's active time (pauses excluded): 0 before the crawl phase, live
// during it, frozen after it. Safe to call while holding any lock.
func (c *Control) Active() time.Duration {
	if e := c.live.Load(); e != nil {
		return e.ActiveTime()
	}
	return time.Duration(c.frozen.Load())
}

// attach hands the engine over right before Engine.Crawl, applying a queued pause.
func (c *Control) attach(e *crawler.Engine) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.engine = e
	c.live.Store(e)
	if c.pending {
		c.pending = false
		e.Pause()
	}
}

// detach ends the crawl phase right after Engine.Crawl returns.
func (c *Control) detach() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.engine != nil {
		c.frozen.Store(int64(c.engine.ActiveTime()))
	}
	c.live.Store(nil)
	c.engine = nil
	c.ended = true
}
```

  `internal/runner/runner.go` `Run`:

  1. After `opts, err := cfg.ToOptions()` and its error check, add:

```go
	opts.PauseOnThrottle = settings.pauseOnThrottle
```

  2. Replace

```go
	progress.setSitemap(sm, engine.Normalize)
	progress.setPhase(PhaseCrawling)

	result, err := engine.Crawl(ctx, seed)
	if err != nil {
		return nil, err
	}
```

  with

```go
	progress.setSitemap(sm, engine.Normalize)
	// attach before announcing the crawling phase, so a pause queued during the pre-pass is
	// already in the engine's state when the phase event goes out.
	settings.control.attach(engine)
	progress.setPhase(PhaseCrawling)

	result, err := engine.Crawl(ctx, seed)
	settings.control.detach()
	if err != nil {
		return nil, err
	}
```

- [ ] **Step 4: Run.** `go test -race ./internal/runner -count=2`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/runner/control.go internal/runner/control_test.go internal/runner/options.go internal/runner/runner.go
git commit -F - <<'EOF'
feat(runner): add Control, WithControl and WithPauseOnThrottle

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PAUSE-6: Pause and resume endpoints, paused job status, auto-pause on the web

**Files:**
- Create: `internal/webserver/pause.go`, `internal/webserver/pause_test.go`
- Modify: `internal/webserver/crawl_job.go`, `internal/webserver/progress.go`,
  `internal/webserver/crawls.go` (`crawlRoutes`, `handleStartCrawl`),
  `internal/webserver/server_test.go` (`pollUntilFinished`),
  `internal/webserver/progress_test.go`

**Interfaces:**
- Consumes: `runner.NewControl`, `Control.Pause/Resume/Active`, `runner.WithControl`,
  `runner.WithPauseOnThrottle` (W2-PAUSE-5); wave 0 `StatusPaused`, `jobManager.cancel`
  (running or paused) and `start` (paused counts toward the cap).
- Produces:
  - `crawlJob.control *runner.Control` (contract)
  - `POST /api/crawls/{id}/pause` → `204`; `404` unknown; `409` when not running or past the
    crawl phase.
  - `POST /api/crawls/{id}/resume`, JSON `{rate?: number}` → `204`; `400` rate `<= 0` or bad
    JSON; `415` non-JSON; `404` unknown; `409` when not paused.
  - `crawlRunOptions(id string, ctl *runner.Control) []runner.RunOption` (adds `WithControl`
    and `WithPauseOnThrottle`); `recordProgress` moves the job between `running` and `paused`.
  - `progress.active_ms` now comes from `Control.Active()` (crawl-phase active time).

Concurrency: the handlers call `Control.Pause/Resume` outside `s.crawls.with` (Pause emits a
progress event whose callback takes the job manager's lock), then update the job inside `with`.
`activeMS` calls `Control.Active()` inside `with`, which is lock-free on `Control`.

- [ ] **Step 1: Write the failing tests** `internal/webserver/pause_test.go`:

```go
package webserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// slowSite serves a home page linking to n pages that each take delay to answer.
func slowSite(t *testing.T, n int, delay time.Duration) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var b strings.Builder
			b.WriteString("<html><head><title>Home</title></head><body>")
			for i := range n {
				fmt.Fprintf(&b, `<a href="/p%d">p%d</a>`, i, i)
			}
			b.WriteString("</body></html>")
			fmt.Fprint(w, b.String())
		case "/robots.txt", "/sitemap.xml", "/sitemap_index.xml", "/llms.txt":
			http.NotFound(w, r)
		default:
			time.Sleep(delay)
			fmt.Fprintf(w, "<html><head><title>%s</title></head><body>page</body></html>", r.URL.Path)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func waitForCrawlState(t *testing.T, srv *Server, id, what string, cond func(job map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
		if cond(job) {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("crawl %s: timed out waiting for %s", id, what)
	return nil
}

func fetchedOf(job map[string]any) float64 {
	p, _ := job["progress"].(map[string]any)
	f, _ := p["fetched"].(float64)
	return f
}

// serveCrawlRequest sends one request without a *testing.T, for use from helper goroutines.
func serveCrawlRequest(srv *Server, method, path, contentType, body string) int {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "localhost:8080"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec.Code
}

func startSlowCrawl(t *testing.T, srv *Server, target *httptest.Server) string {
	t.Helper()
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "concurrency": 1, "max_pages": 0})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start status %d, body %v", rec.Code, started)
	}
	id, _ := started["id"].(string)
	waitForCrawlState(t, srv, id, "the first pages", func(j map[string]any) bool { return fetchedOf(j) >= 2 })
	return id
}

func pauseAndWait(t *testing.T, srv *Server, id string) map[string]any {
	t.Helper()
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/pause", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("pause status %d, want 204", rec.Code)
	}
	return waitForCrawlState(t, srv, id, "paused status", func(j map[string]any) bool { return j["status"] == "paused" })
}

func TestPauseAndResumeCrawl(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 30, 30*time.Millisecond))

	job := pauseAndWait(t, srv, id)
	prog := job["progress"].(map[string]any)
	if prog["paused"] != true || prog["pause_reason"] != "user" {
		t.Errorf("progress after pause = %v", prog)
	}
	time.Sleep(150 * time.Millisecond) // an in-flight page may still land
	_, j1 := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
	time.Sleep(300 * time.Millisecond)
	_, j2 := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
	if fetchedOf(j2) != fetchedOf(j1) {
		t.Fatalf("fetched rose from %v to %v while paused", fetchedOf(j1), fetchedOf(j2))
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/pause", nil); rec.Code != http.StatusConflict {
		t.Errorf("second pause: status %d, want 409", rec.Code)
	}

	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{"rate": 50}); rec.Code != http.StatusNoContent {
		t.Fatalf("resume status %d, want 204", rec.Code)
	}
	resumed := waitForCrawlState(t, srv, id, "running status", func(j map[string]any) bool { return j["status"] != "paused" })
	if p := resumed["progress"].(map[string]any); p["paused"] != false || p["rate"] != float64(50) {
		t.Errorf("progress after resume = %v, want unpaused at rate 50", p)
	}
	// Running or already done by now: either way it is not paused.
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{}); rec.Code != http.StatusConflict {
		t.Errorf("resume of a crawl that isn't paused: status %d, want 409", rec.Code)
	}

	done := pollUntilFinished(t, srv, id)
	if done["status"] != "done" {
		t.Fatalf("final status %v, want done", done["status"])
	}
	if rep := done["report"].(map[string]any); rep["pages_crawled"] != float64(31) {
		t.Errorf("pages_crawled = %v, want 31", rep["pages_crawled"])
	}
}

func TestPauseResumeRejectWrongStates(t *testing.T) {
	srv := newTestServer(t)
	target := startCrawlTarget(t)
	_, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "max_pages": 1})
	id := started["id"].(string)
	pollUntilFinished(t, srv, id)

	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/pause", nil); rec.Code != http.StatusConflict {
		t.Errorf("pause finished crawl: %d, want 409", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{}); rec.Code != http.StatusConflict {
		t.Errorf("resume finished crawl: %d, want 409", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/nope/pause", nil); rec.Code != http.StatusNotFound {
		t.Errorf("pause unknown crawl: %d, want 404", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/nope/resume", map[string]any{}); rec.Code != http.StatusNotFound {
		t.Errorf("resume unknown crawl: %d, want 404", rec.Code)
	}
}

func TestResumeValidatesBody(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 30, 30*time.Millisecond))
	pauseAndWait(t, srv, id)

	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{"rate": -1}); rec.Code != http.StatusBadRequest {
		t.Errorf("negative rate: %d, want 400", rec.Code)
	}
	if code := serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/resume", "text/plain", `{}`); code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: %d, want 415", code)
	}
	if code := serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/resume", "application/json", `{`); code != http.StatusBadRequest {
		t.Errorf("malformed JSON: %d, want 400", code)
	}
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil)
	pollUntilFinished(t, srv, id)
}

func TestCancelPausedCrawl(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 30, 30*time.Millisecond))
	pauseAndWait(t, srv, id)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel paused crawl: %d, want 204", rec.Code)
	}
	done := pollUntilFinished(t, srv, id)
	if done["status"] != "canceled" {
		t.Fatalf("status %v, want canceled", done["status"])
	}
	cov := done["report"].(map[string]any)["coverage"].(map[string]any)
	if cov["interrupted"] != true {
		t.Errorf("coverage = %v, want interrupted", cov)
	}
}

func TestPausedCrawlCountsTowardTheCap(t *testing.T) {
	srv := New(store.New(t.TempDir()), WithMaxRunning(1))
	target := slowSite(t, 30, 30*time.Millisecond)
	id := startSlowCrawl(t, srv, target)
	pauseAndWait(t, srv, id)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("start while one crawl is paused at cap 1: %d, want 429", rec.Code)
	}
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil)
	pollUntilFinished(t, srv, id)
}

func TestPauseIsGuarded(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/crawls/x/pause", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin pause: %d, want 403", rec.Code)
	}
}

func TestProgressMovesJobBetweenRunningAndPaused(t *testing.T) {
	srv := newTestServer(t)
	job, _ := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, srv.maxRunning)
	status := func() Status {
		j, _ := srv.crawls.get(job.ID)
		return j.Status
	}

	srv.recordProgress(job.ID, runner.Progress{
		Progress: crawler.Progress{Fetched: 1, Status: 429, Paused: true, PauseReason: crawler.PauseReasonRateLimited, RetryAfterSeconds: 120},
		Phase:    runner.PhaseCrawling,
	})
	if got := status(); got != StatusPaused {
		t.Fatalf("after an auto-pause event: %s, want paused", got)
	}
	srv.recordProgress(job.ID, runner.Progress{Progress: crawler.Progress{Fetched: 1}, Phase: runner.PhaseCrawling})
	if got := status(); got != StatusRunning {
		t.Fatalf("after a resume event: %s, want running", got)
	}

	// A pause queued during the sitemap phase is not undone by a sitemap-phase event.
	srv.crawls.with(job.ID, func(j *Job[crawlJob]) { j.Status = StatusPaused })
	srv.recordProgress(job.ID, runner.Progress{Phase: runner.PhaseSitemap})
	if got := status(); got != StatusPaused {
		t.Fatalf("after a sitemap-phase event: %s, want still paused", got)
	}

	srv.crawls.finish(job.ID, nil, nil)
	srv.recordProgress(job.ID, runner.Progress{Progress: crawler.Progress{Fetched: 1, Paused: true}, Phase: runner.PhaseCrawling})
	if got := status(); got != StatusDone {
		t.Fatalf("a finished job moved to %s", got)
	}
}

// TestPauseResumeWhilePollingDoesNotDeadlock exercises the lock order: pause/resume emit
// progress events (which take the job manager's lock) while GETs hold that lock to build views
// that call Control.Active.
func TestPauseResumeWhilePollingDoesNotDeadlock(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 60, 10*time.Millisecond))

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					serveCrawlRequest(srv, http.MethodGet, "/api/crawls/"+id, "", "")
				}
			}
		}()
		for range 10 {
			serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/pause", "", "")
			serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/resume", "application/json", `{}`)
		}
		close(stop)
		wg.Wait()
	}()
	select {
	case <-finished:
	case <-time.After(15 * time.Second):
		t.Fatal("pause/resume under concurrent polling deadlocked")
	}
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil)
	pollUntilFinished(t, srv, id)
}
```

  In `internal/webserver/progress_test.go`, `TestCrawlProgressEndToEnd` needs no change.

- [ ] **Step 2: Run and watch them fail.** `go test ./internal/webserver -run 'TestPause|TestResume|TestCancelPaused|TestPausedCrawl|TestProgressMoves'`
  Expected: `FAIL` — without the routes, pause/resume requests fall through to the frontend
  handler (wrong status or an HTML body `doJSON` can't decode), and
  `TestProgressMovesJobBetweenRunningAndPaused` reports `after an auto-pause event: running, want paused`.

- [ ] **Step 3: Implement.**

  `internal/webserver/server_test.go`: in `pollUntilFinished`, replace
  `if job["status"] != "running" {` with

```go
		if job["status"] != "running" && job["status"] != "paused" {
```

  `internal/webserver/crawl_job.go`: add to `crawlJob` (replacing any wave-0 placeholder of the
  same name):

```go
	// control pauses and resumes the crawl. Call its Pause/Resume outside s.crawls.with: they
	// emit a progress event whose callback takes the job manager's lock.
	control *runner.Control
```

  `internal/webserver/progress.go`:

  1. Replace `crawlRunOptions` with:

```go
// crawlRunOptions are the runner options every web crawl gets. Someone can press Resume in the
// UI, so a crawl the server keeps rate limiting pauses itself instead of grinding on.
func (s *Server) crawlRunOptions(id string, ctl *runner.Control) []runner.RunOption {
	return []runner.RunOption{
		runner.WithProgress(func(p runner.Progress) { s.recordProgress(id, p) }),
		runner.WithControl(ctl),
		runner.WithPauseOnThrottle(),
	}
}
```

  2. In `recordProgress`, inside the `with` callback, after the tape append block, add:

```go
		// Follow the engine's pause state, so an automatic pause (and the resume after it)
		// shows in the job status. Only running/paused jobs move. A paused job only goes back
		// to running on a crawling-phase event: during the sitemap phase a queued pause has not
		// reached the engine yet, and its events still say "not paused".
		switch {
		case p.Paused && j.Status == StatusRunning:
			j.Status = StatusPaused
		case !p.Paused && j.Status == StatusPaused && p.Phase == runner.PhaseCrawling:
			j.Status = StatusRunning
		}
```

  3. Replace `activeMS` with:

```go
// activeMS is the crawl phase's active time (pauses excluded). Control.Active is lock-free,
// so this is safe inside s.crawls.with.
func activeMS(j *Job[crawlJob], now time.Time) int64 {
	if j.Data.control != nil {
		return j.Data.control.Active().Milliseconds()
	}
	end := now
	if !j.FinishedAt.IsZero() {
		end = j.FinishedAt
	}
	return end.Sub(j.StartedAt).Milliseconds()
}
```

  `internal/webserver/crawls.go`:

  1. In `crawlRoutes()`, after the cancel route, add:

```go
	s.mux.HandleFunc("POST /api/crawls/{id}/pause", s.handlePauseCrawl)
	s.mux.HandleFunc("POST /api/crawls/{id}/resume", s.handleResumeCrawl)
```

  2. In `handleStartCrawl`, create the control before `start` and pass it twice:

```go
	ctl := runner.NewControl()
	job, ok := s.crawls.start(crawlJob{Seed: seed, MaxPages: cfg.Crawl.MaxPages, control: ctl}, cancel, s.maxRunning)
```

     and change the run call to `runner.Run(ctx, cfg, seed, s.crawlRunOptions(job.ID, ctl)...)`.

  Create `internal/webserver/pause.go`:

```go
package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// resumeRequest is the POST /api/crawls/{id}/resume body. Rate, when set, is the new
// requests-per-second limit; omitted keeps the current one.
type resumeRequest struct {
	Rate *float64 `json:"rate,omitempty"`
}

func (s *Server) handlePauseCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.crawls.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("no crawl with id %q", id))
		return
	}
	if job.Status != StatusRunning || job.Data.control == nil {
		writeError(w, http.StatusConflict, fmt.Errorf("crawl %q is %s, not running", id, job.Status))
		return
	}
	// Outside s.crawls.with: Pause emits a progress event, and the progress callback takes the
	// job manager's lock itself.
	if !job.Data.control.Pause() {
		writeError(w, http.StatusConflict, errors.New("the crawl phase is over; there is nothing left to pause"))
		return
	}
	s.crawls.with(id, func(j *Job[crawlJob]) {
		if j.Status == StatusRunning {
			j.Status = StatusPaused
		}
		// A pause queued during the sitemap phase emits nothing yet, so record it here.
		if p := j.Data.Progress; p != nil && !p.Paused {
			cp := *p
			cp.Paused, cp.PauseReason = true, crawler.PauseReasonUser
			j.Data.Progress = &cp
		}
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResumeCrawl(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var req resumeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	rate := 0.0
	if req.Rate != nil {
		if *req.Rate <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("rate must be a positive number of requests per second"))
			return
		}
		rate = *req.Rate
	}

	id := r.PathValue("id")
	job, ok := s.crawls.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("no crawl with id %q", id))
		return
	}
	if job.Status != StatusPaused || job.Data.control == nil {
		writeError(w, http.StatusConflict, fmt.Errorf("crawl %q is %s, not paused", id, job.Status))
		return
	}
	// Outside s.crawls.with, for the same reason as in handlePauseCrawl.
	if !job.Data.control.Resume(rate) {
		writeError(w, http.StatusConflict, errors.New("the crawl phase is over; there is nothing left to resume"))
		return
	}
	s.crawls.with(id, func(j *Job[crawlJob]) {
		if j.Status == StatusPaused {
			j.Status = StatusRunning
		}
		if p := j.Data.Progress; p != nil && p.Paused {
			cp := *p
			cp.Paused, cp.PauseReason, cp.RetryAfterSeconds = false, "", 0
			if rate > 0 {
				cp.Rate = rate
			}
			j.Data.Progress = &cp
		}
	})
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run.** `go test -race ./internal/webserver -count=2`
  Expected: `ok`.

- [ ] **Step 5: `make check`.** Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add internal/webserver/pause.go internal/webserver/pause_test.go internal/webserver/crawl_job.go internal/webserver/progress.go internal/webserver/crawls.go internal/webserver/server_test.go
git commit -F - <<'EOF'
feat(webserver): pause and resume crawls; web crawls pause themselves when throttled

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W2-PAUSE-7: Docs and changelog for pause and resume

**Files:**
- Modify: `docs/web.md`, `docs/architecture.md`, `docs/output.md`, `docs/README.md`
  (generated), `CHANGELOG.md`

- [ ] **Step 1: `docs/web.md`.** Keep `updated: 2026-10-13`. Add API rows after the cancel row:

```markdown
| `POST /api/crawls/{id}/pause` | Pause a running crawl: no new fetches start, requests already in flight finish. During the sitemap pre-pass the pause is queued and takes effect when crawling starts. `204`; `409` when the crawl isn't running or is already analyzing. |
| `POST /api/crawls/{id}/resume` | Resume a paused crawl. JSON body, optional `rate` (requests per second, `> 0`) to continue at a new rate; `{}` keeps the current one. `204`; `409` when the crawl isn't paused. |
```

  Change the job-status line to: "Job status is one of `running`, `paused`, `done`, `error`,
  `canceled`. A paused crawl still counts toward the running-crawl cap and can be canceled."

  In the Live progress table, change the `active_ms` row to "Active crawl time: time spent
  paused, and the sitemap pre-pass, don't count." and add a row:

```markdown
| `paused`, `pause_reason`, `retry_after_seconds` | Whether the crawl is paused, why (`user`, or `rate_limited` when it paused itself), and the latest `Retry-After` the server sent. |
```

  Add after the table:

```markdown
Web crawls pause themselves when the site keeps rate limiting them: once adaptive delay has
lowered the rate at least twice, ten or more 429/503 responses within a minute pause the crawl
with `pause_reason: "rate_limited"`. The CLI and MCP never pause, because nobody is there to
resume.
```

- [ ] **Step 2: `docs/architecture.md`.** In "The crawl engine", replace "`MaxDuration` bounds
  the whole crawl's wall-clock time via a context deadline." with "`MaxDuration` bounds the
  crawl's active time: a timer that stops while the crawl is paused cancels the crawl when the
  budget is used up." and add a bullet before "Stopping early":

```markdown
- `Engine.Pause` and `Engine.Resume(rate)` hold back and release the frontier: in-flight
  requests finish and are recorded, a task that was waiting on the rate limiter goes back to
  the head of the queue, and cancellation still wakes a paused crawl. With
  `Options.PauseOnThrottle` the engine pauses itself after sustained 429/503 responses.
  `runner.Control` (through `runner.WithControl`) forwards pause and resume to the engine,
  queueing a pause made during the sitemap pre-pass.
```

- [ ] **Step 3: `docs/output.md`.** Replace the `duration_limit_reached` row with:

```markdown
| `duration_limit_reached` | bool | The `--max-duration` budget of active crawl time elapsed (time a web crawl spent paused doesn't count). Implies `interrupted`. Omitted when false. |
```

- [ ] **Step 4: `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`, add:

```markdown
- Pause and resume crawls through the web API (`POST /api/crawls/{id}/pause`, `POST /api/crawls/{id}/resume` with an optional new `rate`, job status `paused`). Web crawls pause themselves after sustained 429/503 responses and report the server's `Retry-After`, and `max_duration` now counts only active time, so a paused crawl doesn't use up its budget.
```

- [ ] **Step 5: Regenerate and check.** `make docs && make check`
  Expected: both succeed.

- [ ] **Step 6: Commit.**

```sh
git add docs/web.md docs/architecture.md docs/output.md docs/README.md CHANGELOG.md
git commit -F - <<'EOF'
docs: document pause, resume and automatic pausing on rate limits

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

### Track: settings and login UI

Run W2-SET-1 … W2-SET-6, then W2-LOGIN-1 … W2-LOGIN-3, on one branch. Rebase onto the merged
progress UI before W2-SET-3.


Frontend tasks have no unit-test runner (spec: `tsc -b`, oxlint, contrast check). The "test"
step of each is `cd web && npm run build && npm run lint` plus the stated manual check.

#### Task W2-SET-1: Settings and header-profile API modules and types

**Files:**
- Create: `web/src/types/settings.ts`, `web/src/types/profiles.ts`, `web/src/api/settings.ts`, `web/src/api/profiles.ts`
- Modify: `web/src/types/crawls.ts` (`StartCrawlParams`)

**Interfaces:**
- Consumes: `request<T>`, `postJSON<T>` (`api/client.ts`), `Theme` (`theme.ts`), W1-SET routes.
- Produces: `type CrawlDefaults`, `interface GUISettings`, `interface SettingsResponse`;
  `getSettings(): Promise<SettingsResponse>`, `saveSettings(s: GUISettings): Promise<SettingsResponse>`;
  `interface HeaderProfile {name, body}`; `listHeaderProfiles(): Promise<{profiles: string[]}>`,
  `getHeaderProfile(name)`, `saveHeaderProfile(name, body): Promise<void>`,
  `deleteHeaderProfile(name): Promise<void>`.

- [ ] **Step 1: Write the code.** In `web/src/types/crawls.ts`, inside `StartCrawlParams`,
  after `header_profile?: string` add:

```ts
  strip_query?: boolean
  adaptive_delay?: boolean
  follow_nofollow?: boolean
  timeout?: string
  bearer_token?: string
```

  Create `web/src/types/settings.ts`:

```ts
import type { Theme } from '../theme'
import type { StartCrawlParams } from './crawls'

// crawlrequest.Params as stored in ~/.gocrawl/gui.json: every field optional, never a secret.
// The server strips bearer_token, basic_auth, cookie, inline headers, and user:password in
// URLs before saving.
export type CrawlDefaults = Partial<Omit<StartCrawlParams, 'save' | 'login_session'>>

export interface GUISettings {
  theme: Theme
  crawl_defaults: CrawlDefaults
}

// GET/PUT /api/settings response: the stored settings plus the read-only store directory.
export interface SettingsResponse extends GUISettings {
  store_dir: string
}
```

  Create `web/src/types/profiles.ts`:

```ts
export interface HeaderProfileList {
  profiles: string[]
}

export interface HeaderProfile {
  name: string
  body: string
}
```

  Create `web/src/api/settings.ts`:

```ts
import type { GUISettings, SettingsResponse } from '../types/settings'
import { postJSON, request } from './client'

export function getSettings(): Promise<SettingsResponse> {
  return request<SettingsResponse>('/api/settings')
}

export function saveSettings(settings: GUISettings): Promise<SettingsResponse> {
  return postJSON<SettingsResponse>('/api/settings', settings, 'PUT')
}
```

  Create `web/src/api/profiles.ts`:

```ts
import type { HeaderProfile, HeaderProfileList } from '../types/profiles'
import { postJSON, request } from './client'

function profilePath(name: string): string {
  return `/api/header-profiles/${encodeURIComponent(name)}`
}

export function listHeaderProfiles(): Promise<HeaderProfileList> {
  return request<HeaderProfileList>('/api/header-profiles')
}

export function getHeaderProfile(name: string): Promise<HeaderProfile> {
  return request<HeaderProfile>(profilePath(name))
}

export function saveHeaderProfile(name: string, body: string): Promise<void> {
  return postJSON<void>(profilePath(name), { body }, 'PUT')
}

export function deleteHeaderProfile(name: string): Promise<void> {
  return request<void>(profilePath(name), { method: 'DELETE' })
}
```

- [ ] **Step 2: Build and lint.** `cd web && npm run build && npm run lint` — expected: build
  succeeds, oxlint reports 0 errors. (If `Theme` or `request`/`postJSON` are named differently
  by wave 0, the build fails here with the exact name to fix.)

- [ ] **Step 3: Manual check.** Start the dev server (see Conventions). In the browser console
  on http://127.0.0.1:8080: `await (await fetch('/api/settings')).json()` shows
  `{theme: "dark", crawl_defaults: {...}, store_dir: "..."}` — the shape of `SettingsResponse`;
  `await (await fetch('/api/header-profiles')).json()` shows `{profiles: []}`.

- [ ] **Step 4: Run.** `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git checkout -- internal/webserver/webui/dist/index.html
git add web/src/types/crawls.ts web/src/types/settings.ts web/src/types/profiles.ts web/src/api/settings.ts web/src/api/profiles.ts
git commit -F - <<'MSG'
feat(web): settings and header-profile API clients

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-SET-2: Load settings at startup and apply the theme

**Files:**
- Create: `web/src/settings/context.ts`, `web/src/settings/SettingsProvider.tsx`
- Modify: `web/src/main.tsx`

**Interfaces:**
- Consumes: `getSettings`, `saveSettings`, `applyTheme`.
- Produces: `useSettings(): SettingsState` where
  `SettingsState = { settings: SettingsResponse | null; error: string; save(next: GUISettings): Promise<SettingsResponse>; saveDefaults(d: CrawlDefaults): Promise<SettingsResponse> }`;
  default export `SettingsProvider`.

- [ ] **Step 1: Write the code.** Create `web/src/settings/context.ts`:

```ts
import { createContext, useContext } from 'react'
import type { CrawlDefaults, GUISettings, SettingsResponse } from '../types/settings'

export interface SettingsState {
  // null until GET /api/settings answers (and stays null if it fails; see error).
  settings: SettingsResponse | null
  error: string
  save: (next: GUISettings) => Promise<SettingsResponse>
  saveDefaults: (defaults: CrawlDefaults) => Promise<SettingsResponse>
}

export const SettingsContext = createContext<SettingsState | null>(null)

export function useSettings(): SettingsState {
  const state = useContext(SettingsContext)
  if (!state) throw new Error('useSettings must be used inside <SettingsProvider>')
  return state
}
```

  Create `web/src/settings/SettingsProvider.tsx`:

```tsx
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { getSettings, saveSettings } from '../api/settings'
import { applyTheme } from '../theme'
import type { CrawlDefaults, GUISettings, SettingsResponse } from '../types/settings'
import { SettingsContext, type SettingsState } from './context'

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

// Loads ~/.gocrawl/gui.json through the API once, applies its theme (dark until it answers),
// and shares it with the New crawl form and the Settings view. Nothing goes to localStorage:
// the desktop shell serves the UI from a new port on every launch.
export default function SettingsProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<SettingsResponse | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    applyTheme('dark')
    getSettings()
      .then((s) => {
        if (cancelled) return
        setSettings(s)
        applyTheme(s.theme)
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(message(e))
      })
    return () => {
      cancelled = true
    }
  }, [])

  const save = useCallback(async (next: GUISettings) => {
    const saved = await saveSettings(next)
    setSettings(saved)
    setError('')
    applyTheme(saved.theme)
    return saved
  }, [])

  const theme = settings?.theme ?? 'dark'
  const saveDefaults = useCallback(
    (defaults: CrawlDefaults) => save({ theme, crawl_defaults: defaults }),
    [save, theme],
  )

  const value = useMemo<SettingsState>(
    () => ({ settings, error, save, saveDefaults }),
    [settings, error, save, saveDefaults],
  )
  return <SettingsContext value={value}>{children}</SettingsContext>
}
```

  In `web/src/main.tsx`, add `import SettingsProvider from './settings/SettingsProvider'` and
  wrap the app: `<StrictMode><SettingsProvider><App /></SettingsProvider></StrictMode>` (keep
  every other line wave 0 / the visual system put there).

- [ ] **Step 2: Build and lint.** `cd web && npm run build && npm run lint` — expected PASS.

- [ ] **Step 3: Manual check.** Start the dev server. With no `gui.json`, the UI is dark. Run
  `curl -s -X PUT -H 'Content-Type: application/json' -d '{"theme":"light","crawl_defaults":{}}' localhost:8080/api/settings`
  and reload: the UI loads light (`document.documentElement.dataset.theme === 'light'` in the
  console). Set `"system"`: the theme follows the OS appearance. Stop the server, delete
  `$HOME/.gocrawl/gui.json` in the temporary home, write `{bad` into it, start again: the UI
  stays dark and loads.

- [ ] **Step 4: Run.** `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git checkout -- internal/webserver/webui/dist/index.html
git add web/src/settings/context.ts web/src/settings/SettingsProvider.tsx web/src/main.tsx
git commit -F - <<'MSG'
feat(web): load GUI settings at startup and apply the saved theme

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-SET-3: Reusable crawl field set; new crawl fields; header-profile dropdown

**Files:**
- Create: `web/src/components/crawlform/values.ts`, `web/src/components/crawlform/CrawlFields.tsx`
- Modify: `web/src/components/CrawlForm.tsx`

**Interfaces:**
- Consumes: `listAnalyzers`, `startCrawl`, `listHeaderProfiles`, `Button`, `Notice`, `CrawlDefaults`;
  the visual system's form markup and `CrawlForm.css` (W1-VIS-6); `SitemapPreviewLine` and
  `DEFAULT_MAX_PAGES` (W2-PROG-UI-4, which must be merged before this task).
- Produces: `type FieldsMode = 'crawl' | 'defaults'`; `interface FormValues`; `blankValues`;
  `splitList(s)`; `fromParams(p?: CrawlDefaults): FormValues`;
  `toParams(v: FormValues, mode: FieldsMode): CrawlDefaults`; default export
  `CrawlFields({ values, onChange, mode, analyzers, profiles, loginSlot?, cookieLocked? })`.
  In `defaults` mode the field set hides the seed URL's siblings that are secrets (Basic Auth,
  bearer token, cookie, inline headers) and the "save" checkbox.

- [ ] **Step 1: Write `values.ts`.** Create `web/src/components/crawlform/values.ts`:

```ts
import type { CrawlDefaults } from '../../types/settings'

// 'crawl' is the New crawl form; 'defaults' is the Settings view, which never holds secrets.
export type FieldsMode = 'crawl' | 'defaults'

// The crawl form's state, as the strings and booleans the inputs bind to. toParams and
// fromParams map it to and from the API's crawl request (crawlrequest.Params in Go).
export interface FormValues {
  url: string
  depth: string
  maxPages: string
  concurrency: string
  rate: string
  maxDuration: string
  timeout: string
  render: string
  adaptiveDelay: boolean
  save: boolean
  respectRobots: boolean
  subdomains: boolean
  followExternal: boolean
  followNofollow: boolean
  stripQuery: boolean
  specialized: boolean
  securityAudit: boolean
  ignoreExternalTagging: boolean
  analyzers: string[]
  include: string
  exclude: string
  userAgent: string
  userAgents: string
  userAgentRotation: string
  proxy: string
  proxies: string
  proxyRotation: string
  basicAuthUser: string
  basicAuthPass: string
  bearerToken: string
  cookieAuth: boolean
  cookie: string
  customHeaders: boolean
  headers: string
  headerProfile: string
}

export const blankValues: FormValues = {
  url: '',
  depth: '',
  maxPages: '',
  concurrency: '',
  rate: '',
  maxDuration: '',
  timeout: '',
  render: 'raw',
  adaptiveDelay: true,
  save: true,
  respectRobots: true,
  subdomains: false,
  followExternal: false,
  followNofollow: false,
  stripQuery: false,
  specialized: false,
  securityAudit: false,
  ignoreExternalTagging: true,
  analyzers: [],
  include: '',
  exclude: '',
  userAgent: '',
  userAgents: '',
  userAgentRotation: 'round-robin',
  proxy: '',
  proxies: '',
  proxyRotation: 'round-robin',
  basicAuthUser: '',
  basicAuthPass: '',
  bearerToken: '',
  cookieAuth: false,
  cookie: '',
  customHeaders: false,
  headers: '',
  headerProfile: '',
}

// Newline-separated rather than comma-separated: a comma is an ordinary character in a
// User-Agent string ("(KHTML, like Gecko)") and in a regex ("{2,4}").
export function splitList(s: string): string[] {
  return s
    .split('\n')
    .map((v) => v.trim())
    .filter(Boolean)
}

function joinList(list: string[] | undefined): string {
  return (list ?? []).join('\n')
}

function numberText(n: number | undefined): string {
  return n === undefined ? '' : String(n)
}

export function fromParams(p: CrawlDefaults | undefined): FormValues {
  if (!p) return { ...blankValues }
  return {
    ...blankValues,
    url: p.url ?? '',
    depth: numberText(p.depth),
    maxPages: numberText(p.max_pages),
    concurrency: numberText(p.concurrency),
    rate: numberText(p.rate),
    maxDuration: p.max_duration ?? '',
    timeout: p.timeout ?? '',
    render: p.render || 'raw',
    adaptiveDelay: p.adaptive_delay ?? true,
    respectRobots: p.respect_robots ?? true,
    subdomains: p.subdomains ?? false,
    followExternal: p.follow_external ?? false,
    followNofollow: p.follow_nofollow ?? false,
    stripQuery: p.strip_query ?? false,
    specialized: p.specialized ?? false,
    securityAudit: p.security_audit ?? false,
    ignoreExternalTagging: p.ignore_external_tagging ?? true,
    analyzers: p.analyzers ?? [],
    include: joinList(p.include),
    exclude: joinList(p.exclude),
    userAgent: p.user_agent ?? '',
    userAgents: joinList(p.user_agents),
    userAgentRotation: p.user_agent_rotation || 'round-robin',
    proxy: p.proxy ?? '',
    proxies: joinList(p.proxies),
    proxyRotation: p.proxy_rotation || 'round-robin',
    customHeaders: Boolean(p.header_profile) || (p.headers?.length ?? 0) > 0,
    headers: joinList(p.headers),
    headerProfile: p.header_profile ?? '',
  }
}

// toParams builds a crawl request. In 'defaults' mode it leaves out the URL and every secret
// (the server strips them anyway; not sending them keeps them out of the request too).
export function toParams(v: FormValues, mode: FieldsMode): CrawlDefaults {
  const p: CrawlDefaults = {
    render: v.render,
    adaptive_delay: v.adaptiveDelay,
    respect_robots: v.respectRobots,
    subdomains: v.subdomains,
    follow_external: v.followExternal,
    specialized: v.specialized,
    security_audit: v.securityAudit,
    ignore_external_tagging: v.ignoreExternalTagging,
  }
  if (mode === 'crawl') p.url = v.url.trim()
  if (v.depth) p.depth = Number(v.depth)
  if (v.maxPages) p.max_pages = Number(v.maxPages)
  if (v.concurrency) p.concurrency = Number(v.concurrency)
  if (v.rate) p.rate = Number(v.rate)
  if (v.maxDuration.trim()) p.max_duration = v.maxDuration.trim()
  if (v.timeout.trim()) p.timeout = v.timeout.trim()
  if (v.followNofollow) p.follow_nofollow = true
  if (v.stripQuery) p.strip_query = true
  if (v.analyzers.length > 0) p.analyzers = [...v.analyzers]
  if (v.include.trim()) p.include = splitList(v.include)
  if (v.exclude.trim()) p.exclude = splitList(v.exclude)
  if (v.userAgent.trim()) p.user_agent = v.userAgent.trim()
  if (v.userAgents.trim()) {
    p.user_agents = splitList(v.userAgents)
    p.user_agent_rotation = v.userAgentRotation
  }
  if (v.proxy.trim()) p.proxy = v.proxy.trim()
  if (v.proxies.trim()) {
    p.proxies = splitList(v.proxies)
    p.proxy_rotation = v.proxyRotation
  }
  if (v.customHeaders) {
    if (v.headerProfile.trim()) p.header_profile = v.headerProfile.trim()
    if (mode === 'crawl' && v.headers.trim()) p.headers = splitList(v.headers)
  }
  if (mode === 'crawl') {
    if (v.basicAuthUser.trim()) p.basic_auth = `${v.basicAuthUser.trim()}:${v.basicAuthPass}`
    if (v.cookieAuth && v.cookie.trim()) p.cookie = v.cookie.trim()
    if (v.bearerToken.trim()) p.bearer_token = v.bearerToken.trim()
  }
  return p
}
```

- [ ] **Step 2: Write `CrawlFields.tsx`.** Create `web/src/components/crawlform/CrawlFields.tsx`:
  the visual system's New crawl markup (W1-VIS-6: `cf-` classes, fieldsets with sentence-case
  legends, `Button` for Select all / Select none), moved out of `CrawlForm`, plus the new
  fields. It imports `CrawlForm.css` itself so the Settings view gets the same styles:

```tsx
import type { ReactNode } from 'react'
import type { AnalyzerInfo } from '../../types/crawls'
import Button from '../ui/Button'
import type { FieldsMode, FormValues } from './values'
import '../CrawlForm.css'

interface Props {
  values: FormValues
  onChange: (patch: Partial<FormValues>) => void
  mode: FieldsMode
  analyzers: AnalyzerInfo[]
  profiles: string[]
  // Rendered after the cookie field in crawl mode: the browser-login panel.
  loginSlot?: ReactNode
  // True while a captured browser login supplies the Cookie header.
  cookieLocked?: boolean
}

// Every crawl option, in the New crawl form's markup (W1-VIS-6: fieldsets with sentence-case
// legends, cf- classes from CrawlForm.css), shared by New crawl and the Settings view's crawl
// defaults. The seed URL and the form's own actions stay in the caller.
export default function CrawlFields({ values: v, onChange, mode, analyzers, profiles, loginSlot, cookieLocked = false }: Props) {
  const crawl = mode === 'crawl'
  const selected = new Set(v.analyzers)
  const profileOptions = v.headerProfile && !profiles.includes(v.headerProfile) ? [...profiles, v.headerProfile] : profiles

  function toggleAnalyzer(name: string) {
    const next = new Set(selected)
    if (next.has(name)) next.delete(name)
    else next.add(name)
    onChange({ analyzers: Array.from(next) })
  }

  return (
    <>
      <fieldset className="cf-section">
        <legend>Crawl behavior</legend>
        <div className="cf-grid">
          <label className="cf-field">
            <span className="cf-label">Depth</span>
            <input type="number" min="0" placeholder="Unlimited" value={v.depth} onChange={(e) => onChange({ depth: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max pages</span>
            <input type="number" min="0" placeholder="500" value={v.maxPages} onChange={(e) => onChange({ maxPages: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Concurrency</span>
            <input type="number" min="1" placeholder="4" value={v.concurrency} onChange={(e) => onChange({ concurrency: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Rate limit (requests per second)</span>
            <input type="number" min="0" step="any" placeholder="Unlimited" value={v.rate} onChange={(e) => onChange({ rate: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max duration</span>
            <input type="text" placeholder="Unlimited, e.g. 90m" value={v.maxDuration} onChange={(e) => onChange({ maxDuration: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Request timeout</span>
            <input type="text" placeholder="15s" value={v.timeout} onChange={(e) => onChange({ timeout: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Render</span>
            <select value={v.render} onChange={(e) => onChange({ render: e.target.value })}>
              <option value="raw">Raw HTML</option>
              <option value="headless">Headless browser</option>
            </select>
          </label>
        </div>
        <label className="cf-check">
          <input type="checkbox" checked={v.adaptiveDelay} onChange={(e) => onChange({ adaptiveDelay: e.target.checked })} />
          Slow down automatically when the site answers 429 or 503
        </label>
        {crawl && (
          <label className="cf-check">
            <input type="checkbox" checked={v.save} onChange={(e) => onChange({ save: e.target.checked })} />
            Save to Crawls when finished
          </label>
        )}
      </fieldset>

      <fieldset className="cf-section">
        <legend>Scope</legend>
        <label className="cf-check">
          <input type="checkbox" checked={v.respectRobots} onChange={(e) => onChange({ respectRobots: e.target.checked })} />
          Respect robots.txt
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={v.subdomains} onChange={(e) => onChange({ subdomains: e.target.checked })} />
          Follow links to subdomains
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={v.followExternal} onChange={(e) => onChange({ followExternal: e.target.checked })} />
          Crawl links that leave the site
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={v.followNofollow} onChange={(e) => onChange({ followNofollow: e.target.checked })} />
          Follow links marked rel=&quot;nofollow&quot;
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={v.stripQuery} onChange={(e) => onChange({ stripQuery: e.target.checked })} />
          Ignore query strings (?a=1 and ?a=2 count as one URL; skips UTM and tracking analysis)
        </label>
      </fieldset>

      <fieldset className="cf-section">
        <legend>Analysis</legend>
        <label className="cf-check">
          <input type="checkbox" checked={v.specialized} onChange={(e) => onChange({ specialized: e.target.checked })} />
          Specialized checks (AEO and GEO heuristics, WordPress security probes)
        </label>
        <label className="cf-check">
          <input type="checkbox" checked={v.securityAudit} onChange={(e) => onChange({ securityAudit: e.target.checked })} />
          Security audit (TLS and certificate, cookies, response headers)
        </label>
        <label className="cf-check">
          <input
            type="checkbox"
            checked={v.ignoreExternalTagging}
            onChange={(e) => onChange({ ignoreExternalTagging: e.target.checked })}
          />
          Ignore UTM tagging issues on links that leave the site
        </label>

        {analyzers.length > 0 && (
          <fieldset className="cf-analyzers">
            <legend>Analyzers</legend>
            <p className="cf-hint">None checked runs all of them.</p>
            <div className="cf-analyzer-actions">
              <Button onClick={() => onChange({ analyzers: analyzers.map((a) => a.name) })}>Select all</Button>
              <Button onClick={() => onChange({ analyzers: [] })}>Select none</Button>
            </div>
            <div className="cf-analyzer-grid">
              {analyzers.map((a) => (
                <label key={a.name} className="cf-check" title={a.description}>
                  <input type="checkbox" checked={selected.has(a.name)} onChange={() => toggleAnalyzer(a.name)} />
                  <span className="mono">{a.name}</span>
                </label>
              ))}
            </div>
          </fieldset>
        )}
      </fieldset>

      <details className="cf-advanced">
        <summary>Advanced</summary>
        <div className="cf-grid cf-grid-wide">
          <label className="cf-field">
            <span className="cf-label">Include (regex, one per line)</span>
            <span className="cf-hint">
              Matched against the full normalized URL. The seed must match too, or the crawl returns zero pages. No
              trailing slash.
            </span>
            <textarea rows={2} placeholder="/blog" value={v.include} onChange={(e) => onChange({ include: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Exclude (regex, one per line)</span>
            <span className="cf-hint">Matched against the full URL.</span>
            <textarea rows={2} placeholder="/admin/" value={v.exclude} onChange={(e) => onChange({ exclude: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent</span>
            <input type="text" value={v.userAgent} onChange={(e) => onChange({ userAgent: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent pool (one per line)</span>
            <textarea rows={2} value={v.userAgents} onChange={(e) => onChange({ userAgents: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent rotation</span>
            <select value={v.userAgentRotation} onChange={(e) => onChange({ userAgentRotation: e.target.value })}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
            </select>
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy</span>
            <input type="text" placeholder="http(s):// or socks5://" value={v.proxy} onChange={(e) => onChange({ proxy: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy pool (one per line)</span>
            <textarea rows={2} value={v.proxies} onChange={(e) => onChange({ proxies: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy rotation</span>
            <select value={v.proxyRotation} onChange={(e) => onChange({ proxyRotation: e.target.value })}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
              <option value="sticky-host">Sticky per host</option>
            </select>
          </label>
        </div>

        {crawl ? (
          <fieldset className="cf-section">
            <legend>Authentication</legend>
            <div className="cf-grid cf-grid-wide">
              <label className="cf-field">
                <span className="cf-label">Basic Auth username</span>
                <input type="text" value={v.basicAuthUser} onChange={(e) => onChange({ basicAuthUser: e.target.value })} />
              </label>
              <label className="cf-field">
                <span className="cf-label">Basic Auth password</span>
                <input type="password" value={v.basicAuthPass} onChange={(e) => onChange({ basicAuthPass: e.target.value })} />
              </label>
              <label className="cf-field">
                <span className="cf-label">Bearer token</span>
                <span className="cf-hint">
                  Sent as &quot;Authorization: Bearer …&quot; to the seed host only. Not together with Basic Auth or a custom
                  Authorization header. Raw rendering only.
                </span>
                <input
                  type="password"
                  autoComplete="off"
                  value={v.bearerToken}
                  onChange={(e) => onChange({ bearerToken: e.target.value })}
                />
              </label>
            </div>
            <label className="cf-check">
              <input
                type="checkbox"
                checked={v.cookieAuth && !cookieLocked}
                disabled={cookieLocked}
                onChange={(e) => onChange({ cookieAuth: e.target.checked })}
              />
              The site needs a session cookie (for example a Shopify storefront password page)
            </label>
            {cookieLocked && <p className="cf-hint">Your browser login supplies the cookies for this crawl.</p>}
            {v.cookieAuth && !cookieLocked && (
              <label className="cf-field">
                <span className="cf-label">Cookie header</span>
                <span className="cf-hint">
                  Paste the session cookie from a browser that is already past the gate (DevTools, Application or Storage,
                  Cookies). Sent only to the seed host. Not supported with headless rendering.
                </span>
                <textarea
                  rows={2}
                  className="mono"
                  placeholder="_shopify_essential=:AZ_..."
                  value={v.cookie}
                  onChange={(e) => onChange({ cookie: e.target.value })}
                />
              </label>
            )}
            {loginSlot}
          </fieldset>
        ) : (
          <p className="cf-hint">
            Passwords, cookies, bearer tokens and inline headers are never saved as defaults. Put headers every crawl
            needs in a header profile.
          </p>
        )}

        <label className="cf-check">
          <input type="checkbox" checked={v.customHeaders} onChange={(e) => onChange({ customHeaders: e.target.checked })} />
          Send custom request headers (for example a Shopify crawler access signature)
        </label>
        {v.customHeaders && (
          <>
            <label className="cf-field">
              <span className="cf-label">Header profile</span>
              <span className="cf-hint">Managed under Settings; stored in ~/.gocrawl/headers on this machine.</span>
              <select value={v.headerProfile} onChange={(e) => onChange({ headerProfile: e.target.value })}>
                <option value="">None</option>
                {profileOptions.map((name) => (
                  <option key={name} value={name}>
                    {profiles.includes(name) ? name : `${name} (missing)`}
                  </option>
                ))}
              </select>
            </label>
            {crawl && (
              <label className="cf-field">
                <span className="cf-label">Extra headers (one &quot;Name: value&quot; per line)</span>
                <span className="cf-hint">
                  Sent only to the seed host. They override the profile by name. Not supported with headless rendering.
                </span>
                <textarea
                  rows={3}
                  className="mono"
                  placeholder="X-Access-Token: abc123"
                  value={v.headers}
                  onChange={(e) => onChange({ headers: e.target.value })}
                />
              </label>
            )}
          </>
        )}
      </details>
    </>
  )
}
```

- [ ] **Step 3: Rewrite `CrawlForm.tsx` on top of the field set.** Replace
  `web/src/components/CrawlForm.tsx` with the version below. It keeps everything the visual
  system (W1-VIS-6) and the progress UI (W2-PROG-UI-4) put in the form: the `cf` markup and
  `CrawlForm.css`, the **Site URL** field, the green **Start crawl** button, the error `Notice`,
  and the `SitemapPreviewLine` with **Raise budget**, now reading `values.*`:

```tsx
import { useEffect, useState } from 'react'
import { listAnalyzers, startCrawl } from '../api/crawls'
import { listHeaderProfiles } from '../api/profiles'
import type { AnalyzerInfo, StartCrawlParams } from '../types/crawls'
import CrawlFields from './crawlform/CrawlFields'
import { blankValues, splitList, toParams, type FormValues } from './crawlform/values'
import SitemapPreviewLine from './progress/SitemapPreviewLine'
import { DEFAULT_MAX_PAGES } from './progress/tapeText'
import Button from './ui/Button'
import Notice from './ui/Notice'
import './CrawlForm.css'

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export default function CrawlForm({ onStarted }: { onStarted: (id: string) => void }) {
  const [values, setValues] = useState<FormValues>(blankValues)
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [profiles, setProfiles] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e: unknown) => setError(message(e)))
    listHeaderProfiles()
      .then((r) => setProfiles(r.profiles))
      .catch((e: unknown) => setError(message(e)))
  }, [])

  function update(patch: Partial<FormValues>) {
    setValues((v) => ({ ...v, ...patch }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!values.url.trim()) return
    if (values.bearerToken.trim() && values.basicAuthUser.trim()) {
      setError('Use a bearer token or Basic Auth, not both: they set the same Authorization header.')
      return
    }
    setSubmitting(true)
    setError('')
    const params: StartCrawlParams = { ...toParams(values, 'crawl'), url: values.url.trim(), save: values.save }
    try {
      const job = await startCrawl(params)
      onStarted(job.id)
    } catch (err) {
      setError(message(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="cf" onSubmit={handleSubmit}>
      <h1>New crawl</h1>
      <label className="cf-field cf-url">
        <span className="cf-label">Site URL</span>
        <input
          type="text"
          inputMode="url"
          placeholder="https://example.com"
          value={values.url}
          onChange={(e) => update({ url: e.target.value })}
          required
        />
      </label>
      <SitemapPreviewLine
        url={values.url}
        include={splitList(values.include)}
        exclude={splitList(values.exclude)}
        subdomains={values.subdomains}
        maxPages={values.maxPages.trim() === '' ? DEFAULT_MAX_PAGES : Number(values.maxPages)}
        onRaiseBudget={(pages) => update({ maxPages: String(pages) })}
      />
      <CrawlFields values={values} onChange={update} mode="crawl" analyzers={analyzers} profiles={profiles} />
      {error && <Notice tone="error">{error}</Notice>}
      <div className="cf-actions">
        <Button type="submit" variant="go" disabled={submitting}>
          {submitting ? 'Starting…' : 'Start crawl'}
        </Button>
      </div>
    </form>
  )
}
```

- [ ] **Step 4: Check nothing else was lost.** `git diff HEAD -- web/src/components/CrawlForm.tsx`
  must show no other behavior from the visual or progress tracks dropped: every input, hint and
  option of the W1-VIS-6 form is now in `CrawlFields.tsx` (same copy, same `cf-` classes), and
  `grep -rnE 'className="(card|form-section|form-section-title|grid|checkbox|form-hint|form-actions)' web/src`
  prints nothing (those pre-visual classes have no styles since W1-VIS-8 deleted `index.css`).

- [ ] **Step 5: Build and lint.** `cd web && npm run build && npm run lint` — expected PASS.

- [ ] **Step 6: Manual check.** Start the dev server with `HOME` set to a temp dir containing
  `.gocrawl/headers/uk.headers` (`H="$(mktemp -d)"; mkdir -p "$H/.gocrawl/headers" && printf 'X-Store: uk\n' > "$H/.gocrawl/headers/uk.headers"`,
  then `make web-build && go build -o /tmp/gocrawl-dev ./cmd/gocrawl && HOME="$H" /tmp/gocrawl-dev serve`). In New crawl: every field from before is there and
  behaves the same (start a crawl of https://example.com with depth 1; it runs). New fields:
  Request timeout, the 429/503 checkbox (on), Follow nofollow, Ignore query strings, Bearer
  token (masked). Advanced → "Send custom request headers" → the Header profile dropdown lists
  `uk`. Fill both Basic Auth username and a bearer token, press Start crawl: the form shows
  "Use a bearer token or Basic Auth, not both" and no crawl starts. In DevTools → Network, a
  started crawl's request body carries `strip_query`, `follow_nofollow`, `timeout`,
  `adaptive_delay` when set.

- [ ] **Step 7: Run.** `make check` — expected PASS.

- [ ] **Step 8: Commit.**

```sh
git checkout -- internal/webserver/webui/dist/index.html
git add web/src/components/crawlform/values.ts web/src/components/crawlform/CrawlFields.tsx web/src/components/CrawlForm.tsx
git commit -F - <<'MSG'
feat(web): reusable crawl field set with timeout, nofollow, strip-query, adaptive delay, bearer token and a header-profile dropdown

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-SET-4: Pre-fill New crawl from `crawl_defaults`; "Save as defaults"

**Files:**
- Modify: `web/src/components/CrawlForm.tsx`, `web/src/components/CrawlForm.css`

**Interfaces:**
- Consumes: `useSettings()` (`settings.crawl_defaults`, `saveDefaults`), `fromParams`, `toParams`.
- Produces: the New crawl form starts from saved defaults (until the user edits it) and has a
  **Save as defaults** action that writes `toParams(values, 'defaults')`.

- [ ] **Step 1: Write the code.** Apply to `web/src/components/CrawlForm.tsx`:
  1. Imports: add `import { useSettings } from '../settings/context'`; change the values
     import to `import { fromParams, splitList, toParams, type FormValues } from './crawlform/values'`.
  2. Replace `const [values, setValues] = useState<FormValues>(blankValues)` with:

```tsx
  const { settings, saveDefaults } = useSettings()
  const defaults = settings?.crawl_defaults
  const [values, setValues] = useState<FormValues>(() => fromParams(defaults))
  const [dirty, setDirty] = useState(false)
  const [defaultsNote, setDefaultsNote] = useState('')
  // Settings load after the first render. Until the user touches the form, follow them.
  const [seenDefaults, setSeenDefaults] = useState(defaults)
  if (defaults !== seenDefaults) {
    setSeenDefaults(defaults)
    if (!dirty) setValues(fromParams(defaults))
  }
```

  3. Replace the `update` function with:

```tsx
  function update(patch: Partial<FormValues>) {
    setDirty(true)
    setDefaultsNote('')
    setValues((v) => ({ ...v, ...patch }))
  }

  async function handleSaveDefaults() {
    setError('')
    setDefaultsNote('')
    try {
      await saveDefaults(toParams(values, 'defaults'))
      setDefaultsNote('Saved. New crawls start with these settings. Passwords, cookies, tokens and inline headers are never saved.')
    } catch (err) {
      setError(`Couldn't save the defaults: ${message(err)}`)
    }
  }
```

  4. Replace the `{error && …}` line and the `cf-actions` block with:

```tsx
      {error && <Notice tone="error">{error}</Notice>}
      {defaultsNote && <Notice tone="info">{defaultsNote}</Notice>}
      <div className="cf-actions">
        <Button type="submit" variant="go" disabled={submitting}>
          {submitting ? 'Starting…' : 'Start crawl'}
        </Button>
        <Button type="button" variant="secondary" onClick={() => void handleSaveDefaults()}>
          Save as defaults
        </Button>
      </div>
```

  5. In `web/src/components/CrawlForm.css`, replace the `.cf-actions` rule with (two buttons
     now share the row):

```css
.cf-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 24px 0 0;
}
```

- [ ] **Step 2: Build and lint.** `cd web && npm run build && npm run lint` — expected PASS
  (`blankValues` is no longer imported here, so `noUnusedLocals` stays quiet).

- [ ] **Step 3: Manual check.** Fresh temp `HOME`, start the dev server. New crawl: set Max pages
  50, tick Ignore query strings, type a bearer token, press **Save as defaults**: the info
  notice appears. `cat "$HOME/.gocrawl/gui.json"` shows `"max_pages": 50`,
  `"strip_query": true`, and no `bearer_token`. Reload: the form opens with Max pages 50 and
  Ignore query strings ticked, the bearer token field empty. Type in the URL before the page
  finishes loading on a throttled connection (DevTools → Network → Slow 3G): your typing is kept.

- [ ] **Step 4: Run.** `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git checkout -- internal/webserver/webui/dist/index.html
git add web/src/components/CrawlForm.tsx web/src/components/CrawlForm.css
git commit -F - <<'MSG'
feat(web): pre-fill New crawl from saved defaults and add Save as defaults

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-SET-5: Settings view

**Files:**
- Modify: `web/src/views/SettingsView.tsx` (replace the wave-0 stub)
- Create: `web/src/views/SettingsView.css` (per-view stylesheet, `set-` prefix, as the visual
  system's convention asks)

**Interfaces:**
- Consumes: `useSettings()`, `applyTheme`, `Theme`, `listAnalyzers`, `listHeaderProfiles`,
  `getHeaderProfile`, `saveHeaderProfile`, `deleteHeaderProfile`, `CrawlFields`, `fromParams`,
  `toParams`, `Button`, `Notice`.
- Produces: default export `SettingsView` with sections Theme, Crawl defaults, Header profiles,
  Crawl store (read-only `store_dir`).

- [ ] **Step 1: Write the code.** Replace `web/src/views/SettingsView.tsx` with:

```tsx
import { useCallback, useEffect, useState } from 'react'
import { listAnalyzers } from '../api/crawls'
import { deleteHeaderProfile, getHeaderProfile, listHeaderProfiles, saveHeaderProfile } from '../api/profiles'
import CrawlFields from '../components/crawlform/CrawlFields'
import { fromParams, toParams, type FormValues } from '../components/crawlform/values'
import Button from '../components/ui/Button'
import Notice from '../components/ui/Notice'
import { useSettings } from '../settings/context'
import { applyTheme, type Theme } from '../theme'
import type { AnalyzerInfo } from '../types/crawls'
import type { CrawlDefaults } from '../types/settings'
import './SettingsView.css'

// Same rule as config.headerProfileName on the server; checked here only for a quicker message.
const PROFILE_NAME = /^[A-Za-z0-9_-][A-Za-z0-9._-]*$/

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export default function SettingsView() {
  const { settings, error: loadError } = useSettings()
  const [profiles, setProfiles] = useState<string[]>([])
  const [profilesError, setProfilesError] = useState('')

  const reloadProfiles = useCallback(async () => {
    try {
      const r = await listHeaderProfiles()
      setProfiles(r.profiles)
      setProfilesError('')
    } catch (e) {
      setProfilesError(message(e))
    }
  }, [])

  useEffect(() => {
    listHeaderProfiles()
      .then((r) => setProfiles(r.profiles))
      .catch((e: unknown) => setProfilesError(message(e)))
  }, [])

  return (
    <div className="set">
      <h1>Settings</h1>
      {loadError && (
        <Notice tone="error">
          Couldn't load your settings: {loadError}. Saving any setting below replaces the settings file.
        </Notice>
      )}
      <ThemeSection />
      <DefaultsSection profiles={profiles} />
      <ProfilesSection profiles={profiles} loadError={profilesError} onChanged={reloadProfiles} />
      <section className="set-panel">
        <h2>Crawl store</h2>
        <p>
          Saved crawls live in <code>{settings?.store_dir ?? '…'}</code>. Start gocrawl with{' '}
          <code>gocrawl serve --store-dir &lt;dir&gt;</code> to use another folder.
        </p>
      </section>
    </div>
  )
}

function ThemeSection() {
  const { settings, save } = useSettings()
  const [status, setStatus] = useState('')
  const theme: Theme = settings?.theme ?? 'dark'

  async function choose(next: Theme) {
    applyTheme(next)
    setStatus('')
    try {
      await save({ theme: next, crawl_defaults: settings?.crawl_defaults ?? {} })
      setStatus('Saved.')
    } catch (e) {
      applyTheme(theme)
      setStatus(`Couldn't save the theme: ${message(e)}`)
    }
  }

  return (
    <section className="set-panel">
      <h2>Theme</h2>
      <label className="cf-field">
        <span className="cf-label">Theme</span>
        <select value={theme} onChange={(e) => void choose(e.target.value as Theme)}>
          <option value="dark">Dark</option>
          <option value="light">Light</option>
          <option value="system">Match the system</option>
        </select>
      </label>
      {status && <p role="status">{status}</p>}
    </section>
  )
}

function DefaultsSection({ profiles }: { profiles: string[] }) {
  const { settings, save } = useSettings()
  const defaults = settings?.crawl_defaults
  const [values, setValues] = useState<FormValues>(() => fromParams(defaults))
  const [seenDefaults, setSeenDefaults] = useState(defaults)
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [error, setError] = useState('')
  const [note, setNote] = useState('')
  // Follow the saved defaults (first load, and the server's stripped copy after each save).
  if (defaults !== seenDefaults) {
    setSeenDefaults(defaults)
    setValues(fromParams(defaults))
  }

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e: unknown) => setError(message(e)))
  }, [])

  async function store(crawlDefaults: CrawlDefaults, done: string) {
    setError('')
    setNote('')
    try {
      await save({ theme: settings?.theme ?? 'dark', crawl_defaults: crawlDefaults })
      setNote(done)
    } catch (e) {
      setError(message(e))
    }
  }

  return (
    <form
      className="set-panel cf"
      onSubmit={(e) => {
        e.preventDefault()
        void store(toParams(values, 'defaults'), 'Saved. The New crawl form starts from these values.')
      }}
    >
      <h2>Crawl defaults</h2>
      <p>These values pre-fill the New crawl form.</p>
      <CrawlFields
        values={values}
        onChange={(patch) => setValues((v) => ({ ...v, ...patch }))}
        mode="defaults"
        analyzers={analyzers}
        profiles={profiles}
      />
      {error && <Notice tone="error">{error}</Notice>}
      {note && <Notice tone="info">{note}</Notice>}
      <div className="cf-actions">
        <Button type="submit" variant="primary">
          Save defaults
        </Button>
        <Button type="button" variant="secondary" onClick={() => void store({}, 'Reset to the built-in defaults.')}>
          Reset to built-in defaults
        </Button>
      </div>
    </form>
  )
}

interface Editing {
  name: string
  body: string
  isNew: boolean
}

function ProfilesSection({
  profiles,
  loadError,
  onChanged,
}: {
  profiles: string[]
  loadError: string
  onChanged: () => Promise<void>
}) {
  const [editing, setEditing] = useState<Editing | null>(null)
  const [error, setError] = useState('')
  const [note, setNote] = useState('')

  async function edit(name: string) {
    setError('')
    setNote('')
    try {
      const p = await getHeaderProfile(name)
      setEditing({ name: p.name, body: p.body, isNew: false })
    } catch (e) {
      setError(message(e))
    }
  }

  function startNew() {
    setError('')
    setNote('')
    setEditing({ name: '', body: '', isNew: true })
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    if (!editing) return
    const name = editing.name.trim()
    setError('')
    setNote('')
    if (!PROFILE_NAME.test(name)) {
      setError("Use letters, digits, '.', '_' and '-' for the name, not starting with '.'.")
      return
    }
    if (editing.isNew && profiles.includes(name)) {
      setError(`A profile named "${name}" already exists. Pick another name, or edit it from the list.`)
      return
    }
    try {
      await saveHeaderProfile(name, editing.body)
      setEditing({ name, body: editing.body, isNew: false })
      setNote(`Saved profile "${name}".`)
      await onChanged()
    } catch (err) {
      setError(message(err))
    }
  }

  async function handleDelete() {
    if (!editing || editing.isNew) return
    if (!window.confirm(`Delete header profile "${editing.name}"? Crawls that use it fail until you pick another.`)) return
    setError('')
    try {
      await deleteHeaderProfile(editing.name)
      setNote(`Deleted profile "${editing.name}".`)
      setEditing(null)
      await onChanged()
    } catch (err) {
      setError(message(err))
    }
  }

  return (
    <section className="set-panel">
      <h2>Header profiles</h2>
      <p>
        Request headers stored on this machine in <code>~/.gocrawl/headers</code>, for example a
        Shopify crawler access signature. A crawl picks one by name, so the values never pass
        through the form.
      </p>
      {loadError && <Notice tone="error">{loadError}</Notice>}
      {profiles.length === 0 ? (
        <p>No header profiles yet. Create one for headers you send on every crawl of a site.</p>
      ) : (
        <ul className="set-profiles">
          {profiles.map((name) => (
            <li key={name}>
              <Button type="button" variant="secondary" onClick={() => void edit(name)} aria-pressed={editing?.name === name}>
                {name}
              </Button>
            </li>
          ))}
        </ul>
      )}
      <Button type="button" variant="secondary" onClick={startNew}>
        New profile
      </Button>

      {editing && (
        <form className="set-editor" onSubmit={handleSave}>
          <label className="cf-field">
            <span className="cf-label">Name</span>
            <input
              type="text"
              value={editing.name}
              readOnly={!editing.isNew}
              onChange={(e) => setEditing({ ...editing, name: e.target.value })}
              placeholder="uk"
            />
          </label>
          <label className="cf-field">
            <span className="cf-label">Headers (one &quot;Name: value&quot; per line)</span>
            <span className="cf-hint">Lines starting with # are comments.</span>
            <textarea
              rows={6}
              spellCheck={false}
              value={editing.body}
              onChange={(e) => setEditing({ ...editing, body: e.target.value })}
              placeholder={'# UK store\nSignature: sig1=:...:'}
            />
          </label>
          <div className="cf-actions">
            <Button type="submit" variant="primary">
              Save profile
            </Button>
            {!editing.isNew && (
              <Button type="button" variant="stop" onClick={() => void handleDelete()}>
                Delete profile
              </Button>
            )}
            <Button type="button" variant="secondary" onClick={() => setEditing(null)}>
              Close
            </Button>
          </div>
        </form>
      )}
      {error && <Notice tone="error">{error}</Notice>}
      {note && <Notice tone="info">{note}</Notice>}
    </section>
  )
}
```

  Create `web/src/views/SettingsView.css`:

```css
/* Settings view: one panel per section. Form fields reuse the New crawl form's cf- classes
   (CrawlForm.css, imported by CrawlFields). */
.set-panel {
  margin: 0 0 20px;
  padding: 14px 18px 18px;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.set-panel > h2 {
  margin-top: 0;
}

.set-profiles {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 12px 0;
  padding: 0;
  list-style: none;
}

.set-editor {
  margin: 16px 0 0;
}

.set-editor textarea {
  font-family: var(--font-mono);
}
```

- [ ] **Step 2: Build and lint.** `cd web && npm run build && npm run lint` — expected PASS.

- [ ] **Step 3: Manual check.** Fresh temp `HOME`, start the dev server, open Settings:
  1. Theme: pick Light — the page switches at once; reload — still light; `gui.json` has
     `"theme": "light"`.
  2. Crawl defaults: there are no Basic Auth, bearer token, cookie or extra-header fields, and
     the hint says so. Set Concurrency 2, Save defaults; open New crawl — Concurrency is 2.
     Reset to built-in defaults — New crawl shows the placeholders again.
  3. Header profiles: "No header profiles yet". New profile, name `uk`, body `nope`, Save: the
     server's error ("…want \"Name: value\"") shows and nothing is listed. Body
     `Signature: abc`, Save: `uk` is listed; `ls -l "$HOME/.gocrawl/headers"` shows
     `-rw-------`. Name `../x`: the name rule message shows. New crawl → custom headers → the
     dropdown lists `uk`. Delete `uk` (confirm): it disappears from both places.
  4. Crawl store shows `$HOME/.gocrawl/crawls` (read-only text).

- [ ] **Step 4: Run.** `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git checkout -- internal/webserver/webui/dist/index.html
git add web/src/views/SettingsView.tsx web/src/views/SettingsView.css
git commit -F - <<'MSG'
feat(web): Settings view with theme, crawl defaults, header profiles and store path

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-SET-6: Docs and changelog for the settings UI

**Files:**
- Modify: `docs/web.md`, `CHANGELOG.md`; generated `docs/README.md` via `make docs`

- [ ] **Step 1: Baseline.** `go test ./internal/repocheck/` — expected PASS.

- [ ] **Step 2: Edit `docs/web.md`.** Frontmatter `updated: 2026-10-13`. In "What the UI does":
  - In the **New crawl** item, delete the sentence "(A few CLI-only flags with no menu
    equivalent, like `--strip-query` and `--adaptive-delay`, aren't in the form either.)" and
    add: "It also has a per-request timeout, nofollow following, query-string stripping, the
    429/503 slow-down, a bearer token, and a header-profile dropdown filled from Settings. It
    opens pre-filled with your saved defaults; **Save as defaults** stores the current values
    (never passwords, cookies, tokens or inline headers)."
  - Add an item: "**Settings** — theme (dark, light, or match the system), crawl defaults
    (the New crawl fields, minus secrets), header profiles (list, create, edit, delete; each is
    validated like `--header-file` and stored with mode `0600` in `~/.gocrawl/headers`), and
    the crawl store's folder (read-only). Saved in `~/.gocrawl/gui.json`, not in the browser, so
    it survives the desktop app's changing port."

- [ ] **Step 3: Edit `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`:

```markdown
- Web UI: a Settings view (theme, crawl defaults, header profiles, store folder), and the New crawl form gains a request timeout, nofollow following, query-string stripping, the 429/503 slow-down toggle, a bearer token and a header-profile dropdown, pre-filled from saved defaults with **Save as defaults**.
```

- [ ] **Step 4: Regenerate and run.** `make docs`, then `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -F - <<'MSG'
docs: describe the Settings view and the new New crawl fields

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-LOGIN-1: Login-session API module and types

**Files:**
- Create: `web/src/types/login.ts`, `web/src/api/login.ts`
- Modify: `web/src/types/crawls.ts` (`StartCrawlParams.login_session`)

**Interfaces:**
- Consumes: `request<T>`, `postJSON<T>`; W1-LOGIN routes.
- Produces: `type LoginState`, `interface LoginSession`, `interface LoginAvailability`;
  `loginAvailability()`, `startLogin(url)`, `getLogin(id)`, `captureLogin(id)`, `cancelLogin(id)`.

- [ ] **Step 1: Write the code.** In `web/src/types/crawls.ts`, inside `StartCrawlParams`, add
  `login_session?: string` (web-only; never part of crawl defaults — `CrawlDefaults` already
  omits it). Create `web/src/types/login.ts`:

```ts
export type LoginState = 'waiting' | 'captured' | 'closed' | 'expired'

// GET /api/login-sessions/{id}. Never carries a cookie value.
export interface LoginSession {
  id: string
  state: LoginState
  host: string
  cookie_names: string[]
  cookie_count: number
  expires_at?: string
}

export interface LoginAvailability {
  available: boolean
  reason?: string
}
```

  Create `web/src/api/login.ts`:

```ts
import type { LoginAvailability, LoginSession } from '../types/login'
import { postJSON, request } from './client'

const base = '/api/login-sessions'

function sessionPath(id: string): string {
  return `${base}/${encodeURIComponent(id)}`
}

export function loginAvailability(): Promise<LoginAvailability> {
  return request<LoginAvailability>(`${base}/available`)
}

export function startLogin(url: string): Promise<LoginSession> {
  return postJSON<LoginSession>(base, { url })
}

export function getLogin(id: string): Promise<LoginSession> {
  return request<LoginSession>(sessionPath(id))
}

export function captureLogin(id: string): Promise<LoginSession> {
  return postJSON<LoginSession>(`${sessionPath(id)}/capture`, {})
}

export function cancelLogin(id: string): Promise<void> {
  return request<void>(sessionPath(id), { method: 'DELETE' })
}
```

- [ ] **Step 2: Build and lint.** `cd web && npm run build && npm run lint` — expected PASS.

- [ ] **Step 3: Manual check.** Dev server running; in the browser console:
  `await (await fetch('/api/login-sessions/available')).json()` matches `LoginAvailability`
  (`{available: true}` with Chrome installed, else `{available: false, reason: "no Chromium-class browser found …"}`).

- [ ] **Step 4: Run.** `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git checkout -- internal/webserver/webui/dist/index.html
git add web/src/types/crawls.ts web/src/types/login.ts web/src/api/login.ts
git commit -F - <<'MSG'
feat(web): login-session API client

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-LOGIN-2: "Log in with browser…" in the New crawl form

**Files:**
- Create: `web/src/components/crawlform/LoginPanel.tsx`
- Modify: `web/src/components/crawlform/values.ts`, `web/src/components/crawlform/CrawlFields.tsx`, `web/src/components/CrawlForm.tsx`, `web/src/components/CrawlForm.css`

**Interfaces:**
- Consumes: `api/login.ts`, `CrawlFields` `loginSlot` / `cookieLocked` props (W2-SET-3).
- Produces: `LOGOUT_EXCLUDE = '(?i)(log|sign)-?out'`, `withLine(list, line)`, `hostOf(url)` in
  `values.ts`; default export `LoginPanel({ url, blockedReason, session, onSession, onStarted })`.
  The start request carries `login_session` (and no `cookie`) once a login is captured.

- [ ] **Step 1: Extend `values.ts`.** Append to `web/src/components/crawlform/values.ts`:

```ts
// Added to the exclude field when a browser login starts, so the crawl doesn't follow a
// logout link and end the session. It stays visible in the field and can be removed.
export const LOGOUT_EXCLUDE = '(?i)(log|sign)-?out'

// withLine appends line to a newline-separated list unless it is already there.
export function withLine(list: string, line: string): string {
  const lines = splitList(list)
  return lines.includes(line) ? list : [...lines, line].join('\n')
}

// hostOf returns the lower-case host name of a seed URL as the server computes it
// (https:// assumed when missing), or '' when it doesn't parse.
export function hostOf(url: string): string {
  const raw = url.trim()
  if (!raw) return ''
  try {
    return new URL(raw.includes('://') ? raw : `https://${raw}`).hostname.toLowerCase().replace(/^\[|\]$/g, '')
  } catch {
    return ''
  }
}
```

- [ ] **Step 2: Show what the logout exclude is for.** In `CrawlFields.tsx` replace
  `import type { FieldsMode, FormValues } from './values'` with
  `import { LOGOUT_EXCLUDE, splitList, type FieldsMode, type FormValues } from './values'`
  and, inside the Exclude `<label className="cf-field">`, directly after its `<textarea … />`, add:

```tsx
            {crawl && splitList(v.exclude).includes(LOGOUT_EXCLUDE) && (
              <span className="cf-hint">
                {LOGOUT_EXCLUDE} keeps the crawl away from logout links, which would end your browser
                login. Remove the line if you need those pages crawled.
              </span>
            )}
```

- [ ] **Step 3: Write `LoginPanel.tsx`.** Create `web/src/components/crawlform/LoginPanel.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react'
import { cancelLogin, captureLogin, getLogin, loginAvailability, startLogin } from '../../api/login'
import type { LoginAvailability, LoginSession } from '../../types/login'
import Button from '../ui/Button'
import Notice from '../ui/Notice'
import { hostOf } from './values'

const POLL_MS = 1500

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

interface Props {
  url: string
  // Why the button can't be used right now ('' when it can), e.g. headless rendering.
  blockedReason: string
  session: LoginSession | null
  onSession: (s: LoginSession | null) => void
  // Called once a browser window has opened, so the form can add the logout exclude.
  onStarted: () => void
}

export default function LoginPanel({ url, blockedReason, session, onSession, onStarted }: Props) {
  const [availability, setAvailability] = useState<LoginAvailability | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    loginAvailability()
      .then(setAvailability)
      .catch((e: unknown) => setAvailability({ available: false, reason: message(e) }))
  }, [])

  // Poll while the window is open, so closing it or the one-hour expiry shows without a click.
  const waitingId = session?.state === 'waiting' ? session.id : ''
  useEffect(() => {
    if (!waitingId) return
    const timer = window.setInterval(() => {
      getLogin(waitingId)
        .then((s) => {
          if (s.state !== 'waiting') onSession(s)
        })
        .catch(() => undefined)
    }, POLL_MS)
    return () => window.clearInterval(timer)
  }, [waitingId, onSession])

  // Leaving the form while a window waits closes that window. A captured login is kept: the
  // crawl the form just started uses it.
  const waitingRef = useRef('')
  useEffect(() => {
    waitingRef.current = waitingId
  }, [waitingId])
  useEffect(
    () => () => {
      if (waitingRef.current) void cancelLogin(waitingRef.current).catch(() => undefined)
    },
    [],
  )

  async function start() {
    setBusy(true)
    setError('')
    try {
      onSession(await startLogin(url.trim()))
      onStarted()
    } catch (e) {
      setError(message(e))
    } finally {
      setBusy(false)
    }
  }

  async function capture() {
    if (!session) return
    setBusy(true)
    setError('')
    try {
      onSession(await captureLogin(session.id))
    } catch (e) {
      setError(message(e))
      // The window may have been closed; show the session's real state.
      getLogin(session.id)
        .then(onSession)
        .catch(() => onSession(null))
    } finally {
      setBusy(false)
    }
  }

  async function forget() {
    if (!session) return
    setBusy(true)
    setError('')
    try {
      await cancelLogin(session.id)
    } catch {
      // Already gone on the server (expired and pruned): nothing left to cancel.
    } finally {
      onSession(null)
      setBusy(false)
    }
  }

  const unavailable = availability !== null && !availability.available
  const host = hostOf(url)
  let body
  if (session?.state === 'waiting') {
    body = (
      <>
        <p role="status">
          A browser window is open on {session.host}. Log in there, then come back and press "I'm logged in".
        </p>
        <div className="cf-actions">
          <Button type="button" variant="primary" disabled={busy} onClick={() => void capture()}>
            I'm logged in
          </Button>
          <Button type="button" variant="secondary" disabled={busy} onClick={() => void forget()}>
            Cancel
          </Button>
        </div>
      </>
    )
  } else if (session?.state === 'captured') {
    body = (
      <>
        <p role="status">
          Logged in to {session.host}: {session.cookie_count} {session.cookie_count === 1 ? 'cookie' : 'cookies'} (
          {session.cookie_names.join(', ')}).{' '}
          {session.expires_at
            ? `The first one expires ${new Date(session.expires_at).toLocaleString()}.`
            : 'All of them last until the browser session ends.'}{' '}
          gocrawl keeps them in memory for an hour and sends them to {session.host} only.
        </p>
        <Button type="button" variant="secondary" disabled={busy} onClick={() => void forget()}>
          Forget this login
        </Button>
      </>
    )
  } else {
    body = (
      <>
        {session?.state === 'closed' && (
          <Notice tone="warning">
            The browser window was closed before you pressed "I'm logged in", so nothing was captured. Log in again to retry.
          </Notice>
        )}
        {session?.state === 'expired' && (
          <Notice tone="warning">This login expired after an hour and its cookies were discarded. Log in again.</Notice>
        )}
        <Button
          type="button"
          variant="secondary"
          disabled={busy || availability === null || unavailable || blockedReason !== '' || host === ''}
          onClick={() => void start()}
        >
          Log in with browser…
        </Button>
        {unavailable ? (
          <p className="cf-hint">
            {availability?.reason || 'No Chromium-class browser found.'} Install Google Chrome, Chromium or Microsoft Edge
            to log in with a browser.
          </p>
        ) : blockedReason ? (
          <p className="cf-hint">{blockedReason}</p>
        ) : host === '' ? (
          <p className="cf-hint">Enter the site URL first.</p>
        ) : (
          <p className="cf-hint">
            Opens a browser window on the site URL in a fresh profile. Log in there (single sign-on, two-factor and
            CAPTCHAs work); the crawl then uses that session's cookies, sent to {host} only.
          </p>
        )}
      </>
    )
  }

  return (
    <fieldset className="cf-login">
      <legend>Browser login</legend>
      {body}
      {error && <Notice tone="error">{error}</Notice>}
    </fieldset>
  )
}
```

- [ ] **Step 4: Wire it into `CrawlForm.tsx`.** Apply to `web/src/components/CrawlForm.tsx`
  (the W2-SET-4 version):
  1. Imports: add `import type { LoginSession } from '../types/login'` and
     `import LoginPanel from './crawlform/LoginPanel'`; change the values import to
     `import { LOGOUT_EXCLUDE, fromParams, hostOf, splitList, toParams, withLine, type FormValues } from './crawlform/values'`.
  2. After `const [error, setError] = useState('')` add:

```tsx
  const [login, setLogin] = useState<LoginSession | null>(null)
  const loginBlocked =
    values.render === 'headless'
      ? 'Browser login works with raw rendering only. Set Render to raw first.'
      : values.cookieAuth && values.cookie.trim()
        ? 'Clear the cookie header first: the browser login supplies the cookies.'
        : ''

  function addLogoutExclude() {
    setDirty(true)
    setValues((v) => ({ ...v, exclude: withLine(v.exclude, LOGOUT_EXCLUDE) }))
  }
```

  3. In `handleSubmit`, directly before `setSubmitting(true)` add:

```tsx
    if (login?.state === 'waiting') {
      setError('Finish the browser login first: press "I\'m logged in", or cancel it.')
      return
    }
    if (login?.state === 'captured' && hostOf(values.url) !== login.host) {
      setError(`The browser login is for ${login.host}, but the seed URL is on ${hostOf(values.url) || 'another host'}. Log in again for this site.`)
      return
    }
```

     and directly after the `const params: StartCrawlParams = …` line add:

```tsx
    if (login?.state === 'captured') {
      params.login_session = login.id
      delete params.cookie
    }
```

  4. Replace the `<CrawlFields … />` element with:

```tsx
      <CrawlFields
        values={values}
        onChange={update}
        mode="crawl"
        analyzers={analyzers}
        profiles={profiles}
        cookieLocked={login?.state === 'captured'}
        loginSlot={
          <LoginPanel
            url={values.url}
            blockedReason={loginBlocked}
            session={login}
            onSession={setLogin}
            onStarted={addLogoutExclude}
          />
        }
      />
```

  5. Append to `web/src/components/CrawlForm.css`:

```css
.cf-login {
  margin: 16px 0 0;
  padding: 12px 14px 14px;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}

.cf-login > legend {
  padding: 0 4px;
  font-size: 15px;
  font-weight: 700;
  color: var(--ink);
}
```

- [ ] **Step 5: Build and lint.** `cd web && npm run build && npm run lint` — expected PASS.

- [ ] **Step 6: Manual check** (needs Chrome, Chromium or Edge; dev server as in Conventions):
  1. Advanced → Browser login: with no site URL the button is disabled ("Enter the site URL
     first"); with Render = headless it is disabled with the raw-rendering hint.
  2. Seed `https://github.com/login`, press **Log in with browser…**: a window opens on the
     page; the form shows the waiting text with **I'm logged in** and **Cancel**; the Exclude
     field now contains `(?i)(log|sign)-?out` with the hint under it.
  3. Press **I'm logged in** before logging in: an error says there are no cookies yet (if
     the site set none) or the session is captured; log in, press it again: the window closes
     and the summary lists cookie names, the count and the first expiry — no values anywhere
     (check DevTools → Network responses).
  4. Change the seed to `https://example.com` and press Start crawl: the host-mismatch error
     shows. Put the seed back, start: the request body has `login_session`, no `cookie`; the
     crawl's pages are the logged-in ones.
  5. Start another login and close the window by hand: within ~2s the form shows the "window
     was closed" notice and the button again.
  6. Start a login, then navigate to Crawls while it waits: the window closes.
  7. Without a browser (rename the Chrome app, or run on a machine without one): the button is
     disabled with the install hint.

- [ ] **Step 7: Run.** `make check` — expected PASS.

- [ ] **Step 8: Commit.**

```sh
git checkout -- internal/webserver/webui/dist/index.html
git add web/src/components/crawlform/LoginPanel.tsx web/src/components/crawlform/values.ts web/src/components/crawlform/CrawlFields.tsx web/src/components/CrawlForm.tsx web/src/components/CrawlForm.css
git commit -F - <<'MSG'
feat(web): log in with a browser from the New crawl form

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

#### Task W2-LOGIN-3: Docs and changelog for the login UI

**Files:**
- Modify: `docs/web.md`, `CHANGELOG.md`; generated `docs/README.md` via `make docs`

- [ ] **Step 1: Baseline.** `go test ./internal/repocheck/` — expected PASS.

- [ ] **Step 2: Edit `docs/web.md`.** Frontmatter `updated: 2026-10-13`. In the **New crawl**
  item of "What the UI does", add: "Under Advanced, **Log in with browser…** opens a real
  browser window for sites behind a login form (see
  [Logging in with a browser](#logging-in-with-a-browser)); the form then shows the captured
  cookie names, adds a logout exclude pattern you can remove, and starts the crawl with that
  session. The button is disabled, with the reason, when no Chromium-class browser is installed
  or Render is headless."

- [ ] **Step 3: Edit `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`:

```markdown
- Web UI: **Log in with browser…** in the New crawl form captures a real browser login (SSO, two-factor and CAPTCHAs work) and crawls with it, showing only cookie names, count and expiry, and excluding logout links by default.
```

- [ ] **Step 4: Regenerate and run.** `make docs`, then `make check` — expected PASS.

- [ ] **Step 5: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -F - <<'MSG'
docs: describe browser login in the New crawl form

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
MSG
```

---

### Track: compare UI

Rebase onto the merged progress UI before W2-CMP-2.

#### Task W2-CMP-1: compare and import API modules

**Files:**
- Create: `web/src/types/compare.ts`, `web/src/api/compare.ts`, `web/src/api/import.ts`

**Interfaces:**
- Consumes: `request<T>`, `postForm<T>` from `web/src/api/client.ts`; `Issue` from `web/src/types/crawls.ts`
- Produces: `CompareMeta`, `CompareResult`, `ImportResult` types;
  `compareCrawls(base: string, current: string): Promise<CompareResult>`;
  `importReport(file: File): Promise<ImportResult>`

- [ ] **Step 1: Create `web/src/types/compare.ts`.**

```ts
// Mirrors internal/diff.Diff (GET /api/compare) and the POST /api/reports/import response.
// Issue and page lists can be null in the JSON when empty (Go nil slices); read them with `?? []`.
import type { Issue } from './crawls'

export interface CompareMeta {
  seed: string
  started_at: string
  finished_at: string
  pages_crawled: number
}

export interface CompareSummary {
  by_severity: Record<string, number>
  by_analyzer: Record<string, number>
  pages_by_status: Record<string, number>
  new_by_severity: Record<string, number>
  resolved_by_severity: Record<string, number>
}

export interface CompareResult {
  base: CompareMeta
  current: CompareMeta
  issues: {
    new: Issue[] | null
    resolved: Issue[] | null
    persisting: Issue[] | null
  }
  summary: CompareSummary
  pages: {
    added: string[] | null
    removed: string[] | null
  }
}

export interface ImportResult {
  id: string
}
```

- [ ] **Step 2: Create `web/src/api/compare.ts`.**

```ts
import { request } from './client'
import type { CompareResult } from '../types/compare'

// A ref is a crawl id from listCrawls, a host such as example.com, or "latest".
export function compareCrawls(base: string, current: string): Promise<CompareResult> {
  const q = new URLSearchParams({ base, current })
  return request<CompareResult>(`/api/compare?${q.toString()}`)
}
```

- [ ] **Step 3: Create `web/src/api/import.ts`.**

```ts
import { postForm } from './client'
import type { ImportResult } from '../types/compare'

// Saves a gocrawl JSON report to the crawl store; resolves to its crawl id.
export function importReport(file: File): Promise<ImportResult> {
  const form = new FormData()
  form.append('file', file, file.name)
  return postForm<ImportResult>('/api/reports/import', form)
}
```

- [ ] **Step 4: Build and lint.** `cd web && npm run build && npm run lint` Expected: both exit 0.

- [ ] **Step 5: Run the checker.** `git checkout -- internal/webserver/webui/dist/index.html && make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add web/src/types/compare.ts web/src/api/compare.ts web/src/api/import.ts
git commit -m "feat(web): add compare and report-import API clients" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W2-CMP-2: move the issue table out of CrawlReport into a reusable `IssueTable`

**Files:**
- Create: `web/src/components/IssueTable.tsx`
- Modify: `web/src/components/CrawlReport.tsx`

**Interfaces:**
- Consumes: `useSort` from `web/src/sort.ts`; `Issue`, `Explanation` from `types/crawls`
- Produces: `export default function IssueTable(props: { issues: Issue[]; explanations: Record<string, Explanation>; review?: { map: ReviewMap; set: Dispatch<SetStateAction<ReviewMap>> } })`,
  `export interface ReviewState`, `export type ReviewMap = Record<number, ReviewState>`.
  Without `review`, the table is read-only: it has no Review column, no bulk bar, and no
  hide-resolved/non-issue toggles.

The block that moves is everything inside the issues tab of the visual system's
`CrawlReport.tsx` (W1-VIS-7): `<div className="cr-filters">`, `<div className="cr-bulk">` and
`<div className="cr-table-scroll">`. The code below is that block with its state and helpers,
so the move changes no behavior and no markup.

- [ ] **Step 1: Create `web/src/components/IssueTable.tsx`.** It is the visual system's
  version of the block (W1-VIS-7), made optional on `review`, and it imports `CrawlReport.css`
  so Compare gets the same `cr-` styles:

```tsx
import { useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { useSort } from '../sort'
import type { Explanation, Issue } from '../types/crawls'
import Button from './ui/Button'
import './CrawlReport.css'

const SEVERITY_RANK: Record<string, number> = { error: 0, warning: 1, info: 2 }

export interface ReviewState {
  resolved: boolean
  nonissue: boolean
  note: string
}

// Review state keyed by index into the issues array.
export type ReviewMap = Record<number, ReviewState>

const EMPTY_REVIEW: ReviewState = { resolved: false, nonissue: false, note: '' }

type IssueRow = Issue & { _key: number }

function issueSortValue(row: IssueRow, key: string): string | number {
  switch (key) {
    case 'severity':
      return SEVERITY_RANK[row.severity] ?? 9
    case 'analyzer':
      return row.analyzer
    case 'code':
      return row.code
    case 'url':
      return row.url
    case 'message':
      return row.message
    default:
      return ''
  }
}

interface IssueTableProps {
  issues: Issue[]
  explanations: Record<string, Explanation>
  // When given, the table shows the Review column, the bulk bar and the hide-resolved /
  // hide-non-issue toggles, and writes review state here. Compare passes nothing: its issue
  // lists are read-only.
  review?: { map: ReviewMap; set: Dispatch<SetStateAction<ReviewMap>> }
}

// The report's issue table, moved out of CrawlReport unchanged (W1-VIS-7 markup and cr-
// classes from CrawlReport.css): severity / analyzer / code / text filters, sortable columns,
// inline explanations and data, and (for a single crawl's report) per-issue review state.
export default function IssueTable({ issues, explanations, review }: IssueTableProps) {
  const [severityOn, setSeverityOn] = useState<Record<string, boolean>>({ error: true, warning: true, info: true })
  const [analyzerFilter, setAnalyzerFilter] = useState('')
  const [search, setSearch] = useState('')
  const [codeOn, setCodeOn] = useState<Record<string, boolean>>({})
  const [codeMenuOpen, setCodeMenuOpen] = useState(false)
  const [hideResolved, setHideResolved] = useState(false)
  const [hideNonissue, setHideNonissue] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const selectAllRef = useRef<HTMLInputElement>(null)
  const reviewMap = review?.map

  useEffect(() => {
    if (!codeMenuOpen) return
    function close() {
      setCodeMenuOpen(false)
    }
    document.addEventListener('click', close)
    return () => document.removeEventListener('click', close)
  }, [codeMenuOpen])

  const analyzers = useMemo(() => [...new Set(issues.map((i) => i.analyzer))].sort(), [issues])
  const codeCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    issues.forEach((i) => {
      counts[i.code] = (counts[i.code] ?? 0) + 1
    })
    return counts
  }, [issues])
  const codeList = useMemo(() => Object.keys(codeCounts).sort(), [codeCounts])
  const hiddenCodeCount = codeList.filter((c) => codeOn[c] === false).length

  const rows: IssueRow[] = useMemo(() => issues.map((i, originalIndex) => ({ ...i, _key: originalIndex })), [issues])

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase()
    return rows.filter((i) => {
      if (severityOn[i.severity] === false) return false
      if (analyzerFilter && i.analyzer !== analyzerFilter) return false
      if (codeOn[i.code] === false) return false
      if (term && !(i.url.toLowerCase().includes(term) || i.message.toLowerCase().includes(term) || i.code.toLowerCase().includes(term))) {
        return false
      }
      const r = reviewMap?.[i._key]
      if (hideResolved && r?.resolved) return false
      if (hideNonissue && r?.nonissue) return false
      return true
    })
  }, [rows, severityOn, analyzerFilter, codeOn, search, hideResolved, hideNonissue, reviewMap])

  const { sorted, toggle, dirFor } = useSort(filtered, issueSortValue, { key: 'severity' })

  const selectedShownCount = sorted.filter((r) => selected.has(r._key)).length
  useEffect(() => {
    if (!selectAllRef.current) return
    selectAllRef.current.indeterminate = selectedShownCount > 0 && selectedShownCount < sorted.length
  }, [selectedShownCount, sorted.length])

  function setReviewFlag(key: number, patch: Partial<ReviewState>) {
    review?.set((prev) => ({
      ...prev,
      [key]: { ...EMPTY_REVIEW, ...prev[key], ...patch },
    }))
  }

  function toggleSelected(key: number) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function selectShown(checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      sorted.forEach((r) => (checked ? next.add(r._key) : next.delete(r._key)))
      return next
    })
  }

  function bulkApply(kind: 'resolved' | 'nonissue' | 'clear') {
    if (!review) return
    review.set((prev) => {
      const next = { ...prev }
      sorted.forEach((r) => {
        if (!selected.has(r._key)) return
        if (kind === 'clear') {
          next[r._key] = { ...EMPTY_REVIEW, note: prev[r._key]?.note ?? '' }
        } else {
          next[r._key] = { ...EMPTY_REVIEW, ...prev[r._key], [kind]: true }
        }
      })
      return next
    })
    setSelected(new Set())
  }

  function clearFilters() {
    setSeverityOn({ error: true, warning: true, info: true })
    setAnalyzerFilter('')
    setSearch('')
    setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))
    setHideResolved(false)
    setHideNonissue(false)
  }

  const filtersActive =
    search.trim() !== '' ||
    analyzerFilter !== '' ||
    hiddenCodeCount > 0 ||
    hideResolved ||
    hideNonissue ||
    Object.values(severityOn).some((v) => v === false)

  return (
    <>
      <div className="cr-filters">
        <div className="cr-sevs" role="group" aria-label="Severity">
          {(['error', 'warning', 'info'] as const).map((sev) => (
            <button
              key={sev}
              type="button"
              className={`cr-sev sev-${sev}`}
              aria-pressed={severityOn[sev]}
              onClick={() => setSeverityOn((prev) => ({ ...prev, [sev]: !prev[sev] }))}
            >
              {sev.charAt(0).toUpperCase() + sev.slice(1)}
            </button>
          ))}
        </div>
        <label className="cr-inline">
          Analyzer
          <select value={analyzerFilter} onChange={(e) => setAnalyzerFilter(e.target.value)}>
            <option value="">All</option>
            {analyzers.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
        <div className={`cr-codes${hiddenCodeCount > 0 ? ' cr-codes-hidden' : ''}`}>
          <Button
            aria-expanded={codeMenuOpen}
            onClick={(e) => {
              e.stopPropagation()
              setCodeMenuOpen((v) => !v)
            }}
          >
            {hiddenCodeCount > 0 ? `Codes (${hiddenCodeCount} hidden)` : 'Codes'}
            <span aria-hidden="true">▾</span>
          </Button>
          <div className="cr-codes-panel" hidden={!codeMenuOpen} onClick={(e) => e.stopPropagation()}>
            <div className="cr-codes-actions">
              <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))}>Show all</Button>
              <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, false])))}>Hide all</Button>
            </div>
            {codeList.map((c) => (
              <label key={c} className="cr-code-option">
                <input
                  type="checkbox"
                  checked={codeOn[c] !== false}
                  onChange={(e) => setCodeOn((prev) => ({ ...prev, [c]: e.target.checked }))}
                />
                <code>{c}</code>
                <span className="num muted">{codeCounts[c]}</span>
              </label>
            ))}
          </div>
        </div>
        <input
          type="search"
          className="cr-search"
          placeholder="Search URL, message, code…"
          aria-label="Search issues"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        {review && (
          <>
            <label className="cr-inline">
              <input type="checkbox" checked={hideResolved} onChange={(e) => setHideResolved(e.target.checked)} />
              Hide resolved
            </label>
            <label className="cr-inline">
              <input type="checkbox" checked={hideNonissue} onChange={(e) => setHideNonissue(e.target.checked)} />
              Hide non-issues
            </label>
          </>
        )}
        <span className="cr-count-line num muted">
          Showing {sorted.length.toLocaleString()} of {rows.length.toLocaleString()} issues
        </span>
        {filtersActive && <Button onClick={clearFilters}>Clear filters</Button>}
      </div>

      {review && (
        <div className="cr-bulk">
          <label className="cr-inline">
            <input
              ref={selectAllRef}
              type="checkbox"
              checked={sorted.length > 0 && selectedShownCount === sorted.length}
              onChange={(e) => selectShown(e.target.checked)}
            />
            Select shown
          </label>
          <Button disabled={selected.size === 0} onClick={() => bulkApply('nonissue')}>
            Mark as non-issue
          </Button>
          <Button disabled={selected.size === 0} onClick={() => bulkApply('resolved')}>
            Mark as resolved
          </Button>
          <Button disabled={selected.size === 0} onClick={() => bulkApply('clear')}>
            Clear review
          </Button>
          {selected.size > 0 && <span className="num muted">{selected.size} selected</span>}
        </div>
      )}

      <div className="cr-table-scroll">
        <table className="cr-issues">
          <thead>
            <tr>
              {(['severity', 'analyzer', 'code', 'url', 'message'] as const).map((col) => (
                <th
                  key={col}
                  className="cr-sortable"
                  onClick={() => toggle(col)}
                  data-dir={dirFor(col)}
                  aria-sort={dirFor(col) === 'asc' ? 'ascending' : dirFor(col) === 'desc' ? 'descending' : undefined}
                >
                  {col === 'url' ? 'URL' : col.charAt(0).toUpperCase() + col.slice(1)}
                </th>
              ))}
              {review && <th>Review</th>}
            </tr>
          </thead>
          <tbody>
            {sorted.map((i) => {
              const explanation = explanations[i.code]
              const r = reviewMap?.[i._key]
              return (
                <tr key={i._key} className={r?.resolved || r?.nonissue ? 'cr-reviewed' : undefined}>
                  <td className={`sev-${i.severity}`}>{i.severity}</td>
                  <td className="mono">{i.analyzer}</td>
                  <td>
                    <code>{i.code}</code>
                  </td>
                  <td className="cr-url mono" title={i.url}>
                    {i.url}
                  </td>
                  <td>
                    {i.message}
                    {explanation && (
                      <details className="cr-explain">
                        <summary>What this means and how to fix it</summary>
                        <dl>
                          <dt>What it is</dt>
                          <dd>{explanation.what}</dd>
                          <dt>Impact</dt>
                          <dd>{explanation.impact}</dd>
                          <dt>How to fix it</dt>
                          <dd>{explanation.fix}</dd>
                        </dl>
                      </details>
                    )}
                    {i.data && (
                      <details className="cr-explain">
                        <summary>Data</summary>
                        <pre>{JSON.stringify(i.data, null, 2)}</pre>
                      </details>
                    )}
                  </td>
                  {review && (
                    <td className="cr-review">
                      <label className="cr-inline">
                        <input type="checkbox" checked={selected.has(i._key)} onChange={() => toggleSelected(i._key)} />
                        Select
                      </label>
                      <label className="cr-inline">
                        <input
                          type="checkbox"
                          checked={!!r?.resolved}
                          onChange={(e) => setReviewFlag(i._key, { resolved: e.target.checked })}
                        />
                        Resolved
                      </label>
                      <label className="cr-inline">
                        <input
                          type="checkbox"
                          checked={!!r?.nonissue}
                          onChange={(e) => setReviewFlag(i._key, { nonissue: e.target.checked })}
                        />
                        Non-issue
                      </label>
                      <textarea
                        rows={2}
                        placeholder="Add a comment…"
                        aria-label="Comment"
                        value={r?.note ?? ''}
                        onChange={(e) => setReviewFlag(i._key, { note: e.target.value })}
                      />
                    </td>
                  )}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </>
  )
}
```

- [ ] **Step 2: Make CrawlReport use it.** Edit `web/src/components/CrawlReport.tsx`:
  1. Imports: delete `import { useSort } from '../sort'`. Add
     `import IssueTable, { type ReviewMap } from './IssueTable'`. Remove `Issue` from the
     `types/crawls` type import. Remove `useMemo` and `useRef` from the `react` import if
     nothing else in the file uses them (`tsc` reports them through `noUnusedLocals`; the
     progress track already moved the poll timer ref out).
  2. Delete the module-level `SEVERITY_RANK`, the `ReviewState` interface, the `IssueRow`
     type, and `issueSortValue`.
  3. In the component, delete the `useState` lines for `severityOn`, `analyzerFilter`,
     `search`, `codeOn`, `codeMenuOpen`, `hideResolved`, `hideNonissue` and `selected`, and
     `selectAllRef`. Change the review state to `const [review, setReview] = useState<ReviewMap>({})`.
  4. Delete the `codeMenuOpen` close effect, `codeCounts`, `codeList`, `hiddenCodeCount`,
     `rows`, `filtered`, the `useSort` call, `selectedShownCount` and its indeterminate
     effect, `emptyReview`, `setReviewFlag`, `toggleSelected`, `selectShown`, `bulkApply`,
     `clearFilters` and `filtersActive`. Keep `exportReviewed`. It still reads `review`.
  5. In the issues branch of the tab ternary, replace the whole fragment
     `<> <div className="cr-filters">…</div> <div className="cr-bulk">…</div> <div className="cr-table-scroll">…</div> </>`
     with:

```tsx
<IssueTable issues={report.issues} explanations={explanations} review={{ map: review, set: setReview }} />
```

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint` Expected: both exit 0.

- [ ] **Step 4: Manual check.** Run `make web-build && go run ./cmd/gocrawl serve`, open
  `http://localhost:8080`, and open a saved crawl from Crawls (run
  `go run ./cmd/gocrawl crawl https://example.com --depth 0 --save` first if none exists).
  Expected: the issue table behaves as before. Severity, analyzer, code and search filters
  work. Columns sort. Explanations expand. Marking a row Resolved and then turning on "Hide
  resolved" hides it. "Export reviewed JSON" still includes the `review` object for that row.

- [ ] **Step 5: Run the checker.** `git checkout -- internal/webserver/webui/dist/index.html && make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add web/src/components/IssueTable.tsx web/src/components/CrawlReport.tsx
git commit -m "refactor(web): move the report's issue table into a reusable IssueTable" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W2-CMP-3: the Compare view

**Files:**
- Create: `web/src/views/compare/statusTape.ts`, `web/src/views/compare/pairs.ts`, `web/src/views/compare.css`
- Modify: `web/src/views/CompareView.tsx` (replaces the wave-0 stub), `web/src/App.tsx`

**Interfaces:**
- Consumes: `compareCrawls`; `listCrawls`, `getCrawl`, `listExplanations`; `IssueTable`; `Tape`
  (`{tape, tapeOff, summary, compact}`); `countTape`, `describeTape` (`components/tape/tapeModel.ts`);
  `Report.coverage.tape` / `tape_off` (TypeScript fields from W1-PROG-6b); `Button`; `Notice`; `href`.
- Produces: `export default function CompareView(props: { base?: string; current?: string })`;
  `statusTapeFromSummary(pagesByStatus: Record<string, number>): string`;
  `tapeSummary(pagesByStatus: Record<string, number>): string`;
  `hostOf(seed: string): string`; `isComparable(item: CrawlListItem): boolean`;
  `newestFirst(items: CrawlListItem[]): CrawlListItem[]`;
  `defaultComparePair(newestFirstItems: CrawlListItem[]): { base: string; current: string } | null`;
  `orderPair(a: CrawlListItem, b: CrawlListItem): { base: string; current: string }`;
  `crawlLabel(item: CrawlListItem): string`.

How the view gets tapes: it calls `getCrawl(id)` for each side. Reports saved by this version
carry the tape in `report.coverage.tape` / `tape_off` (W1-PROG-6b): fetch order, notches for
pages not in the sitemap, the same tape the live view showed. `CompareTape` draws that. Only a
report saved before tapes were stored falls back to `report.summary.pages_by_status`, with the
ticks grouped by status code, lowest first, and a caption that says so. A failed `getCrawl`
(for example a `latest` ref) shows the side's page count in place of a tape. It does not fail
the view.

Selection state lives in the URL (`#/compare?...` through `href`). Changing a picker
navigates, so the view stays shareable and Back works.

- [ ] **Step 1: Create `web/src/views/compare/statusTape.ts`.**

```ts
// A status-only tape for a finished crawl whose report predates coverage.tape, built from its
// summary. summary.pages_by_status counts pages per HTTP status but not the order they were
// fetched in, so the ticks are grouped by status code, lowest first. Status 0 (fetch failed)
// and anything outside 2xx-5xx becomes 'x'.
function tapeChar(code: number): string {
  if (code >= 200 && code < 300) return '2'
  if (code >= 300 && code < 400) return '3'
  if (code >= 400 && code < 500) return '4'
  if (code >= 500 && code < 600) return '5'
  return 'x'
}

export function statusTapeFromSummary(pagesByStatus: Record<string, number>): string {
  return Object.entries(pagesByStatus)
    .map(([code, n]) => ({ code: Number(code), n }))
    .filter((e) => e.n > 0)
    .sort((a, b) => (Number.isNaN(a.code) ? 999 : a.code) - (Number.isNaN(b.code) ? 999 : b.code))
    .map((e) => tapeChar(e.code).repeat(e.n))
    .join('')
}

// "120 pages: 110 2xx, 6 3xx, 4 4xx" — the text twin of the tape.
export function tapeSummary(pagesByStatus: Record<string, number>): string {
  const counts: Record<string, number> = {}
  let total = 0
  for (const [code, n] of Object.entries(pagesByStatus)) {
    if (!(n > 0)) continue
    const c = tapeChar(Number(code))
    counts[c] = (counts[c] ?? 0) + n
    total += n
  }
  const parts = ['2', '3', '4', '5'].filter((c) => counts[c]).map((c) => `${counts[c].toLocaleString()} ${c}xx`)
  if (counts.x) parts.push(`${counts.x.toLocaleString()} failed to fetch`)
  const pages = `${total.toLocaleString()} page${total === 1 ? '' : 's'}`
  return parts.length > 0 ? `${pages}: ${parts.join(', ')}` : pages
}
```

- [ ] **Step 2: Create `web/src/views/compare/pairs.ts`.**

```ts
import type { CrawlListItem } from '../../types/crawls'

export function hostOf(seed: string): string {
  try {
    return new URL(seed).hostname
  } catch {
    return seed
  }
}

// A crawl can be compared once it has a report: finished, or stopped with a partial report.
export function isComparable(item: CrawlListItem): boolean {
  return item.status === 'done' || item.status === 'canceled'
}

function finishedMs(item: CrawlListItem): number {
  const t = Date.parse(item.finished_at ?? '')
  return Number.isNaN(t) ? 0 : t
}

export function newestFirst(items: CrawlListItem[]): CrawlListItem[] {
  return [...items].sort((a, b) => finishedMs(b) - finishedMs(a))
}

// The newest crawl that has an older crawl of the same host, paired with that older crawl.
// Expects items already sorted newest first.
export function defaultComparePair(items: CrawlListItem[]): { base: string; current: string } | null {
  for (let i = 0; i < items.length; i++) {
    const host = hostOf(items[i].seed)
    const older = items.slice(i + 1).find((o) => hostOf(o.seed) === host)
    if (older) return { base: older.id, current: items[i].id }
  }
  return null
}

// Puts the earlier crawl on the base side.
export function orderPair(a: CrawlListItem, b: CrawlListItem): { base: string; current: string } {
  return finishedMs(a) <= finishedMs(b) ? { base: a.id, current: b.id } : { base: b.id, current: a.id }
}

export function crawlLabel(item: CrawlListItem): string {
  const when = item.finished_at ? new Date(item.finished_at).toLocaleString() : 'not finished'
  return `${hostOf(item.seed)} · ${when} · ${(item.pages_crawled ?? 0).toLocaleString()} pages`
}
```

- [ ] **Step 3: Create `web/src/views/compare.css`.**

```css
.compare-pickers {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: flex-end;
  margin-bottom: 1rem;
}
.compare-pickers label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  color: var(--ink-2);
}
.compare-pickers select {
  min-width: 20rem;
}
.compare-tapes {
  display: grid;
  gap: 1rem;
  margin: 1rem 0;
}
.compare-tape-label {
  margin: 0 0 0.25rem;
  color: var(--ink);
}
.compare-tape-counts {
  margin: 0.25rem 0 0;
  color: var(--ink-2);
}
.compare-headline {
  font-size: 1.125rem;
}
.compare-deltas {
  display: flex;
  flex-wrap: wrap;
  gap: 2rem;
  margin: 1rem 0;
}
.compare-deltas caption {
  text-align: left;
  font-weight: 600;
  padding-bottom: 0.25rem;
}
.compare-deltas td {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.compare-pages {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 1.5rem;
  margin-top: 1.5rem;
}
.compare-url-list {
  max-height: 24rem;
  overflow: auto;
  padding-left: 1.25rem;
}
```

- [ ] **Step 4: Replace `web/src/views/CompareView.tsx`.**

```tsx
import { useEffect, useMemo, useState } from 'react'
import { compareCrawls } from '../api/compare'
import { getCrawl, listCrawls, listExplanations } from '../api/crawls'
import IssueTable from '../components/IssueTable'
import Tape from '../components/tape/Tape'
import { countTape, describeTape } from '../components/tape/tapeModel'
import Button from '../components/ui/Button'
import Notice from '../components/ui/Notice'
import { href } from '../routes'
import type { CompareMeta, CompareResult } from '../types/compare'
import type { CrawlListItem, Explanation, Report } from '../types/crawls'
import { crawlLabel, defaultComparePair, isComparable, newestFirst } from './compare/pairs'
import { statusTapeFromSummary, tapeSummary } from './compare/statusTape'
import './compare.css'

type Tab = 'new' | 'resolved' | 'persisting'
const TABS: Tab[] = ['new', 'resolved', 'persisting']
const TAB_LABEL: Record<Tab, string> = { new: 'New', resolved: 'Resolved', persisting: 'Persisting' }
const SEVERITIES = ['error', 'warning', 'info'] as const
const MAX_URLS = 1000

interface Loaded {
  key: string
  diff: CompareResult | null
  baseReport: Report | null
  currentReport: Report | null
  error: string
}

function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function signed(n: number): string {
  if (n > 0) return `+${n.toLocaleString()}`
  if (n < 0) return `−${Math.abs(n).toLocaleString()}`
  return '0'
}

function go(base: string, current: string) {
  window.location.hash = href({ view: 'compare', base, current })
}

function CompareTape({ label, meta, report }: { label: string; meta: CompareMeta; report: Report | null }) {
  const when = meta.finished_at ? new Date(meta.finished_at).toLocaleString() : 'unknown time'
  const heading = `${label}: ${meta.seed}, ${when}`
  if (!report) {
    return (
      <div>
        <p className="compare-tape-label">{heading}</p>
        <p className="compare-tape-counts">
          {meta.pages_crawled.toLocaleString()} pages. The status tape is not available for this crawl.
        </p>
      </div>
    )
  }
  const saved = report.coverage?.tape ?? ''
  if (saved !== '') {
    const savedOff = report.coverage?.tape_off ?? ''
    const counts = describeTape(countTape(saved, savedOff))
    return (
      <div>
        <p className="compare-tape-label">{heading}</p>
        <Tape tape={saved} tapeOff={savedOff} summary={`${label} crawl. ${counts}`} compact />
        <p className="compare-tape-counts">{counts} Ticks are in fetch order.</p>
      </div>
    )
  }
  // Reports saved before coverage.tape existed: a status-only tape grouped by status.
  const byStatus = report.summary.pages_by_status ?? {}
  const counts = tapeSummary(byStatus)
  return (
    <div>
      <p className="compare-tape-label">{heading}</p>
      <Tape tape={statusTapeFromSummary(byStatus)} summary={`${label} crawl, ${counts}`} compact />
      <p className="compare-tape-counts">
        {counts}. This report was saved without a tape, so its ticks are grouped by status, not in fetch order.
      </p>
    </div>
  )
}

function UrlList({ title, urls, empty }: { title: string; urls: string[]; empty: string }) {
  return (
    <section>
      <h2>
        {title} ({urls.length.toLocaleString()})
      </h2>
      {urls.length === 0 ? (
        <p className="muted">{empty}</p>
      ) : (
        <ul className="compare-url-list">
          {urls.slice(0, MAX_URLS).map((u) => (
            <li key={u}>
              <code>{u}</code>
            </li>
          ))}
        </ul>
      )}
      {urls.length > MAX_URLS && <p className="muted">and {(urls.length - MAX_URLS).toLocaleString()} more</p>}
    </section>
  )
}

function DeltaTable({ caption, entries, empty }: { caption: string; entries: [string, number][]; empty: string }) {
  return (
    <table>
      <caption>{caption}</caption>
      <tbody>
        {entries.length === 0 ? (
          <tr>
            <th scope="row">{empty}</th>
          </tr>
        ) : (
          entries.map(([k, v]) => (
            <tr key={k}>
              <th scope="row">{k}</th>
              <td>{signed(v)}</td>
            </tr>
          ))
        )}
      </tbody>
    </table>
  )
}

export default function CompareView({ base, current }: { base?: string; current?: string }) {
  const [crawls, setCrawls] = useState<CrawlListItem[] | null>(null)
  const [listError, setListError] = useState('')
  const [explanations, setExplanations] = useState<Record<string, Explanation>>({})
  const [loaded, setLoaded] = useState<Loaded | null>(null)
  const [tab, setTab] = useState<Tab>('new')

  useEffect(() => {
    listCrawls()
      .then((r) => setCrawls(r.crawls))
      .catch((e) => setListError(errorText(e)))
    listExplanations()
      .then((r) => setExplanations(r.explanations))
      .catch(() => undefined)
  }, [])

  const candidates = useMemo(() => newestFirst((crawls ?? []).filter(isComparable)), [crawls])
  const pair = useMemo(() => (base && current ? { base, current } : defaultComparePair(candidates)), [base, current, candidates])
  const pairBase = pair?.base ?? ''
  const pairCurrent = pair?.current ?? ''
  const key = `${pairBase}\n${pairCurrent}`

  useEffect(() => {
    if (!pairBase || !pairCurrent) return
    let stopped = false
    const loadKey = `${pairBase}\n${pairCurrent}`
    const reportOf = (id: string) =>
      getCrawl(id)
        .then((j) => j.report ?? null)
        .catch(() => null)
    Promise.all([compareCrawls(pairBase, pairCurrent), reportOf(pairBase), reportOf(pairCurrent)])
      .then(([diff, baseReport, currentReport]) => {
        if (!stopped) setLoaded({ key: loadKey, diff, baseReport, currentReport, error: '' })
      })
      .catch((e) => {
        if (!stopped) setLoaded({ key: loadKey, diff: null, baseReport: null, currentReport: null, error: errorText(e) })
      })
    return () => {
      stopped = true
    }
  }, [pairBase, pairCurrent])

  if (listError) return <Notice tone="error">Could not load the crawl list: {listError}</Notice>
  if (!crawls) return <p className="muted">Loading crawls…</p>

  // Refs typed into the URL (a host, "latest") that are not in the list still get an option.
  const extraRefs = [pairBase, pairCurrent].filter((ref, i, all) => ref && all.indexOf(ref) === i && !candidates.some((c) => c.id === ref))

  function choose(side: 'base' | 'current', id: string) {
    const fallback = candidates.find((c) => c.id !== id)?.id ?? id
    const nextBase = side === 'base' ? id : pairBase || fallback
    const nextCurrent = side === 'current' ? id : pairCurrent || fallback
    go(nextBase, nextCurrent)
  }

  const view = loaded && loaded.key === key ? loaded : null
  const diff = view?.diff ?? null
  const lists = diff
    ? { new: diff.issues.new ?? [], resolved: diff.issues.resolved ?? [], persisting: diff.issues.persisting ?? [] }
    : null

  function picker(side: 'base' | 'current', label: string, value: string) {
    return (
      <label>
        {label}
        <select value={value} onChange={(e) => choose(side, e.target.value)}>
          {!value && <option value="">Choose a crawl</option>}
          {candidates.map((c) => (
            <option key={c.id} value={c.id}>
              {crawlLabel(c)}
            </option>
          ))}
          {extraRefs.map((ref) => (
            <option key={ref} value={ref}>
              {ref}
            </option>
          ))}
        </select>
      </label>
    )
  }

  return (
    <div className="compare">
      <h1>Compare crawls</h1>
      {candidates.length < 2 && !pair ? (
        <p className="muted">
          Compare needs two finished crawls. Start another crawl, or import a saved report from{' '}
          <a href={href({ view: 'crawls' })}>Crawls</a>.
        </p>
      ) : (
        <div className="compare-pickers">
          {picker('base', 'Base (earlier)', pairBase)}
          {picker('current', 'Current (later)', pairCurrent)}
          <Button variant="secondary" type="button" disabled={!pair} onClick={() => go(pairCurrent, pairBase)}>
            Swap
          </Button>
        </div>
      )}

      {pair && pairBase === pairCurrent && <Notice tone="info">Both sides are the same crawl, so nothing can differ.</Notice>}
      {pair && !view && <p className="muted">Comparing…</p>}
      {view?.error && <Notice tone="error">{view.error}</Notice>}

      {view && diff && lists && (
        <>
          <section className="compare-tapes" aria-label="Status tapes">
            <CompareTape label="Base" meta={diff.base} report={view.baseReport} />
            <CompareTape label="Current" meta={diff.current} report={view.currentReport} />
          </section>

          <p className="compare-headline">
            {lists.new.length.toLocaleString()} new, {lists.resolved.length.toLocaleString()} resolved,{' '}
            {lists.persisting.length.toLocaleString()} unchanged issues. Pages: {signed((diff.pages.added ?? []).length)} added,{' '}
            {signed(-(diff.pages.removed ?? []).length)} removed.
          </p>

          <div className="compare-deltas">
            <table>
              <caption>Issues by severity</caption>
              <thead>
                <tr>
                  <th scope="col">Severity</th>
                  <th scope="col">New</th>
                  <th scope="col">Resolved</th>
                  <th scope="col">Net change</th>
                </tr>
              </thead>
              <tbody>
                {SEVERITIES.map((sev) => (
                  <tr key={sev}>
                    <th scope="row">{sev}</th>
                    <td>{(diff.summary.new_by_severity?.[sev] ?? 0).toLocaleString()}</td>
                    <td>{(diff.summary.resolved_by_severity?.[sev] ?? 0).toLocaleString()}</td>
                    <td>{signed(diff.summary.by_severity?.[sev] ?? 0)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <DeltaTable
              caption="Issues by analyzer, net change"
              entries={Object.entries(diff.summary.by_analyzer ?? {}).sort(([a], [b]) => a.localeCompare(b))}
              empty="No change"
            />
            <DeltaTable
              caption="Pages by status, net change"
              entries={Object.entries(diff.summary.pages_by_status ?? {}).sort(([a], [b]) => Number(a) - Number(b))}
              empty="No change"
            />
          </div>

          <div className="cr-tabs" role="tablist" aria-label="Issue changes">
            {TABS.map((t) => (
              <button
                key={t}
                type="button"
                role="tab"
                aria-selected={tab === t}
                className={`cr-tab${tab === t ? ' cr-tab-on' : ''}`}
                onClick={() => setTab(t)}
              >
                {TAB_LABEL[t]} ({lists[t].length.toLocaleString()})
              </button>
            ))}
          </div>
          <IssueTable key={`${key}\n${tab}`} issues={lists[tab]} explanations={explanations} />

          <div className="compare-pages">
            <UrlList title="Added pages" urls={diff.pages.added ?? []} empty="No pages were added." />
            <UrlList title="Removed pages" urls={diff.pages.removed ?? []} empty="No pages were removed." />
          </div>
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 5: Pass the route params.** In `web/src/App.tsx`, replace the element wave 0
  renders for the compare route (`<CompareView />`) with
  `<CompareView base={route.base} current={route.current} />`. Leave the condition
  (`route.view === 'compare'`) as it is.

- [ ] **Step 6: Build and lint.** `cd web && npm run build && npm run lint` Expected: both exit 0.

- [ ] **Step 7: Manual check.**

```sh
go run ./cmd/gocrawl crawl https://example.com --depth 0 --save --store-dir /tmp/gc-compare
go run ./cmd/gocrawl crawl https://example.com --depth 1 --save --store-dir /tmp/gc-compare
make web-build && go run ./cmd/gocrawl serve --store-dir /tmp/gc-compare
```

  Open `http://localhost:8080/#/compare`. Expected:
  - Both pickers default to the two example.com crawls, with the older one as the base.
  - Two tapes appear in fetch order, with notches where a page is not in the sitemap and text
    counts under them (both crawls were saved by this version, so their reports carry
    `coverage.tape`). A report saved before W1-PROG-6b shows a status-grouped tape and says so.
  - The severity, analyzer and status delta tables appear.
  - The New, Resolved and Persisting tabs show counts, and each tab shows the issue table
    with working filters.
  - Added and removed page lists appear.
  - **Swap** exchanges the sides, and the URL changes.
  - With an empty store (`--store-dir /tmp/gc-empty`), the view shows the empty-state
    sentence with a link to Crawls.

- [ ] **Step 8: Run the checker.** `git checkout -- internal/webserver/webui/dist/index.html && make check` Expected: exits 0.

- [ ] **Step 9: Commit.**

```sh
git add web/src/views/CompareView.tsx web/src/views/compare.css web/src/views/compare/statusTape.ts web/src/views/compare/pairs.ts web/src/App.tsx
git commit -m "feat(web): add the Compare view" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W2-CMP-4: Crawls list: pick two to compare, and Import report…

**Files:**
- Modify: `web/src/components/History.tsx`

**Interfaces:**
- Consumes: `importReport`; `isComparable`, `orderPair` from `views/compare/pairs`; `Button`, `Notice`, `href`.
- Produces: no exports change (`History` keeps `{ onSelect }`).

The edits below add to the visual system's `History.tsx` (W1-VIS-8: `hist` markup, `<h1>Crawls</h1>`,
the empty state as a ternary, `formatFinished`) and the progress track's additions (W2-PROG-UI-5:
Sitemap coverage column, mini tapes, `CrawlListItemWithProgress`). Keep their columns. Add the
checkbox column as the first column.

- [ ] **Step 1: Edit `web/src/components/History.tsx`.**
  1. Imports: add `useRef` and `type ChangeEvent` to the `react` import, and add (the visual
     system already imports `href` and `Notice`; don't import them twice):

```ts
import { importReport } from '../api/import'
import { isComparable, orderPair } from '../views/compare/pairs'
import Button from './ui/Button'
```

  2. In the component, after the existing `useState` lines, add:

```ts
  const [picked, setPicked] = useState<string[]>([])
  const [importing, setImporting] = useState(false)
  const [importError, setImportError] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  function togglePicked(id: string) {
    setPicked((prev) => (prev.includes(id) ? prev.filter((p) => p !== id) : prev.length >= 2 ? prev : [...prev, id]))
  }

  function openCompare() {
    // items is CrawlListItemWithProgress[] since W2-PROG-UI-5; TypeScript infers the predicate.
    const chosen = picked.map((id) => items.find((i) => i.id === id)).filter((i) => i !== undefined)
    if (chosen.length !== 2) return
    const pair = orderPair(chosen[0], chosen[1])
    window.location.hash = href({ view: 'compare', base: pair.base, current: pair.current })
  }

  async function onImportFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setImporting(true)
    setImportError('')
    try {
      const { id } = await importReport(file)
      window.location.hash = href({ view: 'crawl', id })
    } catch (err) {
      setImportError(err instanceof Error ? err.message : String(err))
    } finally {
      setImporting(false)
    }
  }

  const toolbar = (
    <>
      <div className="crawls-toolbar">
        <Button variant="primary" type="button" disabled={picked.length !== 2} onClick={openCompare}>
          Compare selected ({picked.length} of 2)
        </Button>
        <Button variant="secondary" type="button" disabled={importing} onClick={() => fileRef.current?.click()}>
          {importing ? 'Importing…' : 'Import report…'}
        </Button>
        <input ref={fileRef} type="file" accept=".json,application/json" hidden onChange={onImportFile} />
      </div>
      {importError && <Notice tone="error">Could not import that file: {importError}</Notice>}
    </>
  )
```

  3. In the returned `<div className="hist">`, render `{toolbar}` right after
     `<h1>Crawls</h1>`. The visual system's empty state is already a ternary
     (`items.length === 0 ? <p className="hist-empty">…</p> : (<>…filters and table…</>)`);
     change its sentence to end "…when you <a href={href({ view: 'new' })}>start a crawl</a>, or
     import a JSON report.", keeping the link and the `hist-empty` class.
  4. In the table head, add `<th scope="col">Compare</th>` as the first `<th>` (not sortable,
     so no `hist-sortable` class). In each row, add this as the first cell:

```tsx
<td onClick={(e) => e.stopPropagation()}>
  <input
    type="checkbox"
    aria-label={`Compare ${item.seed}, finished ${formatFinished(item.finished_at)}`}
    checked={picked.includes(item.id)}
    disabled={!isComparable(item) || (picked.length >= 2 && !picked.includes(item.id))}
    onChange={() => togglePicked(item.id)}
  />
</td>
```

  5. Append to `web/src/views/compare.css`. CompareView imports that file, and Vite bundles it
     globally, so History can use the class without importing it:

```css
.crawls-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
  margin-bottom: 0.75rem;
}
```

  Then add `import '../views/compare.css'` at the top of `History.tsx` as well, so the style
  does not depend on whether the Compare view has loaded.

- [ ] **Step 2: Build and lint.** `cd web && npm run build && npm run lint` Expected: both exit 0.

- [ ] **Step 3: Manual check.** Use the store from W2-CMP-3:
  `make web-build && go run ./cmd/gocrawl serve --store-dir /tmp/gc-compare`. On **Crawls**:
  - Tick two rows. **Compare selected (2 of 2)** opens `#/compare` with the older crawl as
    the base.
  - A third checkbox is disabled while two are ticked.
  - A row whose crawl is still running cannot be ticked.
  - Clicking a checkbox does not open the crawl.
  - **Import report…** with a file from
    `go run ./cmd/gocrawl crawl https://example.org --depth 0 -f json > /tmp/org.json`
    navigates to the imported crawl's report.
  - Importing `/etc/hosts` shows the error notice "Could not import that file: not a gocrawl
    JSON report: …".
  - With `--store-dir /tmp/gc-empty`, the toolbar and the empty-state sentence both show.

- [ ] **Step 4: Run the checker.** `git checkout -- internal/webserver/webui/dist/index.html && make check` Expected: exits 0.

- [ ] **Step 5: Commit.**

```sh
git add web/src/components/History.tsx web/src/views/compare.css
git commit -m "feat(web): pick two crawls to compare and import a JSON report from Crawls" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W2-CMP-5: docs and changelog for the Compare UI

**Files:**
- Modify: `docs/web.md`, `CHANGELOG.md`

**Interfaces:** none.

- [ ] **Step 1: Update `docs/web.md`.** Set `updated: 2026-10-13`. In "What the UI does", add
  to the Crawls entry (called **History** before the visual track renamed it): "Tick two
  finished crawls and select **Compare selected** to open them in Compare. **Import report…**
  uploads a gocrawl JSON report (for example one written with `-f json`), saves it to the
  store, and opens it." Add a bullet:

```markdown
- **Compare** — two pickers, defaulting to the newest two crawls of the same host, and a
  **Swap** button. For each side it shows the crawl's tape from the saved report
  (`coverage.tape`, in fetch order, notched where a page is not in the sitemap); a report saved
  before tapes were stored gets a tape grouped by status instead. Then the issue counts by
  severity (new, resolved, net change), the net change by analyzer and by page status, tabs
  for new, resolved and persisting issues using the report's issue table and filters, and the
  added and removed pages. The selection is in the URL (`#/compare?base=…&current=…`), so a
  comparison can be bookmarked.
```

- [ ] **Step 2: Add the changelog line.** Under `## [Unreleased]`, `### Added`:

```markdown
- Web UI: a Compare view (both crawls' tapes, summary deltas, new, resolved and persisting
  issues, added and removed pages), plus compare checkboxes and **Import report…** on the
  Crawls list.
```

- [ ] **Step 3: Regenerate.** `make docs` Expected: exits 0.

- [ ] **Step 4: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 5: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -m "docs: describe the Compare view and report import in the web UI" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Track: redirects UI

#### Task W2-RED-1: redirect-check API module

**Files:**
- Create: `web/src/types/redirects.ts`, `web/src/api/redirects.ts`

**Interfaces:**
- Consumes: `request<T>`, `postForm<T>`
- Produces: types `RedirectVerdict`, `RedirectScope`, `RedirectCheckStatus`, `RedirectRow`,
  `RedirectCheck`, `StartRedirectCheckParams`; functions
  `startRedirectCheck(p: StartRedirectCheckParams): Promise<RedirectCheck>`,
  `listRedirectChecks(): Promise<{ checks: RedirectCheck[] }>`,
  `getRedirectCheck(id: string): Promise<RedirectCheck>`,
  `cancelRedirectCheck(id: string): Promise<void>`,
  `redirectExportURL(id: string): string`.

- [ ] **Step 1: Create `web/src/types/redirects.ts`.**

```ts
// Mirrors internal/webserver redirectJobView and redirectRowView (redirectcheck.Rule and
// redirectcheck.RowResult flattened).
export type RedirectVerdict = 'ok' | 'warning' | 'error' | 'skipped-external' | 'skipped-dynamic'

// '' when the rule could not be classified (its verdict is then 'error').
export type RedirectScope = 'in-scope' | 'external' | 'dynamic-pattern' | ''

export type RedirectCheckStatus = 'running' | 'done' | 'error' | 'canceled'

export interface RedirectRow {
  original: string
  target: string
  redirect_type: string
  redirect_style: string
  priority: string
  match_query_strings: string
  ignore_trailing_slash: string
  ignore_protocol: string
  disable_if_page_exists: string
  note: string
  scope: RedirectScope
  source_status: number
  source_final_url: string
  source_matches_target: boolean
  target_status: number
  original_in_sitemap: boolean
  target_in_sitemap: boolean
  verdict: RedirectVerdict
  notes?: string[]
}

export interface RedirectCheck {
  id: string
  domain: string
  file_name?: string
  status: RedirectCheckStatus
  error?: string
  started_at?: string
  finished_at?: string
  done: number
  total: number
  rows?: RedirectRow[]
}

export interface StartRedirectCheckParams {
  file: File
  domain: string
  sitemapURL?: string
  concurrency?: number
  rate?: number
  adaptiveDelay: boolean
}
```

- [ ] **Step 2: Create `web/src/api/redirects.ts`.**

```ts
import { postForm, request } from './client'
import type { RedirectCheck, StartRedirectCheckParams } from '../types/redirects'

export function startRedirectCheck(p: StartRedirectCheckParams): Promise<RedirectCheck> {
  const form = new FormData()
  form.append('file', p.file, p.file.name)
  form.append('domain', p.domain)
  if (p.sitemapURL) form.append('sitemap_url', p.sitemapURL)
  if (p.concurrency !== undefined) form.append('concurrency', String(p.concurrency))
  if (p.rate !== undefined) form.append('rate', String(p.rate))
  form.append('adaptive_delay', String(p.adaptiveDelay))
  return postForm<RedirectCheck>('/api/redirect-checks', form)
}

export function listRedirectChecks(): Promise<{ checks: RedirectCheck[] }> {
  return request<{ checks: RedirectCheck[] }>('/api/redirect-checks')
}

export function getRedirectCheck(id: string): Promise<RedirectCheck> {
  return request<RedirectCheck>(`/api/redirect-checks/${encodeURIComponent(id)}`)
}

export function cancelRedirectCheck(id: string): Promise<void> {
  return request<void>(`/api/redirect-checks/${encodeURIComponent(id)}/cancel`, { method: 'POST' })
}

export function redirectExportURL(id: string): string {
  return `/api/redirect-checks/${encodeURIComponent(id)}/export`
}
```

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint` Expected: both exit 0.

- [ ] **Step 4: Run the checker.** `git checkout -- internal/webserver/webui/dist/index.html && make check` Expected: exits 0.

- [ ] **Step 5: Commit.**

```sh
git add web/src/types/redirects.ts web/src/api/redirects.ts
git commit -m "feat(web): add the redirect-check API client" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W2-RED-2: the Redirects view (form and session list)

**Files:**
- Create: `web/src/views/redirects.css`
- Modify: `web/src/views/RedirectsView.tsx` (replaces the wave-0 stub)

**Interfaces:**
- Consumes: `startRedirectCheck`, `listRedirectChecks`, `Button` (`variant="primary"`), `Notice`, `href`
- Produces: `export default function RedirectsView()`

- [ ] **Step 1: Create `web/src/views/redirects.css`.**

```css
.redirect-form {
  display: grid;
  gap: 0.875rem;
  max-width: 40rem;
  margin-bottom: 2rem;
}
.redirect-form label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.redirect-form .redirect-inline {
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
}
.redirect-form .redirect-row {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
.redirect-hint {
  color: var(--ink-2);
  font-size: 0.875rem;
}
.redirect-progress {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  margin: 1rem 0;
}
.redirect-progress progress {
  flex: 1 1 16rem;
  height: 0.75rem;
  accent-color: var(--violet);
}
.redirect-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: flex-end;
  margin: 1rem 0;
}
.redirect-muted {
  color: var(--ink-2);
}
.redirect-verdict-error {
  font-weight: 600;
  color: var(--s5);
}
.redirect-table {
  overflow-x: auto;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 5px;
}
.redirect-table th,
.redirect-table td {
  padding: 6px 10px;
  border-top: 1px solid var(--rule);
  vertical-align: top;
}
.redirect-table thead th {
  border-top: 0;
  white-space: nowrap;
}
.redirect-url {
  max-width: 22rem;
  font-family: var(--font-mono);
  font-size: 13px;
  overflow-wrap: anywhere;
}
```

- [ ] **Step 2: Replace `web/src/views/RedirectsView.tsx`.**

```tsx
import { useEffect, useState, type FormEvent } from 'react'
import { listRedirectChecks, startRedirectCheck } from '../api/redirects'
import Button from '../components/ui/Button'
import Notice from '../components/ui/Notice'
import { href } from '../routes'
import type { RedirectCheck } from '../types/redirects'
import './redirects.css'

function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function formatStarted(s?: string): string {
  if (!s) return '—'
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString()
}

export default function RedirectsView() {
  const [file, setFile] = useState<File | null>(null)
  const [domain, setDomain] = useState('')
  const [sitemapURL, setSitemapURL] = useState('')
  const [rate, setRate] = useState('')
  const [concurrency, setConcurrency] = useState('4')
  const [adaptiveDelay, setAdaptiveDelay] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [checks, setChecks] = useState<RedirectCheck[] | null>(null)
  const [listError, setListError] = useState('')

  useEffect(() => {
    listRedirectChecks()
      .then((r) => setChecks(r.checks))
      .catch((e) => setListError(errorText(e)))
  }, [])

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!file) {
      setError('Choose the redirect-rule CSV file first.')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      const check = await startRedirectCheck({
        file,
        domain: domain.trim(),
        sitemapURL: sitemapURL.trim() || undefined,
        rate: rate.trim() === '' ? undefined : Number(rate),
        concurrency: concurrency.trim() === '' ? undefined : Number(concurrency),
        adaptiveDelay,
      })
      window.location.hash = href({ view: 'redirect', id: check.id })
    } catch (err) {
      setError(errorText(err))
      setSubmitting(false)
    }
  }

  return (
    <div className="redirects">
      <h1>Check redirects</h1>
      <p className="redirect-hint">
        Checks every rule in a HubSpot URL Redirects export against the live site: does the old URL still redirect to the
        right place, is the target a live page, and do both agree with the sitemap.
      </p>

      <form className="redirect-form" onSubmit={onSubmit}>
        <label>
          Redirect rules (CSV)
          <input type="file" accept=".csv,text/csv" required onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
          <span className="redirect-hint">A HubSpot URL Redirects export, up to 10 MB.</span>
        </label>
        <label>
          Domain
          <input type="text" required placeholder="example.com" value={domain} onChange={(e) => setDomain(e.target.value)} />
          <span className="redirect-hint">Subdomains count as in scope. Rules that point at other domains are skipped.</span>
        </label>
        <label>
          Sitemap URL (optional)
          <input type="url" placeholder="https://example.com/sitemap.xml" value={sitemapURL} onChange={(e) => setSitemapURL(e.target.value)} />
          <span className="redirect-hint">Leave empty to try /sitemap.xml and /sitemap_index.xml.</span>
        </label>
        <div className="redirect-row">
          <label>
            Rate (requests per second)
            <input type="number" min={0} step="any" placeholder="Unlimited" value={rate} onChange={(e) => setRate(e.target.value)} />
          </label>
          <label>
            Concurrency
            <input type="number" min={1} max={16} step={1} value={concurrency} onChange={(e) => setConcurrency(e.target.value)} />
          </label>
        </div>
        <label className="redirect-inline">
          <input type="checkbox" checked={adaptiveDelay} onChange={(e) => setAdaptiveDelay(e.target.checked)} />
          Slow down when the site answers 429 or 503 (adaptive delay)
        </label>
        {error && <Notice tone="error">{error}</Notice>}
        <div>
          <Button variant="primary" type="submit" disabled={submitting}>
            {submitting ? 'Starting…' : 'Check redirects'}
          </Button>
        </div>
      </form>

      <h2>Checks this session</h2>
      {listError && <Notice tone="error">Could not load the checks: {listError}</Notice>}
      {checks && checks.length === 0 && (
        <p className="muted">No redirect checks yet. Results stay in memory until gocrawl serve stops.</p>
      )}
      {checks && checks.length > 0 && (
        <div className="redirect-table">
        <table>
          <thead>
            <tr>
              <th scope="col">File</th>
              <th scope="col">Domain</th>
              <th scope="col">Status</th>
              <th scope="col">Rules checked</th>
              <th scope="col">Started</th>
            </tr>
          </thead>
          <tbody>
            {checks.map((c) => (
              <tr key={c.id}>
                <td>
                  <a href={href({ view: 'redirect', id: c.id })}>{c.file_name || 'Redirect rules'}</a>
                </td>
                <td>{c.domain}</td>
                <td>{c.status}</td>
                <td>
                  {c.done.toLocaleString()} of {c.total.toLocaleString()}
                </td>
                <td className="redirect-muted">{formatStarted(c.started_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint` Expected: both exit 0.

- [ ] **Step 4: Manual check.** Write `/tmp/rules.csv`:

```sh
printf '%s\n' '"Original URL","Redirect to","Redirect type","Redirect style","Priority","Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"' '/old,/new,301,STANDARD,1,FALSE,TRUE,TRUE,FALSE,' > /tmp/rules.csv
printf '%s\n' '"Original URL","Redirect to"' '/a,/b' > /tmp/bad.csv
make web-build && go run ./cmd/gocrawl serve
```

  On **Redirects** (`#/redirects`), expected:
  - The form shows all six fields. **Check redirects** is the violet primary button.
  - Submitting `/tmp/bad.csv` shows an error notice that starts "parsing the CSV: unexpected CSV
    columns", and no check is listed.
  - Submitting `/tmp/rules.csv` with domain `example.com` navigates to `#/redirect/<id>`.
    That page is the wave-0 stub until W2-RED-3.
  - Going back to Redirects lists that check under "Checks this session".

- [ ] **Step 5: Run the checker.** `git checkout -- internal/webserver/webui/dist/index.html && make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add web/src/views/RedirectsView.tsx web/src/views/redirects.css
git commit -m "feat(web): add the Redirects view with the check form and session list" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W2-RED-3: the redirect-check view (progress, cancel, results, export)

**Files:**
- Modify: `web/src/views/RedirectView.tsx` (replaces the wave-0 stub)

**Interfaces:**
- Consumes: `getRedirectCheck`, `cancelRedirectCheck`, `redirectExportURL`; `Button` (`variant="stop"`); `Notice`; `StatusCode` (`{code}`); `href`
- Produces: `export default function RedirectView(props: { id: string })`

- [ ] **Step 1: Replace `web/src/views/RedirectView.tsx`.**

```tsx
import { useEffect, useMemo, useState } from 'react'
import { cancelRedirectCheck, getRedirectCheck, redirectExportURL } from '../api/redirects'
import Button from '../components/ui/Button'
import Notice from '../components/ui/Notice'
import StatusCode from '../components/ui/StatusCode'
import { href } from '../routes'
import type { RedirectCheck, RedirectScope, RedirectVerdict } from '../types/redirects'
import './redirects.css'

const POLL_MS = 1500

const VERDICTS: RedirectVerdict[] = ['ok', 'warning', 'error', 'skipped-external', 'skipped-dynamic']
const VERDICT_LABEL: Record<RedirectVerdict, string> = {
  ok: 'OK',
  warning: 'Warning',
  error: 'Error',
  'skipped-external': 'Skipped: external',
  'skipped-dynamic': 'Skipped: dynamic pattern',
}
const SCOPES: Exclude<RedirectScope, ''>[] = ['in-scope', 'external', 'dynamic-pattern']
const SCOPE_LABEL: Record<Exclude<RedirectScope, ''>, string> = {
  'in-scope': 'In scope',
  external: 'External',
  'dynamic-pattern': 'Dynamic pattern',
}

function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function Status({ code }: { code: number }) {
  return code > 0 ? <StatusCode code={code} /> : <span className="redirect-muted">—</span>
}

function yesNo(b: boolean): string {
  return b ? 'yes' : 'no'
}

export default function RedirectView({ id }: { id: string }) {
  const [check, setCheck] = useState<RedirectCheck | null>(null)
  const [error, setError] = useState('')
  const [canceling, setCanceling] = useState(false)
  const [verdict, setVerdict] = useState<RedirectVerdict | ''>('')
  const [scope, setScope] = useState<RedirectScope | 'all'>('all')

  useEffect(() => {
    let stopped = false
    let timer: number | undefined
    async function poll() {
      try {
        const c = await getRedirectCheck(id)
        if (stopped) return
        setCheck(c)
        setError('')
        if (c.status === 'running') timer = window.setTimeout(poll, POLL_MS)
      } catch (e) {
        if (!stopped) setError(errorText(e))
      }
    }
    poll()
    return () => {
      stopped = true
      window.clearTimeout(timer)
    }
  }, [id])

  const rows = check?.rows
  const verdictCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const r of rows ?? []) counts[r.verdict] = (counts[r.verdict] ?? 0) + 1
    return counts
  }, [rows])
  const shown = useMemo(
    () =>
      (rows ?? [])
        .map((row, index) => ({ row, index }))
        .filter(({ row }) => (verdict === '' || row.verdict === verdict) && (scope === 'all' || row.scope === scope)),
    [rows, verdict, scope],
  )

  async function onCancel() {
    setCanceling(true)
    try {
      await cancelRedirectCheck(id)
    } catch (e) {
      setError(errorText(e))
    } finally {
      setCanceling(false)
    }
  }

  if (!check) {
    return error ? <Notice tone="error">{error}</Notice> : <p className="muted">Loading…</p>
  }

  const back = <a href={href({ view: 'redirects' })}>Back to Redirects</a>

  return (
    <div className="redirects">
      <h1>Redirect check: {check.domain}</h1>
      <p className="redirect-muted">
        {check.file_name || 'Redirect rules'} · {check.total.toLocaleString()} rules · {check.status}
      </p>
      {error && <Notice tone="error">{error}</Notice>}

      {check.status === 'running' && (
        <div className="redirect-progress">
          <progress max={Math.max(check.total, 1)} value={check.done} aria-label="Rules checked" />
          <span>
            {check.done.toLocaleString()} of {check.total.toLocaleString()} rules checked
          </span>
          <Button variant="stop" type="button" disabled={canceling} onClick={onCancel}>
            Cancel check
          </Button>
        </div>
      )}

      {check.status === 'canceled' && (
        <Notice tone="info" action={back}>
          Check canceled after {check.done.toLocaleString()} of {check.total.toLocaleString()} rules. A canceled check keeps
          no results; run it again for the full table.
        </Notice>
      )}

      {check.status === 'error' && (
        <Notice tone="error" action={back}>
          {check.error}
          {/sitemap/i.test(check.error ?? '') && ' Fill in the Sitemap URL field on the Redirects page and run the check again.'}
        </Notice>
      )}

      {check.status === 'done' && rows && (
        <>
          <p>
            {VERDICTS.filter((v) => verdictCounts[v])
              .map((v) => `${verdictCounts[v].toLocaleString()} ${VERDICT_LABEL[v].toLowerCase()}`)
              .join(' · ')}
          </p>
          <div className="redirect-filters">
            <label>
              Verdict
              <select value={verdict} onChange={(e) => setVerdict(e.target.value as RedirectVerdict | '')}>
                <option value="">All verdicts</option>
                {VERDICTS.map((v) => (
                  <option key={v} value={v}>
                    {VERDICT_LABEL[v]} ({(verdictCounts[v] ?? 0).toLocaleString()})
                  </option>
                ))}
              </select>
            </label>
            <label>
              Scope
              <select value={scope} onChange={(e) => setScope(e.target.value as RedirectScope | 'all')}>
                <option value="all">All scopes</option>
                {SCOPES.map((s) => (
                  <option key={s} value={s}>
                    {SCOPE_LABEL[s]}
                  </option>
                ))}
              </select>
            </label>
            <span className="redirect-muted num">
              Showing {shown.length.toLocaleString()} of {rows.length.toLocaleString()} rules
            </span>
            <a className="btn btn-secondary" href={redirectExportURL(id)}>
              Export CSV
            </a>
          </div>

          <div className="redirect-table">
            <table>
              <thead>
                <tr>
                  <th scope="col">#</th>
                  <th scope="col">Original URL</th>
                  <th scope="col">Redirect to</th>
                  <th scope="col">Scope</th>
                  <th scope="col">Source status</th>
                  <th scope="col">Final URL</th>
                  <th scope="col">Target status</th>
                  <th scope="col">In sitemap (source / target)</th>
                  <th scope="col">Verdict</th>
                  <th scope="col">Notes</th>
                </tr>
              </thead>
              <tbody>
                {shown.map(({ row, index }) => (
                  <tr key={index}>
                    <td>{index + 1}</td>
                    <td className="redirect-url" title={row.original}>
                      {row.original}
                    </td>
                    <td className="redirect-url" title={row.target}>
                      {row.target}
                    </td>
                    <td>{row.scope ? SCOPE_LABEL[row.scope] : '—'}</td>
                    <td>
                      <Status code={row.source_status} />
                    </td>
                    <td className="redirect-url" title={row.source_final_url}>
                      {row.source_final_url || '—'}
                    </td>
                    <td>
                      <Status code={row.target_status} />
                    </td>
                    <td>
                      {row.scope === 'in-scope' ? `${yesNo(row.original_in_sitemap)} / ${yesNo(row.target_in_sitemap)}` : '—'}
                    </td>
                    <td className={row.verdict === 'error' ? 'redirect-verdict-error' : undefined}>{VERDICT_LABEL[row.verdict]}</td>
                    <td>{(row.notes ?? []).join('; ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Check the route param.** No `App.tsx` change: wave 0 (W0-5) and the rail
  rewrite (W1-VIS-3) already render `<RedirectView key={route.id} id={route.id} />`. Confirm
  with `grep -n 'RedirectView key={route.id} id={route.id}' web/src/App.tsx` (one line).

- [ ] **Step 3: Build and lint.** `cd web && npm run build && npm run lint` Expected: both exit 0.

- [ ] **Step 4: Manual check.** Run `make web-build && go run ./cmd/gocrawl serve`, then:
  1. Check `/tmp/rules.csv` (from W2-RED-2) with domain `example.com`. example.com has no
     sitemap. Expected: the progress bar shows briefly, then an error notice reads "could not
     fetch/parse a sitemap … Fill in the Sitemap URL field …", with **Back to Redirects**.
  2. Check a CSV of your own redirects against a domain that has `/sitemap.xml`. Expected: the
     `done of total` text and the bar advance about every 1.5 s. When the check is done, the
     verdict counts, both filters, and the table appear. Source and target statuses show as
     colored `StatusCode` cells, and `—` appears where nothing was fetched. Filtering by
     verdict "Skipped: external" or by scope "External" narrows the rows. **Export CSV**
     downloads `redirect-check-<domain>.csv` with the verdict columns.
  3. Start a large CSV with rate `0.5`, then select **Cancel check**. Expected: within a poll
     or two, the info notice "Check canceled after N of M rules…" shows, with no table.

- [ ] **Step 5: Run the checker.** `git checkout -- internal/webserver/webui/dist/index.html && make check` Expected: exits 0.

- [ ] **Step 6: Commit.**

```sh
git add web/src/views/RedirectView.tsx
git commit -m "feat(web): show redirect-check progress, results and export" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

#### Task W2-RED-4: docs and changelog for the Redirects UI

**Files:**
- Modify: `docs/web.md`, `CHANGELOG.md`

**Interfaces:** none.

- [ ] **Step 1: Update `docs/web.md`.** Set `updated: 2026-10-13`. In "What the UI does", add:

```markdown
- **Redirects** — upload a HubSpot-format redirect-rule CSV, enter the domain, and
  optionally a sitemap URL, rate, concurrency and adaptive delay, then select **Check
  redirects**. The check page shows a `done of total` progress bar (polled every 1.5 s) and
  **Cancel check**. When it finishes, it shows a results table with HTTP status codes,
  filterable by verdict and by scope, and **Export CSV**. When no sitemap is found, the check
  ends with that error and a hint to fill in the Sitemap URL field. The Redirects page lists
  this session's checks, which are kept in memory only.
```

- [ ] **Step 2: Add the changelog line.** Under `## [Unreleased]`, `### Added`:

```markdown
- Web UI: a Redirects view to upload a redirect-rule CSV, follow the check's progress, filter
  the results by verdict and scope, and export them as CSV.
```

- [ ] **Step 3: Regenerate.** `make docs` Expected: exits 0.

- [ ] **Step 4: Run the checker.** `make check` Expected: exits 0.

- [ ] **Step 5: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -m "docs: describe the Redirects view in the web UI" -m "Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

## Wave 3: pause UI

### Track: pause UI

#### Task W3-PAUSE-UI-1: Pause, Resume crawl, Stop crawl and the auto-pause notice

**Files:**
- Create: `web/src/api/pause.ts`, `web/src/components/progress/CrawlControls.tsx`,
  `web/src/components/progress/AutoPauseNotice.tsx`
- Modify: `web/src/components/progress/RunPhasePill.tsx`,
  `web/src/components/progress/RunningCrawl.tsx`, `web/src/components/progress/progress.css`,
  `web/src/components/CrawlReport.tsx`

**Interfaces:**
- Consumes: pause/resume endpoints (W2-PAUSE-6), `useCrawlPoll().refresh`, `Button` variants
  `go` / `pause` / `stop`, `Notice`.
- Produces: `pauseCrawl(id)`, `resumeCrawl(id, rate?)`;
  `<CrawlControls id status phase resumeRate onChanged onError />`;
  `<AutoPauseNotice progress rate onRateChange />`; `<RunPhasePill phase paused />`.

- [ ] **Step 1: Create `web/src/api/pause.ts`:**

```ts
import { postJSON, request } from './client'

export function pauseCrawl(id: string): Promise<void> {
  return request(`/api/crawls/${encodeURIComponent(id)}/pause`, { method: 'POST' })
}

/** Resumes a paused crawl; a positive rate replaces the request rate, otherwise it is kept. */
export function resumeCrawl(id: string, rate?: number): Promise<void> {
  return postJSON(`/api/crawls/${encodeURIComponent(id)}/resume`, rate !== undefined && rate > 0 ? { rate } : {})
}
```

- [ ] **Step 2: Create `web/src/components/progress/CrawlControls.tsx`:**

```tsx
import { useState } from 'react'
import Button from '../ui/Button'
import { cancelCrawl } from '../../api/crawls'
import { pauseCrawl, resumeCrawl } from '../../api/pause'
import type { CrawlStatus } from '../../types/crawls'
import type { Phase } from '../../types/progress'

interface Props {
  id: string
  status: CrawlStatus
  phase?: Phase
  /** Rate for Resume crawl; undefined keeps the current rate. */
  resumeRate?: number
  /** Called after an action succeeds, to poll right away. */
  onChanged: () => void
  onError: (message: string) => void
}

/** Yellow Pause while running, green Resume crawl while paused, red Stop crawl always. */
export default function CrawlControls({ id, status, phase, resumeRate, onChanged, onError }: Props) {
  const [busy, setBusy] = useState(false)

  async function act(action: () => Promise<void>) {
    setBusy(true)
    try {
      await action()
      onChanged()
    } catch (err) {
      onError(String(err))
    } finally {
      setBusy(false)
    }
  }

  function stop() {
    if (!window.confirm('Stop this crawl? The report keeps the pages fetched so far.')) return
    void act(() => cancelCrawl(id))
  }

  return (
    <>
      {status === 'paused' ? (
        <Button type="button" variant="go" disabled={busy} onClick={() => void act(() => resumeCrawl(id, resumeRate))}>
          Resume crawl
        </Button>
      ) : (
        phase !== 'analyzing' && (
          <Button type="button" variant="pause" disabled={busy} onClick={() => void act(() => pauseCrawl(id))}>
            Pause
          </Button>
        )
      )}
      <Button type="button" variant="stop" disabled={busy} onClick={stop}>
        Stop crawl
      </Button>
    </>
  )
}
```

- [ ] **Step 3: Create `web/src/components/progress/AutoPauseNotice.tsx`:**

```tsx
import { useId } from 'react'
import Notice from '../ui/Notice'
import type { ProgressView } from '../../types/progress'
import { formatCount, formatRate } from './tapeText'

interface Props {
  progress: ProgressView
  /** The "Resume at" field's text; starts at half the current rate. */
  rate: string
  onRateChange: (value: string) => void
}

/** Explains an automatic pause after repeated 429/503 responses and asks for a lower rate. */
export default function AutoPauseNotice({ progress, rate, onRateChange }: Props) {
  const inputId = useId()
  const retry = progress.retry_after_seconds ?? 0
  return (
    <Notice
      tone="warning"
      action={
        <div className="ap-rate">
          <label htmlFor={inputId}>Resume at (requests per second)</label>
          <input
            id={inputId}
            type="number"
            min="0.001"
            step="any"
            inputMode="decimal"
            value={rate}
            onChange={(e) => onRateChange(e.target.value)}
          />
        </div>
      }
    >
      <p>
        The site kept answering 429 Too Many Requests (or 503), even after slowing to {formatRate(progress.rate)} requests per
        second, so the crawl paused itself.
        {retry > 0 ? ` The server asked for a ${formatCount(retry)}-second wait (Retry-After).` : ''} Resume at a lower rate, or
        stop and keep the {formatCount(progress.fetched)} pages fetched so far.
      </p>
    </Notice>
  )
}
```

- [ ] **Step 4: Paused pill.** Replace the default export of
  `web/src/components/progress/RunPhasePill.tsx` with (the visual system's `PhasePill` already
  draws `paused` as a static yellow dot):

```tsx
/** Green pulsing dot while crawling, yellow static dot while paused, violet otherwise. */
export default function RunPhasePill({ phase, paused = false }: { phase?: Phase; paused?: boolean }) {
  if (paused) return <PhasePill phase="paused">Paused</PhasePill>
  const p = PILL[phase ?? 'sitemap']
  return <PhasePill phase={p.phase}>{p.label}</PhasePill>
}
```

  In `RunningCrawl.tsx` change `<RunPhasePill phase={progress?.phase} />` to
  `<RunPhasePill phase={progress?.phase} paused={paused} />`, and change the `controls` prop
  comment to `/** Header buttons: Pause / Resume crawl / Stop crawl. */`.

  Append to `progress.css`:

```css
.ap-rate {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.ap-rate label {
  font-size: 13px;
  color: var(--ink-2);
}
.ap-rate input {
  width: 110px;
  font: inherit;
  padding: 5px 8px;
  background: var(--surface);
  border: 1px solid var(--field);
  border-radius: 5px;
  color: var(--ink);
}
```

- [ ] **Step 5: Wire it into `web/src/components/CrawlReport.tsx`.**

  1. Imports: add

```tsx
import AutoPauseNotice from './progress/AutoPauseNotice'
import CrawlControls from './progress/CrawlControls'
import { halfRate } from './progress/tapeText'
```

  2. Add `refresh` to the `useCrawlPoll` destructuring:

```tsx
  const { job, progress, tape, tapeOff, recent, receivedAt, error: pollError, refresh } = useCrawlPoll(id)
```

  3. Next to the other `useState` hooks (before any early return), add:

```tsx
  // null until the user edits "Resume at": the field then shows half the current rate.
  const [resumeRateInput, setResumeRateInput] = useState<string | null>(null)
```

  4. Delete the `stopCrawl` function added in W2-PROG-UI-2 and replace the running branch with:

```tsx
  if (job.status === 'running' || job.status === 'paused') {
    const autoPaused = job.status === 'paused' && progress?.pause_reason === 'rate_limited'
    const rateText = resumeRateInput ?? (progress ? halfRate(progress.rate) : '')
    const rateNumber = Number(rateText)
    return (
      <RunningCrawl
        job={job}
        progress={progress}
        tape={tape}
        tapeOff={tapeOff}
        recent={recent}
        receivedAt={receivedAt}
        notice={
          autoPaused && progress ? (
            <AutoPauseNotice progress={progress} rate={rateText} onRateChange={setResumeRateInput} />
          ) : undefined
        }
        controls={
          <CrawlControls
            id={id}
            status={job.status}
            phase={progress?.phase}
            resumeRate={autoPaused && rateNumber > 0 ? rateNumber : undefined}
            onChanged={() => {
              setResumeRateInput(null)
              refresh()
            }}
            onError={setError}
          />
        }
      />
    )
  }
```

  5. Keep the `Button` import: the finished report's export and review actions still use it.

- [ ] **Step 6: Build and lint.** `cd web && npm run build && npm run lint`
  Expected: success; the contrast check still passes (no new tokens).

- [ ] **Step 7: Manual check, user pause.** With the fixture on :8099: start a crawl with Rate
  limit `2`. While crawling, the header shows a yellow-outlined **Pause** and a red **Stop
  crawl**. Click **Pause**: within a poll the pill turns to a static yellow "Paused", the
  button becomes a green **Resume crawl**, the tape stops growing (at most one more tick), the
  elapsed clock freezes, and "request rate when paused" shows `2.0/s`. Click **Resume crawl**:
  the pill pulses green again and ticks resume. **Stop crawl** asks for confirmation and the
  view switches to the finished report with the interrupted-coverage banner. During
  "Analyzing", only **Stop crawl** is shown.

- [ ] **Step 8: Manual check, auto-pause.** Save this throttling server as
  `/tmp/throttle/main.go` and run it with `go run /tmp/throttle/main.go`:

```go
package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Serves a home page with 200 links; after 20 page requests every request gets a 429.
func main() {
	var n atomic.Int64
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, "<html><body>")
			for i := 0; i < 200; i++ {
				fmt.Fprintf(w, `<a href="/p%d">p%d</a>`, i, i)
			}
			fmt.Fprint(w, "</body></html>")
		case "/robots.txt":
			http.NotFound(w, r)
		default:
			if n.Add(1) > 20 {
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			fmt.Fprint(w, "<html><body>ok</body></html>")
		}
	})
	_ = http.ListenAndServe("127.0.0.1:8098", nil)
}
```

  Crawl `http://127.0.0.1:8098/` with default settings. After the first 20 pages, orange ticks
  appear, "429 or 503 responses in the last minute" turns yellow and climbs, and within about a
  minute the crawl pauses itself: pill "Paused", a warning notice explaining the 429s that
  names the 2-second Retry-After and the pages fetched so far, and a **Resume at** field
  prefilled with half the current rate (for example `0.13` when the rate is `0.25/s`). Change it
  to `0.1` and click **Resume crawl**: the crawl continues and "request rate" reads `0.1/s`.
  Stop the crawl afterwards.

- [ ] **Step 9: `make check`.** Expected: exits 0.

- [ ] **Step 10: Commit.**

```sh
git restore internal/webserver/webui/dist/index.html
git add web/src/api/pause.ts web/src/components/progress/CrawlControls.tsx web/src/components/progress/AutoPauseNotice.tsx web/src/components/progress/RunPhasePill.tsx web/src/components/progress/RunningCrawl.tsx web/src/components/progress/progress.css web/src/components/CrawlReport.tsx
git commit -F - <<'EOF'
feat(web): Pause, Resume crawl and Stop crawl controls with an auto-pause notice

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

---

#### Task W3-PAUSE-UI-2: Docs and changelog for the pause UI

**Files:**
- Modify: `docs/web.md`, `docs/README.md` (generated), `CHANGELOG.md`

- [ ] **Step 1: `docs/web.md`.** Keep `updated: 2026-10-13`. In "What the UI does", change the
  running-crawl sentence about the header to "The header shows the host, a phase pill, a yellow
  **Pause** and a red **Stop crawl**." and add a "Crawl (paused)" bullet:

```markdown
- **Crawl (paused)** — the pill reads **Paused** and the actions become a green **Resume
  crawl** and **Stop crawl**. Elapsed time stops while paused. When the crawl paused itself
  because the site kept answering 429 or 503, a notice explains it, shows the server's
  `Retry-After`, and offers a **Resume at** rate prefilled at half the current rate.
```

- [ ] **Step 2: `CHANGELOG.md`.** Under `## [Unreleased]` → `### Added`, add:

```markdown
- Pause, Resume crawl and Stop crawl buttons in the web UI. When a crawl pauses itself because the site keeps rate limiting it, the UI explains why, shows the server's Retry-After, and offers to resume at half the current rate.
```

- [ ] **Step 3: Regenerate and check.** `make docs && make check`
  Expected: both succeed.

- [ ] **Step 4: Commit.**

```sh
git add docs/web.md docs/README.md CHANGELOG.md
git commit -F - <<'EOF'
docs(web): describe the paused crawl view and its controls

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
EOF
```

## Self-review

### Spec coverage map

| Spec section | Tasks |
| --- | --- |
| Constraints: no `localStorage`, no download dependency, guard model, no new runtime deps, bundled fonts | W0-2 (`requireJSON`, `parseMultipart`), W1-VIS-1, W1-VIS-2, W1-SET-6, W2-SET-2; guard tests in every route task |
| Backend 1, engine hook (`OnProgress`, `Progress`, rate, throttled) | W1-PROG-1, W1-PROG-2 |
| Backend 1, runner (`RunOption`, `WithProgress`, phases) | W0-3, W1-PROG-6 |
| Backend 1, sitemap pre-pass (`InScope`, `Discover`, `WithURLs`, fetched once) | W1-PROG-3, W1-PROG-4, W1-PROG-5 |
| Backend 1, sitemap totals, `Coverage.sitemap_*`, budget note | W1-PROG-5, W1-PROG-6 |
| Backend 1, sitemap preview | W1-PROG-7, W1-PROG-9 |
| Backend 1, tape stream, `tape_from`, `JobView.progress` | W1-PROG-8 |
| Backend 1, saved tapes (superseded by the coordinator: saved in `coverage.tape`) | W1-PROG-6b, W1-PROG-8 |
| Backend 2, pause and resume, `MaxDuration` active time | W2-PAUSE-1, W2-PAUSE-2, W2-PAUSE-3 |
| Backend 2, auto-pause on sustained rate limiting | W1-PROG-1, W2-PAUSE-4 |
| Backend 2, `runner.Control`, API, `paused` status | W2-PAUSE-5, W2-PAUSE-6 |
| Backend 3, compare | W1-CMP-1, W1-CMP-2 |
| Backend 4, report import | W1-CMP-3 |
| Backend 5, redirect checks as jobs (generic job manager, caps, `OnRow`, routes) | W0-1, W1-RED-1, W1-RED-2, W1-RED-3 |
| Backend 6, crawl parameters, bearer token, CLI flag, config key | W1-SET-1, W1-SET-2, W1-SET-3, W1-SET-5 |
| Backend 6, header profiles | W1-SET-4, W1-SET-7 |
| Backend 6, GUI settings (`gui.json`, secrets stripped, store dir) | W1-SET-6 |
| Backend 7, interactive browser login (window, capture, states, expiry, logout guard, availability) | W1-LOGIN-1, W1-LOGIN-2, W1-LOGIN-3, W1-LOGIN-4, W2-LOGIN-2 |
| Frontend, information architecture (rail, hash routing, `#/history`) | W0-5, W1-VIS-3 |
| View: New crawl | W1-VIS-6, W2-PROG-UI-4, W2-SET-3, W2-SET-4, W2-LOGIN-2 |
| View: Crawl (running) | W2-PROG-UI-2 |
| View: Crawl (paused) | W3-PAUSE-UI-1 |
| View: Crawl (finished) | W1-VIS-7, W2-PROG-UI-3, W2-CMP-2 |
| View: Crawls | W1-VIS-8, W2-PROG-UI-5, W2-CMP-4 |
| View: Compare | W2-CMP-1, W2-CMP-2, W2-CMP-3 |
| View: Redirects | W2-RED-1, W2-RED-2, W2-RED-3 |
| View: Settings | W2-SET-1, W2-SET-2, W2-SET-5 |
| Visual system: type, color tokens, controls, phase pill, tape, motion, copy | W1-VIS-1 … W1-VIS-8 |
| Accessibility guard (`contrast.mjs` in `npm run lint`) | W1-VIS-1 |
| Error handling (JSON errors, 400/404/409/413/415/429/503, pre-pass never fails a crawl) | W1-PROG-5, W1-PROG-8, W1-PROG-9, W2-PAUSE-6, W1-SET-5/6/7, W1-LOGIN-3/4, W1-CMP-2/3, W1-RED-2/3 |
| Testing list (engine, runner, sitemap, redirectcheck, config, webserver, frontend, `make check`) | the test steps of every task; Review Focus above |
| Documentation (`web.md`, `mcp.md`, `configuration.md`, `output.md`, `architecture.md`, CHANGELOG per PR) | W0-5, W1-VIS-8, W1-PROG-10, W1-SET-8, W1-LOGIN-5, W1-CMP-4, W1-RED-4, W2-PROG-UI-6, W2-PAUSE-7, W2-SET-6, W2-LOGIN-3, W2-CMP-5, W2-RED-4, W3-PAUSE-UI-2 |
| Delivery order (7 PRs) | Tracks map onto the spec's PRs: visual (1), progress backend + UI (2), pause backend + UI (3), settings backend + UI (4), compare backend + UI (5), redirects backend + UI (6), login backend + UI (7) |

### Placeholder scan

`grep -nE 'TBD|TODO|FIXME|similar to|add appropriate|implement later|fill in the details|as needed|fills? in'`
over this file finds no placeholder step. The remaining hits are deliberate: wave 0 describing
the empty route stubs it creates for later tracks (W0-2 Step 7, the `redirectJob` comment, the
W0-5 commit message), W1-SET-6 telling the implementer to fill in that stub, and the
Redirects UI copy asking the user to "fill in the Sitemap URL field". Every code step carries
complete code; the only conditional instructions left ("skip the `Button` import if the file
already imports it") name the exact line to check.

### Type-consistency notes

- Go names match the contract above: `crawlJob{Seed, Report, StoreID, MaxPages, Progress,
  Tape, TapeOff, control}`; `runSettings{progress, control, pauseOnThrottle}`;
  `crawlRunOptions(id)` grows to `crawlRunOptions(id, ctl)` in W2-PAUSE-6; `Run`'s variadic
  parameter is `runOpts` everywhere.
- One tape encoding everywhere: `crawler.TapeChar` (runner reporter, `recordProgress`,
  `classesFromByStatus`) and `runner.Progress.TapeOffChar` (runner reporter and web server),
  so `coverage.tape` equals the live tape (`TestRunSavesTheLiveTapeInCoverage`).
  `crawler.TapePreviewLen` (160) equals `MAX_COMPACT_TICKS`, and Go's `tapeRank`
  (5 > x > 4 > 3 > 2) matches `RANK` in `tapeModel.ts`, with the same run length
  `ceil(len / limit)`, so a server preview draws exactly like a client-side aggregation.
- JSON keys and their TypeScript mirrors: `tape`, `tape_off`, `tape_from`, `max_pages`,
  `active_ms` (`types/progress.ts` `ProgressView`); `status_classes`, `sitemap_total`,
  `sitemap_fetched`, `tape_preview` (`CrawlListItemWithProgress`); `coverage.sitemap_total`,
  `sitemap_fetched`, `tape`, `tape_off` (`types/crawls.ts` `Coverage`, W1-PROG-6b).
- Two different `Phase` types exist on purpose: `types/progress.ts` (`sitemap|crawling|analyzing`,
  the run phase) and `components/ui/PhasePill.tsx` (pill look); `RunPhasePill` imports the
  latter as `PillPhase`. `tapeSummary` exists in `progress/tapeText.ts` and
  `views/compare/statusTape.ts` with different signatures; no file imports both. `hostOf`
  exists in several modules with local meanings (`crawlform/values.ts` returns the lower-case
  ASCII hostname, which is what the login host check needs).
- `CrawlStatus` gains `'paused'` once (W2-PROG-UI-1) together with its `STATUS_PILL` entry;
  `isComparable`, `CrawlControls` and `useCrawlPoll.isLive` handle it.
- `CrawlDefaults = Partial<Omit<StartCrawlParams, 'save' | 'login_session'>>` compiles before
  W2-LOGIN-1 adds `login_session` (Omit tolerates a missing key).
- Components are default exports with the visual system's props: `Button`
  (`variant` + native props), `Notice` (`tone`, `children`, `action?`), `StatusCode` (`code`),
  `PhasePill` (`phase`, `children`), `Tape` (contract props), `TapeLegend` (`sitemap?`,
  `finished?`).

## Appendix: section self-reviews

Each section's own self-review (spec items, contract names, gaps it filled, riskiest edge
cases), after reconciliation.

### Self-review: prep and visual system

**Spec items covered**

- Backend 5 (redirect checks as jobs), first bullet: `jobManager` generic over its result
  type, each kind with its own cap (W0-1, W0-2; caps: `maxRunning` for crawls, the redirect
  track adds its own).
- Backend 1 runner: `runner.Run(ctx, cfg, seed, opts ...RunOption)` with existing callers
  unchanged (W0-3). `WithProgress`, `WithControl`, `WithPauseOnThrottle` are left to their
  tracks.
- Constraints: guard model reused (`requireJSON` 415, `parseMultipart` 415/413/400; origin
  check stays in `guard`), no `localStorage` (theme only sets `data-theme`; persistence is the
  settings track's `gui.json`), no new frontend runtime dependencies, fonts bundled not
  CDN-loaded.
- Frontend information architecture: left rail with New crawl, Crawls, Compare, Redirects,
  Settings; hash routing, no router dependency; `#/history` still works (W0-5, VIS-3).
- Visual system: principle (status colors, violet chrome), type (Atkinson Hyperlegible Next
  and Mono WOFF2 under `web/src/fonts/` with the OFL), every color token in both themes with
  the spec's ratios reproduced by the guard, filled-button text colors, controls (green
  start, yellow pause outline, red stop, violet primary), phase pill (green pulsing, yellow
  static, violet analyzing), focus ring 2px ink, sentence case, no all-caps labels.
- The tape: all seven tick kinds, height encoding, 5xx cap, notch, both outlines,
  aggregation above 20,000 with worst status, `role="img"` with a text summary, counts
  available as text via `describeTape`.
- Motion: tick drop-in and pulsing dot, both off under `prefers-reduced-motion`.
- Copy: Stop crawl → "Crawl stopped"; errors say what happened and what to do (Crawls load
  error); the Crawls empty state uses the spec's sentence.
- Accessibility guard: `web/scripts/contrast.mjs` reads the token file for both themes,
  4.5:1 text, 3:1 non-text, exit 1 on failure, run by `npm run lint`.
- Testing: generic job manager (cap counts running and paused, `with`, `finish` apply,
  `cancel`), request helpers, runner options; frontend by `tsc -b`, oxlint, and the contrast
  check; `make check` after every task.
- Documentation: `docs/web.md`, `docs/architecture.md`, frontmatter `updated: 2026-10-13`,
  `make docs`, one `CHANGELOG.md` line per wave.
- `make web-build` and the committed placeholder `internal/webserver/webui/dist/index.html`
  flow verified in VIS-8 step 8.

Not in this section (other tracks): progress hook, sitemap pre-pass, tape stream API, pause
and resume, compare, import, redirect-check jobs, settings and profiles, browser login, and
wiring `Tape` into the crawl and Crawls views.

**Contract names used exactly:** `Status` and the five `Status*` constants, `Job[T]`,
`jobManager[T]`, `newJobManager[T]`, `start`, `get`, `list`, `with`, `finish`, `cancel`,
`crawlJob` (`Seed`, `Report`, `StoreID`), `Server` fields `store`, `crawls`, `redirects`,
`logins`, `guiSettingsPath`, `mux`, `allowAnyHost`, `maxRunning`, `redirectJob`,
`loginManager`, `newLoginManager`, `crawlRoutes`, `sitemapRoutes`, `compareRoutes`,
`importRoutes`, `redirectRoutes`, `settingsRoutes`, `profileRoutes`, `loginRoutes`,
`requireJSON`, `parseMultipart`, `writeJSON`, `writeError`, `formatTime`, `RunOption`,
`runSettings`, `Run`, `WithGUISettingsPath`; `api/client.ts` `request`, `postJSON`,
`postForm`; `api/crawls.ts`, `types/crawls.ts`, `routes.ts` `Route`, `parseHash`, `href`;
`views/CompareView.tsx`, `RedirectsView.tsx`, `RedirectView.tsx`, `SettingsView.tsx`;
`styles/tokens.css` with all sixteen listed variables; `theme.ts` `Theme`, `applyTheme`;
`fonts/` + `OFL.txt`, `styles/fonts.css`; `components/ui/Button.tsx` (variants
`go|stop|pause|primary|secondary`), `Notice.tsx` (tones `warning|error|info`, `action`),
`StatusCode.tsx` (`code`); `components/tape/Tape.tsx` with the contract's props;
`components/Rail.tsx`; `scripts/contrast.mjs` in `npm run lint`.

**Contract gaps filled (for the coordinator to reconcile)**

1. Token file path: the spec says `web/src/tokens.css`, the contract says `styles/tokens.css`.
   This plan uses `web/src/styles/tokens.css` (contract) and `contrast.mjs` reads that path.
2. `Run`'s variadic parameter is named `runOpts`, not `opts`: `Run` already has a local
   `opts` (the `crawler.Options`). The type signature is the contract's.
3. `guiSettingsPath` would fail golangci-lint `unused` with no writer, so wave 0 adds the
   contract's `WithGUISettingsPath(path string) Option` in `settings.go`. The settings track
   must not redeclare it; it only adds the `serve.go` default and the handlers.
4. `crawlJob` in wave 0 has only `Seed`, `Report`, `StoreID`. `Progress *runner.Progress`,
   `Tape`, `TapeOff` (progress track) and `control *runner.Control` (pause track) are added by
   those tracks: their types don't exist yet, and unused fields would fail lint.
5. `setStoreID` is gone; the crawl goroutine saves before `finish` and sets `Report` and
   `StoreID` in `finish`'s `apply`. Tracks that extend the crawl goroutine should keep that
   order. `finish` runs `apply` for every outcome, including errors (the contract's wording).
6. Added `(*Job[T]).active()` (running or paused), used by `start` and `cancel`.
7. The spec's tick table has no row for a fetch error (`x`). This plan draws it tall in
   slate (`--ink-2`) and ranks it between 5xx and 4xx when aggregating.
8. Additions beside `Tape`: `components/tape/tapeModel.ts` (`buildTicks`, `countTape`,
   `describeTape`, `MAX_TICKS = 20000`, `MAX_COMPACT_TICKS = 160`) and
   `components/tape/TapeLegend.tsx` (`sitemap`, `finished`). A compact tape aggregates above
   160 ticks so it stays one row; the full tape scrolls past `max-height: 50vh` instead of the
   mockup's fixed 104px, which would clip any crawl above about 1,000 pages.
9. Other additions: `web/src/status.ts` (`StatusClass`, `statusClass`, `STATUS_LABEL`),
   `components/ui/PhasePill.tsx` (`Phase = crawling|paused|analyzing|done|stopped|failed`),
   `web/src/crawlStatus.ts` (`STATUS_PILL`, a `Record<CrawlStatus, …>`: the pause track must add
   a `paused` entry when it adds `'paused'` to `CrawlStatus`), `routes.ts` `navigate`,
   `theme.ts` `isTheme`, and the font custom properties `--font-text` / `--font-mono` in
   `styles/fonts.css`.
10. `CrawlStatus` in `types/crawls.ts` stays without `'paused'` in wave 0 (pure move); the
    pause track adds it, plus polling while paused in `CrawlReport`.
11. `StatusCode` colors 2xx green (`--s2`), like the tape (coordinator decision; the mockup's
    secondary-ink 2xx was dropped); 3xx/4xx/5xx use `--s3`/`--s4`/`--s5`. Green on the panel
    surface is 5.94 (dark) and 5.37 (light), both already checked pairs.
12. Severity colors are not in the spec: error `--s5`, warning `--s3`, info `--ink-2`
    (global `.sev-*` classes in `CrawlReport.css`).
13. Styling convention for all tracks: each component imports its own colocated stylesheet
    (`components/…/Name.css`) with a per-view class prefix (`cf-`, `cr-`, `hist-`, `sm-`,
    `rail-`, `btn`, `notice`, `phase`, `tape`/`tick`); `styles/base.css` holds only global
    element styles and the layout. There is no shared app stylesheet to edit.
14. The `system` theme is resolved in CSS (a `prefers-color-scheme: light` block that repeats
    the light values, checked for drift by `contrast.mjs`), not in JS; `applyTheme('system')`
    just sets the attribute.
15. Fonts ship upright only (variable weight 200–800, about 74 KB together); the restyle
    uses no italics.

### Self-review: progress and pause

#### Spec items covered

| Spec item | Task |
| --- | --- |
| `crawler.Options.OnProgress`, `Progress` emitted after each recorded page, non-blocking | W1-PROG-2 |
| `Throttled` (429/503 in last 60s), `Rate` from the adaptive limiter | W1-PROG-1, W1-PROG-2 |
| `(*Engine).InScope` with enqueue's host/subdomain/include/exclude rules | W1-PROG-3 |
| `sitemap.Discover` extracted from the analyzer; analyzer gets the set through an option | W1-PROG-4 |
| Sitemap pre-pass for every crawl (CLI, MCP, web) on the governed fetcher; fetched once | W1-PROG-5 |
| `runner.Run(ctx, cfg, seed, opts...)`, `WithProgress`, phases sitemap/crawling/analyzing | W1-PROG-5, W1-PROG-6 |
| `SitemapTotal`, `SitemapFetched`, `OffSitemap` | W1-PROG-6 |
| `Coverage.sitemap_total` / `sitemap_fetched`, omitted without a sitemap | W1-PROG-5 |
| Budget-below-sitemap note in `Report.Notes` | W1-PROG-5 |
| `POST /api/sitemap-preview` (robots.txt respected, crawl User-Agent) | W1-PROG-7, W1-PROG-9 |
| Tape / tape_off, `?tape_from=N`, `JobView.progress` | W1-PROG-8 |
| Tape saved in reports (`coverage.tape` / `tape_off`, coordinator decision; replaces the spec's status-only saved tape), `tape_preview` in crawl lists | W1-PROG-6b, W1-PROG-8 |
| Pre-pass failure never fails the crawl | W1-PROG-5 (`discoverSitemap` never errors) |
| Running view: phase pill, tape, sitemap/off-sitemap/queue/elapsed/rate counts, status-class totals, "Fetching now", budget notice | W2-PROG-UI-2 |
| Finished view: tape on top ("Not reached by links"), coverage; saved crawls use the saved tape (status-only only for older reports) | W2-PROG-UI-3 |
| New crawl: debounced sitemap preview line, "No sitemap found", **Raise budget to N** | W2-PROG-UI-4 |
| Crawls list: small tape and sitemap coverage per crawl | W2-PROG-UI-5 |
| `Engine.Pause/Resume(rate)`; `frontier.next` blocks while paused; in-flight requests finish; cancel wakes a paused crawl | W2-PAUSE-1, W2-PAUSE-3 |
| `MaxDuration` counts active time only | W2-PAUSE-2, W2-PAUSE-3 |
| `PauseOnThrottle`: >= 10 throttles in 60s after >= 2 backoffs; latest Retry-After recorded | W2-PAUSE-4 |
| `runner.Control` / `WithControl` (pre-crawl calls queued, post-crawl no-ops), `WithPauseOnThrottle` | W2-PAUSE-5 |
| Pause/resume endpoints, `paused` status, 409 rules, web API turns PauseOnThrottle on | W2-PAUSE-6 |
| Pause (yellow), Resume crawl (green), Stop crawl (red), paused pill, auto-pause notice with Retry-After and "Resume at" at half the rate | W3-PAUSE-UI-1 |
| Docs (`web.md`, `output.md`, `architecture.md`) and one CHANGELOG line per wave | W1-PROG-10, W2-PROG-UI-6, W2-PAUSE-7, W3-PAUSE-UI-2 |

#### Contract names used exactly

`crawler.Progress` (all contract fields and JSON tags), `Options.OnProgress`,
`(*Engine).InScope`, `sitemap.Discover`, `sitemap.WithURLs`, `runner.Phase`,
`runner.Progress` (embedding `crawler.Progress`, all contract fields), `runner.WithProgress`,
`Coverage.SitemapTotal` / `SitemapFetched` (`sitemap_total` / `sitemap_fetched`),
`JobView.Progress *progressView` (`progress,omitempty`) with `tape`, `tape_off`, `tape_from`,
`max_pages`; `GET /api/crawls/{id}?tape_from=N`; `POST /api/sitemap-preview` body
`{url, include, exclude, allow_subdomains}` → `{found, total, sources}`;
`(*Engine).Pause()`, `(*Engine).Resume(rate float64)`, `Options.PauseOnThrottle`,
`runner.Control`, `NewControl`, `(*Control).Pause() bool`, `(*Control).Resume(rate) bool`,
`WithControl`, `WithPauseOnThrottle`; `POST /api/crawls/{id}/pause`, `/resume` `{rate?}`;
`StatusPaused`; `crawlJob.Progress`, `Tape`, `TapeOff`, `control`; `runSettings.progress`,
`control`, `pauseOnThrottle`; `jobManager.with/start/get/list/finish/cancel`; `requireJSON`;
frontend `request`, `postJSON`, `Button` variants `go`/`pause`/`stop`/`secondary`,
`Notice` `tone`/`action`, `StatusCode`, `Tape` props.

#### Contract gaps filled (for the coordinator to reconcile)

1. `crawler.Progress` gains `FinalURL string json:"final_url,omitempty"` (sitemap membership of
   redirected pages) and `DurationMS int64 json:"duration_ms"` ("Fetching now" timing).
2. `Progress.Status` is the **first redirect hop's** status when a fetch was redirected (so the
   tape shows 3xx); `report.Summary.ByStatus` keeps final statuses, so a saved crawl's tape can
   differ from the live one.
3. New exported `Engine` methods: `SetSeed(seed) (string, error)`, `Normalize(rawURL) string`,
   `RobotsFor(ctx, rawURL) *RobotsData`, `Paused() bool`, `ActiveTime() time.Duration`.
4. New `AdaptiveLimiter` methods: `RecentThrottled`, `LastRetryAfter`, `EffectiveRate`,
   `SetRate`, `ResetRecent`.
5. `report.Coverage` is `*crawler.Coverage`, so the sitemap fields land on `crawler.Coverage`.
6. `sitemap.Option` type, `New(fetcher, opts ...Option)`, plus `sitemap.Discovery` and
   `sitemap.DiscoverAll` (sources and robots origin for the preview).
7. `runner.RegistryOptions.SitemapOptions []sitemap.Option`.
8. `runner.SitemapPreview` / `runner.PreviewSitemap`; the preview response adds
   `via_robots bool` to `{found, total, sources}`, so the UI can say "via robots.txt".
9. `(*Control).Active() time.Duration` (lock-free) and `progressView.active_ms`.
10. `crawlJob.MaxPages int` (source of `progress.max_pages`).
11. `crawlListItem` gains `status_classes`, `sitemap_total`, `sitemap_fetched`, `tape_preview`;
    `store.Entry` gains `ByStatus` (`pages_by_status`), `SitemapTotal`, `SitemapFetched`,
    `TapePreview`.
12. Pause/resume answer `204` (no body), like cancel; the UI re-polls via `refresh()`
    (adopted by the coordinator).
13. `Control.Pause` returns true (queued) during the sitemap phase and false once the crawl
    phase has ended, as the spec says (adopted by the coordinator; the contract now says so).
14. Behavior change: the pre-pass reads `Sitemap:` lines from the seed host's robots.txt only;
    the old post-crawl analyzer also read robots.txt of other crawled hosts (subdomains). A
    caller deadline on `ctx` no longer sets `DurationLimitReached`; only `MaxDuration` does.
15. Frontend: `types/crawls.ts` gets `'paused'` in `CrawlStatus` (W2-PROG-UI-1) and the
    `Coverage` fields `sitemap_total`, `sitemap_fetched`, `tape`, `tape_off` (W1-PROG-6b);
    `crawlStatus.ts` gets the `paused` pill entry; everything else is in new
    `types/progress.ts` / `api/progress.ts` / `api/pause.ts`.
16. Coordinator decision 1: `crawler.Coverage.Tape` / `TapeOff`, `crawler.TapeChar`,
    `crawler.TapePreview`, `crawler.TapePreviewLen = 160`, `runner.Progress.TapeOffChar()`.
    `runner.Run` always installs its progress reporter (it records the tape), with or without
    `WithProgress`.
17. Reconciled with the visual system: the run-phase pill is `components/progress/RunPhasePill.tsx`
    on top of `components/ui/PhasePill.tsx` (no second pill or pill CSS); `tapeText.ts` takes
    its status classes and labels from `web/src/status.ts`; errors render in `Notice`.

#### Riskiest edge cases, and the test that pins each

- Page events arrive in recording order, one per page (tape alignment):
  `TestEngineOnProgressMatchesRecordedPages`; state events never add a tick:
  `TestRecordProgressGrowsTapeAndSlicesByTapeFrom`.
- Sitemap fetched exactly once per crawl: `TestRunFetchesSitemapOnce`; credentials and robots
  rules still apply to the moved pre-pass: existing `TestRunDoesNotLeakBasicAuthToSitemapHost`,
  `TestRunScopesCustomHeadersForAnalyzerFetches`, `TestRunAnalyzerProbesRespectRobots`.
- `tape_from` past the end, negative, non-numeric: `TestRecordProgressGrowsTapeAndSlicesByTapeFrom`.
- Saved tape equals the live tape, with and without `WithProgress`:
  `TestRunSavesTheLiveTapeInCoverage`, `TestRunSavesTheTapeWithoutProgressOption`; a sitemap
  listing only out-of-scope hosts: `TestRunCountsOnlyInScopeSitemapURLs`.
- Very large crawls in lists (100,000 pages, worst status kept, at most 160 characters):
  `TestTapePreview`, `TestTapePreviewLargeCrawl`, `TestListCarriesStatusCountsAndSitemapCoverage`.
- A job forgotten by a server restart: `TestGetCrawlForgottenJobIs404`.
- Pause queued in the sitemap phase, then canceled: `TestRunCanceledWhileQueuedPauseHolds`.
- Pause blocks new fetches but lets in-flight ones land; a task waiting on the limiter is
  requeued at the head: `TestEnginePauseStopsNewFetchesAndResumeFinishes`,
  `TestFrontierPushFrontRequeuesAtHead`.
- Cancel while paused returns a partial result: `TestEngineCancelWhilePausedReturnsPartialResult`,
  `TestFrontierPausedStillWakesOnCancel`, `TestCancelPausedCrawl`.
- A paused crawl with nothing left still ends: `TestFrontierPausedWithNothingPendingIsExhausted`.
- `MaxDuration` excludes paused time: `TestEngineMaxDurationExcludesPausedTime`,
  `TestActiveClockFiresOnlyOnActiveTime`.
- Auto-pause needs the option, two backoffs and ten throttles, and records Retry-After:
  `TestMaybeAutoPauseNeedsOptionBackoffsAndThreshold`, `TestEngineAutoPausesOnSustained429`
  (fake 429 server).
- Pause queued during the sitemap phase reaches the engine and isn't undone by phase events:
  `TestControlQueuesPauseBeforeTheCrawlPhase`, `TestRunWithControlPausesAndResumes`,
  `TestProgressMovesJobBetweenRunningAndPaused`.
- Lock-order deadlock between pause/resume progress events and GET views:
  `TestPauseResumeWhilePollingDoesNotDeadlock` (under `-race`).
- 409/404/400/415/403 rules on the new routes: `TestPauseResumeRejectWrongStates`,
  `TestResumeValidatesBody`, `TestPauseIsGuarded`, `TestSitemapPreviewRejectsBadInput`;
  paused crawls count toward the cap: `TestPausedCrawlCountsTowardTheCap`.

### Self-review: settings and login

#### Spec items covered

| Spec item | Task(s) |
| --- | --- |
| §6 `Params` gains `strip_query`, `adaptive_delay`, `follow_nofollow`, `timeout`, `bearer_token`; MCP gets them | W1-SET-2 (schema test pins MCP), docs W1-SET-8 |
| §6 Bearer token via custom-headers path, confined by `authHostAllowed` | W1-SET-1 (header), W1-SET-3 (`TestCrawlBearerTokenOnlyReachesSeedHost`) |
| §6 Bearer + Basic Auth / custom `Authorization` rejected: 400 API, error CLI | W1-SET-1, W1-SET-3, W1-SET-5 |
| §6 `--bearer-token`, `crawl.bearer_token` (+ env, `gocrawl init` template, interactive menu) | W1-SET-1, W1-SET-3 |
| §6 Header profiles: list/save/delete (+ read), name regex, `ParseHeaderLines`, 0600, atomicfile; routes | W1-SET-4, W1-SET-7 |
| §6 GUI settings `GET`/`PUT /api/settings`, `gui.json` atomic, theme default dark, secrets stripped, store dir read-only | W1-SET-6, W2-SET-5 |
| §7.1–4 Visible window, throwaway profile, capture over CDP incl. HttpOnly, seed-host filter, close + delete profile, API returns names/count/earliest expiry only, states | W1-LOGIN-1, W1-LOGIN-2, W1-LOGIN-3 |
| §7.5 `login_session` → existing `Cookie` option | W1-LOGIN-4 |
| §7.6 1-hour expiry; process exit; window closed → `closed` | W1-LOGIN-2 (`TestLoginManagerExpiresSessions`, `…UserClosingWindow…`), W1-LOGIN-3 (`Server.Close`, serve.go) |
| §7.7 Logout guard shown in the form, removable | W2-LOGIN-2 |
| §7.8 Web/desktop only (not MCP); disabled with hint without a browser | W1-SET-2 (schema test asserts no `login_session`), W1-LOGIN-3 (`/available`), W2-LOGIN-2 |
| §7 `loginBrowser` interface + fake for handler tests; one real chromedp test skipped without Chromium | W1-LOGIN-2/3 (fake), W1-LOGIN-1 (real test) |
| New crawl view: new fields, bearer token, profile dropdown, prefill, Save as defaults, Log in with browser… | W2-SET-3, W2-SET-4, W2-LOGIN-2 |
| Settings view: theme, crawl defaults, header profiles editor, store path | W2-SET-5 |
| App loads `/api/settings` at startup and applies the theme (dark until loaded) | W2-SET-2 |
| Error handling: 400/404/409/415/429/503 JSON errors; guard on new POST/PUT/DELETE | W1-SET-6/7, W1-LOGIN-3/4 tests |
| Docs (`configuration.md`, `mcp.md`, `web.md`) + CHANGELOG per wave/track | W1-SET-8, W1-LOGIN-5, W2-SET-6, W2-LOGIN-3 |

#### Contract names used exactly

`crawlrequest.Params.{StripQuery, AdaptiveDelay *bool, FollowNofollow, Timeout, BearerToken}`
with the contract's JSON tags; `config.{ListHeaderProfiles, ReadHeaderProfile, SaveHeaderProfile, DeleteHeaderProfile}`;
`CrawlConfig.BearerToken \`mapstructure:"bearer_token"\``; `--bearer-token`;
`webserver.GUISettings{Theme, CrawlDefaults}`; `WithGUISettingsPath`; `Server.guiSettingsPath`;
`Server.logins *loginManager`, `newLoginManager()`; `settingsRoutes`, `profileRoutes`,
`loginRoutes`; `startCrawlRequest.LoginSession \`json:"login_session,omitempty"\``;
`loginBrowser`, `loginWindow` (exact method sets), `WithLoginBrowser`; all routes and view
fields as listed; `requireJSON`, `writeJSON`, `writeError`, `formatTime`, `srv.crawls`;
frontend `api/client.ts` `request`/`postJSON`, `theme.ts` `applyTheme`/`Theme`, `Button`
variants `go`/`primary`/`secondary`/`stop`, `Notice` tones, `views/SettingsView.tsx`.

#### Contract gaps filled (defined inside this track's own files)

1. `GET`/`PUT /api/settings` responses carry a read-only `store_dir` (from `store.Root()`).
2. `webserver.DefaultGUISettingsPath()` (`~/.gocrawl/gui.json`), used by `serve.go`.
3. `PUT /api/settings` with no settings path: 503. Empty `theme` means `dark`; an unknown theme
   in a hand-edited file loads as `dark`; a corrupt file is a 500 naming the file.
4. Secret stripping goes beyond the spec's three fields: inline `headers` and user:password in
   `url` / `proxy` / `proxies` are stripped too (gui.json must never hold a secret). Stripped on
   load as well as on save.
5. `config.ErrInvalidHeaderProfile` (400) and `fs.ErrNotExist` wrapping (404);
   `HeaderProfilePath`'s message now starts "invalid header profile: name …".
6. Header-profile `PUT` returns 204 (contract didn't fix a status).
7. `POST /api/crawls` compiles `cfg.ToOptions()` up front so every option error is a 400, not a
   failed job (needed for the bearer-conflict 400 when the `Authorization` comes from a profile).
8. Login: `GET /api/login-sessions/available` is backed by an optional
   `loginProber{ Available() error }` on the `loginBrowser`; at most 2 windows wait at once
   (429); capture with no seed-host cookie yet is 409 and keeps the window open; a session's
   host must equal the crawl seed's host (400); `login_session` + `render: headless` is 400.
9. `func (s *Server) Close()` closes login windows on shutdown (`serve.go` defers it).
10. Chromium code lives in `internal/render/login.go` (`FindBrowser`, `LoginAvailable`,
    `OpenLoginWindow`, `ErrNoBrowser`, `ErrNoDisplay`, `ErrLoginWindowClosed`); browser lookup
    mirrors chromedp's unexported list (minus `headless_shell`, plus Edge) and launches with
    `ExecPath` so detection and launch agree.
11. Session ids are 128-bit random (`newLoginID`), not `newJobID`'s 64 bits.
12. Login hosts compare in ASCII (`asciiHost`, IDNA lookup rules): a Unicode seed matches the
    punycode cookie domains Chromium stores (`TestLoginSessionMatchesIDNHosts`).
13. Reconciled with wave 0 and the visual system: `settings.go` keeps wave 0's
    `WithGUISettingsPath` (not redeclared); the UI uses the visual system's `cf-` form markup
    and `CrawlForm.css`, carries the progress track's `SitemapPreviewLine`, and the Settings
    view has its own `SettingsView.css`.
14. Open for a human decision: the coordinator asked to allow `login_session` with
    `render: headless`, on the premise that `internal/render/render.go` already applies
    `opts.Cookie`. It does not: `NewHeadlessFetcher` returns an error for any `opts.Cookie`
    (render.go:58), so allowing the combination would turn a clear 400 into a crawl that fails
    when it starts. This plan keeps the 400 and its test until that is decided.

#### Riskiest edge cases and the tests that pin them

| Risk | Test |
| --- | --- |
| Bearer token sent to a third-party host on `--external` | `TestCrawlBearerTokenOnlyReachesSeedHost` (localhost vs 127.0.0.1, because scoping ignores ports) |
| Token echoed in an error (CLI, MCP, API) | `TestToOptionsBearerTokenConflicts`, `…RejectsHeaderInjection`, `TestCrawlBearerTokenConflictsWithBasicAuth`, `TestStartCrawlRejectsBearerTokenConflicts` |
| Conflict hidden in a header profile or seed userinfo | same tests, profile and `alice:pw@` cases |
| CR/LF injection through the token or a profile | `TestToOptionsBearerTokenRejectsHeaderInjection`, `TestSaveHeaderProfileValidates` |
| Profile name path traversal (incl. `%2F`) | `TestSaveHeaderProfileValidates`, `TestHeaderProfileFuncsRejectBadNames`, `TestHeaderProfileRoutesRejectBadNames` |
| Profile / gui.json file mode | `TestHeaderProfilesSaveListReadDelete` (0600 file, 0700 dir), `TestPutSettingsPersistsAndStripsSecrets` |
| Secrets in gui.json or in a GET after hand-editing | `TestPutSettingsPersistsAndStripsSecrets`, `TestGetSettingsStripsHandEditedSecrets` |
| Cookie values in any login API response or job view | `TestLoginSessionFlow`, `TestStartCrawlWithLoginSessionSendsCookiesToSeed` |
| Cookies for other hosts (IdP, sibling subdomain, suffix tricks, expired) captured | `TestCookiesForHostFollowsBrowserDomainRules` |
| Login cookies used for a different seed host | `TestStartCrawlLoginSessionErrors` (other host) |
| Unicode (IDN) seed host vs. punycode cookie domains | `TestLoginSessionMatchesIDNHosts` |
| Orphaned browser windows / profile dirs (user close, cancel during launch, expiry, shutdown) | `TestLoginManagerUserClosingWindowEndsSession`, `…RemoveWhileOpening`, `…ExpiresSessions`, `TestServerCloseClosesLoginWindows`, real `TestLoginWindowReadsHTTPOnlyCookiesAndRemovesProfile` |
| HttpOnly cookies missed by the capture | real `TestLoginWindowReadsHTTPOnlyCookiesAndRemovesProfile` (runs headless in CI where Chrome exists) |
| Guard bypass on new mutating routes | `TestPutSettingsGuards`, `TestHeaderProfileRoutesGuards`, `TestStartLoginGuards` |
| `WithLoginBrowser` applied before `New` builds `s.logins` (wave-0 ordering) | `TestLoginSessionFlow` fails with an explicit hint |

Verification done while writing this plan: every Go block of wave 1 was applied to a scratch
copy of the repo (with a minimal wave-0 shim: `logins`/`guiSettingsPath` fields, the three
route calls, `requireJSON`, `srv.jobs` in place of `srv.crawls`) and `make check` passed,
including golangci-lint (0 issues) and the real-Chromium login test on macOS. That run is what
surfaced the `configs/example.yaml` / docs example-block copies now in W1-SET-1. Every wave-2
file and edit was likewise applied on top of shims for the wave-0 modules (`api/client.ts`,
`api/crawls.ts`, `types/crawls.ts`, `theme.ts`, default-export `Button`/`Notice`): `tsc -b`,
`oxlint` (0 warnings) and `vite build` pass.

Not covered by automated tests (manual steps instead): the visible window and the "person
closes the tab" signal (W1-LOGIN-1 Step 4), and every frontend behavior (no frontend test
runner by spec).

### Self-review: compare, import and redirects

#### Spec items covered

| Spec item | Task |
| --- | --- |
| §3 `GET /api/compare`, refs through `store.Resolve` (IDs, `latest`, hosts), 400 missing, 404 unresolved, `diff.Compare` JSON | W1-CMP-1, W1-CMP-2 |
| §4 `POST /api/reports/import`: multipart, 64 MB, decode `report.Report`, reject unparseable or seedless files, `store.Save`, visible in Crawls, export, compare | W1-CMP-3 |
| §5 `OnRow(done,total)`, `RowResult` JSON tags | W1-RED-1 |
| §5 multipart 10 MB, `domain`, `sitemap_url`, `concurrency`, `rate`, `adaptive_delay`; `ParseCSV` up front → 400; 202; cap 2 → 429 | W1-RED-2 |
| §5 GET status, done/total, rows once finished; cancel; export through `WriteCSV`; in memory only | W1-RED-2, W1-RED-3 |
| Error handling: 400/404/409/413/415/429 JSON errors | W1-CMP-2, W1-CMP-3, W1-RED-2, W1-RED-3 |
| Guard on new POST and multipart routes (cross-origin 403, content type 415) | W1-CMP-3, W1-RED-2, W1-RED-3 |
| Testing: `redirectcheck` `OnRow` reaches `total`; httptest for every new route | W1-RED-1..3, W1-CMP-2..3 |
| Crawls view: checkboxes to pick two → Compare; **Import report…** | W2-CMP-4 |
| Compare view: pickers defaulting to the newest two crawls of the same host, both tapes (saved `coverage.tape`; status-only fallback for older reports), severity and analyzer deltas, new/resolved/persisting tabs with the report's issue table and filters, added and removed pages | W2-CMP-2, W2-CMP-3 |
| Redirects view: CSV, domain, sitemap URL, rate, concurrency, **Check redirects** (violet primary), progress bar, results filterable by verdict and scope, **Export CSV** | W2-RED-2, W2-RED-3 |
| Constraints: no `localStorage` (state in the URL or the server); no feature depends on a download (export is a plain link); no new runtime deps | All W2 tasks |
| Docs: `docs/web.md`, `docs/redirect-check.md`, a CHANGELOG line per PR | W1-CMP-4, W1-RED-4, W2-CMP-5, W2-RED-4 |

#### Contract names used

`jobManager[T]` with `start` / `get` / `list` / `with` / `finish(id, err, apply)` / `cancel`;
`Job[T]`; `StatusRunning` / `StatusPaused` / `StatusDone`; `crawlJob{Seed, Report}`;
`Server.redirects`; `redirectJob{Domain, Total, Done, Rules, Rows}`; `compareRoutes`,
`importRoutes`, `redirectRoutes`; `parseMultipart`; `writeJSON`, `writeError`, `formatTime`;
`redirectcheck.RunOptions.OnRow func(done, total int)`; `GET /api/compare?base=&current=`;
`POST /api/reports/import` field `file` → 201 `{id}`; multipart fields
`file, domain, sitemap_url, concurrency, rate, adaptive_delay`. Frontend: `api/client.ts`
`request` / `postForm`; `api/crawls.ts`; `types/crawls.ts`; `routes.ts` `href` and the
`compare` / `crawls` / `crawl` / `redirects` / `redirect` routes; `views/CompareView.tsx`,
`views/RedirectsView.tsx`, `views/RedirectView.tsx`; `Button` variants `primary`,
`secondary` and `stop`; `Notice` tones `error` and `info` with `action`; `StatusCode {code}`;
`Tape {tape, summary, compact}`.

#### Contract gaps filled (explicit)

1. **`store.ResolveStored(ref)`** (new). `store.Resolve` reads any JSON file named by a path,
   which turns into a local-file read once the API exposes it. The API resolves refs without
   the file-path branch, and `Resolve` delegates to `ResolveStored`.
2. **Compare refs also accept crawl-job IDs from this process.** `GET /api/crawls` lists
   in-memory jobs, saved ones included, under their job ID. `store.Resolve` cannot resolve a
   job ID. A running or paused job returns 409; a failed job returns 404.
3. **`GET /api/redirect-checks`** list endpoint, returning `{"checks": [view without rows]}`.
4. **`redirectJob.FileName`** is added for the session list.
5. **`redirectcheck.NewFetcher`, `DefaultTimeout`, `DefaultUserAgent`** form the shared
   constructor. The CLI now uses them.
6. **`Rule` gets JSON tags too.** Each API row is `Rule` and `RowResult` flattened into one
   object. `notes` is `omitempty`.
7. **Test seams are package vars** (`newRedirectFetcher`, `maxImportBytes`,
   `maxRedirectCSVBytes`) and not `Server` fields, so `Server` (a wave-0 file) does not change.
8. **Compare tapes come from `report.coverage.tape` / `tape_off`** (stored per page by
   W1-PROG-6b, coordinator decision). Only reports saved before that fall back to
   `summary.pages_by_status`, grouped by status, with a caption that says so.
9. **A cancel during sitemap discovery is recorded as `canceled`, not `error`.** A canceled
   check keeps no rows, and its export returns 409.
10. **Import hardening beyond the spec.** The seed must be an http(s) URL with a host. Nil
    summary maps and issues are filled with empty values. An import with the same host and
    start time as a saved crawl replaces it (documented).
11. **`components/IssueTable.tsx`** is extracted from the visual system's `CrawlReport.tsx`
    (W1-VIS-7 markup and `cr-` classes, imported `CrawlReport.css`), after the progress UI's
    edits to that file have merged (merge order in wave 2: progress UI first).
12. **Reconciled with the visual system:** no pre-visual classes (`card`, `empty`, `tabs`,
    `table-scroll`, `url-cell`, `filter-count`); Compare uses `cr-tabs` and `muted`, Redirects
    use their own `redirect-` classes in `redirects.css`, export links use `btn btn-secondary`.
    `RedirectView` already receives `id` from wave 0's `App.tsx`.

#### Riskiest edge cases pinned with tests

- A path to a file outside the store passed as a compare ref → 404
  (`TestCompareNeverReadsFilePaths`, `TestResolveStoredNeverReadsFilePaths`).
- A running in-memory crawl as a compare ref → 409; finished → 200; failed → 404
  (`TestCompareUsesCrawlJobsFromThisProcess`).
- An import with no seed, a `file://` seed, `null`, an array, or bad JSON → 400 and nothing
  saved. Oversized → 413. Not multipart → 415. Cross-origin → 403 and nothing saved
  (`TestImportReport*`).
- `OnRow` calls are serialized and strictly increasing to `total`, including skipped rows,
  under `-race` with concurrency 3 (`TestRunReportsEveryRowThroughOnRow`).
- A real HTTP round trip through `redirectcheck.NewFetcher` to an httptest site, including a
  301 and sitemap membership (`TestRedirectCheckRunsToCompletion`).
- No sitemap → job `error` with the message and no rows
  (`TestRedirectCheckWithoutSitemapEndsInError`).
- Cancel while the sitemap fetch is blocked → `canceled`, not `error`. A second cancel →
  409. Export after cancel → 409. The slot is freed under the cap (`TestCancelRedirectCheck`).
- The cap of 2 → 429 (`TestRedirectCheckCapReturns429`). Export while running → 409
  (`TestRedirectCheckExportBeforeDoneIs409`).
- Thirteen invalid-input cases → 400 with no job started. Domain normalization from a pasted
  URL (`TestStartRedirectCheckRejectsBadInput`, `TestNormalizeRedirectDomain`).
- Guards on the multipart routes and on cancel (`TestStartRedirectCheckGuards`,
  `TestCancelRedirectCheckRejectsCrossOrigin`).

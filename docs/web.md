---
id: "web"
title: "Web UI"
scope: "Running `gocrawl serve`, the embedded browser UI, the REST API it's built on, and building the frontend from source."
topic: "reference"
status: "current"
superseded_by: []
updated: 2026-10-14
depends_on: ["mcp.md", "output.md", "redirect-check.md"]
sources: ["../internal/runner/runner.go", "../internal/webserver", "../cmd/gocrawl/serve.go", "../web", "../web/src/fonts/OFL.txt", "../web/src/styles/tokens.css", "../web/src/theme.ts", "../web/scripts/contrast.mjs", "../web/src/styles/base.css", "../web/src", "../web/src/routes.ts", "../web/src/api/client.ts", "../web/src/api/crawls.ts", "../web/src/types/crawls.ts", "../internal/webserver/job.go"]
---
# Web UI guide

gocrawl can run as a small web application: an HTTP API to start and inspect crawls, backed
by the same [`runner.Run`](../internal/runner/runner.go) seam the CLI and MCP server use, plus
a built-in single-page app that drives it from a browser. Source:
[`internal/webserver`](../internal/webserver), wired up in
[`cmd/gocrawl/serve.go`](../cmd/gocrawl/serve.go). The frontend lives in
[`web/`](../web) and is embedded into the binary, so `gocrawl serve` stays a single
executable — nothing else to deploy.

## Starting the server

```sh
gocrawl serve
```

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `127.0.0.1:8080` | Address to listen on. Loopback by default; see [Exposure](#exposure) before binding anything else. |
| `--store-dir` | `~/.gocrawl/crawls` | Crawl history root, same store used by `--save`, `gocrawl history`, and `gocrawl compare`. |

Open `http://localhost:8080` in a browser. Ctrl-C shuts the server down gracefully (in-flight
HTTP requests are given up to 10s to finish; crawl jobs already running detach from the
request that started them, so they aren't tied to a shutdown).

## Exposure

The server has **no authentication**. It is built to be driven from a browser on the same
machine, and a few guards keep it that way:

- It listens on loopback by default, and rejects (`403`) any request whose `Host` header is
  not `localhost` / `127.0.0.1` / `[::1]`. That stops DNS-rebinding pages, which reach
  `127.0.0.1` under their own hostname.
- Non-`GET` API requests with an `Origin` header must come from the same origin as the
  server (`403` otherwise), and `POST /api/crawls` insists on `Content-Type:
  application/json` and upload routes on `multipart/form-data` (`415` otherwise). Together these stop any other web page you have open
  from starting or cancelling crawls.
- At most 4 crawls run at once; a fifth `POST /api/crawls` returns `429` until one finishes
  or is cancelled.
- At most 2 redirect checks run at once; a third `POST /api/redirect-checks` returns `429`.

Binding a non-loopback `--addr` (for example `:8080` or `0.0.0.0:8080`) switches the `Host`
check off, prints a warning, and makes the API reachable by anyone on that network. A crawl
runs from this machine with whatever proxy, Basic Auth, cookie, or custom headers the request
supplies, and can name any header profile stored on it, so only do this behind something that
authenticates (a reverse proxy, VPN, or SSH tunnel). The same goes for the stored data: anyone
on that network can also read and overwrite header-profile bodies (which hold secrets) and the
GUI settings.

## Building the real UI into the binary

A plain `go build`/`make build` embeds a placeholder page (no Node required) so the Go build
never depends on the frontend toolchain. To ship the actual UI:

```sh
make web-build   # npm ci && npm run build in web/, output lands in
                  # internal/webserver/webui/dist/
make build       # embeds it via internal/webserver/assets.go
```

`web/` is a standard Vite + React + TypeScript app; `npm run dev` in `web/` proxies `/api/*`
to `gocrawl serve` running on `:8080` for frontend development with hot reload.

## What the UI does

The navigation lists New crawl, Crawls, Compare, Redirects, and Settings. Compare,
Redirects, and Settings are placeholders until those features land; the Compare view and the
report import form arrive with the GUI views, on top of the compare and import endpoints
already in the API below. The working views, all against the API below:

- **New crawl** — a form for the seed URL and every crawl option the interactive menu
  exposes: depth, page cap, concurrency, rate limit, max duration, render mode,
  robots.txt/subdomain/external-link scope, analyzer selection, specialized checks, security
  audit, and an option to ignore UTM tagging issues on external links, and an "Advanced"
  section for include/exclude regexes, User-Agent (or a rotating pool), proxy (or a rotating
  pool), HTTP Basic Auth, and a session-cookie field (a checkbox reveals a paste box) for sites
  gated by an app-level cookie, e.g. a Shopify storefront password page, and a custom-headers
  field (inline `Name: value` lines and/or a stored header profile name, e.g. for a Shopify
  crawler access signature). It also has a per-request timeout, nofollow following, query-string stripping, the
  429/503 slow-down, a bearer token, and a header-profile dropdown filled from Settings. It
  opens pre-filled with your saved defaults; **Save as defaults** stores the current values
  (never passwords, cookies, tokens or inline headers). Under Advanced, **Log in with browser…** opens a real
  browser window for sites behind a login form (see
  [Logging in with a browser](#logging-in-with-a-browser)); the form then shows the captured
  cookie names, adds a logout exclude pattern you can remove, and starts the crawl with that
  session. The button is disabled, with the reason, when no Chromium-class browser is installed
  or Render is headless. Under the URL, a line reports the site's sitemap as you type (for example
  "Found sitemap.xml via robots.txt: 1,240 pages in scope"), and when the page budget is
  smaller, a notice offers **Raise budget to 1,240**. Starts a job and jumps to its report.
- **Crawl (running)** — The header shows the host, a phase pill, a yellow
  **Pause** and a red **Stop crawl**. Below it: a notice when the budget is below the sitemap
  total, the crawl tape (one tick per page, colored and sized by status class, notched when
  the page isn't in the sitemap, with outline ticks for sitemap pages not fetched yet or past
  the budget), counts (sitemap pages fetched of the total, fetched pages not in the sitemap,
  queue, elapsed time, request rate), status-class totals, and a "Fetching now" table of the
  last eight pages with status and timing.
- **Crawl (paused)** — the pill reads **Paused** and the actions become a green **Resume
  crawl** and **Stop crawl**. Elapsed time stops while paused. When the crawl paused itself
  because the site kept answering 429 or 503, a notice explains it, shows the server's
  `Retry-After`, and offers a **Resume at** rate prefilled at half the current rate.
- **Crawl (finished)** — The tape sits at the top of the report, followed by the sitemap
  coverage line. The report saves the tape (`coverage.tape` / `tape_off`), so a crawl reopened
  later, even after a server restart, shows the same tape; reports saved before this version
  get a status-only tape. Then: a partial-coverage banner when the crawl didn't reach the
  whole site, summary counts by severity/analyzer/page-status, an issues table (filterable by
  severity, analyzer, and free-text search) with each issue's what/impact/fix explanation and
  raw `data` expandable inline, any notes advisories, and export links. The table draws 100
  issues at a time (**Show 100 more**, or **Show all**); filters, sort, the counts and the bulk
  review actions (**Select all matching**) apply to every matching issue, not just the rows
  drawn. Compare's issue lists work the same way.
- **Crawls** — every crawl saved to the store (`save: true` on start), reopen-able as a
  report. Each row shows a small tape (a 160-tick preview, worst status per tick) and its
  sitemap coverage. Tick two finished crawls and select **Compare selected** to open them in Compare. **Import report…**
  uploads a gocrawl JSON report (for example one written with `-f json`), saves it to the
  store, and opens it. Its old address, `#/history`, still opens it.
- **Compare** — two pickers, defaulting to the newest two crawls of the same host, and a
  **Swap** button. For each side it shows the crawl's tape from the saved report
  (`coverage.tape`, in fetch order, notched where a page is not in the sitemap); a report saved
  before tapes were stored gets a tape grouped by status instead. Then the issue counts by
  severity (new, resolved, net change), the net change by analyzer and by page status, tabs
  for new, resolved and persisting issues using the report's issue table and filters, and the
  added and removed pages. The selection is in the URL (`#/compare?base=…&current=…`), so a
  comparison can be bookmarked.
- **Redirects** — upload a HubSpot-format redirect-rule CSV, enter the domain, and
  optionally a sitemap URL, rate, concurrency and adaptive delay, then select **Check
  redirects**. The check page shows a `done of total` progress bar (polled every 1.5 s) and
  **Cancel check**. When it finishes, it shows a results table with HTTP status codes,
  filterable by verdict and by scope, and **Export CSV**. When no sitemap is found, the check
  ends with that error and a hint to fill in the Sitemap URL field. The Redirects page lists
  this session's checks, which are kept in memory only.
- **Settings** — theme (dark, light, or match the system), crawl defaults
  (the New crawl fields, minus secrets), header profiles (list, create, edit, delete; each is
  validated like `--header-file` and stored with mode `0600` in `~/.gocrawl/headers`), and
  the crawl store's folder (read-only). Saved in `~/.gocrawl/gui.json`, not in the browser, so
  it survives the desktop app's changing port.

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

## API

All endpoints are under `/api`; everything else falls through to the embedded frontend.

| Method & path | Description |
| --- | --- |
| `GET /api/analyzers` | List available analyzers (same as `list_analyzers` over MCP). |
| `GET /api/explanations` | Every issue code's what/impact/fix explanation, keyed by code — the same text baked into the HTML report, used by the live report view. |
| `POST /api/crawls` | Start a crawl. Body is JSON (`Content-Type: application/json` required) with the same field set as the [MCP `crawl` tool](mcp.md#crawl) (`url` required) plus `save: bool` to persist the report to the store when it finishes. Returns `202` with the job immediately; the crawl runs in the background. `429` when the running-crawl cap is reached. Optional `login_session` (web only, not MCP) names a captured browser login ([below](#logging-in-with-a-browser)) whose cookies become the crawl's `Cookie` header; `400` together with `cookie`, with `render: headless`, or for a different host than the seed, `404` for an unknown id, `409` when it isn't captured (or its cookies have expired). |
| `GET /api/crawls` | List every job (in-memory, this process) merged with the store's saved history, newest first. Each item also carries `status_classes` (pages per status class: `2`, `3`, `4`, `5`, `x`), `tape_preview` (the crawl tape shortened to at most 160 characters, each standing for a run of pages and showing the worst status among them) and, when known, `sitemap_total` / `sitemap_fetched`. List items never carry the full tape. |
| `GET /api/crawls/{id}` | One job's status, its live `progress` (see [Live progress](#live-progress)) and, once finished, its full `Report` (see [Output reference](output.md)). `?tape_from=N` returns only the tape characters from index `N`; a negative or non-numeric `N` is a `400`. Falls back to the store for an id that isn't a live job; saved crawls have no `progress`, but their report's `coverage.tape` / `tape_off` hold the same tape. An id that is neither (for example a crawl that was running when the server restarted) is a `404` with the fixed message `crawl not found` (never the store path). |
| `POST /api/crawls/{id}/cancel` | Cancel a running crawl. Like Ctrl-C on the CLI, this doesn't error the crawl — it stops early and still returns whatever was fetched as a partial report (`Report.Coverage.Interrupted`), tracked here as job status `canceled`. |
| `POST /api/crawls/{id}/pause` | Pause a running crawl: no new fetches start, requests already in flight finish. During the sitemap pre-pass the pause is queued and takes effect when crawling starts. `204`; `409` when the crawl isn't running or is already analyzing. |
| `POST /api/crawls/{id}/resume` | Resume a paused crawl. JSON body, optional `rate` (requests per second, `> 0`) to continue at a new rate; `{}` keeps the current one. `204`; `409` when the crawl isn't paused. |
| `GET /api/crawls/{id}/export?format=json\|csv\|html` | Download the finished report in any of the three formats gocrawl already writes, via `report.For(format)`. `409` for a live job that has no report yet (`crawl has not finished yet`) or that failed (`crawl failed, so there is no report to export`); `404` with the fixed message `crawl not found` for an id that is neither a live job nor a saved crawl. |
| `POST /api/sitemap-preview` | Find a site's sitemap without crawling. JSON body `{url, include, exclude, allow_subdomains}`; returns `{found, total, sources, via_robots}`, where `total` counts only sitemap URLs inside that scope. Respects robots.txt and sends the crawl's default User-Agent. The preview sends no credentials (no Basic Auth, cookie or custom headers), so a site gated behind them may report no sitemap. |
| `GET /api/settings` | The UI's saved settings: `theme` (`dark`, `light` or `system`), `crawl_defaults` (a crawl request body that pre-fills the New crawl form), and the read-only `store_dir`. |
| `PUT /api/settings` | Save `theme` and `crawl_defaults` (JSON). Written atomically to `~/.gocrawl/gui.json` with mode `0600`. `bearer_token`, `basic_auth`, `cookie`, inline `headers`, and user:password in the URL or proxy URLs are stripped before saving. `400` for an unknown theme or an unusable default. |
| `GET /api/header-profiles` | `{"profiles": [...]}`: the names in `~/.gocrawl/headers/`. |
| `GET /api/header-profiles/{name}` | `{"name", "body"}` of one profile; the body is returned as-is, header values included. `404` when it doesn't exist, `400` for an invalid name. |
| `PUT /api/header-profiles/{name}` | Create or replace a profile from JSON `{"body": "Name: value\n…"}`, validated like `--header-file`. `204`; `400` with the parser's message when invalid. |
| `DELETE /api/header-profiles/{name}` | Delete a profile. `204`, or `404`. |
| `GET /api/login-sessions/available` | `{"available": bool, "reason"?: string}`: whether a Chromium-class browser (and, on Linux, a display) is there to log in with. |
| `POST /api/login-sessions` | JSON `{"url"}`: open a visible browser window on that URL in a throwaway profile. `202` with the session; `400` for a URL that isn't http(s) with a host; `429` when two login windows are already waiting; `503` "no Chromium-class browser found" (also when the browser fails to start or the server is shutting down). |
| `GET /api/login-sessions/{id}` | `{"id", "state", "host", "cookie_names", "cookie_count", "expires_at"?}`. `state` is `waiting`, `captured`, `closed` (window closed before capture) or `expired` (one hour after the window opened). `expires_at` is the earliest cookie expiry, omitted when all are session cookies. Never cookie values. `404` for an unknown id. |
| `POST /api/login-sessions/{id}/capture` | Read the browser's cookies (HttpOnly included), keep those a browser would send to the session's host, close the window and delete its profile. `404` unknown id; `409` when the window is gone, already captured, expired, or has no cookie for the host yet (the window then stays open). |
| `DELETE /api/login-sessions/{id}` | Close the window and forget the session and its cookies. `204`; `404` for an unknown id. |
| `GET /api/redirect-checks` | List the redirect checks started in this process, newest first, without their rows: `{"checks": [...]}`. |
| `POST /api/redirect-checks` | Start a redirect-rule check, the same check as [`gocrawl check-redirects`](redirect-check.md). Multipart form: `file` (the HubSpot-format CSV, up to 10 MB), `domain` (required), and optionally `sitemap_url`, `concurrency` (1 to 16, default 4), `rate` (requests per second, default 0 = unlimited) and `adaptive_delay` (default `true`). The CSV is parsed before the job starts, so a wrong header is a `400` and not a failed job. `domain` must be a bare host name (a URL is reduced to its host); a value with a path or a user name or password is a `400`, and so is a `sitemap_url` that is not an `http` or `https` URL or that carries a user name or password. Returns `202` with the job. `413` over 10 MB, `415` when the request is not multipart, `429` when 2 checks are already running. |
| `GET /api/redirect-checks/{id}` | One check: `status`, `done` and `total` rules, and, once the status is `done`, `rows`. Each row is the CSV columns (`original`, `target`, …, `note`) plus the result (`scope`, `source_status`, `source_final_url`, `source_matches_target`, `target_status`, `original_in_sitemap`, `target_in_sitemap`, `verdict`, `notes`). When no sitemap is found, the status is `error` with the reason; a sitemap URL you supplied is replaced by "the sitemap URL" in that message. |
| `POST /api/redirect-checks/{id}/cancel` | Stop a running check; returns `204`. A canceled check keeps no rows. `404` for an unknown id, `409` when the check is not running. |
| `GET /api/redirect-checks/{id}/export` | Download the result CSV as an attachment: the input columns plus the verdict columns. `404` for an unknown id, `409` until the check is `done`. |
| `GET /api/compare?base=<ref>&current=<ref>` | Diff two crawls, `base` being the earlier one. The response is the same JSON as `gocrawl compare --format json`: new, resolved and persisting issues, summary deltas, and added and removed pages. A ref is a crawl ID from `GET /api/crawls` (a live job or a saved crawl), a host such as `example.com` (its newest saved crawl), or `latest`. File paths are not accepted. `400` when a ref is missing, `404` when one does not resolve, `409` when one names a crawl that is still running or paused. |
| `POST /api/reports/import` | Save a gocrawl JSON report to the store, the web equivalent of `gocrawl render`. Multipart form with the report in the field `file`, up to 64 MB. Returns `201` with `{"id": "<host>/<timestamp>"}`; the crawl then appears in the crawl list and works with export and compare. `400` when the file is not a report or its `seed` is not an http(s) URL with a usable host, `413` when it is too large, `415` when the request is not `multipart/form-data`. An import with the same host and start time as a saved crawl replaces it. |

Job status is one of `running`, `paused`, `done`, `error`,
`canceled`. A paused crawl still counts toward the running-crawl cap and can be canceled. The redirect-check endpoints use
fixed error messages that never repeat the id, the domain, or a sitemap URL you sent, so a
token in a sitemap URL cannot leak through an error.

Details of the compare and import endpoints:

- **Compare refs.** A live job ID wins over a stored ID of the same text. A crawl that was
  canceled still has its partial report, so it can be compared; a job that ended in `error`
  has no report and returns `404`. A ref the store cannot resolve returns a generic "not
  found" message rather than the store's own error, which names its directory.
- **Import.** The seed must be an `http` or `https` URL whose host is not empty or made of
  dots alone. Unicode hosts and `xn--` (punycode) hosts are accepted, and a port is dropped
  from the filing host. A missing `started_at` or `finished_at` is filled from the other one,
  or with the import time when both are missing, because the store files a crawl under its
  start time. That ID has one-second resolution, so two imports with the same host that start
  in the same second replace each other.

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
| `sitemap_found`, `sitemap_total`, `sitemap_fetched`, `off_sitemap`, `in_sitemap` | Progress against the site's sitemap: in-scope sitemap URLs, how many of them were fetched, fetched pages the sitemap doesn't list (equal to `fetched` when `sitemap_found` is false, since nothing is listed), and whether `current_url` is in it. `in_sitemap` also matches when the page redirected to a sitemap URL (`final_url`). |
| `max_pages` | The page budget (`0` = unlimited). |
| `active_ms` | Active crawl time: time spent paused, and the sitemap pre-pass, don't count. |
| `paused`, `pause_reason`, `retry_after_seconds` | Whether the crawl is paused, why (`user`, or `rate_limited` when it paused itself), and the latest `Retry-After` the server sent. |
| `tape`, `tape_off`, `tape_from` | One character per recorded page, in fetch order. `tape` holds the status class (`2`, `3`, `4`, `5`, or `x` for a failed fetch); `tape_off` holds `1` for a page the sitemap doesn't list (and `0` otherwise). `tape_from` is the index the returned slice starts at. |

Web crawls pause themselves when the site keeps rate limiting them: once adaptive delay has
lowered the rate at least twice, ten or more 429/503 responses within a minute pause the crawl
with `pause_reason: "rate_limited"`. The CLI and MCP never pause, because nobody is there to
resume.

Before every crawl, CLI and MCP included, gocrawl reads the site's sitemap once: robots.txt
`Sitemap:` lines, then `/sitemap.xml` and `/sitemap_index.xml`, following indexes. Only URLs
inside the crawl scope count toward `sitemap_total`. The `sitemap` analyzer reuses the same
set instead of fetching it again.

## Logging in with a browser

For sites behind a login form (staging, member areas, a Shopify password page), the UI can
borrow a real browser session: gocrawl opens a normal Chrome, Chromium or Edge window on the
site in a fresh, temporary profile, you log in there (SSO, two-factor and CAPTCHAs work), and
gocrawl reads that window's cookies when you say you're done. It keeps only the cookies a
browser would send to the site's host, closes the window, and deletes the profile. The New crawl
form's **Log in with browser…** button drives this API.

The flow, over the API:

1. `POST /api/login-sessions` with the site's URL. At most 2 sessions can be waiting for a
   login at once. The session id is 128 random bits and is the only thing that refers to the
   captured cookies.
2. The person logs in in the window.
3. `POST /api/login-sessions/{id}/capture`. This only succeeds once the site has set a cookie
   for its host; until then it returns `409` and leaves the window open so they can finish.
4. `POST /api/crawls` with `"login_session": "<id>"`. The login URL's host must equal the
   crawl seed's host (compared in ASCII/punycode form, port ignored), or the request is a `400`.

A session ends after one hour whether or not it was captured, when its window is closed before
capture, or when it is deleted. The hour runs from when the login window opens, not from when
the cookies are captured.

- Cookies stay in the server's memory: never written to disk, never returned by the API
  (only names, the count and the earliest expiry), never logged, dropped after one hour or when
  `gocrawl serve` stops.
- `login_session` can't be combined with `cookie` (`400`), and it needs `render: raw`, the
  default: headless rendering refuses cookies because it can't keep them on the seed host.
- The crawl sends the captured cookies as one `Cookie` header with the same scoping as
  `--cookie`: the seed host only (plus subdomains when `subdomains` is on), never over an
  `https` to `http` downgrade. That header is built from name and value alone, so each
  cookie's `Path` and `Secure` attributes are ignored, and a host-only cookie may reach the
  subdomains too when `subdomains` is on.
- The API does not add a logout exclusion. The UI adds the exclude pattern
  `(?i)/(log|sign)-?out(/|$|\?)` to the crawl form, so the crawl doesn't
  follow a logout link and end the session. It stays visible in the exclude field; remove it
  if you need those pages. API clients should add their own `exclude`.
- Each login window uses a temporary browser profile named `gocrawl-login-*` in the system
  temp directory. It is deleted when the window closes. If the server was killed before it
  could clean up, `gocrawl serve` removes profiles older than one hour from the temp
  directory when it starts.
- `gocrawl serve` shuts down gracefully on Ctrl-C and on SIGTERM: it stops the HTTP server,
  then closes any open login windows and deletes their profiles.
- Web only; the MCP `crawl` tool has no equivalent. Some identity providers (Google among
  them) may refuse to sign in from a browser that is being remote-controlled; use a cookie or
  a bearer token for those.

## Notes

- Jobs live in memory for the life of the `gocrawl serve` process; restarting it drops
  in-flight/unsaved jobs (saved ones are still in the store and show up in history).
- The web API builds each crawl's config from `config.Default()` plus the request body — like
  the MCP tool, it does not read a YAML config file or `GOCRAWL_*` environment variables. The
  UI's own defaults live in `~/.gocrawl/gui.json` and are applied by the form, not the server. A
  start request whose options can't compile (a bad regex, a bearer token next to Basic Auth, a
  missing header profile) is a `400`.
- Secrets stay out of errors and storage. Error messages from the settings and header-profile
  routes never contain a secret value, and `gui.json` never stores `bearer_token`,
  `basic_auth`, `cookie`, inline `headers`, or credentials embedded in a URL. Header profiles are
  the exception by design: they are the place for secrets, and `GET /api/header-profiles/{name}`
  returns a body as-is.
- The Settings view edits all of this through these routes.
- Redirect-check results live in memory only, like unsaved crawls.

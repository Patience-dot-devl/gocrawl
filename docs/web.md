---
id: "web"
title: "Web UI"
scope: "Running `gocrawl serve`, the embedded browser UI, the REST API it's built on, and building the frontend from source."
topic: "reference"
status: "current"
superseded_by: []
updated: 2026-10-13
depends_on: ["mcp.md", "output.md"]
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
  application/json` (`415` otherwise). Together these stop any other web page you have open
  from starting or cancelling crawls.
- At most 4 crawls run at once; a fifth `POST /api/crawls` returns `429` until one finishes
  or is cancelled.

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
Redirects, and Settings are placeholders until those features land. The working views, all
against the API below:

- **New crawl** — a form for the seed URL and every crawl option the interactive menu
  exposes: depth, page cap, concurrency, rate limit, max duration, render mode,
  robots.txt/subdomain/external-link scope, analyzer selection, specialized checks, security
  audit, and an option to ignore UTM tagging issues on external links, and an "Advanced"
  section for include/exclude regexes, User-Agent (or a rotating pool), proxy (or a rotating
  pool), HTTP Basic Auth, and a session-cookie field (a checkbox reveals a paste box) for sites
  gated by an app-level cookie, e.g. a Shopify storefront password page, and a custom-headers
  field (inline `Name: value` lines and/or a stored header profile name, e.g. for a Shopify
  crawler access signature). (A few CLI-only flags
  with no menu equivalent, like `--strip-query` and `--adaptive-delay`, aren't in the form
  either.) Starts a job and jumps to its report.
- **Report** — polls the running job (with a Stop crawl button while it runs), then shows a partial-coverage banner when the crawl
  didn't reach the whole site, summary counts by severity/analyzer/page-status, an issues
  table (filterable by severity, analyzer, and free-text search) with each issue's
  what/impact/fix explanation and raw `data` expandable inline, any notes advisories, and
  export links.
- **Crawls** — every crawl saved to the store (`save: true` on start), reopen-able as a
  report. Its old address, `#/history`, still opens it.

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
| `GET /api/crawls/{id}` | One job's status, its live `progress` (see [Live progress](#live-progress)) and, once finished, its full `Report` (see [Output reference](output.md)). `?tape_from=N` returns only the tape characters from index `N`; a negative or non-numeric `N` is a `400`. Falls back to the store for an id that isn't a live job; saved crawls have no `progress`, but their report's `coverage.tape` / `tape_off` hold the same tape. An id that is neither (for example a crawl that was running when the server restarted) is a `404`. |
| `POST /api/crawls/{id}/cancel` | Cancel a running crawl. Like Ctrl-C on the CLI, this doesn't error the crawl — it stops early and still returns whatever was fetched as a partial report (`Report.Coverage.Interrupted`), tracked here as job status `canceled`. |
| `GET /api/crawls/{id}/export?format=json\|csv\|html` | Download the finished report in any of the three formats gocrawl already writes, via `report.For(format)`. |
| `POST /api/sitemap-preview` | Find a site's sitemap without crawling. JSON body `{url, include, exclude, allow_subdomains}`; returns `{found, total, sources, via_robots}`, where `total` counts only sitemap URLs inside that scope. Respects robots.txt and sends the crawl's default User-Agent. The preview sends no credentials (no Basic Auth, cookie or custom headers), so a site gated behind them may report no sitemap. |
| `GET /api/settings` | The UI's saved settings: `theme` (`dark`, `light` or `system`), `crawl_defaults` (a crawl request body that pre-fills the New crawl form), and the read-only `store_dir`. |
| `PUT /api/settings` | Save `theme` and `crawl_defaults` (JSON). Written atomically to `~/.gocrawl/gui.json` with mode `0600`. `bearer_token`, `basic_auth`, `cookie`, inline `headers`, and user:password in the URL or proxy URLs are stripped before saving. `400` for an unknown theme or an unusable default. |
| `GET /api/header-profiles` | `{"profiles": [...]}`: the names in `~/.gocrawl/headers/`. |
| `GET /api/header-profiles/{name}` | `{"name", "body"}` of one profile; the body is returned as-is, header values included. `404` when it doesn't exist, `400` for an invalid name. |
| `PUT /api/header-profiles/{name}` | Create or replace a profile from JSON `{"body": "Name: value\n…"}`, validated like `--header-file`. `204`; `400` with the parser's message when invalid. |
| `DELETE /api/header-profiles/{name}` | Delete a profile. `204`, or `404`. |
| `GET /api/login-sessions/available` | `{"available": bool, "reason"?: string}`: whether a Chromium-class browser (and, on Linux, a display) is there to log in with. |
| `POST /api/login-sessions` | JSON `{"url"}`: open a visible browser window on that URL in a throwaway profile. `202` with the session; `400` for a URL that isn't http(s) with a host; `429` when two login windows are already waiting; `503` "no Chromium-class browser found" (also when the browser fails to start or the server is shutting down). |
| `GET /api/login-sessions/{id}` | `{"id", "state", "host", "cookie_names", "cookie_count", "expires_at"?}`. `state` is `waiting`, `captured`, `closed` (window closed before capture) or `expired` (after one hour). `expires_at` is the earliest cookie expiry, omitted when all are session cookies. Never cookie values. `404` for an unknown id. |
| `POST /api/login-sessions/{id}/capture` | Read the browser's cookies (HttpOnly included), keep those a browser would send to the session's host, close the window and delete its profile. `404` unknown id; `409` when the window is gone, already captured, expired, or has no cookie for the host yet (the window then stays open). |
| `DELETE /api/login-sessions/{id}` | Close the window and forget the session and its cookies. `204`; `404` for an unknown id. |

Job status is one of `running`, `done`, `error`, `canceled`.

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
| `tape`, `tape_off`, `tape_from` | One character per recorded page, in fetch order. `tape` holds the status class (`2`, `3`, `4`, `5`, or `x` for a failed fetch); `tape_off` holds `1` for a page the sitemap doesn't list (and `0` otherwise). `tape_from` is the index the returned slice starts at. |

Before every crawl, CLI and MCP included, gocrawl reads the site's sitemap once: robots.txt
`Sitemap:` lines, then `/sitemap.xml` and `/sitemap_index.xml`, following indexes. Only URLs
inside the crawl scope count toward `sitemap_total`. The `sitemap` analyzer reuses the same
set instead of fetching it again.

## Logging in with a browser

For sites behind a login form (staging, member areas, a Shopify password page), the UI can
borrow a real browser session: gocrawl opens a normal Chrome, Chromium or Edge window on the
site in a fresh, temporary profile, you log in there (SSO, two-factor and CAPTCHAs work), and
gocrawl reads that window's cookies when you say you're done. It keeps only the cookies a
browser would send to the site's host, closes the window, and deletes the profile. The login
UI lands with the GUI views; the API above is what it will call.

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
capture, or when it is deleted.

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
  `(?i)(log|sign)-?out` to the crawl form (it lands with the GUI views), so the crawl doesn't
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
- The Settings view that edits all of this lands with the GUI settings view; until then these
  routes are the interface.

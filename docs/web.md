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
runs from this machine with whatever proxy, Basic Auth, or cookie the request supplies, so
only do this behind something that authenticates (a reverse proxy, VPN, or SSH tunnel).

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

Three views, all against the API below:

- **New crawl** — a form for the seed URL and every crawl option the interactive menu
  exposes: depth, page cap, concurrency, rate limit, max duration, render mode,
  robots.txt/subdomain/external-link scope, analyzer selection, specialized checks, security
  audit, and an option to ignore UTM tagging issues on external links, and an "Advanced"
  section for include/exclude regexes, User-Agent (or a rotating pool), proxy (or a rotating
  pool), HTTP Basic Auth, and a session-cookie field (a checkbox reveals a paste box) for sites
  gated by an app-level cookie, e.g. a Shopify storefront password page. (A few CLI-only flags
  with no menu equivalent, like `--strip-query` and `--adaptive-delay`, aren't in the form
  either.) Starts a job and jumps to its report.
- **Report** — polls the running job, then shows a partial-coverage banner when the crawl
  didn't reach the whole site, summary counts by severity/analyzer/page-status, an issues
  table (filterable by severity, analyzer, and free-text search) with each issue's
  what/impact/fix explanation and raw `data` expandable inline, any notes advisories, and
  export links.
- **History** — every crawl saved to the store (`save: true` on start), reopen-able as a
  report.

There is no live per-page progress in this version — `crawler.Engine` has no progress hook
today, only a start/finish boundary, so the UI polls for `running` → `done` rather than
streaming page-by-page.

## API

All endpoints are under `/api`; everything else falls through to the embedded frontend.

| Method & path | Description |
| --- | --- |
| `GET /api/analyzers` | List available analyzers (same as `list_analyzers` over MCP). |
| `GET /api/explanations` | Every issue code's what/impact/fix explanation, keyed by code — the same text baked into the HTML report, used by the live report view. |
| `POST /api/crawls` | Start a crawl. Body is JSON (`Content-Type: application/json` required) with the same field set as the [MCP `crawl` tool](mcp.md#crawl) (`url` required) plus `save: bool` to persist the report to the store when it finishes. Returns `202` with the job immediately; the crawl runs in the background. `429` when the running-crawl cap is reached. |
| `GET /api/crawls` | List every job (in-memory, this process) merged with the store's saved history, newest first. |
| `GET /api/crawls/{id}` | One job's status and, once finished, its full `Report` (see [Output reference](output.md)). Falls back to the store for an id that isn't a live job. |
| `POST /api/crawls/{id}/cancel` | Cancel a running crawl. Like Ctrl-C on the CLI, this doesn't error the crawl — it stops early and still returns whatever was fetched as a partial report (`Report.Coverage.Interrupted`), tracked here as job status `canceled`. |
| `GET /api/crawls/{id}/export?format=json\|csv\|html` | Download the finished report in any of the three formats gocrawl already writes, via `report.For(format)`. |

Job status is one of `running`, `done`, `error`, `canceled`.

## Notes

- Jobs live in memory for the life of the `gocrawl serve` process; restarting it drops
  in-flight/unsaved jobs (saved ones are still in the store and show up in history).
- The web API builds its config from `config.Default()` plus the fields in the request body —
  like the MCP tool, it does not read a YAML config file or `GOCRAWL_*` environment variables.

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
| `--addr` | `:8080` | Address to listen on. |
| `--store-dir` | `~/.gocrawl/crawls` | Crawl history root, same store used by `--save`, `gocrawl history`, and `gocrawl compare`. |

Open `http://localhost:8080` in a browser. Ctrl-C shuts the server down gracefully (in-flight
HTTP requests are given up to 10s to finish; crawl jobs already running detach from the
request that started them, so they aren't tied to a shutdown).

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

- **New crawl** — a form for the seed URL and the common crawl options (depth, page cap,
  concurrency, render mode, analyzer selection, specialized checks), which starts a job and
  jumps to its report.
- **Report** — polls the running job, then shows the summary counts, an issues table
  (filterable by severity/analyzer), any coverage/notes advisories, and export links.
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
| `POST /api/crawls` | Start a crawl. Body is the same field set as the [MCP `crawl` tool](mcp.md#crawl) (`url` required) plus `save: bool` to persist the report to the store when it finishes. Returns `202` with the job immediately; the crawl runs in the background. |
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

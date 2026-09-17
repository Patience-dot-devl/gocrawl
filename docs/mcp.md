# MCP server guide

gocrawl can run as a [Model Context Protocol](https://modelcontextprotocol.io) server over
stdio, so agentic tools such as Conductor or Claude Code can drive crawls directly and reason
over the results. Source: [`internal/mcpserver/server.go`](../internal/mcpserver/server.go),
wired up in [`cmd/gocrawl/mcp.go`](../cmd/gocrawl/mcp.go).

## Starting the server

```sh
gocrawl mcp
```

This starts a Model Context Protocol server on **stdin/stdout** (the
`mcp.StdioTransport`). It speaks the protocol — it is not meant to be used interactively in a
terminal; an MCP client launches and talks to it.

## Registering with a client

**Claude Code (CLI):**

```sh
claude mcp add gocrawl -- gocrawl mcp
```

**Conductor / Claude Code config block** (`mcpServers`):

```json
{
  "mcpServers": {
    "gocrawl": { "command": "gocrawl", "args": ["mcp"] }
  }
}
```

The `command` must be on the client's `PATH` (e.g. installed via
`go install github.com/Patience-dot-devl/gocrawl/cmd/gocrawl@latest`), or use an absolute path
to the built binary.

## Tools

The server exposes two tools.

### `crawl`

Crawl a website, run the analyzer pipeline, and return a structured report.

**Input** ([`CrawlInput`](../internal/mcpserver/server.go)) — only `url` is required; omitted
fields fall back to the built-in defaults:

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `url` | string | *(required)* | Seed URL (e.g. `https://example.com`). A bare host gets `https://` prepended. |
| `depth` | int | `0` | Maximum link hops from the seed (`0` = unlimited; the crawl is bounded by `max_pages`). |
| `max_pages` | int | `500` | Hard cap on pages crawled — the primary bound on crawl size. |
| `concurrency` | int | `4` | Parallel fetch workers. |
| `rate` | float | `0` | Max requests per second (`0` = unlimited). |
| `max_duration` | string | *(unlimited)* | Wall-clock budget for the whole crawl as a Go duration string, e.g. `90m`; on expiry the crawl stops early and still returns a partial report. |
| `render` | string | `raw` | `raw` (HTTP fetch) or `headless` (chromedp — renders JS and captures Core Web Vitals; needs a Chromium-class browser on PATH). |
| `analyzers` | string[] | *(all)* | Subset of analyzer names to run; empty runs all. |
| `specialized` | bool | `false` | Enable the opt-in specialized checks: AEO answer-lead, GEO quotable-density, WordPress security probes, Shopify `/products.json` probe. |
| `security_audit` | bool | `false` | Enable the opt-in [security audit](analyzers.md#security-audit-opt-in): TLS/certificate, cookie, and response-header checks. |
| `ignore_external_tagging` | bool | `true` | Suppress the `utm` analyzer's tagging-quality warnings for links leaving the domain. |
| `respect_robots` | bool | `true` | Obey `robots.txt`. |
| `subdomains` | bool | `false` | Follow links to subdomains of the seed host. |
| `follow_external` | bool | `false` | Follow links that leave the seed host entirely. |
| `include` | string[] | *(none)* | Only crawl URLs matching at least one regex. |
| `exclude` | string[] | *(none)* | Skip URLs matching any regex. |
| `user_agent` | string | *(built-in)* | `User-Agent` header sent on every request. |
| `user_agents` | string[] | *(none)* | Pool of `User-Agent` strings to rotate across; supersedes `user_agent`. |
| `user_agent_rotation` | string | `round-robin` | `off`, `round-robin`, or `random`. |
| `proxy` | string | *(none)* | Proxy URL (`http(s)://` or `socks5://`; supports `user:pass@host`). |
| `proxies` | string[] | *(none)* | Pool of proxy URLs to rotate across. |
| `proxy_rotation` | string | `round-robin` | `off`, `round-robin`, `random`, or `sticky-host`. |
| `basic_auth` | string | *(none)* | HTTP Basic Auth credentials as `user:pass`, for sites gated by server-level Basic Auth (see [Configuration](configuration.md#http-basic-auth)). |
| `cookie` | string | *(none)* | Raw `Cookie` header sent on every request, for sites gated by an app-level session cookie rather than server-level Basic Auth, e.g. a Shopify storefront password page (see [Configuration](configuration.md#cookie-gated-sites-eg-a-shopify-storefront-password)). |

**Output** ([`CrawlOutput`](../internal/mcpserver/server.go)): `{ "report": <Report> }`, where
`<Report>` is the full crawl report documented in the [Output reference](output.md).

Example call arguments:

```json
{ "url": "https://example.com", "depth": 2, "analyzers": ["seo", "links"] }
```

Trimmed example response:

```json
{
  "report": {
    "seed": "https://example.com",
    "started_at": "2026-06-09T10:00:00Z",
    "finished_at": "2026-06-09T10:00:07Z",
    "pages_crawled": 12,
    "summary": {
      "by_severity": { "error": 1, "warning": 3, "info": 20 },
      "by_analyzer": { "seo": 8, "links": 16 },
      "pages_by_status": { "200": 11, "404": 1 }
    },
    "issues": [
      {
        "analyzer": "seo",
        "url": "https://example.com/about",
        "severity": "warning",
        "code": "missing-h1",
        "message": "Page has no <h1>"
      }
    ]
  }
}
```

### `list_analyzers`

Lists the available analyzers and their descriptions. Takes no input.

**Output** ([`ListAnalyzersOutput`](../internal/mcpserver/server.go)):

```json
{
  "analyzers": [
    { "name": "seo", "description": "On-page technical SEO: title, meta, canonical, headings, lang, viewport, charset, social tags" },
    { "name": "redirects", "description": "HTTP status codes, redirect chains/loops, slow responses, mixed content" }
  ]
}
```

See the [Analyzer reference](analyzers.md) for the full list and every issue code.

## Notes

- The MCP `crawl` tool builds its configuration from [`config.Default()`](../internal/config/config.go)
  and applies only the fields you pass — it does **not** read a YAML config file or
  `GOCRAWL_*` environment variables. Pass options as tool arguments.
- `render: "headless"` is accepted but currently falls back to raw fetching (see the
  [roadmap](roadmap.md)).

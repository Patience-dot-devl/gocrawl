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
| `depth` | int | `2` | Maximum link depth from the seed (`0` = seed only). |
| `max_pages` | int | `500` | Hard cap on pages crawled. |
| `concurrency` | int | `4` | Parallel fetch workers. |
| `render` | string | `raw` | `raw` or `headless` (headless is stubbed). |
| `analyzers` | string[] | *(all)* | Subset of analyzer names to run; empty runs all. |
| `respect_robots` | bool | `true` | Obey `robots.txt`. |
| `subdomains` | bool | `false` | Follow links to subdomains of the seed host. |
| `include` | string[] | *(none)* | Only crawl URLs matching at least one regex. |
| `exclude` | string[] | *(none)* | Skip URLs matching any regex. |
| `user_agent` | string | *(built-in)* | `User-Agent` header sent on every request. |
| `user_agents` | string[] | *(none)* | Pool of `User-Agent` strings to rotate across; supersedes `user_agent`. |
| `user_agent_rotation` | string | `round-robin` | `off`, `round-robin`, or `random`. |
| `proxy` | string | *(none)* | Proxy URL (`http(s)://` or `socks5://`; supports `user:pass@host`). |
| `proxies` | string[] | *(none)* | Pool of proxy URLs to rotate across. |
| `proxy_rotation` | string | `round-robin` | `off`, `round-robin`, `random`, or `sticky-host`. |

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

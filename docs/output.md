# Output / report reference

A crawl produces a single **report**, written as JSON (default) or CSV. The report is the
same object whether it's written to a file (`--out`), stdout, or returned by the
[MCP `crawl` tool](mcp.md#crawl). Source:
[`internal/report/report.go`](../internal/report/report.go).

Choose the destination and format with `--out`/`-o` and `--format`/`-f` (or the `output.path`
/ `output.format` config keys). With no `--out`, the report goes to stdout; a short
human-readable summary is always printed to **stderr** so it doesn't pollute piped output.

```sh
gocrawl crawl https://example.com --format json --out report.json
gocrawl crawl https://example.com --format csv  --out issues.csv
```

## JSON schema

The top-level [`Report`](../internal/report/report.go):

| Field | Type | Description |
| --- | --- | --- |
| `seed` | string | The (normalized) seed URL the crawl started from. |
| `started_at` | string | Crawl start time, RFC 3339 (e.g. `2026-06-09T10:00:00Z`). |
| `finished_at` | string | Crawl finish time, RFC 3339. |
| `pages_crawled` | int | Number of pages fetched. |
| `summary` | object | Aggregated counts — see [Summary](#summary). |
| `issues` | array | Every [Issue](#issue) emitted by the analyzers. |

### Summary

[`Summary`](../internal/report/report.go) aggregates the run:

| Field | Type | Description |
| --- | --- | --- |
| `by_severity` | object | Issue counts keyed by severity (`error`, `warning`, `info`). |
| `by_analyzer` | object | Issue counts keyed by analyzer name. |
| `pages_by_status` | object | Page counts keyed by HTTP status code (as a string, e.g. `"200"`). |

### Issue

Each entry in `issues` is an [`Issue`](../internal/analyze/analyze.go):

| Field | Type | Description |
| --- | --- | --- |
| `analyzer` | string | Name of the analyzer that produced it (`seo`, `links`, …). |
| `url` | string | The URL the finding applies to. For the `robots` analyzer's per-host findings this is `host <hostname>`; for crawl-wide notices it may be the seed. |
| `severity` | string | `error`, `warning`, or `info`. |
| `code` | string | Stable machine-readable code (e.g. `missing-title`). See the [Analyzer reference](analyzers.md). |
| `message` | string | Human-readable description. |
| `data` | object | Optional analyzer-specific details; omitted when empty. |

### Example JSON report

```json
{
  "seed": "https://example.com",
  "started_at": "2026-06-09T10:00:00Z",
  "finished_at": "2026-06-09T10:00:07Z",
  "pages_crawled": 12,
  "summary": {
    "by_severity": { "error": 1, "warning": 3, "info": 20 },
    "by_analyzer": { "seo": 8, "redirects": 2, "links": 16 },
    "pages_by_status": { "200": 11, "404": 1 }
  },
  "issues": [
    {
      "analyzer": "redirects",
      "url": "https://example.com/old",
      "severity": "info",
      "code": "redirect",
      "message": "Page redirects",
      "data": { "to": "https://example.com/new", "status": 301 }
    },
    {
      "analyzer": "seo",
      "url": "https://example.com/about",
      "severity": "warning",
      "code": "long-title",
      "message": "Title may be truncated in SERPs",
      "data": { "length": 73, "title": "About our company — …" }
    }
  ]
}
```

## CSV schema

The CSV form ([`CSVReporter`](../internal/report/report.go)) writes **one row per issue**,
with a header row. Columns, in order:

| Column | Notes |
| --- | --- |
| `analyzer` | Analyzer name. |
| `severity` | `error` / `warning` / `info`. |
| `code` | Issue code. |
| `url` | Target URL. |
| `message` | Human-readable description. |
| `data` | The issue's `data` map, JSON-encoded into a single cell (empty if none). |

The summary block is **not** included in CSV output — only the issue rows. Use JSON if you
need the aggregated counts.

```csv
analyzer,severity,code,url,message,data
redirects,info,redirect,https://example.com/old,Page redirects,"{""status"":301,""to"":""https://example.com/new""}"
seo,warning,long-title,https://example.com/about,Title may be truncated in SERPs,"{""length"":73,""title"":""About our company — …""}"
```

## What is (and isn't) in the report

The report contains the **issues** analyzers produced, not the raw crawled page bodies. Each
crawled [`Page`](../internal/crawler/types.go) holds the response body, parsed HTML document,
and headers while analyzers run, but those fields are intentionally **not serialized** — only
findings reach the report. If you need raw page data, it lives in the in-memory
`crawler.Result`, consumed by the analyzer pipeline.

## See also

- [Analyzer reference](analyzers.md) — every `code`, its severity, and the `data` it attaches.
- [Configuration reference](configuration.md) — output format and destination options.

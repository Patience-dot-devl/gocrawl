# Output / report reference

A crawl produces a single **report**, written as JSON (default), CSV, or HTML. The report is
the same object whether it's written to a file (`--out`), stdout, or returned by the
[MCP `crawl` tool](mcp.md#crawl). Source:
[`internal/report/report.go`](../internal/report/report.go).

Choose the destination and format with `--out`/`-o` and `--format`/`-f` (or the `output.path`
/ `output.format` config keys). With no `--out`, the report goes to stdout; a short
human-readable summary is always printed to **stderr** so it doesn't pollute piped output.

```sh
gocrawl crawl https://example.com --format json --out report.json
gocrawl crawl https://example.com --format csv  --out issues.csv
gocrawl crawl https://example.com --format html --out report.html
```

## Site map

Every crawl also produces a **site map** — the crawled site as a tree, annotated with the
issues found on each page. It's built from the same crawl result and issues that feed the
report (nothing is re-fetched). Source: [`internal/sitemapgen`](../internal/sitemapgen).

### In the HTML report (a tab)

The HTML report (`--format html`) carries the site map as a second tab next to **Issues &
summary** — no extra flags. It's drawn as a **node-link diagram** (an org-chart-style tree of
cards connected by lines), so the structure of the site is visible at a glance. It doubles as
an audit navigator:

- Each page is a **card** whose top border is colored by that page's health — green (clean),
  blue (info), amber (warning), or red (error); synthetic path nodes that were never crawled
  directly are neutral.
- A card shows the path segment (a link to the page), the HTTP status pill, and severity-count
  badges. **Clicking a card** pops up the full list of findings on that page (code, analyzer,
  message), sorted worst-first; **clicking the label** opens the page itself.
- Branches collapse and expand with the `−`/`+` toggle on each card (deep branches start
  collapsed to keep the chart compact); **Expand all** / **Collapse all** controls sit above
  it. A collapsed card shows a muted roll-up (`∑ N`) of how many issues hide beneath it.
- Findings that don't belong to a single crawled page (e.g. the `robots` analyzer's per-host
  checks) are listed in a **Site-wide issues** section at the bottom.

### As a standalone `sitemap.xml`

To also emit a standard, machine-readable [sitemaps.org](https://www.sitemaps.org/protocol.html)
`sitemap.xml` (a `urlset` you can host or submit to search engines), pass `--sitemap`
(config key `output.sitemap_path`). This is independent of `--format`, so you can produce it
alongside any report:

```sh
gocrawl crawl https://example.com --depth 3 --out report.html --format html --sitemap sitemap.xml
```

Both the tab and `sitemap.xml` include only successful (HTTP 200) HTML pages on the seed host
(or its subdomains); redirects, errors, assets, and off-site pages are excluded, matching what
belongs in a sitemap. URLs use the canonical (post-redirect) location, and `<lastmod>` is
taken from each page's `Last-Modified` response header when present (crawl time is never used
as a fallback, since it reflects when the page was fetched, not when its content changed). A
note is added to the report when `sitemap.xml` is written.

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
| `notes` | array | Advisories about the run itself (e.g. analyzers skipped because `strip_query` is on, or [partial coverage](#coverage)), not page findings. Omitted when empty. |
| `coverage` | object | Whether the crawl reached every in-scope URL it found — see [Coverage](#coverage). Omitted only when re-rendering an older report that predates the field. |
| `site_map` | object | The crawled site as a tree (the same structure the HTML Site map tab draws), with issues attached to each page node. Serialized so a JSON report is a complete artifact that [`gocrawl render`](#re-rendering-a-saved-report) can turn back into HTML without recrawling. Omitted when empty. |

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
| `code` | string | Stable machine-readable code (e.g. `seo-missing-title`). See the [Analyzer reference](analyzers.md). |
| `message` | string | Human-readable description. |
| `data` | object | Optional analyzer-specific details; omitted when empty. |

### Coverage

`coverage` reports whether the crawl actually fetched every in-scope URL it discovered, so
that **"0 broken links" can't be mistaken for "no broken links"** when the crawl simply didn't
reach them. Broken-link detection only flags a dead link whose target the crawler fetched, so
coverage is the context you need to trust that result.

| Field | Type | Description |
| --- | --- | --- |
| `complete` | bool | `true` when no in-scope, robots-allowed URL was left un-fetched because of a limit. |
| `discovered_not_crawled` | int | Count of distinct in-scope URLs discovered but never fetched. Omitted when 0. |
| `page_limit_reached` | bool | The `--max-pages` budget cut the crawl short. Omitted when false. |
| `depth_limit_reached` | bool | The `--depth` limit cut the crawl short. Omitted when false. |
| `max_pages` / `max_depth` | int | The limits in effect (`0` = unlimited). |

When `complete` is false, a `notes` entry spells out which limit was hit and how to lift it,
and the **HTML report shows a prominent banner** at the top of the page. By default the crawl
is bounded by `--max-pages` (500) with unlimited depth, so it walks the whole site up to that
budget rather than stopping at a shallow depth — re-crawl with a higher `--max-pages` (or `0`)
when you see the partial-coverage banner.

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
  ],
  "coverage": { "complete": true, "max_pages": 500 }
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

## HTML report

The HTML form ([`HTMLReporter`](../internal/report/report.go)) renders the same `Report`
struct as a **self-contained HTML page** — inline CSS and a small inline script for the
interactive toolbar, with no external assets or network calls. That makes it easy to share
as an artifact, attach to a ticket, or host as a static file.

The page has three blocks:

- **Header** — seed URL, start/finish timestamps, pages crawled.
- **Summary cards** — issue counts by severity, by analyzer, and pages by HTTP status (the
  same fields as the JSON [`summary`](#summary)).
- **Issues table** — one row per issue with a severity badge, analyzer, code, clickable URL,
  and message. When an issue has a `data` map, a `<details>` toggle reveals it as
  pretty-printed JSON. A toolbar above the table lets you full-text search, toggle
  severities, filter by analyzer, and **multi-select issue codes** — uncheck codes in the
  *Codes* dropdown to hide them (e.g. hide `seo-meta-noindex` / `seo-x-robots-noindex` noise when
  auditing a staging site that is deliberately noindexed). You can also mark rows
  resolved / non-issue, add comments, and **Save to file** to bake that review state back
  into a shareable copy.

Untrusted strings from crawled pages (URLs, titles, analyzer messages) are escaped through
Go's `html/template`, so HTML in a crawled page can't break the report layout. Open the file
directly in any browser:

```sh
gocrawl crawl https://example.com --format html --out report.html
open report.html
```

## Re-rendering a saved report

`gocrawl render <report.json>` reads a JSON report you saved earlier and writes it out in
another format — **without recrawling**. This is the fast way to regenerate an HTML report
after upgrading gocrawl (e.g. to pick up template improvements), or to produce CSV/HTML from a
JSON you already have:

```sh
gocrawl crawl  https://example.com --format json --out report.json   # crawl once
gocrawl render report.json --out report.html                         # re-render any time, no recrawl
gocrawl render report.json --format csv --out issues.csv             # or to CSV
gocrawl render report.json --out report.html --sitemap sitemap.xml   # also (re)emit sitemap.xml
```

Flags mirror `crawl`'s output flags: `--out`/`-o` (default stdout), `--format`/`-f` (default
`html`), and `--sitemap`. Because the JSON report carries the [`site_map`](#json-schema) tree,
the HTML **Site map** tab is reproduced too. Only the page bodies aren't stored (see below),
so re-rendering can't run new analyzers — for that, recrawl.

## What is (and isn't) in the report

The report contains the **issues** analyzers produced, not the raw crawled page bodies. Each
crawled [`Page`](../internal/crawler/types.go) holds the response body, parsed HTML document,
and headers while analyzers run, but those fields are intentionally **not serialized** — only
findings reach the report. If you need raw page data, it lives in the in-memory
`crawler.Result`, consumed by the analyzer pipeline.

## See also

- [Analyzer reference](analyzers.md) — every `code`, its severity, and the `data` it attaches.
- [Configuration reference](configuration.md) — output format and destination options.

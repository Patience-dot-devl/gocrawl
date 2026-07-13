# `check-redirects`: verify a redirect-rule export against a live site

Status: approved (design)
Date: 2026-07-13

## Problem

Site migrations/rebrands accumulate large redirect-rule exports (e.g. from HubSpot's URL
Redirects tool). Over time these rules rot: destinations get renamed or removed, source pages
come back to life, or a rule was never actually wired up correctly. Nobody currently checks a
CSV export of these rules against the real, live site to find rules that are broken.

The immediate case: a ~1,557-row HubSpot export for `endeavour.nl` and its subdomains, in the
column schema:

```
"Original URL","Redirect to","Redirect type","Redirect style","Priority",
"Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"
```

## Goal

A new `gocrawl check-redirects` CLI subcommand that reads a redirect-rule CSV in this schema,
checks each in-scope rule against the live site, and writes back the same CSV with appended
columns describing what's actually happening — so it can be filtered/sorted in a spreadsheet.

Out of scope: checking rules whose target domain isn't the configured main domain or one of its
subdomains (those are treated as external/skipped, not fetched, to avoid an uncontrolled
cross-domain crawl). Rows using HubSpot's regex/named-group syntax with `{placeholder}`
substitution in the target are also skipped (can't be resolved to one concrete URL without a
sample value) — reported separately as `dynamic-pattern`, not silently dropped.

## Architecture

### New package: `internal/redirectcheck`

- `parse.go` — reads the CSV into a `Rule` struct (one per row: `Original`, `Target`, `Type`,
  `Style`, `Priority`, `MatchQueryStrings`, `IgnoreTrailingSlash`, `IgnoreProtocol`,
  `DisableIfPageExists`, `Note`), preserving row order and raw values for pass-through output.
  Fails fast with a clear error if the CSV doesn't match the expected column schema.
- `scope.go` — classifies each rule:
  - `in-scope`: target resolves to `--domain` or a subdomain of it.
  - `external`: target resolves to a different registrable domain. Skipped — not fetched.
  - `dynamic-pattern`: Original URL uses regex syntax (e.g. `(?P<name>...)`), or Target contains
    a `{placeholder}`. Skipped — not fetched.
  Relative-path targets/originals (`/en/insights/...`) resolve against `--domain`.
- `check.go` — for in-scope, non-dynamic rules: fetches Original URL and Target URL via
  `crawler.HTTPFetcher` (constructed directly — no crawl engine, no link graph, just the
  existing single-URL `Fetcher.Fetch`), applies verdict logic (below), and cross-references
  sitemap membership.
- `sitemap.go` — fetches the sitemap via the shared helper described below, using
  `--sitemap-url` if given, else `https://<domain>/sitemap.xml` then `/sitemap_index.xml`.
- `report.go` — writes the annotated CSV, preserving original row order and all 1,557 rows
  (including skipped ones).

### Refactor: `internal/analyze/sitemap`

Extract the existing private recursive fetch/parse loop (currently inline in `Analyzer.Analyze`,
handling both `sitemapindex` and `urlset` XML, recursing into index children) into an exported
function:

```go
func Fetch(ctx context.Context, fetcher crawler.Fetcher, candidates map[string]bool) (urls map[string]bool, err error)
```

`Analyzer.Analyze` keeps its robots.txt-candidate-gathering and crawl-coverage-diffing logic, but
delegates the fetch-and-parse loop to this shared function. `redirectcheck` calls the same
function with its own candidate set. This avoids duplicating sitemap-index/urlset XML parsing
in two places.

### New CLI file: `cmd/gocrawl/checkredirects.go`

Thin Cobra command (same pattern as `crawl.go`), wiring flags to `redirectcheck.Run(...)`.

## CLI

```
gocrawl check-redirects --input redirects.csv --domain endeavour.nl [--output results.csv] [--sitemap-url URL]
                         [--concurrency N] [--rate-limit N] [--timeout D] [--user-agent STR]
```

- `--input` (required): path to the redirect-rule CSV.
- `--domain` (required): main domain; subdomains auto-included as in-scope, everything else
  marked `external`.
- `--output` (optional): defaults to stdout.
- `--sitemap-url` (optional): only needed if both default sitemap locations fail to
  fetch/parse — in that case the command errors out immediately with a message asking for this
  flag (no interactive prompt; this is a batch CLI).
- Politeness flags (`--concurrency`, `--rate-limit`, `--timeout`, `--user-agent`) mirror `crawl`'s
  existing flags/defaults, so the check doesn't hammer the live site.
- `-h`/`--help` — standard Cobra help, automatic, lists all flags above.

## Data flow & verdict logic

Per in-scope, non-dynamic rule:

1. Resolve Original URL and Target URL (relative paths resolve against `--domain`; absolute
   URLs with an in-scope host are used as given).
2. Fetch Original URL via `HTTPFetcher.Fetch` → `StatusCode`, `Redirects` chain, `FinalURL`.
3. Fetch Target URL (independently) → final `StatusCode` after following any further redirects.
4. Look up both (normalized) against the fetched sitemap URL set.
5. Verdict, evaluated in this order (a row can trigger more than one; `verdict` is the worst
   severity seen, `notes` lists every triggered reason):
   - Original URL fetch errors, or returns 4xx/5xx via some path other than the expected
     redirect → **error** ("source broken").
   - Original URL returns 200 (no redirect fired):
     - `DisableIfPageExists = TRUE` → **warning** ("redirect suppressed by config — source page
       still exists, consider removing rule").
     - `DisableIfPageExists = FALSE` → **error** ("rule requires unconditional redirect but
       source is still live").
   - Original URL redirects, but final destination doesn't match `Target` (normalized per
     `Ignore trailing slash`/`Ignore protocol`) → **error** ("redirects to unexpected
     destination").
   - Target URL's final status is 4xx/5xx → **error** ("redirect target is broken/404").
   - Original URL found in the sitemap → **warning** ("stale sitemap entry — source URL still
     listed as canonical").
   - Target URL not found in the sitemap → **warning** ("target not confirmed in sitemap").
   - Otherwise → **ok**.

## Output CSV schema

Original 10 columns are echoed verbatim, with these appended:

| Column | Meaning |
|---|---|
| `scope` | `in-scope` / `external` / `dynamic-pattern` |
| `source_status` | HTTP status of Original URL (final, after any redirects it took) |
| `source_final_url` | Where Original URL actually ended up |
| `source_matches_target` | `TRUE`/`FALSE` — does `source_final_url` match `Redirect to`? |
| `target_status` | HTTP status of `Redirect to`, after following any further redirects |
| `original_in_sitemap` | `TRUE`/`FALSE` |
| `target_in_sitemap` | `TRUE`/`FALSE` |
| `verdict` | `ok` / `warning` / `error` / `skipped-external` / `skipped-dynamic` |
| `notes` | semicolon-separated list of triggered reasons |

Rows with `scope = external` or `dynamic-pattern` get `verdict = skipped-*` and blank
status/sitemap columns (not fetched) — every input row is present in the output, in order.

## Error handling

- Malformed CSV (wrong/missing columns) → fail immediately, naming the expected schema.
- Per-row fetch errors (timeout, DNS failure) are not fatal to the run — recorded as `error`
  verdicts with the failure message in `notes`; the command continues through remaining rows.
- Sitemap discovery failing entirely (no default location works, no `--sitemap-url` given) →
  the command errors out before processing any rows, since sitemap columns would otherwise be
  silently wrong for every row.

## Testing

- `parse_test.go`: CSV parsing against a fixture matching the real schema, including a
  regex-pattern row and a placeholder-target row.
- `scope_test.go`: domain/subdomain/external classification, relative-path resolution, and
  dynamic-pattern detection.
- `check_test.go`: verdict logic against a fake `crawler.Fetcher`, table-driven — clean redirect,
  wrong destination, 404 target, live source with `DisableIfPageExists` true/false, stale
  sitemap entry, target missing from sitemap, fetch error.
- `internal/analyze/sitemap`: existing tests continue to pass against the refactored exported
  `Fetch` helper; add a case exercising it directly.
- `report_test.go`: output CSV has the right appended columns, skipped rows blank/labeled
  correctly, all input rows preserved in order.
- No live-site fixture crawling in tests — everything driven through a fake `Fetcher`,
  consistent with how other analyzers are tested (`testdata/` fixtures + fake fetchers).

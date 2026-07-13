# Verifying a redirect-rule export (`check-redirects`)

`gocrawl check-redirects` verifies a redirect-rule CSV export — the format produced by
HubSpot's URL Redirects tool — against a live site. It answers: are these redirects actually
working, and do they point at live pages rather than 404s?

## Usage

```sh
gocrawl check-redirects --input redirects.csv --domain example.com --output results.csv
```

| Flag | Meaning |
| --- | --- |
| `--input` (required) | Path to the redirect-rule CSV. |
| `--domain` (required) | Main domain. Subdomains are automatically in-scope; every other domain is skipped as external. |
| `--output` | Output CSV path (default: stdout). |
| `--sitemap-url` | Sitemap URL to use if `/sitemap.xml` and `/sitemap_index.xml` aren't reachable. |
| `--concurrency` | Parallel fetch workers (default 4). |
| `--rate` | Max requests per second (default unlimited). |
| `--timeout` | Per-request timeout (default 15s). |
| `--user-agent` | User-Agent header to send. |

## Input schema

The CSV must have exactly these columns, in order (HubSpot's export format):

```
"Original URL","Redirect to","Redirect type","Redirect style","Priority",
"Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"
```

Rows are classified before fetching:

- **in-scope** — the target resolves to `--domain` or a subdomain of it. Checked.
- **external** — the target resolves to a different domain. Skipped (not fetched), to avoid
  an uncontrolled cross-domain crawl.
- **dynamic-pattern** — the rule uses HubSpot's regex/named-group syntax or a `{placeholder}`
  substitution in the target, so it can't be resolved to one concrete URL. Skipped.

## Output

The input CSV is echoed back with these columns appended:

| Column | Meaning |
| --- | --- |
| `scope` | `in-scope` / `external` / `dynamic-pattern` |
| `source_status` | HTTP status of the Original URL, after following any redirects |
| `source_final_url` | Where the Original URL actually ended up |
| `source_matches_target` | Does the source's final URL match `Redirect to`? |
| `target_status` | HTTP status of `Redirect to`, after following any further redirects |
| `original_in_sitemap` | Is the Original URL still listed in the sitemap? |
| `target_in_sitemap` | Is the target listed in the sitemap? |
| `verdict` | `ok` / `warning` / `error` / `skipped-external` / `skipped-dynamic` |
| `notes` | Every finding that contributed to the verdict, semicolon-separated |

A row is flagged `error` when: the source can't be fetched, the source no longer redirects
and `Disable if page exists` is `FALSE`, the source redirects somewhere other than the
expected target, or the target itself returns a 4xx/5xx. A row is flagged `warning` when the
source no longer redirects but `Disable if page exists` is `TRUE` (HubSpot's own suppression —
worth a look, not necessarily broken), the source is still listed in the sitemap as canonical
(stale entry), or the target isn't found in the sitemap.

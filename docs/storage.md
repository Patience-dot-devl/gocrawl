# Crawl storage & comparison

gocrawl can save crawls to disk and diff them over time, so you can answer "did my fixes
land?" and "what regressed since last week?" without eyeballing two reports side by side.

This builds on the fact that a JSON report is already a complete, self-contained artifact (see
[Output / report](output.md)). The store is a thin filesystem layer over it; comparison is a
pure diff of two reports.

## The store

`gocrawl crawl --save` writes the crawl to a store directory in addition to any `--out`
report. The default root is `~/.gocrawl/crawls` (or `./.gocrawl/crawls` if the home directory
can't be determined). Override it with `--store-dir`, the `store.dir` config key, or
`GOCRAWL_STORE_DIR`.

Layout:

```
<root>/<host>/<timestamp>.json
e.g. ~/.gocrawl/crawls/example.com/20260630T120000Z.json
```

Each crawl has an **ID** of the form `<host>/<timestamp>` — sortable (newest is
lexicographically greatest within a host) and readable. The timestamp comes from the report's
start time, in UTC.

```sh
gocrawl crawl https://example.com --save
# → Saved crawl example.com/20260630T120000Z
```

## Listing crawls — `gocrawl history`

```sh
gocrawl history                 # every saved crawl, newest first
gocrawl history example.com     # only that host
gocrawl history --format json   # machine-readable
```

```
ID                            FINISHED                   PAGES  ERR  WARN  INFO
example.com/20260630T120000Z  2026-06-30T12:00:00Z       128   3    14    52
example.com/20260623T120000Z  2026-06-23T12:00:00Z       126   5    18    49
```

## Comparing crawls — `gocrawl compare`

```sh
gocrawl compare <base> <current>
```

`<base>` is the earlier crawl and `<current>` the later one. Each argument is a **crawl
reference**, resolved in this order:

1. a path to a JSON report file on disk (e.g. `before.json`);
2. `latest` — the newest crawl in the store;
3. a crawl ID `<host>/<timestamp>`;
4. a bare host name (e.g. `example.com`) — that site's newest saved crawl.

Examples:

```sh
gocrawl compare before.json after.json
gocrawl compare example.com/20260623T120000Z latest
gocrawl compare example.com latest
```

### What it reports

Issue identity is `(analyzer, code, url)`. Against that, the diff buckets every finding:

- **new** — present in the current crawl but not the base (regressions, or freshly found);
- **resolved** — present in the base but gone now (fixed, or no longer reached);
- **persisting** — present in both.

It also reports **page** deltas (URLs crawled now vs. before, from the site-map entries) and
**summary** deltas (issue-count change per severity and per analyzer).

Text output (default):

```
Comparing crawls of https://example.com/
  base:    2026-06-23T12:00:00Z  (126 pages)
  current: 2026-06-30T12:00:00Z  (128 pages)

Issues: 1 error new, 3 warning resolved, 60 unchanged

NEW issues (regressions) (1):
  [error] seo/missing-title  https://example.com/new-page

RESOLVED issues (fixed) (3):
  [warning] images/img-missing-alt  https://example.com/about
  ...
```

`--format json` emits the full structured diff. `--out <file>` writes to a file instead of
stdout.

### CI gate — `--fail-on-new`

```sh
gocrawl compare main-baseline.json pr-crawl.json --fail-on-new
```

Exits non-zero when the current crawl introduces any new issue, so a pull request that adds
broken links or strips a title fails the build.

## See also

- [Configuration](configuration.md) — the `store.dir` key and `--store-dir` flag.
- [Output / report](output.md) — the report format the store persists and compare reads.

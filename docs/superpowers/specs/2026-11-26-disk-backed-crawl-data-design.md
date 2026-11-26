---
id: "2026-11-26-disk-backed-crawl-data-design"
title: "Streaming analysis and a crawl database"
scope: "Proposal to analyze each page as it is fetched and drop its HTML, then keep crawl data in a per-crawl SQLite database; the base for the per-URL table, agent-sized MCP tools and resumable crawls"
topic: "spec"
status: "current"
superseded_by: []
updated: 2026-11-26
depends_on: []
sources: []
---
# Streaming analysis and a crawl database

Date: 2026-11-26
Status: proposed, waiting for review

## Problem

gocrawl holds every crawled page in memory, HTML and parsed DOM included, until every
analyzer has finished. Memory grows linearly with the crawl and is only released at the very
end (`Result.ReleaseBodies` in `runner.Run`).

Measured on a 20,000-page local test site with realistic page sizes and per-page variation
(November 2026, Go 1.27.4):

| What is held | Live heap | Per page |
| --- | --- | --- |
| The crawl result with bodies and DOMs, as today during analysis | 574 MB | 28.7 KB |
| The same result after `ReleaseBodies` (metadata, headers, 260k links) | 78 MB | 3.9 KB |

**86% of what a crawl keeps in memory is HTML and DOM.** Process memory is higher still,
about 100 KB per page in the November load test (2 GB at 20,000 pages), because of garbage
collector headroom and the issue list. At that rate a 100,000-page site needs around 10 GB.

Three roadmap items are blocked on the same missing piece, a place to keep crawl data that is
not one big in-memory `Result`:

- **The per-URL table** (one row per URL: indexability, canonical, title length, word count,
  inlinks, depth, response time). Its values exist during analysis and are thrown away.
- **Agent-sized MCP tools.** The `crawl` tool returns every issue in one blocking response;
  agents need to start a crawl, poll it, and page through results by code or URL.
- **Resumable crawls.** The frontier and the visited set live only in memory, so a stopped
  crawl cannot continue.

## Key finding: no analyzer reads another page's HTML

All 25 analyzers were checked. Every cross-page check reads only small facts about other
pages, never their body or DOM:

| Analyzer | What it reads about other pages |
| --- | --- |
| `links`, `pagination`, `amp` | status, final URL and redirects of link targets (`Result.Page`) |
| `links` (inbound counts), `structured` (opportunity ranking) | the internal link graph (`analyze.Inbound`) |
| `duplicates` | title, meta description and content hash of every page |
| `content` | every page's word count, for the crawl-wide average |
| `hreflang` | every page's hreflang annotations, for reciprocity |
| `structured`, `shopify` | per-page schema types and URL templates, rolled up per template |
| `security` (audit) | headers and TLS of one representative page per host |
| `sitemap` | the set of crawled URLs and their status |

Everything else is strictly per page. So a page's HTML can be analyzed **as soon as it is
fetched** and dropped right after, as long as each analyzer keeps the few facts its cross-page
pass needs. That is the core of this design, and it needs no new dependency.

## Goals

1. Bound memory by what the cross-page passes need, not by page size: drop each page's body
   and DOM right after its per-page analysis.
2. Keep a crawl's data (pages, links, facts, issues) in a per-crawl database, so that the
   per-URL table, the MCP query tools and resumable crawls can all read and write it.
3. Keep the analyzer seam: checks still never fetch the crawl and never see the engine.
4. Keep the report contract: the same issue codes, fields and JSON output, and `compare`
   working across the change.
5. Ship in stages, each useful on its own and each behind passing tests.

## Non-goals

- Storing page bodies. The database holds metadata, links, facts and issues, not HTML. An
  opt-in body archive can come later.
- A server-side database or multi-user storage. One file per crawl, local to the machine.
- Changing what any analyzer checks.

## Design

### Stage 1: streaming analysis (no new dependency)

**A two-phase analyzer interface,** added next to the existing one:

```go
// PageAnalyzer analyzes each page while its body and DOM are in memory.
type PageAnalyzer interface {
    Analyzer
    // AnalyzePage runs once per page, right after it is fetched, concurrently with other
    // pages. It returns the page's issues and a small value of facts the analyzer's Finish
    // reads later; it must not keep a reference to the page.
    AnalyzePage(ctx context.Context, p *crawler.Page) (issues []Issue, facts any)
    // Finish runs once after the crawl. It sees every page's metadata, links and the facts
    // AnalyzePage returned for it, but no bodies or DOMs.
    Finish(ctx context.Context, result *crawler.Result, facts Facts) []Issue
}
```

- `Facts` maps a page to the value its `AnalyzePage` returned for this analyzer. Analyzers stay
  stateless, so one registry can serve concurrent crawls as it does today, and the facts can
  later be written to the database unchanged.
- A per-page-only analyzer (most of them) returns nil facts and has an empty `Finish`.
- **Where it runs:** the engine calls the per-page phase from the crawl worker right after the
  fetch, through a hook the runner installs (`Options.OnPage`), so the engine still knows
  nothing about specific checks. The worker then drops `Body`, `RawBody` and `Doc`.
  Concurrency stays bounded by the worker pool.
- **Facts the engine records for everyone:** the page title (the site-map tree reads it today
  from `Doc`), word count and content hash move into a small per-page record, so `sitemapgen`
  and `report.Build` no longer need the DOM either.
- **Migration:** analyzers move one at a time. Until every *selected* analyzer implements
  `PageAnalyzer`, the runner keeps bodies as today and runs the old analyzers after the crawl.
  So a half-migrated tree works, just without the memory gain. Per-page-only analyzers move
  first; then `duplicates`, `content`, `hreflang`, `structured`, `shopify`, `security` and
  `sitemap` with their facts.
- **Output order:** per-page issues arrive in fetch order, not registry order. The runner sorts
  the final issue list by registry order, then by the page's position in the crawl, which is
  today's order. The golden-report test checks this byte for byte.
- **Expected result:** about 3.9 KB of heap per page plus issues, instead of 28.7 KB: a 20,000-
  page crawl in roughly 80 MB of live heap instead of 574 MB. Memory is still linear, but
  around seven times lower.
- **Side benefit:** issues exist while the crawl runs, so the web UI could show them live.

### Stage 2: a per-crawl SQLite database

Stage 1 leaves page records, links, facts and issues in memory. Stage 2 moves them into one
SQLite file per crawl, written as the crawl runs.

**Storage engine: SQLite through `modernc.org/sqlite`,** a pure-Go port. It needs no cgo, so
the binary stays static and cross-compiles as today. It was measured against the alternatives:

| Option | Binary size added | Queries | Verdict |
| --- | --- | --- | --- |
| `modernc.org/sqlite` | +4.6 MB (19.1 MB to about 23.7 MB, stripped) | SQL: filter, sort, page, join | **Recommended** |
| `go.etcd.io/bbolt` | +0.1 MB | key/value only; every filter or sort needs a hand-built index | Too little for the per-URL table and MCP queries |
| JSON Lines + an index file | none | hand-written scans | Same problem, more code to own |
| SQLite through cgo | small | SQL | Breaks the static, cross-compiled release builds |

The per-URL table and the MCP tools both need "filter by these columns, sort by that one,
give me page 3", which is what SQL does. A 24% larger binary is the cost.

**Schema** (one file per crawl):

| Table | Holds |
| --- | --- |
| `crawl` | seed, options, start and end time, coverage, notes, gocrawl version, schema version |
| `pages` | one row per fetched URL: requested and final URL, status, depth, content type, response time, fetched at, title, meta description, canonical, indexable, word count, content hash, inlinks (computed at the end) |
| `redirects` | the hops of each page's redirect chain |
| `links` | source page, target URL, anchor, rel, nofollow, external |
| `facts` | page, analyzer, encoded facts value |
| `issues` | analyzer, code, severity, URL, message, data (JSON), and the page it belongs to |
| `frontier` | URLs queued but not fetched, with depth and referrer; plus the visited set |

- **Writes** are batched per transaction from a single writer goroutine, so crawl workers never
  wait on disk. WAL mode lets readers (the web UI, MCP queries) read during the crawl.
- **Location:** a crawl saved with `--save` gets `<store>/<host>/<timestamp>.db` next to its JSON
  report. An unsaved crawl uses a temporary file that is deleted when it ends.
- **`crawler.Result` becomes a view over the database:** `Pages` iteration, `Page(url)` lookups
  and link iteration read from SQLite through accessors. Analyzers already reach pages through
  `analyze.EachPage`, `Result.Page` and `Result.ResolveHref`; stage 1 moves the remaining direct
  `range result.Pages` loops onto accessors, so this switch does not touch analyzers again.
- **The JSON report stays.** It is the output contract, and `compare`, `render`, `history` and
  older stored crawls all read it. `report.Build` streams from the database instead of from
  memory. `history` can read the `crawl` table instead of parsing every stored JSON report.
- **Expected result:** memory bounded by batch size and caches rather than by crawl size. The
  database grows by roughly the link count (about 30 MB for the 20,000-page test site; to be
  measured in stage 2).

### Stage 3: features on the database

- **Per-URL table (roadmap #7):** a query over `pages`, exposed as `--format pages-csv`, a
  `pages` array in the JSON report behind an option, and a sortable, paged table in the web UI.
- **Agent-sized MCP tools (roadmap #8):** `start_crawl` (returns a crawl ID at once),
  `crawl_status`, `query_issues` (filter by code, severity, analyzer or URL prefix, with
  paging), `get_page` (one row of the per-URL table, with its issues) and `compare`. The current
  `crawl` tool stays for compatibility but returns the summary and the first N issues, not all
  of them.
- **Resumable crawls:** `gocrawl crawl --resume <crawl id>` reopens the database, restores the
  frontier and the visited set, and continues. Pages already analyzed keep their issues and
  facts, and `Finish` runs once at the end as usual.

## Testing

- Every stage keeps `make check` green, including the golden-report test, which locks the JSON
  report byte for byte.
- An equivalence test crawls the same fixture sites with the old path and the new path and
  requires identical reports, for every analyzer as it migrates.
- The 20,000-page load test from the November review is rerun after each stage, recording live
  heap and process memory per page, so the expected results above get measured numbers.
- Stage 2 adds tests that a crawl killed mid-run leaves a readable database, and that
  `--resume` finishes it with the same report as an uninterrupted crawl.

## Risks

- **Migration effort.** 25 analyzers move to the two-phase interface. Each is small, but the
  facts for `structured` and `shopify` take care. The adapter keeps unmigrated analyzers
  working, so nothing has to move in one go.
- **Concurrency.** The per-page phase runs concurrently. Analyzers are stateless today, so this
  should hold, but `go test -race` on the equivalence tests has to prove it.
- **Report order and bytes.** Streaming changes the order issues are produced in; the final
  sort restores today's order. The golden test catches any drift.
- **Binary size.** +4.6 MB for SQLite. Stage 1 alone adds nothing, so if the size is not
  acceptable, stage 1's memory gain still stands and stage 2 can pick bbolt with hand-built
  indexes instead.
- **Disk use.** A large crawl writes a database of tens of megabytes. Unsaved crawls delete it
  at the end.

## Decisions wanted

1. **SQLite through `modernc.org/sqlite` (+4.6 MB)** as the crawl database, or bbolt (+0.1 MB)
   with hand-built indexes. Recommended: SQLite.
2. **Keep the JSON report** as the stored and exported format next to the database.
   Recommended: yes; it is the contract `compare` and `render` read.
3. **The two-phase `PageAnalyzer` interface** as the new way to write a check, documented in
   `AGENTS.md` once stage 1 lands. Recommended: yes, with the adapter for unmigrated analyzers.
4. **Order of work:** stage 1, then stage 2, then the per-URL table, the MCP tools and resume
   on top. Recommended as listed: stage 1 pays off alone, and each later stage builds on the
   one before.

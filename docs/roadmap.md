# Feature roadmap

Where gocrawl is today and where it's headed. This is a direction document, not a dated
release plan — the project is an early, working vertical slice, so items are grouped by status
and theme rather than committed to timelines.

**Status legend:** ✅ shipped &nbsp;·&nbsp; 🚧 in progress / stubbed &nbsp;·&nbsp; 📋 planned

---

## ✅ Shipped (v1)

The current baseline. Everything here works today.

- **Concurrent crawl engine** — raw-HTML fetching with configurable depth, page cap,
  worker concurrency, and per-request timeout. ([`internal/crawler`](../internal/crawler))
- **Scope & politeness controls** — `robots.txt` compliance, rate limiting,
  include/exclude URL regexes, subdomain and external-link toggles, custom User-Agent.
- **Redirect capture** — full redirect chains recorded per page, with loop detection.
- **Twenty-one analyzers** — the SEO/technical set `seo`, `redirects`, `links`, `robots`,
  `sitemap`, `structured`, `perf`, the content & technical breadth set `images`, `urls`,
  `security`, `pagination`, `hreflang`, `amp`, `duplicates`, `content`, the CMS-specific
  `wordpress`, the SEA set `utm`, `tracking`, `landing`, and the AI-search set `aeo`, `geo`.
  See the [Analyzer reference](analyzers.md).
- **Screaming Frog parity — tier 1** — image alt/dimension checks (`images`), URL hygiene
  (`urls`), security headers and insecure forms (`security`), `rel=next/prev` pagination
  (`pagination`), hreflang validity and reciprocity (`hreflang`), AMP markup (`amp`), exact
  duplicate content/titles/descriptions (`duplicates`), and thin-content detection
  (`content`) — plus extensions to existing analyzers: `X-Robots-Tag`/meta-refresh directives
  (`seo`), schema.org required-field validation (`structured`), and inbound link counts
  (`links`). All added on the analyzer seam with no engine changes.
- **Headless rendering (chromedp)** — `--render headless` (or `render: headless` in YAML)
  renders pages in a real Chromium tab via [chromedp][chromedp], capturing the post-JS DOM
  for downstream analyzers. It also captures the raw pre-JS HTML alongside the rendered DOM so
  the [`geo`](analyzers.md#geo--generative-engine-optimization-ai-search) analyzer can flag
  JavaScript-dependent content that non-executing AI crawlers miss. Requires a Chromium-class
  browser on PATH; falls back to a raw HTTP fetch per page on rendering error so a single
  broken page doesn't stall the crawl.
- **Lab-mode Core Web Vitals** — the [`perf`](analyzers.md#perf--core-web-vitals) analyzer
  reports real LCP, FCP, CLS, TBT, and TTFB from the headless renderer against Google's
  thresholds. INP is field-only and not directly measurable in synthetic crawls; gocrawl
  follows the Lighthouse convention and reports TBT as a lab-mode responsiveness proxy.

[chromedp]: https://github.com/chromedp/chromedp
- **SEA analyzers** — UTM-parameter auditing (`utm`), tracking-pixel / GTM / GA4 / Meta-Pixel
  detection (`tracking`), and landing-page relevance (`landing`). Each was added as a new
  `analyze.Analyzer` with **no changes to the crawl engine** — the worked example for the
  analyzer architecture (see [Architecture](architecture.md#adding-an-analyzer)).
- **JSON, CSV & HTML reports** — JSON (default) and CSV for tooling, plus a self-contained
  HTML page (inline CSS, no JS, no external assets) for sharing as an artifact. See the
  [Output reference](output.md).
- **MCP server** — drive crawls from agentic tools via the `crawl` and `list_analyzers`
  tools. See the [MCP guide](mcp.md).
- **Layered configuration** — defaults → YAML → env → flags, with `gocrawl init` to
  scaffold a config. See the [Configuration reference](configuration.md).
- **Persistent crawl storage** — `gocrawl crawl --save` writes a crawl to an on-disk store
  (`~/.gocrawl/crawls` by default), addressable by a `<host>/<timestamp>` ID. `gocrawl
  history` lists saved crawls. ([`internal/store`](../internal/store))
- **Crawl comparison** — `gocrawl compare <base> <current>` diffs two crawls (by stored ID,
  `latest`, a bare host, or a report file path) into new / resolved / persisting issues plus
  page and summary deltas, as text or JSON. `--fail-on-new` makes it a CI gate.
  ([`internal/diff`](../internal/diff))

## 🚧 In progress / stubbed

No tracked items at the moment.

## 📋 Planned — hardening & correctness (July 2026 code review)

Findings from a full codebase review (July 2026). Build, vet, tests, and the race detector
all pass; the items below are verified bugs and robustness gaps, grouped by theme and
ordered roughly by impact within each group.

### Security & credential handling

- **Strip userinfo from seed URLs** — credentials embedded in a seed
  (`https://user:pass@host`) currently survive `canonicalURL`, propagate into every resolved
  page/issue URL, and leak into JSON/CSV/HTML reports, the crawl store, and `gocrawl history`.
  Move them into the Basic Auth options at seed intake (CLI, MCP, and interactive mode) and
  strip them from the URL. This path also bypasses the headless-mode Basic Auth rejection.
- **CSV formula-injection guard** — crawled page content (titles etc.) flows into CSV report
  cells unescaped; prefix cells starting with `=`, `+`, `-`, `@` so audit CSVs are safe to
  open in Excel/Sheets.
- **Scope-check redirect targets** — the fetcher follows redirects to any host, bypassing
  crawl scope, include/exclude rules, and robots.txt (SSRF surface when running as an MCP
  server). Re-check scope and robots per redirect hop.
- **Harden the crawl store against hostile paths** — `url.Hostname()` can return `..`, and
  store IDs are joined unvalidated, so saves/reads can escape the store root. Sanitize hosts
  and verify resolved paths stay under the root.

### Crawl-engine correctness

- **Treat robots.txt 5xx as disallow-all** — a 5xx (or network error) on `/robots.txt`
  currently means "allow everything"; RFC 9309 requires the opposite. 4xx → allow stays
  correct. Add direct tests for `internal/crawler/robots.go` (currently untested).
- **Honor `<base href>` when resolving links** — relative links on base-tag sites resolve
  against the wrong base today, producing phantom 404s and missed pages.
- **Allow Basic Auth on same-host http→https upgrades** — the scheme-equality check that
  closed the downgrade leak also blocks the safe upgrade direction, so an http seed that
  301s to https gets every page 401'd.
- **Surface truncated bodies** — mid-body read errors and the `MaxBodyBytes` cap are silent,
  so analyzers report "missing title/H1" for pages that were cut off. Record the error or a
  `Truncated` flag.
- **Mark redirect final URLs as visited** — `/old` → `/new` plus a direct link to `/new`
  fetches the page twice and duplicates every per-page issue for it.
- **Cap honored `Retry-After`** — a `503` with `Retry-After: 86400` currently stalls the
  crawl for up to a day with no output; cap it (~5 min) and log when the cap is hit.
- **Decode non-UTF-8 pages** — bodies are parsed as UTF-8 regardless of charset, degrading
  text-based analyzers on ISO-8859-1 / Shift-JIS sites
  (`golang.org/x/net/html/charset`).

### Analyzer correctness

- **Count runes, not bytes, in title/description length checks** (`seo`) — non-ASCII sites
  get systematic `seo-long-title` false positives today.
- **Detect real redirect loops** (`redirects`) — a genuine loop exits the fetcher as a
  generic "too many redirects" fetch error, so the `http-redirect-loop` code is unreachable;
  run loop detection over the recorded chain when the fetch errored.
- **Add the missing `datalayer-*` explanations** — all 18 codes emitted by the `datalayer`
  analyzer lack entries in `internal/report/explanations.go`; add them, plus a contract test
  asserting every emitted code is explained.
- **Accept valid BCP-47 hreflang tags** (`hreflang`) — `es-419`, `zh-Hant`, lowercase
  regions, and 3-letter language codes are all falsely flagged; validate with
  `golang.org/x/text/language` instead of the hand-rolled regex.
- **Resolve relative hrefs and ignore trailing-slash redirects in `amp`/`pagination`** —
  both diverge from the `links` analyzer; factor a shared "is this target broken?" helper
  on `crawler.Result`.
- **Restrict mixed-content checks to loading `<link>` rels** (`redirects`) — RSS/hreflang
  alternates over http are false positives; `iframe`/`video`/`source` are missed.
- **Handle gzip and >5 MB sitemaps** (`sitemap`) — `.xml.gz` and large sitemaps are falsely
  reported as `sitemap-invalid`; also distinguish fetch failures from a genuinely missing
  sitemap.

### CLI & robustness

- **Graceful SIGINT with a partial report** — Ctrl-C on a long crawl currently loses
  everything; the engine is context-aware and the coverage banner can label partial results.
- **Register all config keys for env-var resolution** — viper only resolves env vars for
  keys with registered defaults, so e.g. `GOCRAWL_CRAWL_BASIC_AUTH` is silently ignored.
- **Atomic report/store/sitemap writes** — write to a temp file and rename, so a failed
  write can't truncate yesterday's good artifact.
- **Refresh the `gocrawl init` template** — it still calls headless rendering "stubbed",
  omits the `html` format and the `botwall`/`datalayer` analyzers, and mislabels
  `max_depth: 0` as "seed only" (it means unlimited).
- **Let `gocrawl render` accept store IDs** — `gocrawl history` says its IDs work with
  `render`, but `render` only reads file paths.
- **Bound memory on large crawls** — every page retains its body and parsed DOM for the
  whole run; add an option to drop them post-analysis, and replace goroutine-per-URL with a
  worker pool so `--max-pages 0` doesn't grow goroutines with the frontier.

## 📋 Planned — engine & data model

Checks and capabilities that need post-crawl graph passes or richer crawl data before an
analyzer can consume them.

- **Internal Link Score** — PageRank-style score computed over the crawled link graph
  (post-crawl graph pass feeding a new analyzer).
- **Orphan pages** — URLs known from sitemaps (and, later, GA/GSC) but never reached by the
  crawl. Partially served today by the [`sitemap`](analyzers.md) coverage check.
- **Near-duplicate / semantic similarity** — cluster semantically similar or off-topic pages
  via embeddings. A chance to lean on the LLM tooling already in the stack rather than copy
  [Screaming Frog][sf]'s implementation.
- **Custom extraction** — user-defined XPath / CSS-selector / regex extraction rules surfaced
  in the report.
- **Custom source search** — match arbitrary patterns against raw and rendered HTML.
- **XML Sitemap generation** — emit a sitemap from the crawled URL set (gocrawl currently only
  audits sitemaps).
- **Accessibility & mobile usability** — axe/WCAG checks and Lighthouse mobile-usability
  signals under headless rendering.

## 📋 Planned — platform

Broader capabilities beyond individual checks.

- **Scheduling & auto-export** — recurring crawls with automatic report export. Builds on the
  now-shipped crawl store and comparison.
- **API integrations** — Google Search Console (incl. URL Inspection), PageSpeed Insights /
  CrUX, and backlink data (Ahrefs/Majestic/Moz). Subsumes the earlier "export integrations"
  item.
- **Authenticated crawling** — forms- and cookie-based auth to reach gated areas.
- **Resumable crawls** — checkpoint and continue large crawls.

[sf]: https://www.screamingfrog.co.uk/seo-spider/

---

> Want to help, or have a check in mind? The analyzer interface is the extension point — see
> [CONTRIBUTING.md](../CONTRIBUTING.md). The README carries a short summary of this roadmap.

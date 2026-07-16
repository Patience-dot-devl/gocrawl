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

## ✅ Shipped — hardening & correctness (July 2026 code review)

A full codebase review (July 2026) of the crawl engine, all analyzers, and the CLI /
config / report / MCP surface produced 28 verified findings, since fixed across four PRs
([#43], [#44], [#45], [#47]):

- **Security & credential handling** — seed-URL credentials (`https://user:pass@host`) are
  now stripped at intake and routed into the Basic Auth path instead of leaking into
  reports, the store, and resolved URLs; CSV report cells are guarded against formula
  injection; redirect targets are re-checked against crawl scope and robots.txt (SSRF
  surface as an MCP server); the crawl store rejects hostile hosts/IDs that could escape
  its root.
- **Crawl-engine correctness** — robots.txt 5xx/network errors now disallow (RFC 9309)
  instead of allowing everything; `<base href>` is honored when resolving links; Basic Auth
  survives same-host http→https upgrades; truncated bodies are surfaced
  (`Page.Truncated` → `http-body-truncated`); redirect destinations are marked visited (no
  more double-fetch/double-report); `Retry-After` is capped at 5 minutes; non-UTF-8 pages
  are charset-decoded before parsing; a data race in the headless renderer was fixed.
- **Analyzer correctness** — title/description lengths count runes, not bytes; real
  redirect loops now reach `http-redirect-loop`; the missing `datalayer-*` and `wp-*`
  explanations were added plus a contract test asserting every emitted code is explained;
  hreflang tags are validated with `golang.org/x/text/language` (no more `es-419` /
  `zh-Hant` false positives); `amp`/`pagination` resolve relative hrefs and ignore
  trailing-slash redirects via the shared `Result.ResolveHref`; mixed-content checks filter
  `<link>` rels and cover `iframe`/`video`/`source` et al.; gzip and truncated sitemaps are
  handled.
- **CLI & robustness** — SIGINT produces a partial report instead of losing the crawl;
  `--max-duration` bounds wall-clock time the same way; all config keys resolve from env
  vars; file outputs are atomic (`internal/atomicfile`); the `gocrawl init` template was
  refreshed; `gocrawl render` accepts store IDs; a frontier worker pool bounds goroutines
  and gives true BFS; page bodies/DOMs are released after reporting to bound memory.

[#43]: https://github.com/Patience-dot-devl/gocrawl/pull/43
[#44]: https://github.com/Patience-dot-devl/gocrawl/pull/44
[#45]: https://github.com/Patience-dot-devl/gocrawl/pull/45
[#47]: https://github.com/Patience-dot-devl/gocrawl/pull/47

Remaining follow-ups from the review, still open:

- 📋 **Per-host politeness / `Crawl-delay`** — the frontier worker pool was built as the
  natural home for per-host rate limiting and robots.txt `Crawl-delay`, but neither is
  enforced yet.
- 📋 **Distinguish sitemap fetch failures from a missing sitemap** — a transport error on
  every sitemap candidate currently produces the same `sitemap-missing` warning as a
  genuine 404, so a network blip can falsely report "no sitemap".

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

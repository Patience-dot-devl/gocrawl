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

## 🚧 In progress / stubbed

No tracked items at the moment.

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

- **Persistent crawl storage** — save crawls to disk and reopen them. Prerequisite for crawl
  comparison and scheduling below.
- **Crawl comparison** — diff two crawls to track progress over time and map staging vs.
  production.
- **Scheduling & auto-export** — recurring crawls with automatic report export.
- **API integrations** — Google Search Console (incl. URL Inspection), PageSpeed Insights /
  CrUX, and backlink data (Ahrefs/Majestic/Moz). Subsumes the earlier "export integrations"
  item.
- **Authenticated crawling** — forms- and cookie-based auth to reach gated areas.
- **Resumable crawls** — checkpoint and continue large crawls.

[sf]: https://www.screamingfrog.co.uk/seo-spider/

---

> Want to help, or have a check in mind? The analyzer interface is the extension point — see
> [CONTRIBUTING.md](../CONTRIBUTING.md). The README carries a short summary of this roadmap.

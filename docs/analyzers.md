# Analyzer reference

An **analyzer** is a single, self-contained check. Each one consumes the crawl result and
emits zero or more [`Issue`](output.md#issue) values. An issue has a `severity`
(`error`, `warning`, or `info`), a stable `code`, a `message`, and an optional `data` map.

gocrawl ships twelve analyzers, run in this registration order
([`runner.BuildRegistry`](../internal/runner/runner.go)):
`seo`, `redirects`, `links`, `robots`, `sitemap`, `structured`, `perf`, the SEA
analyzers `utm`, `tracking`, `landing`, and the AI-search analyzers `aeo`, `geo`.

List them at any time:

```sh
gocrawl analyzers list
```

Select a subset with `--analyzers` or the `analyzers.enabled` / `analyzers.disabled` config
keys — see [Selecting analyzers](configuration.md#selecting-analyzers).

> **Severity is a classification, not a pass/fail.** Several analyzers emit `info` issues
> (e.g. `link-summary`, `sitemap-coverage`, `response-time`) that report findings rather than
> problems. Filter on `severity == "error"` / `"warning"` for actionable items.

---

## `seo` — On-page technical SEO

Source: [`internal/analyze/seo/seo.go`](../internal/analyze/seo/seo.go). Runs on every HTML
page that returned `200`.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `missing-title` | error | No `<title>` in `<head>` | — |
| `short-title` | warning | Title shorter than 10 chars | `length`, `title` |
| `long-title` | warning | Title longer than 60 chars (may truncate in SERPs) | `length`, `title` |
| `missing-meta-description` | warning | No `<meta name="description">` (or empty) | — |
| `short-meta-description` | info | Description shorter than 50 chars | `length` |
| `long-meta-description` | info | Description longer than 160 chars | `length` |
| `meta-noindex` | warning | `<meta name="robots">` contains `noindex` | `robots` |
| `meta-nofollow` | info | `<meta name="robots">` contains `nofollow` | `robots` |
| `multiple-canonical` | warning | More than one `<link rel="canonical">` | — |
| `missing-canonical` | info | No canonical link | — |
| `missing-h1` | warning | No `<h1>` element | — |
| `multiple-h1` | info | More than one `<h1>` | `count` |
| `missing-lang` | info | `<html>` has no `lang` attribute | — |
| `missing-viewport` | info | No `<meta name="viewport">` (mobile-friendliness) | — |
| `missing-charset` | info | No `<meta charset>` and no `content-type` http-equiv | — |
| `missing-opengraph` | info | No `<meta property="og:*">` tags | — |

**Thresholds:** title 10–60 chars, meta description 50–160 chars.

---

## `redirects` — HTTP status, redirects, slow responses, mixed content

Source: [`internal/analyze/httpx/httpx.go`](../internal/analyze/httpx/httpx.go). Runs on
every page. (The analyzer's internal package is `httpx`; its registered name is `redirects`.)

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `fetch-error` | error | The page failed to fetch | — |
| `server-error` | error | Status `>= 500` | `status` |
| `client-error` | error | Status `400`–`499` | `status` |
| `redirect-loop` | error | A URL repeats in the redirect chain | `chain` |
| `redirect-chain` | warning | More than one redirect before the final URL | `hops`, `chain` |
| `redirect` | info | A single redirect | `to`, `status` |
| `slow-response` | warning | Response slower than the threshold (default **2s**) | `duration_ms` |
| `mixed-content` | warning | HTTPS page loads `http://` subresources | `count`, `examples` |

**Threshold:** slow-response fires above 2 seconds. Mixed-content reports up to 5 example
URLs.

---

## `links` — Link analysis

Source: [`internal/analyze/links/links.go`](../internal/analyze/links/links.go). Cross-
references each page's outbound links against the crawled page set.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `broken-link` | error | Internal link points to a crawled page with status `>= 400` | `target`, `status`, `anchor` |
| `link-to-redirect` | warning | Internal link points to a crawled page that redirects | `target`, `final` |
| `empty-anchor` | info | The page has links with empty anchor text | `count` |
| `link-summary` | info | Per-page link counts (always emitted when a page has links) | `total`, `external`, `nofollow` |

> Broken-link / link-to-redirect detection only covers internal links to URLs that were
> **actually crawled**. Links outside the crawl scope (excluded, external, beyond max-depth,
> or past the page cap) are not status-checked.

---

## `robots` — robots.txt

Source: [`internal/analyze/robotscheck/robotscheck.go`](../internal/analyze/robotscheck/robotscheck.go).
Reports per host. Issue `url` is `host <hostname>` for the per-host findings.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `no-robots` | info | No `robots.txt` found for the host | `status` |
| `no-sitemap-declared` | info | `robots.txt` declares no `Sitemap:` | — |
| `sitemaps-declared` | info | `robots.txt` declares one or more sitemaps | `sitemaps` |
| `crawled-disallowed` | warning | A crawled URL is disallowed by `robots.txt` | — |

> `crawled-disallowed` can only occur when you crawled with `--respect-robots=false`; with
> the default `true`, disallowed URLs are never fetched.

---

## `sitemap` — sitemap.xml discovery and coverage

Source: [`internal/analyze/sitemap/sitemap.go`](../internal/analyze/sitemap/sitemap.go).
Looks for sitemaps declared in `robots.txt` plus the conventional `/sitemap.xml` and
`/sitemap_index.xml`, following sitemap-index files up to two levels deep.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `invalid-sitemap` | warning | A fetched sitemap parsed as neither a `urlset` nor an index | — |
| `no-sitemap` | warning | No sitemap found at any candidate location | — |
| `sitemap-coverage` | info | Cross-check of sitemap URLs vs. crawled pages | `sitemap_urls`, `crawled_pages`, `crawled_not_in_sitemap`, `in_sitemap_not_crawled` |

The coverage `data` lets you spot pages that are crawlable but missing from the sitemap
(`crawled_not_in_sitemap`) and sitemap entries that weren't reached in the crawl
(`in_sitemap_not_crawled`).

---

## `structured` — JSON-LD structured data

Source: [`internal/analyze/structured/structured.go`](../internal/analyze/structured/structured.go).
Runs on every HTML page that returned `200`. Extracts `<script type="application/ld+json">`
blocks and reports their schema.org `@type` values (descending into `@graph` and arrays).

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `invalid-jsonld` | warning | A JSON-LD block is not valid JSON | `error` |
| `no-structured-data` | info | The page has no JSON-LD blocks | — |
| `structured-data` | info | JSON-LD found; lists the de-duplicated `@type`s | `types` |

---

## `perf` — Core Web Vitals

Source: [`internal/analyze/perf/perf.go`](../internal/analyze/perf/perf.go).

Runs in two modes depending on how the crawl was fetched:

- **Headless mode (`--render headless`).** Reads lab-mode Core Web Vitals captured by the
  chromedp renderer (`PerformanceObserver` for LCP / FCP / CLS / long-task TBT, Navigation
  Timing for TTFB) and emits per-page findings against [Google's CWV thresholds][cwv].
- **Raw mode.** Falls back to a single `cwv-not-collected` notice and a per-page
  `response-time` proxy from the raw fetch's TTFB.

> **INP is field-only.** It requires real user interactions and cannot be measured in a
> synthetic crawl. gocrawl reports **TBT (Total Blocking Time)** as a lab-mode proxy for
> responsiveness, matching Lighthouse's behavior.

[cwv]: https://web.dev/articles/vitals

### Thresholds

| Metric | Good      | Needs improvement | Poor       |
| ------ | --------- | ----------------- | ---------- |
| LCP    | ≤ 2500 ms | ≤ 4000 ms         | > 4000 ms  |
| FCP    | ≤ 1800 ms | ≤ 3000 ms         | > 3000 ms  |
| CLS    | ≤ 0.1     | ≤ 0.25            | > 0.25     |
| TBT    | ≤ 200 ms  | ≤ 600 ms          | > 600 ms   |
| TTFB   | ≤ 800 ms  | ≤ 1800 ms         | > 1800 ms  |

### Issue codes

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `cwv-measured` | info | Per rendered `200` HTML page — all five metrics in one record | `lcp_ms`, `fcp_ms`, `cls`, `tbt_ms`, `ttfb_ms` |
| `lcp-needs-improvement` / `lcp-poor` | warning / error | LCP above the band | `value_ms`, `threshold_ms` |
| `fcp-needs-improvement` / `fcp-poor` | warning / error | FCP above the band | `value_ms`, `threshold_ms` |
| `cls-needs-improvement` / `cls-poor` | warning / error | CLS above the band | `value`, `threshold` |
| `tbt-needs-improvement` / `tbt-poor` | warning / error | TBT above the band | `value_ms`, `threshold_ms` |
| `ttfb-needs-improvement` / `ttfb-poor` | warning / error | TTFB above the band | `value_ms`, `threshold_ms` |
| `cwv-render-failed` | info | Headless rendering errored on a page; CWV unavailable for it | `note` |
| `cwv-not-collected` | info | Raw-mode fallback (once on the seed) — reminds to enable `--render headless` | — |
| `response-time` | info | Raw-mode per-page TTFB proxy from raw fetch duration | `duration_ms` |

---

## `utm` — UTM campaign-tag auditing (SEA)

Source: [`internal/analyze/utm/utm.go`](../internal/analyze/utm/utm.go). Audits each page's
**outbound links** for UTM tagging. Links with no UTM parameters are skipped (only counted in
the summary). The "required" trio is `utm_source`, `utm_medium`, `utm_campaign`. The issue
`url` is the page carrying the link; the link target is in `data.target`.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `utm-partial-tagging` | warning | A tagged link has some but not all of the required trio | `target`, `present`, `missing`, `anchor` |
| `utm-empty-value` | warning | A present UTM parameter has an empty value | `target`, `keys` |
| `utm-duplicate-param` | warning | A UTM parameter appears more than once | `target`, `keys` |
| `utm-inconsistent-casing` | info | A UTM key is not lowercase (analytics tools are case-sensitive) | `target`, `keys` |
| `utm-internal-tagged` | info | A UTM-tagged link points to the same site (starts a new analytics session) | `target` |
| `utm-summary` | info | Per-page rollup, emitted for every page that has links | `total_links`, `tagged_links`, `external_tagged`, `internal_tagged` |

> Auditing is link-based. Tagging is validated on the links a page points to, not on the
> landing page itself — that's the `landing` analyzer's job.

---

## `tracking` — marketing/analytics tag detection (SEA)

Source: [`internal/analyze/tracking/tracking.go`](../internal/analyze/tracking/tracking.go).
Runs on every HTML page that returned `200`. Scans `<script src>`, inline `<script>` bodies,
`<img>` pixels, and `<noscript>` contents for known tags: Google Tag Manager, GA4, Universal
Analytics, Google Ads, Meta (Facebook) Pixel, LinkedIn Insight, Microsoft/Bing UET, and
TikTok Pixel.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `tracking-tags` | info | One or more tags detected; lists each tag and its IDs | `tags` |
| `no-tracking-tags` | info | HTML page with no detectable tags | — |
| `duplicate-tracking-tag` | warning | The same tag family is installed with two or more distinct IDs (double-counting risk) | `tag`, `ids`, `count` |
| `mixed-ga-versions` | info | Both Universal Analytics and GA4 are present | `ua_ids`, `ga4_ids` |

> **Static detection.** Tags injected at runtime by a tag manager are only visible via their
> container (e.g. a `GTM-…` ID), so `no-tracking-tags` is informational rather than a warning.
> A pixel installed via both a `<script>` and its standard `<noscript>` fallback with the same
> ID counts as one install, not a duplicate.

---

## `landing` — landing-page relevance (SEA)

Source: [`internal/analyze/landing/landing.go`](../internal/analyze/landing/landing.go). A
crawled HTML `200` page is treated as a **landing page** when its own URL carries campaign UTM
parameters (`utm_term` / `utm_campaign` / `utm_content`), or when another crawled page links
to it with such parameters. Campaign keywords are derived entirely from the crawl's own link
data — no external campaign feed is needed.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `landing-keyword-mismatch` | warning | Campaign keyword coverage of the title/H1/H2 is below 20% | `campaign_terms`, `matched`, `missing`, `coverage` |
| `landing-keyword-weak` | info | Coverage is 20–50% | `campaign_terms`, `matched`, `missing`, `coverage` |
| `landing-keyword-aligned` | info | Coverage is 50% or higher | `campaign_terms`, `matched`, `missing`, `coverage` |
| `landing-noindex` | error | The paid landing page is marked `noindex` | `robots` |
| `landing-not-https` | warning | The landing page is not served over HTTPS | — |
| `landing-missing-title` | warning | The landing page has no `<title>` | — |
| `landing-missing-h1` | warning | The landing page has no `<h1>` | — |
| `landing-missing-description` | info | The landing page has no meta description | — |

> Issues fire only for pages identified as landing pages, so this analyzer deliberately
> re-checks a few `seo` signals (title, H1, description) at a stricter, ad-quality bar with
> distinct codes. Because external destinations are usually not crawled, coverage is best for
> internally-reachable and self-tagged landing pages.

---

## `aeo` — Answer Engine Optimization (AI search)

Source: [`internal/analyze/aeo/aeo.go`](../internal/analyze/aeo/aeo.go). A static, per-page
check that runs on every HTML `200` page. It scores how readily the page can be surfaced as a
direct answer in featured snippets, "People Also Ask", and voice results. A heading counts as
a **question** when it ends with `?` or opens with an interrogative (how, what, why, …).

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `aeo-answer-schema` | info | Page has `FAQPage`, `QAPage`, `HowTo`, or `Question` JSON-LD (a positive signal) | `types` |
| `aeo-faq-candidate` | warning | At least 3 question-style headings but no `FAQPage`/`QAPage` structured data | `questions` |
| `aeo-answer-too-long` | info | The text following a question heading exceeds 60 words (long for a snippet) | `question`, `answer_words` |
| `aeo-no-answer-lead` ⚙︎ | info | The `<h1>` is a question but the lead paragraph is missing or exceeds 60 words (the answer is buried, not led) | `title`, `lead_words` |
| `aeo-no-list-format` | info | A `<main>`/`<article>` region has ≥300 words but no list or table to extract | `words` |

> The answer following a question heading is measured as the heading's sibling content up to
> the next heading, so the snippet-length check works best on conventional article markup. The
> lead paragraph for `aeo-no-answer-lead` is the first `<p>` inside `<main>`/`<article>` (or the
> body when neither is present).
>
> ⚙︎ `aeo-no-answer-lead` is an **opt-in specialized check**, off by default. Turn it on with
> `--specialized` (or `analyzers.specialized: true` in YAML). See [the note below](#specialized-ai-search-checks).

---

## `geo` — Generative Engine Optimization (AI search)

Source: [`internal/analyze/geo/geo.go`](../internal/analyze/geo/geo.go). Assesses whether AI
answer engines (ChatGPT, Perplexity, Google AI Overviews, Gemini, Claude) can access the site
and trust its content. It combines a per-host robots.txt view, a per-site `/llms.txt` fetch
(using a raw fetcher, like `sitemap`), and per-page citability signals on every HTML `200`
page.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `geo-ai-crawler-blocked` | info | A host's `robots.txt` disallows one or more known AI crawlers at the site root | `blocked` |
| `geo-llms-txt` | info | The seed host serves an `/llms.txt` content map (a positive signal) | — |
| `geo-no-llms-txt` | info | No `/llms.txt` was found at the seed host root | — |
| `geo-missing-author` | info | An article-like page (`<article>` or `Article`/`BlogPosting`/… JSON-LD) has no author attribution | — |
| `geo-missing-date` | info | An article-like page has no published or modified date | — |
| `geo-no-main-landmark` | info | A page with ≥300 words of prose has no `<main>` or `<article>` landmark | `words` |
| `geo-js-dependent-content` | info | Under `--render headless`, the pre-JS HTML holds less than half the rendered prose (≥300 words), so non-executing AI crawlers miss most content | `rendered_words`, `raw_words` |
| `geo-low-quotable-density` ⚙︎ | info | A page with ≥300 words of prose has fewer than ~0.5 concrete data points (numbers, stats, dates) per 100 words | `words`, `data_points` |

> AI crawlers checked include `GPTBot`, `OAI-SearchBot`, `ChatGPT-User`, `ClaudeBot`,
> `anthropic-ai`, `PerplexityBot`, `Google-Extended`, `Applebot-Extended`, `CCBot`,
> `Bytespider`, and `meta-externalagent`. Blocking them is a legitimate choice, so
> `geo-ai-crawler-blocked` is informational — it surfaces a policy that is often set
> unintentionally. Author/date attribution is read from JSON-LD, `rel`/`itemprop="author"`,
> `<meta name="author">`, `<time datetime>`, and OpenGraph `article:*` tags.
>
> `geo-js-dependent-content` only runs under `--render headless`: the renderer captures the
> raw pre-JS HTML alongside the post-JS DOM and compares the prose each contains. In raw crawl
> mode there is nothing to compare, so the check is silent. "Prose" for both this check and
> `geo-low-quotable-density` is the text of `<p>` and `<li>` elements.
>
> ⚙︎ `geo-low-quotable-density` is an **opt-in specialized check**, off by default. See
> [the note below](#specialized-ai-search-checks).

---

## Specialized AI-search checks

Two of the AI-search checks — `aeo-no-answer-lead` and `geo-low-quotable-density` (marked ⚙︎
above) — are deliberately fuzzier heuristics. They judge editorial quality (does the page lead
with a direct answer? does it cite concrete facts?) rather than a hard technical fault, so they
are **off by default** and run only on demand:

```bash
# CLI flag
gocrawl crawl https://example.com --specialized
```

```yaml
# YAML config
analyzers:
  specialized: true
```

The MCP `crawl` tool exposes the same toggle as a `specialized` boolean. When off, the `aeo`
and `geo` analyzers still run all of their default checks — only these two heuristics are
suppressed.

---

## Adding your own analyzer

Every check implements the `analyze.Analyzer` interface, and new analyzers slot in without
touching the crawl engine. See [Architecture](architecture.md#adding-an-analyzer) and
[CONTRIBUTING.md](../CONTRIBUTING.md#adding-a-new-analyzer). This is the same seam through
which the [SEA analyzers](roadmap.md) (`utm`, `tracking`, `landing`) were added.

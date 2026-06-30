# Analyzer reference

An **analyzer** is a single, self-contained check. Each one consumes the crawl result and
emits zero or more [`Issue`](output.md#issue) values. An issue has a `severity`
(`error`, `warning`, or `info`), a stable `code`, a `message`, and an optional `data` map.

gocrawl ships twenty-one analyzers, run in this registration order
([`runner.BuildRegistry`](../internal/runner/runner.go)):
the technical/on-page set `seo`, `redirects`, `links`, `robots`, `sitemap`, `structured`,
`perf`, `images`, `urls`, `security`, `pagination`, `hreflang`, `amp`, `duplicates`,
`content`, the CMS-specific `wordpress`, the SEA analyzers `utm`, `tracking`, `datalayer`,
`landing`, and the AI-search analyzers `aeo`, `geo`.

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
| `x-robots-noindex` | warning | The `X-Robots-Tag` HTTP header contains `noindex` | `x_robots_tag` |
| `x-robots-nofollow` | info | The `X-Robots-Tag` HTTP header contains `nofollow` | `x_robots_tag` |
| `meta-refresh` | warning | Page uses a `<meta http-equiv="refresh">` redirect (prefer an HTTP 3xx) | `content` |
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
| `inbound-links` | info | Per-page count of internal inbound links (emitted for every HTML `200` page) | `count`, `anchors` |

> `inbound-links` counts internal links pointing **at** each page from other crawled pages
> and samples up to ten distinct inbound anchor texts. A count of `0` flags a page nothing
> internally links to (a possible orphan, subject to crawl scope).

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
| `structured-missing-required` | warning | A typed object of a recognized schema.org type omits a required field | `type`, `missing` |

> Required-field validation covers a pragmatic subset of common types (`Product`, `Article`,
> `Event`, `Organization`, `BreadcrumbList`, `FAQPage`, `Recipe`, …), not the full schema.org
> vocabulary. It descends into `@graph` the same way type extraction does.

---

## `perf` — Core Web Vitals

Source: [`internal/analyze/perf/perf.go`](../internal/analyze/perf/perf.go).

Runs in two modes depending on how the crawl was fetched:

- **Headless mode (`--render headless`).** Reads lab-mode Core Web Vitals captured by the
  chromedp renderer (`PerformanceObserver` for LCP / FCP / CLS / long-task TBT, Navigation
  Timing for TTFB) and emits per-page findings against [Google's CWV thresholds][cwv].
  If a page is snapshotted before it finishes rendering, the rendered DOM comes back far
  thinner than the raw HTML; the renderer detects this, **analyzes the raw HTML instead** (so
  structural checks like the H1 aren't false-negatives) and emits a `render-incomplete`
  warning marking that page's CWV as unreliable.
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
| `render-incomplete` | warning | The rendered DOM came back far thinner than the raw HTML (page not finished rendering); gocrawl analyzed the raw HTML instead, and this page's CWV are unreliable | `rendered_bytes`, `raw_bytes` |
| `cwv-not-collected` | info | Raw-mode fallback (once on the seed) — reminds to enable `--render headless` | — |
| `response-time` | info | Raw-mode per-page TTFB proxy from raw fetch duration | `duration_ms` |

---

## `images` — image alt text and dimensions

Source: [`internal/analyze/images/images.go`](../internal/analyze/images/images.go). Runs on
every HTML `200` page. Aggregates per page (one issue per code, not per image).

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `img-missing-alt` | warning | One or more `<img>` have no `alt` attribute at all | `count`, `sample` |
| `img-missing-dimensions` | info | One or more `<img>` are missing a `width` or `height` attribute | `count` |

> An explicit empty `alt=""` is **valid** for decorative images and is not flagged — only a
> missing `alt` attribute counts. `sample` lists up to five offending `src` values. Broken or
> oversized images are out of scope (they would require fetching the image bytes).

---

## `urls` — URL hygiene

Source: [`internal/analyze/urls/urls.go`](../internal/analyze/urls/urls.go). Runs on every
crawled page with a non-empty final URL (any status). At most one issue per code per page.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `url-uppercase` | info | The URL path contains uppercase ASCII letters | `url` |
| `url-underscore` | info | The URL path contains an underscore (`_`) | `url` |
| `url-non-ascii` | info | The URL contains non-ASCII characters | `url` |
| `url-too-long` | info | The full URL is longer than 115 characters | `url`, `length` |

---

## `security` — security headers and insecure forms

Source: [`internal/analyze/security/security.go`](../internal/analyze/security/security.go).
Runs on every HTML `200` page. Header checks are skipped when no response headers were
captured; the form check still runs.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `missing-hsts` | warning | An HTTPS page sends no `Strict-Transport-Security` header | — |
| `missing-csp` | info | The page sends no `Content-Security-Policy` header | — |
| `missing-x-content-type-options` | info | No `X-Content-Type-Options: nosniff` header | — |
| `insecure-form` | warning | An HTTPS page has a `<form>` posting to an `http://` action | `action` |

> Mixed subresource content is reported separately by the [`redirects`](#redirects--http-status-redirects-slow-responses-mixed-content)
> analyzer (`mixed-content`); `security` focuses on response headers and form targets.

---

## `pagination` — rel=next/prev sequences

Source: [`internal/analyze/pagination/pagination.go`](../internal/analyze/pagination/pagination.go).
Runs on every HTML `200` page that declares `rel="next"` or `rel="prev"` head links.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `pagination-detected` | info | The page declares a `rel="next"` and/or `rel="prev"` link | `next`, `prev` |
| `pagination-broken` | warning | A `next`/`prev` target was crawled and returns `>= 400` or redirects | `target`, `rel`, `status` |

> `pagination-broken` only fires for targets that were actually crawled — same crawl-scope
> caveat as the [`links`](#links--link-analysis) analyzer.

---

## `hreflang` — international targeting

Source: [`internal/analyze/hreflang/hreflang.go`](../internal/analyze/hreflang/hreflang.go).
Collects `<link rel="alternate" hreflang="…">` clusters across all HTML `200` pages, then
validates each cluster (a two-pass analyzer, since reciprocity compares pages to each other).

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `hreflang-invalid-code` | warning | A `hreflang` value isn't a valid language / region code or `x-default` | `code` |
| `hreflang-missing-x-default` | info | A cluster has no `x-default` entry | — |
| `hreflang-missing-self` | info | No entry in the cluster points back to the page's own URL | — |
| `hreflang-no-return-link` | warning | A page references a crawled target that does not link back (no reciprocal annotation) | `target` |

> Valid codes match `xx` or `xx-XX` (plus `x-default`). Reciprocity and self-reference checks
> resolve hrefs against the crawled page set, so they cover internally-reachable language
> variants best.

---

## `amp` — Accelerated Mobile Pages

Source: [`internal/analyze/amp/amp.go`](../internal/analyze/amp/amp.go). Runs on every HTML
`200` page. A page is AMP when its `<html>` element carries an `amp` or `⚡` attribute.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `amp-detected` | info | The page is an AMP document | — |
| `amp-missing-canonical` | warning | An AMP page has no `<link rel="canonical">` | — |
| `amp-missing-runtime` | error | An AMP page does not load the AMP runtime (`https://cdn.ampproject.org/v0.js`) | — |
| `amp-amphtml-linked` | info | A non-AMP page links to an AMP version via `<link rel="amphtml">` | `target` |
| `amp-broken-amphtml` | warning | The linked `amphtml` target was crawled and returns `>= 400` or redirects | `target` |

---

## `duplicates` — duplicate content, titles, descriptions

Source: [`internal/analyze/duplicates/duplicates.go`](../internal/analyze/duplicates/duplicates.go).
A cross-page analyzer over all HTML `200` pages. Body comparison uses an MD5 hash of the
whitespace-collapsed, lowercased `<body>` text.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `duplicate-content` | warning | Two or more pages share identical body content | `duplicates`, `group_size` |
| `duplicate-title` | warning | Two or more pages share an identical `<title>` | `title`, `duplicates`, `group_size` |
| `duplicate-meta-description` | info | Two or more pages share an identical meta description | `duplicates`, `group_size` |

> One issue is emitted per page in each duplicate group; `duplicates` lists up to ten of the
> other URLs in the group. Empty bodies/titles/descriptions are ignored.

---

## `content` — thin and below-average content

Source: [`internal/analyze/content/content.go`](../internal/analyze/content/content.go). A
cross-page analyzer: it counts `<body>` words per HTML `200` page and compares each page to
the crawl's mean.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `thin-content` | warning | The page has fewer than 100 words | `words` |
| `low-content` | info | The page has fewer than half the site-average word count (≥ 3 pages crawled; not already thin) | `words`, `site_average` |

> `thin-content` uses an absolute 100-word floor; `low-content` is relative to the crawl
> average and is suppressed on pages already flagged as thin, so the two never double-report.

---

## `wordpress` — WordPress detection and WP-specific checks (CMS)

Source: [`internal/analyze/wordpress/wordpress.go`](../internal/analyze/wordpress/wordpress.go).
First fingerprints WordPress from the crawled HTML (generator meta tag, `/wp-content/`,
`/wp-includes/`, `/wp-json/` asset paths, the `X-Pingback` header, and the `api.w.org` REST
discovery `Link`); on a non-WordPress site it stays completely silent. Because most fingerprints
live in the shared header/footer template and repeat on every page, the passive findings are
aggregated and emitted **once per site** (issue `url` is the site base URL); only the ugly-
permalink check is per page.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `wp-detected` | info | The site is identified as WordPress; carries the gathered context | `version`, `seo_plugin`, `i18n_plugin`, `plugins`, `plugin_count` |
| `wp-version-exposed` | warning | The core version is disclosed in the generator meta tag (maps the install to known CVEs) | `version` |
| `wp-emoji-enabled` | info | The `wp-emoji` script is loaded sitewide (usually safe to dequeue) | — |
| `wp-jquery-migrate` | info | The jQuery Migrate compatibility shim is loaded | — |
| `wp-many-plugin-assets` | warning | At least 10 distinct plugins ship front-end assets (render-blocking request pile-up) | `plugins`, `plugin_count` |
| `wp-default-tagline` | warning | The site still uses the default "Just another WordPress site" tagline | — |
| `wp-no-seo-plugin` | info | No SEO plugin (Yoast, Rank Math, All in One SEO) detected | — |
| `wp-multiple-seo-plugins` | warning | More than one SEO plugin is active (conflicting/duplicate meta output) | `plugins` |
| `wp-multilingual-detected` | info | A multilingual plugin (WPML, Polylang, TranslatePress, Weglot) is active | `plugins` |
| `wp-i18n-no-hreflang` | warning | A multilingual plugin is active but no `hreflang` alternate links were found on any crawled page | `plugins` |
| `wp-ugly-permalink` | info | A page is served under a default plain permalink (`?p=N`, `?page_id=N`, `?cat=N`) rather than a pretty URL | `param` |
| `wp-i18n-lang-query-param` | info | Language is negotiated via a `?lang=` query parameter rather than a per-language path or subdomain | `lang` |
| `wp-html-lang-mismatch` | warning | The `<html lang>` attribute disagrees with the language requested in the URL (`?lang=`) | `html_lang`, `url_lang` |
| `wp-acf-leaked-markup` | warning | Unrendered Advanced Custom Fields markup (a field tag or shortcode) is output as visible text | `snippet` |
| `wp-indexable-attachment` | warning | An indexable attachment page (`?attachment_id=N`) — thin auto-generated content | `id` |
| `wp-indexable-search` | warning | An indexable internal search results page (`?s=…`), which should be noindex | — |
| `wp-indexable-author-archive` | info | An indexable author archive (`/author/<login>/`) — often duplicates the blog index | — |
| `wp-indexable-date-archive` | info | An indexable date archive (`/YYYY/`, `/YYYY/MM/`) — thin, duplicate-prone listing | — |
| `wp-xmlrpc-enabled` ⚙︎ | warning | `xmlrpc.php` answers (brute-force amplification / pingback-DDoS vector) | — |
| `wp-user-enumeration-rest` ⚙︎ | warning | `/wp-json/wp/v2/users` returns usernames | `usernames`, `count` |
| `wp-user-enumeration-author` ⚙︎ | warning | `/?author=1` redirects to `/author/<login>/`, leaking a username | `username` |
| `wp-directory-listing` ⚙︎ | warning | `/wp-content/uploads/` is browsable (directory listing enabled) | — |
| `wp-readme-exposed` ⚙︎ | warning | `readme.html` is reachable and discloses the core version | `version` |

> ⚙︎ The five probe codes require **active fetches** of well-known endpoints beyond the crawl,
> so they are **opt-in** and run only with `--specialized` (or `analyzers.specialized: true`).
> Without it the analyzer is passive — it reads only the already-crawled HTML. SEO-plugin
> detection covers Yoast SEO, Rank Math, and All in One SEO via their front-end signatures.
>
> The `wp-indexable-*` checks fire only when the page is genuinely **indexable** — a `noindex`
> directive (meta or `X-Robots-Tag`) or a `<link rel="canonical">` pointing elsewhere already
> handles the page, so it is not flagged. Attachment-page detection relies on the
> `?attachment_id=N` form; pretty attachment URLs are not detectable from the crawl alone.
>
> The multilingual checks stop at WordPress-specific signals — which plugin is active, whether it
> emits `hreflang` at all, and the language-negotiation style. `hreflang` correctness itself
> (invalid codes, missing return links, missing `x-default`) is the dedicated
> [`hreflang`](#hreflang--international-targeting) analyzer's job and is not duplicated here.
>
> `wp-acf-leaked-markup` scans the page's **visible text** (script, style, and `code`/`pre`/
> `textarea` blocks excluded, so ACF tutorials are not flagged) for field tags or shortcodes that
> were printed instead of executed. ACF field *values*, once rendered, are ordinary HTML and are
> not distinguishable from hand-written content, so the analyzer only catches this leak failure.

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

## `datalayer` — GTM / dataLayer audit (SEA)

Source: [`internal/analyze/datalayer/datalayer.go`](../internal/analyze/datalayer/datalayer.go).
Where `tracking` answers "which tags are installed?", `datalayer` answers "are they wired up
correctly and what are they actually measuring?". It runs two tiers on every HTML `200` page.

**Static tier** (any mode — reads the HTML):

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `gtm-noscript-missing` | warning | A GTM container is present but its `<noscript>` `ns.html` iframe fallback is absent | `containers` |
| `gtm-snippet-not-in-head` | info | The GTM container snippet/loader is not inside `<head>` | `containers` |
| `datalayer-push-before-init` | warning | A `dataLayer.push` runs before the dataLayer is initialized (early pushes are lost) | — |
| `datalayer-init-missing` | warning | A tag manager is present but no dataLayer is initialized in the HTML | — |
| `gtag-config-id-mismatch` | warning | `gtag('config', X)` targets an ID with no matching `gtag/js` loader (and no GTM that could load it) | `config_id`, `loaded_ids` |
| `consent-mode-present` | info | Google Consent Mode signals (`gtag('consent', …)`) detected | — |
| `consent-mode-missing` | info | Analytics/ads tags present but no Consent Mode default found | — |

**Runtime tier** (requires `--render headless` — reads the post-JS `window.dataLayer` and the
page's network beacons):

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `datalayer-empty` | warning | A tag manager is present but `window.dataLayer` is empty/absent after rendering | — |
| `datalayer-events` | info | Inventory of every event pushed, with per-event counts | `events` |
| `page-view-missing` | warning | The dataLayer has events but no `page_view`/GTM lifecycle event, and no gtag config implying GA4 auto page_view | — |
| `ecommerce-event-invalid` | warning | A GA4 e-commerce event (`purchase`, `add_to_cart`, …) is missing required parameters | `event`, `missing` |
| `datalayer-param-type` | warning | An e-commerce parameter has the wrong type (`value` not numeric, `currency` not ISO-4217, `items` not a non-empty array) | `event`, `param`, `want`, `got` |
| `duplicate-event` | warning | A conversion event (`purchase`, `generate_lead`, …) fired more than once | `event`, `count` |
| `duplicate-transaction` | warning | The same purchase `transaction_id` fired more than once | `transaction_id`, `count` |
| `datalayer-pii` | warning | An email (anywhere) or a phone-shaped value under a phone-like key was pushed into the dataLayer | `kind`, `key`, `value` (redacted) |
| `tag-not-firing` | warning | A tag detected in the HTML issued no matching network beacon during render | `tag` |
| `tags-firing` | info | Tags confirmed to have issued a network beacon | `tags` |
| `datalayer-not-collected` | info | Emitted once when no page was rendered; runtime checks need `--render headless` | — |

> **Why two tiers.** The dataLayer and the events pushed into it only exist after JavaScript
> runs, so the event inventory, e-commerce validation, duplicate-conversion, PII, and
> tag-firing checks need headless rendering. The wiring checks (noscript fallback, snippet
> placement, push-before-init, Consent Mode) are visible in the static HTML and run in any
> mode. gtag's `gtag('event', …)` arguments-object pushes are normalized alongside GTM
> `{event: …}` pushes, so both styles are audited. Captured dataLayer entries are **never**
> written to the report (they can contain PII) — only sanitized findings are.

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

The same `specialized` flag also enables the `wordpress` analyzer's active endpoint probes
(the ⚙︎ codes above: `wp-xmlrpc-enabled`, `wp-user-enumeration-rest`,
`wp-user-enumeration-author`, `wp-directory-listing`, `wp-readme-exposed`). These issue extra
HTTP requests to well-known WordPress paths, so they are gated behind the same opt-in; the
analyzer's detection and passive checks always run.

---

## Adding your own analyzer

Every check implements the `analyze.Analyzer` interface, and new analyzers slot in without
touching the crawl engine. See [Architecture](architecture.md#adding-an-analyzer) and
[CONTRIBUTING.md](../CONTRIBUTING.md#adding-a-new-analyzer). This is the same seam through
which the [SEA analyzers](roadmap.md) (`utm`, `tracking`, `landing`) were added.

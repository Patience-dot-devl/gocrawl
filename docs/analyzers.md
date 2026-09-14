# Analyzer reference

An **analyzer** is a single, self-contained check. Each one consumes the crawl result and
emits zero or more [`Issue`](output.md#issue) values. An issue has a `severity`
(`error`, `warning`, or `info`), a stable `code`, a `message`, and an optional `data` map.

gocrawl ships twenty-five analyzers, run in this registration order
([`runner.BuildRegistry`](../internal/runner/runner.go)):
the technical/on-page set `seo`, `redirects`, `links`, `robots`, `sitemap`, `structured`,
`perf`, `images`, `urls`, `security`, `pagination`, `hreflang`, `amp`, `duplicates`,
`content`, `botwall`, the CMS-specific `wordpress` and `shopify`, the SEA analyzers `utm`,
`tracking`, `datalayer`, `landing`, `consent`, and the AI-search analyzers `aeo`, `geo`.

List them at any time:

```sh
gocrawl analyzers list
```

Select a subset with `--analyzers` or the `analyzers.enabled` / `analyzers.disabled` config
keys — see [Selecting analyzers](configuration.md#selecting-analyzers).

> **Severity is a classification, not a pass/fail.** Several analyzers emit `info` issues
> (e.g. `link-summary`, `sitemap-coverage`, `perf-response-time`) that report findings rather than
> problems. Filter on `severity == "error"` / `"warning"` for actionable items.

---

## `seo` — On-page technical SEO

Source: [`internal/analyze/seo/seo.go`](../internal/analyze/seo/seo.go). Runs on every HTML
page that returned `200`.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `seo-missing-title` | error | No `<title>` in `<head>` | — |
| `seo-short-title` | warning | Title shorter than 10 chars | `length`, `title` |
| `seo-long-title` | warning | Title longer than 60 chars (may truncate in SERPs) | `length`, `title` |
| `seo-missing-meta-description` | warning | No `<meta name="description">` (or empty) | — |
| `seo-short-meta-description` | info | Description shorter than 50 chars | `length` |
| `seo-long-meta-description` | info | Description longer than 160 chars | `length` |
| `seo-meta-noindex` | warning | `<meta name="robots">` contains `noindex` | `robots` |
| `seo-meta-nofollow` | info | `<meta name="robots">` contains `nofollow` | `robots` |
| `seo-x-robots-noindex` | warning | The `X-Robots-Tag` HTTP header contains `noindex` | `x_robots_tag` |
| `seo-x-robots-nofollow` | info | The `X-Robots-Tag` HTTP header contains `nofollow` | `x_robots_tag` |
| `seo-meta-refresh` | warning | Page uses a `<meta http-equiv="refresh">` redirect (prefer an HTTP 3xx) | `content` |
| `seo-multiple-canonical` | warning | More than one `<link rel="canonical">` | — |
| `seo-missing-canonical` | info | No canonical link | — |
| `seo-missing-h1` | warning | No `<h1>` element | — |
| `seo-multiple-h1` | info | More than one `<h1>` | `count` |
| `seo-skipped-heading-level` | info | Heading hierarchy skips a level (e.g. h1 → h3) | `from`, `to` |
| `seo-empty-heading` | warning | A heading element has no text content | `tag` |
| `seo-missing-lang` | info | `<html>` has no `lang` attribute | — |
| `seo-missing-viewport` | info | No `<meta name="viewport">` (mobile-friendliness) | — |
| `seo-missing-charset` | info | No `<meta charset>` and no `content-type` http-equiv | — |
| `seo-missing-opengraph` | info | No `<meta property="og:*">` tags | — |

**Thresholds:** title 10–60 chars, meta description 50–160 chars.

---

## `redirects` — HTTP status, redirects, slow responses, mixed content

Source: [`internal/analyze/httpx/httpx.go`](../internal/analyze/httpx/httpx.go). Runs on
every page. (The analyzer's internal package is `httpx`; its registered name is `redirects`.)

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `http-fetch-error` | error | The page failed to fetch | — |
| `http-server-error` | error | Status `>= 500` | `status` |
| `http-client-error` | error | Status `400`–`499` | `status` |
| `http-redirect-loop` | error | A URL repeats in the redirect chain | `chain` |
| `http-redirect-chain` | warning | More than one redirect before the final URL | `hops`, `chain` |
| `http-redirect` | info | A single redirect | `to`, `status` |
| `http-slow-response` | warning | Response slower than the threshold (default **2s**) | `duration_ms` |
| `http-mixed-content` | warning | HTTPS page loads `http://` subresources | `count`, `examples` |

**Threshold:** slow-response fires above 2 seconds. Mixed-content reports up to 5 example
URLs.

---

## `links` — Link analysis

Source: [`internal/analyze/links/links.go`](../internal/analyze/links/links.go). Cross-
references each page's outbound links against the crawled page set.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `link-broken` | error | Internal link points to a crawled page with status `>= 400` | `target`, `status`, `anchor` |
| `link-to-redirect` | warning | Internal link points to a crawled page that redirects | `target`, `final` |
| `link-empty-anchor` | info | The page has links with empty anchor text | `count` |
| `link-summary` | info | Per-page link counts (always emitted when a page has links) | `total`, `external`, `nofollow` |
| `link-inbound` | info | Per-page count of internal inbound links (emitted for every HTML `200` page) | `count`, `anchors` |

> `link-inbound` counts internal links pointing **at** each page from other crawled pages
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
| `robots-missing` | info | No `robots.txt` found for the host | `status` |
| `robots-no-sitemap-declared` | info | `robots.txt` declares no `Sitemap:` | — |
| `robots-sitemaps-declared` | info | `robots.txt` declares one or more sitemaps | `sitemaps` |
| `robots-crawled-disallowed` | warning | A crawled URL is disallowed by `robots.txt` | — |

> `robots-crawled-disallowed` can only occur when you crawled with `--respect-robots=false`; with
> the default `true`, disallowed URLs are never fetched.

---

## `sitemap` — sitemap.xml discovery and coverage

Source: [`internal/analyze/sitemap/sitemap.go`](../internal/analyze/sitemap/sitemap.go).
Looks for sitemaps declared in `robots.txt` plus the conventional `/sitemap.xml` and
`/sitemap_index.xml`, following sitemap-index files up to two levels deep.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `sitemap-invalid` | warning | A fetched sitemap parsed as neither a `urlset` nor an index | — |
| `sitemap-missing` | warning | No sitemap found at any candidate location | — |
| `sitemap-coverage` | info | Cross-check of sitemap URLs vs. crawled pages | `sitemap_urls`, `crawled_pages`, `crawled_not_in_sitemap`, `in_sitemap_not_crawled` |

The coverage `data` lets you spot pages that are crawlable but missing from the sitemap
(`crawled_not_in_sitemap`) and sitemap entries that weren't reached in the crawl
(`in_sitemap_not_crawled`).

---

## `structured` — JSON-LD structured data

Runs on every HTML page that returned `200`. Reads the page's `<script
type="application/ld+json">` blocks through the shared schema.org graph parser (which flattens
nested objects, `@graph`, and arrays into an addressable node list), then checks what a page
declares against what its rich results actually require, whether declarations contradict each
other or the page they sit on, and which pages look like they are missing markup they should
have.

| Code | Severity | Scope | Triggered when | `data` |
| --- | --- | --- | --- | --- |
| `structured-invalid-jsonld` | warning | page | A JSON-LD block is not valid JSON | `error` |
| `structured-none` | info | page | The parsed graph has zero typed nodes and zero parse errors | — |
| `structured-data` | info | page | JSON-LD found; lists the de-duplicated `@type`s, including nested ones | `types` |
| `structured-missing-required` | warning | page | A typed object omits a field its rich result requires | `type`, `missing`, `path` |
| `structured-missing-recommended` | info | **site** | A type omits recommended fields, aggregated across the crawl | `type`, `missing`, `fields`, `pages`, `examples` |
| `structured-missing-merchant` | info | **site** | `Product` omits Google Merchant listing fields, aggregated | `type`, `missing`, `fields`, `pages`, `examples` |
| `structured-duplicate-type` | warning | page | A page-level type is declared in two or more JSON-LD blocks | `type`, `blocks` |
| `structured-conflicting-value` | error | page | Duplicate blocks disagree on `name`, `sku`, or an `offers` field | `type`, `field`, `values` |
| `structured-unresolved-id` | warning | page | An `@id` reference has no matching node on the page | `id`, `property`, `type` |
| `structured-relative-url` | warning | page | A URL property holds a relative path | `type`, `property`, `value` |
| `structured-invalid-date` | warning | page | A date property is not ISO 8601 | `type`, `property`, `value` |
| `structured-malformed-price` | warning | page | A price string carries a symbol, separator, or range | `type`, `property`, `value` |
| `structured-price-mismatch` | warning | page | `offers.price` differs from the single price rendered on the page | `markup`, `page` |
| `structured-breadcrumb-candidate` | warning | page | Breadcrumb-styled nav with ≥2 links, no `BreadcrumbList` | `links` |
| `structured-product-candidate` | warning | page | Product/price microdata, or a price plus a cart call-to-action, with no `Product`/`Offer` | `signal` |
| `structured-article-candidate` | warning | page | A 150+ word `<article>` with an author or date signal, no article type | `words` |
| `structured-video-candidate` | warning | page | A `<video>` or YouTube/Vimeo embed, no `VideoObject` | `src` |

> **Field tiers.** Each recognized type carries a *required* set (absence blocks the rich
> result), a *recommended* set (absence degrades it), and — for `Product` — a *merchant* set
> feeding Shopping and free listings. A field written with `|` separators is an any-of group:
> `gtin|gtin8|gtin12|gtin13|gtin14|mpn` is satisfied by any one identifier.

> **Why two of them are site-scoped.** A theme either emits `aggregateRating` or it does not,
> so a recommended-field gap repeats identically on every page of a template. Those two codes
> aggregate into one issue per type, carrying the affected page count and up to five example
> URLs, instead of one issue per page.

> **Thin copies are exempt.** Objects nested under `itemListElement`, `hasVariant`,
> `isVariantOf`, `isSimilarTo`, `isRelatedTo` or `isAccessoryOrSparePartFor` are deliberately
> minimal — a collection page's product tiles carry a name and a URL and nothing else — so
> eligibility and duplicate checks skip them.

> The `*-candidate` checks are low-noise heuristics: they only fire on a fairly specific
> on-page signal and never fire when a matching `@type` is already present anywhere on the page.

Source: [`internal/analyze/structured/`](../internal/analyze/structured/), reading pages
through the shared [`internal/analyze/schemaorg`](../internal/analyze/schemaorg/) parser.

---

## `perf` — Core Web Vitals

Source: [`internal/analyze/perf/perf.go`](../internal/analyze/perf/perf.go).

Runs in two modes depending on how the crawl was fetched:

- **Headless mode (`--render headless`).** Reads lab-mode Core Web Vitals captured by the
  chromedp renderer (`PerformanceObserver` for LCP / FCP / CLS / long-task TBT, Navigation
  Timing for TTFB) and emits per-page findings against [Google's CWV thresholds][cwv].
  If a page is snapshotted before it finishes rendering, the rendered DOM comes back far
  thinner than the raw HTML; the renderer detects this, **analyzes the raw HTML instead** (so
  structural checks like the H1 aren't false-negatives) and emits a `perf-render-incomplete`
  warning marking that page's CWV as unreliable.
- **Raw mode.** Falls back to a single `perf-cwv-not-collected` notice and a per-page
  `perf-response-time` proxy from the raw fetch's TTFB.

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
| `perf-cwv-measured` | info | Per rendered `200` HTML page — all five metrics in one record | `lcp_ms`, `fcp_ms`, `cls`, `tbt_ms`, `ttfb_ms` |
| `perf-lcp-needs-improvement` / `perf-lcp-poor` | warning / error | LCP above the band | `value_ms`, `threshold_ms` |
| `perf-fcp-needs-improvement` / `perf-fcp-poor` | warning / error | FCP above the band | `value_ms`, `threshold_ms` |
| `perf-cls-needs-improvement` / `perf-cls-poor` | warning / error | CLS above the band | `value`, `threshold` |
| `perf-tbt-needs-improvement` / `perf-tbt-poor` | warning / error | TBT above the band | `value_ms`, `threshold_ms` |
| `perf-ttfb-needs-improvement` / `perf-ttfb-poor` | warning / error | TTFB above the band | `value_ms`, `threshold_ms` |
| `perf-cwv-render-failed` | info | Headless rendering errored on a page; CWV unavailable for it | `note` |
| `perf-render-incomplete` | warning | The rendered DOM came back far thinner than the raw HTML (page not finished rendering); gocrawl analyzed the raw HTML instead, and this page's CWV are unreliable | `rendered_bytes`, `raw_bytes` |
| `perf-cwv-not-collected` | info | Raw-mode fallback (once on the seed) — reminds to enable `--render headless` | — |
| `perf-response-time` | info | Raw-mode per-page TTFB proxy from raw fetch duration | `duration_ms` |

---

## `images` — image alt text and dimensions

Source: [`internal/analyze/images/images.go`](../internal/analyze/images/images.go). Runs on
every HTML `200` page. Aggregates per page (one issue per code, not per image).

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `img-missing-alt` | warning | One or more `<img>` have no `alt` attribute at all | `count`, `sample`, `sources` |
| `img-empty-alt` | info | One or more `<img>` have an explicit `alt=""` (or whitespace-only) | `count`, `images_total`, `sample`, `sources` |
| `img-duplicate-alt` | warning | The same non-empty alt text is used by more than one image on the page | `count`, `duplicates` |
| `img-alt-is-filename` | warning | The alt text is just the filename restated (`kantoor.jpg` → `alt="Kantoor"`) | `count`, `sample`, `sources` |
| `img-nondescriptive-filename` | info | The filename carries no meaning: pure digits, camera/export defaults, a content hash, or a stem of 3 characters or fewer | `count`, `sample`, `sources` |
| `img-missing-dimensions` | info | One or more `<img>` are missing a `width` or `height` attribute | `count`, `sample`, `sources` |

### Three alt states, not two

The analyzer distinguishes **missing** (`<img src=…>`), **empty** (`alt=""`) and **present**
(`alt="…"`). Empty alt is the *correct* markup for a decorative image, so it is reported as
`info` rather than treated as a defect — but it is reported, because a theme or CMS that emits
`alt=""` for every content image is indistinguishable from a correctly decorated page unless
you can see the list. `images_total` on that finding gives the ratio at a glance (e.g. 42 of
67). Deciding which of those images are actually decorative is a human call.

### Image URLs in `data`

`sample` lists up to five image URLs for readability; `sources` lists every one (deduplicated,
capped at 100 per finding per page, with `truncated: true` when the cap is hit) so tooling can
act on the full set. URLs are resolved against the page URL. `src` is preferred, falling back
to `data-src` / `data-original` and then the first `srcset` candidate, so lazy-loading themes
still report the real image rather than a placeholder.

`duplicates` is a list of `{alt, count, sample}` groups, most-repeated first; the finding's
`count` is the total number of images involved in any duplicate group.

> Anything requiring the image bytes is out of scope: gocrawl never fetches images, so file
> size, real dimensions, and format-vs-content mismatches (a photo shipped as PNG) are not
> checked. Filename and alt checks are markup-only.

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

## `security` — security headers, insecure forms, and the opt-in TLS/cookie audit

Source: [`internal/analyze/security/`](../internal/analyze/security/). The analyzer runs in
two passes: a baseline pass that is always on, and an audit pass enabled by
`--security-audit`.

### Baseline (always on)

Runs on every HTML `200` page. Header checks are skipped when no response headers were
captured; the form check still runs.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `security-missing-hsts` | warning | An HTTPS page sends no `Strict-Transport-Security` header | — |
| `security-missing-csp` | info | The page sends no `Content-Security-Policy` header | — |
| `security-missing-x-content-type-options` | info | No `X-Content-Type-Options: nosniff` header | — |
| `security-insecure-form` | warning | An HTTPS page has a `<form>` posting to an `http://` action | `action` |

> Mixed subresource content is reported separately by the [`redirects`](#redirects--http-status-redirects-slow-responses-mixed-content)
> analyzer (`http-mixed-content`); the baseline `security` pass focuses on response headers
> and form targets.

### Security audit (opt-in)

> ⚙︎ These checks are **off by default**. Turn them on with `--security-audit` (or
> `analyzers.security_audit: true` in YAML, `security_audit` in MCP / the web API).

The audit inspects transport and cookie configuration rather than page content. It is
**passive** — it reads the responses the crawl already made and opens no extra connections,
unlike the `wordpress` analyzer's `--specialized` probes.

Everything it looks at is host-wide server configuration, so **findings are emitted once per
host**, reported against the first page crawled on that host, rather than repeating on every
page. Cookie findings are reported once per distinct cookie name.

**TLS and certificates.** Read from the handshake behind each response.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `security-no-https` | error | Pages end on `http://` after redirects | `count`, `example` |
| `security-tls-obsolete-version` | error | Negotiated TLS 1.0 or 1.1 | `version` |
| `security-tls-legacy-version` | info | Negotiated TLS 1.2 rather than 1.3 | `version` |
| `security-tls-weak-cipher` | error | Negotiated a suite Go classifies as insecure (RC4, 3DES, vulnerable CBC) | `cipher_suite`, `version` |
| `security-tls-cert-expired` | error | The leaf certificate is past `notAfter` | `subject`, `issuer`, `expires_at` |
| `security-tls-cert-expiring-soon` | warning / **error** | Expires within **30 days**; escalates to error inside **14 days** | + `days_remaining` |
| `security-tls-cert-not-yet-valid` | error | `notBefore` is in the future | `subject`, `issuer`, `expires_at` |
| `security-tls-cert-self-signed` | error | The leaf is self-signed | `subject`, `issuer`, `expires_at` |
| `security-tls-incomplete-chain` | warning | Only the leaf was sent, without its intermediate(s) | `subject`, `issuer` |
| `security-tls-weak-signature` | error | A chain certificate is signed with MD2/MD5/SHA-1 | `subject`, `signature_algorithm` |
| `security-tls-weak-key` | error | RSA under 2048 bits or a curve under 256 bits | `subject`, `key_type`, `key_bits` |
| `security-tls-ok` | info | Positive signal: the handshake and chain passed every check | `version`, `cipher_suite`, `alpn`, `issuer`, `expires_at`, `days_remaining` |

**Cookies.** Parsed from `Set-Cookie` response headers.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `security-cookie-no-secure` | error | A cookie set over HTTPS lacks `Secure` | `cookie` |
| `security-cookie-samesite-none-insecure` | error | `SameSite=None` without `Secure` (browsers reject it) | `cookie` |
| `security-cookie-no-samesite` | warning | No `SameSite` attribute at all | `cookie` |
| `security-cookie-no-httponly` | warning | A session-style cookie lacks `HttpOnly` | `cookie` |
| `security-cookie-prefix-violation` | warning | `__Host-`/`__Secure-` prefix used without meeting its requirements | `cookie`, `requirement` |
| `security-cookie-long-lived` | info | Requested lifetime exceeds the 400-day browser cap | `cookie`, `requested_days` |

**Response-header policy.** Checked once per host, against the first HTML `200` page.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `security-hsts-short-max-age` | warning | HSTS `max-age` under **180 days** | `max_age` |
| `security-hsts-no-subdomains` | info | HSTS without `includeSubDomains` | `value` |
| `security-missing-referrer-policy` | info | No `Referrer-Policy` header and no `<meta name="referrer">` | — |
| `security-missing-frame-protection` | warning | Neither `X-Frame-Options` nor a CSP `frame-ancestors` directive | — |
| `security-version-disclosure` | info | `Server` / `X-Powered-By` / `X-AspNet-Version` / `X-Generator` publishes a version number | `header`, `value` |

**Thresholds:** certificate renewal warns at 30 days and errors at 14; HSTS `max-age` floor is
15552000 seconds (180 days); the cookie lifetime cap is 400 days; RSA keys must be ≥ 2048 bits
and elliptic-curve keys ≥ 256 bits.

#### Limits worth knowing

- **Only what the crawl already fetched.** Go verifies certificates before returning a
  response, so a chain that is expired, self-signed, or issued for the wrong hostname normally
  surfaces as `http-fetch-error` from the `redirects` analyzer and never reaches the audit.
  Those checks still fire when the crawl runs through a TLS-terminating proxy that supplies
  its own trust anchor — exactly the setup where a bad origin certificate would go unnoticed.
- **Raw mode only, for TLS.** Headless rendering serves responses from the browser rather than
  Go's TLS stack, so no handshake is captured. The cookie and header checks still run, and the
  report carries a note explaining what was skipped.
- **Selective, not exhaustive.** `security-cookie-no-httponly` fires only on cookies whose
  name suggests session or credential state (`sess`, `sid`, `auth`, `token`, `login`,
  `remember`, `jwt`, …); analytics and preference cookies are read by JavaScript by design.
  Likewise, a self-signed **root** is exempt from the signature check — a trust anchor is
  trusted by identity, not by its own signature.

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
> other URLs in the group. Empty bodies/titles/descriptions are ignored. URLs that differ only
> by query string (e.g. `?solution=onboarding`) or `#fragment` are collapsed to a single page
> first, so query/anchor variants of one page are not reported as duplicates of each other.

---

## `content` — thin and below-average content

Source: [`internal/analyze/content/content.go`](../internal/analyze/content/content.go). A
cross-page analyzer: it counts `<body>` words per HTML `200` page and compares each page to
the crawl's mean.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `content-thin` | warning | The page has fewer than 100 words | `words` |
| `content-low` | info | The page has fewer than half the site-average word count (≥ 3 pages crawled; not already thin) | `words`, `site_average` |

> `content-thin` uses an absolute 100-word floor; `content-low` is relative to the crawl
> average and is suppressed on pages already flagged as thin, so the two never double-report.

---

## `botwall` — CAPTCHA / bot-protection challenge detection

Source: [`internal/analyze/botwall/botwall.go`](../internal/analyze/botwall/botwall.go). Flags
pages that served a CAPTCHA or bot-protection wall instead of the real content — so a crawl
that was silently blocked isn't mistaken for a successful audit (challenge pages usually
return HTTP `200`, then trip a page's worth of false "missing title / thin content" findings).

It scans each page's body, **response headers** (catches `cf-mitigated`, `x-datadome`,
challenge cookies), and — in headless mode — the **outbound request URLs** the renderer
captured, against signatures for: Cloudflare (challenge + Turnstile), Google reCAPTCHA,
hCaptcha, AWS WAF, DataDome, PerimeterX/HUMAN, and Imperva Incapsula.

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `botwall-challenge` | warning | A vendor interstitial marker matched, or a CAPTCHA-widget marker matched on a page that looks like a wall (blocking status `401/403/429/503`, a challenge-style `<title>`, or almost no other content). A challenge-style title on a blocking status with no known vendor is reported as `provider: "Unknown"`. | `provider`, `signals`, `status` |
| `botwall-captcha-widget` | info | A reCAPTCHA / hCaptcha / Turnstile widget is embedded in an otherwise-normal page (e.g. a contact form) — present, but not a block | `provider`, `providers`, `signals` |

> The widget-vs-wall distinction is deliberate: a reCAPTCHA on a full contact page is `info`,
> while the same widget on a thin/blocking page is a `botwall-challenge`. When you see
> `botwall-challenge`, treat that page's other findings as unreliable and re-crawl from an
> allow-listed IP/User-Agent or at a lower rate.

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

## `shopify` — Shopify detection and store-specific structured-data checks (CMS)

Source: [`internal/analyze/shopify/`](../internal/analyze/shopify/). Like `wordpress`, it stays
completely silent on a site it does not recognize, so enabling it costs nothing on the rest of
the web.

**Detection is tiered, not one flat list of fingerprints.** *Strong* fingerprints — the inline
`Shopify.theme` bootstrap object a theme's layout sets and the `shopify-features` storefront
runtime script — appear only in a Shopify theme's own layout, so either one alone means the
crawled site *is* a Shopify storefront; the `X-ShopId` and `X-Shopify-Stage` response headers
identify a store on their own too. *Weak* fingerprints — `cdn.shopify.com` asset URLs, a
`.myshopify.com` reference, `/cdn/shop/` paths — are recorded in `shopify-detected`'s `signals`
data but never flip detection by themselves. The reason: a site that is not itself hosted on
Shopify but embeds a Shopify Buy Button widget (a single product's checkout dropped into a
WordPress, Squarespace, or hand-rolled page) serves exactly those weak markers and nothing
else, and treating them as sufficient would make the analyzer invent per-template
structured-data findings against a site that has no Shopify templates to check. The deliberate
cost of that caution: a headless Shopify storefront (Hydrogen) that strips both strong markers
from its rendered output goes undetected, and the analyzer stays silent on it rather than
guess — one silently-skipped site costs less than a confidently wrong report.

Once a store is detected, every crawled URL is classified into a **template** by its path
shape — Shopify's URL structure is fixed by the platform rather than chosen per store, which is
what makes classifying by path reliable here in a way it would not be on an arbitrary site —
and checked against the schema.org types that template should carry. A gap is a property of
the template, not of one page: `shopify-template-schema-gap` fires per (template, missing type)
pair, carrying the count of pages found missing it and up to five example URLs, rather than
repeating the same fact once per page.

| Code | Severity | Scope | Triggered when | `data` |
| --- | --- | --- | --- | --- |
| `shopify-detected` | info | **site** | A strong fingerprint (`Shopify.theme`, `shopify-features`) is found on any crawled page, or a response carries `X-ShopId`/`X-Shopify-Stage`; weak fingerprints alone never trigger this | `signals`, `theme`, `theme_id` |
| `shopify-template-schema-gap` | warning | **site** | One or more crawled, `200`-status HTML pages of a template lack a schema.org type that template should carry | `template`, `expected`, `pages`, `examples` |
| `shopify-schema-app-conflict` | error | page | The page's `Product` JSON-LD is attributed to two or more different script sources (typically the theme and an SEO app) | `sources` |
| `shopify-schema-client-injected` | info | page | The page's raw HTML has zero JSON-LD nodes, its template is not `utility`/`unknown`, and a recognized structured-data app's script is present | `app`, `apps`, `template` |
| `shopify-flat-variant-product` | warning | page | A `product`-template page exposes 2+ variants in the DOM, declares one or more `Product` nodes, none of which carries `hasVariant`/`isVariantOf`, and the page declares no `ProductGroup` type either | `variants` |
| `shopify-single-offer-range` | info | page | The page's embedded variant JSON has 2+ distinct prices, a `Product` node declares exactly one `Offer`, and no `AggregateOffer` is present | `prices`, `variants` |

**Expected schema per template:**

| Template | Path shape | Expected | Notes |
| --- | --- | --- | --- |
| `home` | `/` | `Organization` (or `LocalBusiness`), and `WebSite` | |
| `product` | `/products/<handle>`, `/collections/<handle>/products/<handle>` | `Product` (or `ProductGroup`), and `BreadcrumbList` | |
| `collection` | `/collections/<handle>` | `CollectionPage` or `ItemList`, and `BreadcrumbList` | |
| `article` | `/blogs/<blog>/<article>` | `BlogPosting` (or `Article`/`NewsArticle`), and `BreadcrumbList` | |
| `blog` | `/blogs/<blog>`, `/blogs/<blog>/tagged/<tag>` | `Blog` or `CollectionPage` | A tag-filtered listing (`/tagged/<tag>`) routes here, not to `article` — it is a filtered index of posts, not a single post, and demanding `BlogPosting` on it would false-positive on every tag page of every store with a blog. |
| `page` | `/pages/<handle>` | `WebPage`, `AboutPage`, `ContactPage`, or `FAQPage` | |
| `policy` | `/policies/<handle>` | nothing asserted | Shopify's built-in legal pages (refund, privacy, terms). Deliberately **not** folded into `utility`: `utility` means "should not be indexable," and a later check flags indexable utility pages, but policy pages are meant to be indexed — filing them under `utility` would make that check false-positive on every store's policy pages. |
| `utility` | `/cart`, `/search`, `/account/*`, `/challenge`, `/checkouts/*`, `/orders/*`, `/password` | nothing asserted | A cart, search, account, or password-gate page has nothing to say to a search engine. |
| `unknown` | anything else (app-proxy routes such as `/apps/<app>`, custom page types) | nothing asserted | Not a route Shopify's own templates render, so no schema expectation is known to assert. |

> **Shopify Markets locale prefixes are skipped for classification only, never stripped from
> the URL.** `/en-ca/products/tee` and `/fr/collections/all` classify identically to their
> unprefixed equivalents — a Markets storefront serves every route under a locale prefix, and
> refusing to recognize it would silently drop a whole localized store's URLs into `unknown`.
> The prefix is preserved in the URL itself; only the classification logic skips over it,
> because a later check that rebuilds a canonical URL from the page's own URL must keep the
> prefix or it will point a canonical at a path that does not exist in that market.

> **Attribution is by script attributes, never contents.** Each JSON-LD block is attributed to
> the app named in its own `src`/`id`/`class`, else to the nearest recognized app script before
> it, else to the theme. An app's name appears inside unrelated inline JSON often enough that
> matching on script *contents* would misattribute blocks. Recognized apps: JSON-LD for SEO,
> Schema Plus, SearchPie, Schema App, SEOAnt, Yoast for Shopify, TinyIMG, Smart SEO, Avada SEO.

> **Raw mode sees what every crawler sees.** Shopify themes render JSON-LD server-side, so a
> raw crawl finds it. Several SEO apps inject it with JavaScript instead; when one is installed
> and the raw HTML has none, `shopify-schema-client-injected` says so rather than reporting the
> page as bare. Re-run with `--render headless` to see what the app emits.

> **Variant counts take the maximum across DOM selectors, not the sum.** A theme commonly
> renders its variant picker more than once — a `<select>` for narrow viewports, radio inputs
> for wide — and summing every selector would double-count, letting a single-variant product
> trip the two-variant gate on nothing more than a duplicated control. `<option>` elements with
> no value or an empty one (a placeholder such as "Choose an option") are filtered out of the
> two option-based selectors so a placeholder is never counted as a variant either. This remains
> an undercount in one direction: `[data-variant-id]` attached to several swatch or thumbnail
> elements per variant, or a duplicated `input[name="id"]` for the same variant, can still
> overcount.

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

By default (`analyzers.ignore_external_tagging`, `--ignore-external-tagging`), the 4
tagging-quality codes above (`utm-partial-tagging`, `utm-empty-value`, `utm-duplicate-param`,
`utm-inconsistent-casing`) are not emitted for links where the target leaves the crawled
domain; `utm-internal-tagged` and `utm-summary` are unaffected.

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
| `tracking-none` | info | HTML page with no detectable tags | — |
| `tracking-duplicate-tag` | warning | The same tag family is installed with two or more distinct IDs (double-counting risk) | `tag`, `ids`, `count` |
| `tracking-mixed-ga-versions` | info | Both Universal Analytics and GA4 are present | `ua_ids`, `ga4_ids` |

> **Static detection.** Tags injected at runtime by a tag manager are only visible via their
> container (e.g. a `GTM-…` ID), so `tracking-none` is informational rather than a warning.
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
| `datalayer-gtm-noscript-missing` | warning | A GTM container is present but its `<noscript>` `ns.html` iframe fallback is absent | `containers` |
| `datalayer-gtm-snippet-not-in-head` | info | The GTM container snippet/loader is not inside `<head>` | `containers` |
| `datalayer-push-before-init` | warning | A `dataLayer.push` runs before the dataLayer is initialized (early pushes are lost) | — |
| `datalayer-init-missing` | warning | A tag manager is present but no dataLayer is initialized in the HTML | — |
| `datalayer-gtag-config-id-mismatch` | warning | `gtag('config', X)` targets an ID with no matching `gtag/js` loader (and no GTM that could load it) | `config_id`, `loaded_ids` |
| `datalayer-consent-mode-present` | info | Google Consent Mode signals (`gtag('consent', …)`) detected | — |
| `datalayer-consent-mode-missing` | info | Analytics/ads tags present but no Consent Mode default found | — |

**Runtime tier** (requires `--render headless` — reads the post-JS `window.dataLayer` and the
page's network beacons):

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `datalayer-empty` | warning | A tag manager is present but `window.dataLayer` is empty/absent after rendering | — |
| `datalayer-events` | info | Inventory of every event pushed, with per-event counts | `events` |
| `datalayer-page-view-missing` | warning | The dataLayer has events but no `page_view`/GTM lifecycle event, and no gtag config implying GA4 auto page_view | — |
| `datalayer-ecommerce-event-invalid` | warning | A GA4 e-commerce event (`purchase`, `add_to_cart`, …) is missing required parameters | `event`, `missing` |
| `datalayer-param-type` | warning | An e-commerce parameter has the wrong type (`value` not numeric, `currency` not ISO-4217, `items` not a non-empty array) | `event`, `param`, `want`, `got` |
| `datalayer-duplicate-event` | warning | A conversion event (`purchase`, `generate_lead`, …) fired more than once | `event`, `count` |
| `datalayer-duplicate-transaction` | warning | The same purchase `transaction_id` fired more than once | `transaction_id`, `count` |
| `datalayer-pii` | warning | An email (anywhere) or a phone-shaped value under a phone-like key was pushed into the dataLayer | `kind`, `key`, `value` (redacted) |
| `datalayer-tag-not-firing` | warning | A tag detected in the HTML issued no matching network beacon during render | `tag` |
| `datalayer-tags-firing` | info | Tags confirmed to have issued a network beacon | `tags` |
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

## `consent` — cookie consent & Google Consent Mode (SEA / compliance)

Source: [`internal/analyze/consent/`](../internal/analyze/consent/). Answers two questions:
**is consent asked for correctly**, and **is it respected**.

The second question is answerable because of a property of the crawl itself: **gocrawl never
clicks a consent banner.** Every page it fetches is a visit by someone who has consented to
nothing, so whatever the site sets or sends during that visit is its pre-consent behaviour.

Consent configuration lives in the shared template, so findings are aggregated and **emitted
once per host**; cookies are reported once per distinct name.

### Consent configuration

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `consent-cmp-detected` | info | A consent management platform was identified | `cmp` |
| `consent-no-cmp` | warning | Analytics/advertising tags are present but no CMP was detected | — |
| `consent-mode-v1-only` | warning | Consent Mode omits the v2 signals (`ad_user_data`, `ad_personalization`) | `missing`, `declared` |
| `consent-mode-default-granted` | error | A `default` call grants tracking storage before the visitor chooses | `granted` |
| `consent-mode-no-wait-for-update` | info | No `wait_for_update`, so tags may outrun an async CMP | — |
| `consent-mode-after-tags` | warning | The Consent Mode default is declared *after* the `gtag.js`/`gtm.js` loader | — |

Consent Mode is read from both forms sites use: `gtag('consent', 'default', {…})` and the
`dataLayer.push(['consent', 'default', {…}])` it compiles to. Defaults scoped with the
`region` key are exempt from `consent-mode-default-granted` — granting outside the EEA while
denying inside it is a legitimate configuration.

Roughly 20 CMPs are recognised (Cookiebot, OneTrust, Usercentrics, CookieYes, Cookie-Script,
Complianz, Didomi, Iubenda, Termly, Osano, Sourcepoint, TrustArc, Quantcast, Axeptio,
CookieFirst, Borlabs, Real Cookie Banner, Klaro, tarteaucitron, Cookie Notice), with a generic
IAB TCF fallback for the long tail — any TCF-compliant CMP exposes `__tcfapi`.

### Pre-consent behaviour

| Code | Severity | Triggered when | `data` |
| --- | --- | --- | --- |
| `consent-preconsent-tracking-cookie` | error | A known analytics/advertising cookie was set with no consent given | `cookie`, `vendor`, `purpose`, `domain`, `scope`, `source` |
| `consent-preconsent-tracker-request` | error | Measurement/advertising endpoints were contacted with no consent given | `endpoints`, `examples` |
| `consent-cookie-inventory` | info | Rollup of every cookie observed before consent | `total`, `tracking_count`, `other_count`, `other`, `source` |

> **Render mode changes what is visible, a lot.** Most tracking cookies are set by JavaScript,
> which response headers cannot see:
>
> | | Raw (`--render raw`, default) | Headless (`--render headless`) |
> | --- | --- | --- |
> | Cookie source | `Set-Cookie` headers only | The browser's full cookie jar |
> | JS-set cookies (`_ga`, `_fbp`, …) | ✗ invisible | ✓ captured |
> | Third-party cookies | ✗ invisible | ✓ captured |
> | Pre-consent beacons | ✗ tags never run | ✓ captured |
>
> Every cookie finding carries a `source` field naming the evidence it used, so a clean raw
> result is not mistaken for a clean site. **Run the consent audit with `--render headless`**
> if you want the pre-consent test to mean anything.

#### Limits worth knowing

- **Static CMP detection.** A CMP injected at runtime by a tag manager or an application
  bundle may leave no trace in the served HTML, so `consent-no-cmp` can be a false positive in
  raw mode — confirm before acting on it. (Detection does read resource hints, which catches
  first-party-proxied CMPs like a self-hosted `sourcepoint.<site>.com`.)
- **Vendor names are matched integration-shaped**, never as bare brand names — hostnames,
  script filenames, JS globals, CSS class prefixes. Otherwise a "OneTrust alternative"
  comparison page reads as a OneTrust install.
- **Consent-platform cookies are exempt.** A CMP cannot remember a refusal without storing it,
  so its own state cookie is strictly necessary and is never flagged.
- **"Is Consent Mode wired at all" belongs to [`datalayer`](#datalayer--gtm--datalayer-audit-sea)**
  (`datalayer-consent-mode-present` / `-missing`). This analyzer judges a configuration it can
  see rather than duplicating that finding.
- **Not legal advice.** Whether a specific cookie is lawful depends on context a crawler
  cannot see. These are engineering signals for a human review.

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

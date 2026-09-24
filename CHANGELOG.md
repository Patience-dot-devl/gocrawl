# Changelog

All notable changes to `gocrawl` are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **`redirects` analyzer files findings under the served URL.** `http-client-error`,
  `http-server-error`, `http-body-truncated`, `http-slow-response`, and `http-mixed-content`
  on a page reached through a redirect were attributed to the requested URL; they now use the
  final URL like every other per-page analyzer. The redirect codes themselves still sit on
  the requested URL. Saved reports compared with `gocrawl compare` will show these findings
  as moved, once.
- **robots.txt fetched once per host.** Concurrent workers reaching a new host all fetched
  its robots.txt; the first now fetches and the rest wait for its result.

### Changed

- **Analyzer fetches obey the crawl's politeness rules.** The extra requests analyzers make
  after the crawl — `sitemap.xml`, `llms.txt`, and the `--specialized` WordPress and Shopify
  probes — now go through the crawl's rate limiter and, when `respect_robots` is on, its
  robots.txt policy (`crawler.Engine.Govern`). Previously they ran unthrottled and ignored
  `Disallow` rules the crawl itself honoured.
- **`gocrawl serve` hardening.** The web server binds `127.0.0.1:8080` instead of every
  interface, rejects requests whose `Host` header is not a loopback address (DNS rebinding),
  rejects cross-origin `POST`s (CSRF), requires `Content-Type: application/json` on
  `POST /api/crawls`, caps concurrent crawls at 4 (`429` beyond that), and sets header/idle
  timeouts on the listener. Binding a non-loopback `--addr` relaxes the `Host` check and
  prints a warning, since the API has no authentication.

### Security

- Upgraded `golang.org/x/net` (v0.33.0 → v0.59.0) and pinned the Go toolchain to 1.26.6,
  clearing 15 `govulncheck` findings reachable from the HTML fetch path. CI now runs
  `govulncheck`, and Dependabot tracks Go, npm, and GitHub Actions updates.

## [0.8.0] - 2026-09-17

### Added

- **`shopify` analyzer.** Detects Shopify storefronts and stays silent everywhere else.
  Detection is tiered: the `Shopify.theme` bootstrap object, the `shopify-features` script, or
  the `X-ShopId`/`X-Shopify-Stage` headers identify a store, while weak markers
  (`cdn.shopify.com`, `.myshopify.com`) alone never do, so sites embedding a Buy Button widget
  are not misread as stores. Once detected, every URL is classified into its platform-fixed
  template (home, product, collection, article, blog, page, policy, utility; Markets locale
  prefixes are understood) and checked:
  - `shopify-template-schema-gap` rolls up, per template, the schema.org types its pages are
    missing, with a page count and example URLs.
  - `shopify-schema-app-conflict` flags `Product` markup emitted by both the theme and an SEO
    app; `shopify-schema-client-injected` flags pages whose SEO app likely injects JSON-LD
    client-side, invisible in raw HTML.
  - `shopify-flat-variant-product` and `shopify-single-offer-range` flag product markup that
    flattens away variants or a price range.
  - `shopify-indexable-utility`, `shopify-indexable-facet` and
    `shopify-duplicate-product-path` cover the crawl-hygiene problems Shopify creates by
    default: indexable `/search`/`/cart`/`/account` pages, self-canonical sorted or filtered
    collections, and collection-nested product URLs without a canonical to `/products/<handle>`.
  - `shopify-products-json-exposed` (opt-in via `--specialized`) makes one request to
    `/products.json?limit=1` to check whether the full catalogue is served unauthenticated.
- **Deeper `structured` analyzer.** JSON-LD is now read through a new shared `schemaorg`
  parser that flattens `@graph`, nested objects and arrays into an addressable node graph with
  `@id` resolution. On top of it:
  - Fields are tiered by rich-result requirement, checked against Google's published
    requirements. Required gaps stay per page (`structured-missing-required`, now with a
    `path`); recommended and Google Merchant gaps roll up site-wide into one issue per type
    (`structured-missing-recommended`, `structured-missing-merchant`). `ProductGroup` has its
    own merchant tier, and merchant fields are accepted on variant offers.
  - New integrity checks: `structured-duplicate-type`, `structured-conflicting-value`,
    `structured-unresolved-id`, `structured-relative-url`, `structured-empty-url`,
    `structured-invalid-date`, `structured-malformed-price` and `structured-price-mismatch`
    (price comparison understands European decimal commas and stays silent when the page
    shows more than one price).
  - `structured-variant-incomplete` checks a `ProductGroup`'s inline `hasVariant` entries, and
    `structured-identifier-on-offer` reports `gtin`/`mpn` placed on the `Offer` instead of the
    `Product`.
  - Site-wide rollups count each page once per canonical URL, so a product reached through
    several collection paths is one page.

### Changed

- **Severities.** `structured-invalid-jsonld`, `structured-malformed-price` and
  `structured-price-mismatch` are now `error` (were `warning`); `structured-missing-merchant`
  is `warning`.
- **`structured-breadcrumb-candidate` is site-scoped.** It now aggregates into one issue with
  a page count instead of repeating per page, so `gocrawl compare` against a report saved
  with an earlier version re-keys it once.
- **`structured-product-candidate` is stricter.** A price must sit next to an actual
  cart/buy control (same `<form>` or a bounded ancestor), so a sitewide free-shipping banner
  plus a mini-cart button no longer fires it on every page of a store.
- **`structured-data` and `structured-none`** now account for nested types, and a bare
  top-level `Offer` is no longer required-field checked.
- `--specialized` help text names the Shopify probe alongside the WordPress probes.

### Fixed

- **`gocrawl compare` merged distinct findings.** Rollups that emit several issues for the
  same analyzer, code and URL (one per type or template) were paired as one; issues can now
  carry `data.instance`, and pairing is count-based.
- **Nondeterministic structured-data output.** JSON-LD node order and unresolved-`@id` issue
  order depended on Go map iteration, producing spurious diffs between crawls of an unchanged
  page.

## [0.7.0] - 2026-09-07

### Added

- **Three alt-text states in the `images` analyzer.** The analyzer previously only knew
  "has an `alt` attribute" vs "has none", so a page where every content image carries
  `alt=""` looked clean — the common CMS/theme failure mode. It now distinguishes
  **missing**, **empty** and **present**, and adds the checks that only make sense once alt
  values are compared to each other and to the filename: `img-empty-alt` (info, since
  `alt=""` is correct markup for decorative images — reported with an `images_total` ratio
  rather than flagged as a defect), `img-duplicate-alt` (warning), `img-alt-is-filename`
  (warning, e.g. `kantoor.jpg` with `alt="Kantoor"`) and `img-nondescriptive-filename`
  (info: pure digits, camera/export defaults, content hashes, stems of three characters or
  fewer). Every per-image finding now also carries a full `sources` list (deduplicated,
  capped at 100 per page) alongside the existing five-item `sample`, with image URLs
  resolved against the page URL and falling back to `data-src` / `data-original` / `srcset`
  so lazy-loading themes report the real image instead of a placeholder.
- **`--cookie` flag for app-level session-cookie auth.** Sites gated by a session cookie
  rather than server-level HTTP Basic Auth (a Shopify storefront password page being the
  common case) had no way to authenticate. `--cookie` sends a raw `Cookie` header, scoped
  and leak-guarded exactly like `--basic-auth` (seed host plus subdomains only, no scheme
  downgrade, rejected under `--render headless`). Exposed in the CLI, interactive menu,
  config, MCP, web API and the web UI's crawl form.
- **Ignore external-link UTM tagging by default.** The `utm` analyzer's tagging-quality
  warnings (partial/empty/duplicate/casing) no longer fire for outbound links to other
  domains, since the site owner doesn't control third-party tagging (e.g. a widget's own
  "powered by" badge link). Toggle with `--ignore-external-tagging=false` / `analyzers.
  ignore_external_tagging: false` / `ignore_external_tagging: false` (MCP/web API) to restore
  them.

### Changed

- **Web UI and HTML export unified.** The live SPA and the static HTML export had drifted
  into two visual languages and two feature sets. Both now share one token system (colors,
  type, shadows; dark mode added to the export), and the SPA gained the export's
  functionality: severity toggles, a codes multiselect, sortable/sticky tables, a per-issue
  review workflow with bulk actions and reviewed-JSON export, and a Site map tab driven by
  the report's existing site-map tree. Plus a sectioned crawl form, analyzer
  select-all/none, cancel confirmation, an elapsed-time/progress indicator, and
  history search/sort.

### Fixed

- **`gocrawl mcp` panicked on startup.** The MCP `crawl` tool returned the full
  `report.Report`, whose `SiteMap` embeds a self-referential `*sitemapgen.Node`
  (`Children []*Node`); the go-sdk schema reflector cannot express cyclic types and panicked
  in `AddTool` on every invocation. The tool now returns a flat `CrawlReport` (seed, summary,
  issues, notes, coverage). CLI and web JSON reports are unaffected and still include the
  full site map.

## [0.6.0] - 2026-07-29

### Added

- **Full crawl-option parity in the web UI.** The web form/API now expose everything the
  CLI/interactive menu support — `follow_external`, rate limiting, include/exclude regex,
  User-Agent/proxy pools, and Basic Auth — and the live report view matches the exported HTML
  report, with per-issue explanations, a coverage banner, and analyzer/status breakdowns
  backed by a new `GET /api/explanations` endpoint.

### Fixed

- **Basic Auth host leak.** Credentials configured for the seed host were being sent to
  third-party hosts via `FollowExternal`-followed links and via the sitemap analyzer fetching
  whatever URL a `robots.txt` `Sitemap:` directive names. An `authHostAllowed` guard is now
  threaded through every `HTTPFetcher` instance a crawl builds, independent of
  `FollowExternal`'s scope bypass. Also fixes an IPv6 seed Basic Auth drop caused by using a
  bracket-stripping hostname instead of the bracket-preserving host for the same check.

## [0.5.0] - 2026-07-27

### Added

- **Opt-in security audit (`--security-audit`).** Extends the `security` analyzer with a
  second, off-by-default pass covering the transport and cookie layers: TLS protocol/cipher
  strength, certificate expiry and chain validity, cookie hygiene (`Secure`, `HttpOnly`,
  `SameSite`, `__Host-`/`__Secure-` prefixes), and header policy (HSTS quality, framing
  protection, `Referrer-Policy`, version disclosure). Findings aggregate per host rather than
  per page, since everything it inspects is host-wide server configuration. Surfaced as
  `--security-audit` / `analyzers.security_audit` in YAML / `security_audit` in MCP and the
  web API.
- **Consent analyzer, on by default.** Reports both halves of a GDPR/ePrivacy review: whether
  consent is asked for correctly (CMP detection across ~20 vendors plus a generic IAB TCF
  fallback, Consent Mode v2 signal completeness, granted-by-default, declaration ordering) and
  whether it's respected (classifies cookies observed on gocrawl's pre-consent crawl by
  vendor/purpose, flagging measurement endpoints contacted before consent). Under `--render
  headless` it reads the real browser cookie jar via CDP; in raw mode it degrades to
  `Set-Cookie` headers and labels each finding's evidence with a `source` field.

## [0.4.0] - 2026-07-20

### Added

- **Web UI & REST API (`gocrawl serve`).** Runs gocrawl as a small web application: an
  embedded single-page app (start a crawl, watch it run, browse the report, review history)
  backed by a REST API (`internal/webserver`) that reuses the same `runner.Run` seam as the
  CLI and MCP server — start/poll/cancel a crawl, export a finished report as JSON/CSV/HTML,
  and list crawl history. See [docs/web.md](docs/web.md).
- **Prebuilt binary releases.** Tagged releases now publish cross-compiled binaries for
  linux/darwin/windows (amd64/arm64) via GoReleaser, with the web UI built in. See
  [docs/install.md](docs/install.md#download-a-prebuilt-binary-all-platforms).

### Changed

- The crawl-request-to-config mapping used by the MCP `crawl` tool moved into
  `internal/crawlrequest`, now shared with the new web API.

## [0.3.0] - 2026-06-30

### Added

- **Persistent crawl storage (`gocrawl crawl --save`).** Crawls can now be saved to an on-disk
  store (default `~/.gocrawl/crawls`, configurable via `store.dir`, `--store-dir`, or
  `GOCRAWL_STORE_DIR`). Each crawl is keyed by a sortable, readable `<host>/<timestamp>` ID.
  The store is a thin layer over the existing JSON report, so saved crawls are the same
  artifact `gocrawl render` already consumes.
- **List saved crawls (`gocrawl history [host]`).** Shows saved crawls newest-first as a table
  (or `--format json`), with page counts and issue counts by severity. Pass a host to scope to
  one site.
- **Compare two crawls (`gocrawl compare <base> <current>`).** Diffs an earlier crawl against a
  later one into **new** / **resolved** / **persisting** issues, plus page-set and summary
  deltas, as text or JSON. Each argument is a crawl reference: a report file path, a stored
  crawl ID, the word `latest`, or a bare host name (that site's newest crawl). `--fail-on-new`
  exits non-zero when the current crawl introduces any new issue, for use as a CI regression
  gate. See [docs/storage.md](docs/storage.md).

## [0.2.2] - 2026-06-30

### Added

- **Bulk review in the HTML report.** Each issue row has a **Select** checkbox, a **Select
  shown** master toggle ticks every currently-visible row, and toolbar buttons mark the
  selection as **Non-issue** / **Resolved** (or clear it) in one action. Bulk actions only
  ever touch issues that are *currently shown*, so you can filter to a code or analyzer (e.g.
  all `seo-meta-noindex` on a staging site) and dismiss the whole set at once without
  affecting filtered-out rows. Selection state is transient — only the review flags are saved.

### Fixed

- **`duplicate-title` / `duplicate-meta-description` / `duplicate-content` no longer fire on
  query-string or fragment variants of one page.** URLs that differ only by query parameters
  (e.g. `?solution=onboarding`) or a `#fragment` address the same page, so the duplicates
  analyzer now collapses them to a single page before comparing — a page is no longer reported
  as duplicating itself. Genuinely distinct paths are still flagged.
- **HTML report: the `data` / "what this means" disclosure toggles no longer show a stray
  full-width focus box.** The `<summary>` outline is now hugged to the text and only shown for
  keyboard focus.
- **`link-to-redirect` no longer fires on trailing-slash-only redirects.** The crawl index
  strips a trailing slash to deduplicate URLs, so it would fetch a link authored as `/page/`
  (the canonical form on WordPress and many other CMSes) as `/page` and follow the site's
  `301 → /page/`, then report the link as pointing at a redirect. Every internal link on a
  trailing-slash site was flagged. Links now carry their resolved address with the slash
  preserved, and the analyzer only flags a redirect that genuinely changes scheme, host, or
  path — so real redirects are still caught while the self-induced slash hop is not. The
  issue's `target` now shows the link as authored rather than the slash-stripped form.
- **HTML report links open in a new tab.** Clicking a page URL in the issues table or a node
  label in the site map now opens in a new foreground tab (`target="_blank"`) instead of
  navigating away from the report.

## [0.2.1] - 2026-06-30

### Added

- **`botwall` analyzer — CAPTCHA / bot-challenge detection.** Flags pages that served a
  reCAPTCHA, hCaptcha, Turnstile, or a Cloudflare / DataDome / AWS WAF / PerimeterX / Imperva
  challenge wall instead of the real content, so a silently-blocked crawl isn't mistaken for a
  successful audit. Emits `botwall-challenge` (warning) for walls and `botwall-captcha-widget`
  (info) for a CAPTCHA legitimately embedded on a real page. Scans the body, response headers,
  and (in headless mode) captured request URLs. (#31)
- **User-Agent in the interactive menu.** The bare `gocrawl` command now accepts
  `--user-agent` (e.g. `gocrawl --user-agent endeavour-bot`) to pre-fill the menu, and the
  menu has a User-Agent field — handy when a site allow-lists a specific UA to exempt the
  crawler from a CAPTCHA. `gocrawl crawl --user-agent` is unchanged. (#32)
- **"Keep this Mac awake" toggle in the interactive menu.** On macOS the menu (`gocrawl` with
  no arguments) now offers a keep-awake toggle that holds a `caffeinate -i` power assertion for
  the duration of the crawl, so a locked screen or idle-sleep timer doesn't pause a long crawl
  or drop in-flight connections. The toggle is hidden on platforms without `caffeinate`. For
  non-interactive runs, wrap the command: `caffeinate -i gocrawl crawl …`.
- **Crawl coverage signal** — the report now reports whether the crawl actually reached the
  whole site. When a depth or page limit leaves in-scope URLs un-fetched, a `coverage` object
  is emitted, a `notes` advisory names the limit, and the HTML report shows a prominent
  **"Partial coverage"** banner. This stops `0 broken links` from being misread as a clean
  site when the broken links simply weren't reached. (#29)

### Changed

- **Issue codes are now consistently prefixed with their analyzer name** so a report sorts and
  filters cleanly by analyzer (e.g. `missing-title` → `seo-missing-title`, `broken-link` →
  `link-broken`, `cls-poor` → `perf-cls-poor`, `no-robots` → `robots-missing`, `bot-challenge`
  → `botwall-challenge`). The `redirects` analyzer's HTTP-level codes use the `http-` prefix
  (e.g. `http-client-error`, `http-redirect-chain`). The already-consistent short prefixes
  `wp-`, `img-`, and `url-` are unchanged. **Breaking:** any tooling that matches on issue
  `code` strings (saved JSON reports, scripts, dashboards) must be updated to the new codes.
  See [docs/analyzers.md](docs/analyzers.md) for the full list.
- **The crawl is now bounded by total pages, not link depth, by default.** `--depth`/`max_depth`
  defaults to `0` (unlimited) and `--max-pages` (500) is the primary bound, so a default crawl
  walks the whole site up to the page budget instead of stopping shallow at depth 2.
  **Breaking:** `--depth 0` now means *unlimited* (it previously meant *seed only*). (#29)

### Fixed

- **Headless rendering no longer reports false `seo-missing-h1` / `seo-missing-*` /
  `content-thin` on slow pages.** When a page is snapshotted before it finishes rendering, the
  rendered DOM comes back far thinner than the raw HTML; the renderer now detects this and
  analyzes the raw HTML instead, and emits a `perf-render-incomplete` warning marking that
  page's Core Web Vitals as unreliable. (#30)

## [0.2.0] - 2026-06-30

### Added

- **`gocrawl render <report.json>`** — re-emit a saved JSON report as HTML (or CSV) **without
  recrawling**. The fast way to regenerate a report after a gocrawl upgrade, or to produce
  another format from a JSON you already have. Mirrors `crawl`'s output flags
  (`--out`/`-o`, `--format`/`-f` default `html`, `--sitemap`). (#27)
- **Visual site map** — the HTML report's Site map tab is now a node-link (org-chart-style)
  diagram: each page is a card colored by health, connected by elbow lines, with collapsible
  branches, Expand/Collapse-all controls, and a click-to-open issues popover per node. (#27)
- **Multi-select issue-code filter** in the HTML report toolbar — uncheck codes in the *Codes*
  dropdown to hide them (e.g. silence `meta-noindex` / `x-robots-noindex` noise when auditing a
  deliberately-noindexed staging site). Composes with the existing search, severity, and
  analyzer filters. (#26)

### Changed

- The JSON report now includes the site-map tree under `site_map`, so a JSON report is a
  complete artifact that `gocrawl render` can turn back into HTML (including the Site map tab)
  without recrawling. (#27)

## [0.1.0] - 2026-06-29

Initial public release: a concurrent website crawler for SEO/SEA audits with a pluggable
analyzer pipeline (technical SEO, redirects, broken links, `robots.txt`, `sitemap.xml`
coverage, structured data, Core Web Vitals, and AI-search readiness), JSON / CSV / HTML
reports, standalone `sitemap.xml` output, and an MCP server for agentic tooling.

[Unreleased]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Patience-dot-devl/gocrawl/releases/tag/v0.1.0

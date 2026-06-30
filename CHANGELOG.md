# Changelog

All notable changes to `gocrawl` are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`botwall` analyzer — CAPTCHA / bot-challenge detection.** Flags pages that served a
  reCAPTCHA, hCaptcha, Turnstile, or a Cloudflare / DataDome / AWS WAF / PerimeterX / Imperva
  challenge wall instead of the real content, so a silently-blocked crawl isn't mistaken for a
  successful audit. Emits `bot-challenge` (warning) for walls and `captcha-widget` (info) for
  a CAPTCHA legitimately embedded on a real page. Scans the body, response headers, and (in
  headless mode) captured request URLs.
- **Crawl coverage signal** — the report now reports whether the crawl actually reached the
  whole site. When a depth or page limit leaves in-scope URLs un-fetched, a `coverage` object
  is emitted, a `notes` advisory names the limit, and the HTML report shows a prominent
  **"Partial coverage"** banner. This stops `0 broken links` from being misread as a clean
  site when the broken links simply weren't reached.

### Changed

- **The crawl is now bounded by total pages, not link depth, by default.** `--depth`/`max_depth`
  defaults to `0` (unlimited) and `--max-pages` (500) is the primary bound, so a default crawl
  walks the whole site up to the page budget instead of stopping shallow at depth 2.
  **Breaking:** `--depth 0` now means *unlimited* (it previously meant *seed only*).

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

[Unreleased]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Patience-dot-devl/gocrawl/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Patience-dot-devl/gocrawl/releases/tag/v0.1.0

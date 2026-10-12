# gocrawl documentation

Deep reference docs for [gocrawl](../README.md), the FOSS SEO/SEA website crawler. Start with
the [project README](../README.md) for install and a quick start; come here for the details.

<!-- Generated from each document's frontmatter by `make docs`; do not edit by hand. -->
<!-- index:start -->
| Guide | What's in it |
| --- | --- |
| [Install & run](install.md) | Per-platform install on Windows, macOS, and Linux — `go install`, building from source, PATH setup, verifying, running, and the optional headless browser. |
| [Configuration](configuration.md) | Every option, flag, env var, and default; config layering; analyzer selection; annotated example config. |
| [Analyzers](analyzers.md) | What each of the twenty-five analyzers checks (technical SEO + SEA + AI search + WordPress + Shopify), with every issue code, severity, threshold, and `data` field. |
| [Output / report](output.md) | The JSON, CSV, and HTML report formats — `Report`, `Summary`, `Issue` — with examples. |
| [Storage & comparison](storage.md) | Saving crawls (`--save`), listing them (`gocrawl history`), and diffing them over time (`gocrawl compare`, with a `--fail-on-new` CI gate). |
| [Redirect-rule verification](redirect-check.md) | Checking a HubSpot-style redirect-rule CSV export against a live site with `gocrawl check-redirects`. |
| [MCP server](mcp.md) | Running gocrawl as an MCP server, registering it with clients, and the `crawl` / `list_analyzers` tool schemas. |
| [Web UI](web.md) | Running `gocrawl serve`, the embedded browser UI, the REST API it's built on, and building the frontend from source. |
| [Architecture](architecture.md) | How the engine, analyzer pipeline, and report builder fit together; the package map; adding an analyzer. |
| [Roadmap](roadmap.md) | What's shipped (incl. the SEA and AI-search analyzers), what's in progress, and what's planned. |

### Design records

Dated specs, plans, and findings written while building gocrawl. They are a historical
record: `archived` ones describe the code as it was planned, not as it is now.

**Specs**

| Record | Status | Updated |
| --- | --- | --- |
| [`check-redirects`: verify a redirect-rule export against a live site](superpowers/specs/2026-07-13-check-redirects-design.md) | archived | 2026-07-13 |
| [Ignore UTM tagging-quality warnings on external links, by default](superpowers/specs/2026-07-29-ignore-external-utm-tagging-design.md) | archived | 2026-07-29 |
| [Structured-data depth and a Shopify analyzer](superpowers/specs/2026-09-14-structured-data-shopify-design.md) | archived | 2026-09-14 |
| [Fixes from the dermalogica.nl functional test](superpowers/specs/2026-09-17-dermalogica-functional-fixes-design.md) | archived | 2026-09-17 |
| [Ranking structured-data opportunities](superpowers/specs/2026-10-09-structured-opportunity-ranking-design.md) | current | 2026-10-12 |

**Plans**

| Record | Status | Updated |
| --- | --- | --- |
| [check-redirects Implementation Plan](superpowers/plans/2026-07-13-check-redirects.md) | archived | 2026-07-13 |
| [Ignore External UTM Tagging Implementation Plan](superpowers/plans/2026-07-29-ignore-external-utm-tagging.md) | archived | 2026-07-29 |
| [Structured-Data Depth and Shopify Analyzer Implementation Plan](superpowers/plans/2026-09-14-structured-data-shopify.md) | archived | 2026-09-14 |

**Findings**

| Record | Status | Updated |
| --- | --- | --- |
| [Structured-data & Shopify analyzer — follow-up work](superpowers/findings/2026-09-14-structured-data-shopify-followups.md) | current | 2026-09-17 |
<!-- index:end -->

Contributing? See [CONTRIBUTING.md](../CONTRIBUTING.md).

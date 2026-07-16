# gocrawl documentation

Deep reference docs for [gocrawl](../README.md), the FOSS SEO/SEA website crawler. Start with
the [project README](../README.md) for install and a quick start; come here for the details.

| Guide | What's in it |
| --- | --- |
| [Install & run](install.md) | Per-platform install on Windows, macOS, and Linux — `go install`, building from source, PATH setup, verifying, running, and the optional headless browser. |
| [Configuration](configuration.md) | Every option, flag, env var, and default; config layering; analyzer selection; annotated example config. |
| [Analyzers](analyzers.md) | What each of the twenty-one analyzers checks (technical SEO + SEA + AI search + WordPress), with every issue code, severity, threshold, and `data` field. |
| [Output / report](output.md) | The JSON, CSV, and HTML report formats — `Report`, `Summary`, `Issue` — with examples. |
| [Storage & comparison](storage.md) | Saving crawls (`--save`), listing them (`gocrawl history`), and diffing them over time (`gocrawl compare`, with a `--fail-on-new` CI gate). |
| [Redirect-rule verification](redirect-check.md) | Checking a HubSpot-style redirect-rule CSV export against a live site with `gocrawl check-redirects`. |
| [MCP server](mcp.md) | Running gocrawl as an MCP server, registering it with clients, and the `crawl` / `list_analyzers` tool schemas. |
| [Architecture](architecture.md) | How the engine, analyzer pipeline, and report builder fit together; the package map; adding an analyzer. |
| [Roadmap](roadmap.md) | What's shipped (incl. the SEA analyzers), what's stubbed, and what's planned (headless rendering, and more). |

Contributing? See [CONTRIBUTING.md](../CONTRIBUTING.md).

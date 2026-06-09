# gocrawl documentation

Deep reference docs for [gocrawl](../README.md), the FOSS SEO/SEA website crawler. Start with
the [project README](../README.md) for install and a quick start; come here for the details.

| Guide | What's in it |
| --- | --- |
| [Configuration](configuration.md) | Every option, flag, env var, and default; config layering; analyzer selection; annotated example config. |
| [Analyzers](analyzers.md) | What each of the ten analyzers checks (SEO + SEA), with every issue code, severity, threshold, and `data` field. |
| [Output / report](output.md) | The JSON, CSV, and HTML report formats — `Report`, `Summary`, `Issue` — with examples. |
| [MCP server](mcp.md) | Running gocrawl as an MCP server, registering it with clients, and the `crawl` / `list_analyzers` tool schemas. |
| [Architecture](architecture.md) | How the engine, analyzer pipeline, and report builder fit together; the package map; adding an analyzer. |
| [Roadmap](roadmap.md) | What's shipped (incl. the SEA analyzers), what's stubbed, and what's planned (headless rendering, and more). |

Contributing? See [CONTRIBUTING.md](../CONTRIBUTING.md).

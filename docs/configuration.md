# Configuration reference

gocrawl reads its settings from four layers. Later layers override earlier ones:

1. **Built-in defaults** — see [`config.Default()`](../internal/config/config.go) and
   [`crawler.DefaultOptions()`](../internal/crawler/types.go).
2. **YAML config file** — passed with `--config <path>` (`-c`). Generate a starting file
   with `gocrawl init`.
3. **Environment variables** — `GOCRAWL_*` (see [Environment variables](#environment-variables)).
4. **Command-line flags** — e.g. `--depth`, `--max-pages`.

So a value set in YAML overrides the default, an env var overrides YAML, and a CLI flag
overrides everything.

> The positional seed URL argument to `gocrawl crawl <url>` overrides the `seed:` field in
> the config file.

## Options

Every option, its YAML key, the CLI flag that sets it (if any), type, and the **effective
default** (the value used when you set nothing).

| YAML key | CLI flag | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `seed` | *(positional arg)* | string | — | Seed URL to start from. The `crawl <url>` argument overrides it. |
| `render` | `--render` | string | `raw` | `raw` (HTTP fetch) or `headless` (chromedp — renders JS and captures Core Web Vitals; requires a Chromium-class browser on PATH). |
| `crawl.max_depth` | `--depth` / `-d` | int | `2` | Link hops from the seed (`0` = seed page only). |
| `crawl.max_pages` | `--max-pages` | int | `500` | Hard cap on pages crawled (`0` = unlimited). |
| `crawl.concurrency` | `--concurrency` | int | `4` | Parallel fetch workers (`<=0` is treated as 1). |
| `crawl.rate_per_second` | `--rate` | float | `0` | Max requests/second across the crawl (`0` = unlimited). |
| `crawl.user_agent` | `--user-agent` | string | `gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)` | `User-Agent` header sent on every request. |
| `crawl.timeout` | — | duration | `15s` | Per-request timeout (e.g. `"10s"`, `"500ms"`). |
| `crawl.max_body_bytes` | — | int | `5242880` (5 MiB) | Cap on a single response body. |
| `crawl.respect_robots` | `--respect-robots` | bool | `true` | Obey `robots.txt` while crawling. |
| `crawl.allow_subdomains` | `--subdomains` | bool | `false` | Follow links to subdomains of the seed host. |
| `crawl.follow_external` | `--external` | bool | `false` | Crawl links that leave the seed host. |
| `crawl.follow_nofollow` | — | bool | `false` | Follow links marked `rel="nofollow"`. |
| `crawl.include` | `--include` | list of regex | *(none)* | Only crawl URLs matching at least one pattern. |
| `crawl.exclude` | `--exclude` | list of regex | *(none)* | Skip URLs matching any pattern. |
| `output.format` | `--format` / `-f` | string | `json` | `json`, `csv`, or `html`. |
| `output.path` | `--out` / `-o` | string | *(empty = stdout)* | File to write the report to. |
| `analyzers.enabled` | `--analyzers` | list | *(empty)* | Allow-list of analyzers (see below). |
| `analyzers.disabled` | — | list | *(empty)* | Deny-list of analyzers (see below). |
| `analyzers.specialized` | `--specialized` | bool | `false` | Enable the opt-in specialized AI-search checks (see below). |

### A note on flag defaults

The `--depth`, `--max-pages`, and `--concurrency` flags show `0` as their flag default, but
a flag is only applied when you actually pass it. If you omit the flag, the value comes from
the config file or the built-in defaults in the table above (depth `2`, max-pages `500`,
concurrency `4`). Passing `--max-pages 0` explicitly means *unlimited*.

### include / exclude patterns

`include` and `exclude` are [Go regular expressions](https://pkg.go.dev/regexp/syntax)
matched against the full URL string. `exclude` is evaluated first; if `include` is non-empty,
a URL must match at least one include pattern to be crawled. The example config excludes
common asset types:

```yaml
crawl:
  exclude:
    - "\\.(?:png|jpe?g|gif|svg|webp|ico|css|js|pdf|zip)(?:\\?|$)"
```

## Selecting analyzers

Which analyzers run is decided by `analyzers.enabled` / `analyzers.disabled` (see
[`Registry.Select`](../internal/analyze/analyze.go)):

- If **`enabled` is non-empty**, only those analyzers run (any unknown names are ignored).
- Otherwise **all** analyzers run **except** any listed in `disabled`.

The `--analyzers` CLI flag sets `enabled`. Analyzer names: `seo`, `redirects`, `links`,
`robots`, `sitemap`, `structured`, `perf`, `images`, `urls`, `security`, `pagination`,
`hreflang`, `amp`, `duplicates`, `content`, the SEA analyzers `utm`, `tracking`, `landing`,
and the AI-search analyzers `aeo`, `geo`. See the [Analyzer reference](analyzers.md).

```sh
# Only SEO, links, and redirects
gocrawl crawl https://example.com --analyzers seo,links,redirects
```

```yaml
analyzers:
  disabled: ["perf"]   # run everything except perf
```

### Specialized AI-search checks

`analyzers.specialized` (or the `--specialized` flag) is independent of the allow/deny lists.
It turns on two opt-in, lower-confidence AI-search heuristics that are off by default —
`aeo-no-answer-lead` and `geo-low-quotable-density`. The `aeo` and `geo` analyzers always run
their other checks; this toggle only adds these two. See the
[Specialized AI-search checks](analyzers.md#specialized-ai-search-checks) note for details.

```sh
gocrawl crawl https://example.com --specialized
```

## Environment variables

Environment variables use the prefix `GOCRAWL_`, with the config key uppercased and dots
replaced by underscores. They override the YAML file and are overridden by CLI flags.

| Config key | Environment variable |
| --- | --- |
| `crawl.max_depth` | `GOCRAWL_CRAWL_MAX_DEPTH` |
| `crawl.max_pages` | `GOCRAWL_CRAWL_MAX_PAGES` |
| `crawl.concurrency` | `GOCRAWL_CRAWL_CONCURRENCY` |
| `crawl.rate_per_second` | `GOCRAWL_CRAWL_RATE_PER_SECOND` |
| `crawl.user_agent` | `GOCRAWL_CRAWL_USER_AGENT` |
| `crawl.timeout` | `GOCRAWL_CRAWL_TIMEOUT` |
| `crawl.max_body_bytes` | `GOCRAWL_CRAWL_MAX_BODY_BYTES` |
| `crawl.respect_robots` | `GOCRAWL_CRAWL_RESPECT_ROBOTS` |
| `render` | `GOCRAWL_RENDER` |
| `output.format` | `GOCRAWL_OUTPUT_FORMAT` |

```sh
GOCRAWL_CRAWL_MAX_DEPTH=1 GOCRAWL_CRAWL_CONCURRENCY=8 gocrawl crawl https://example.com
```

> Environment overrides are wired for the core crawl options listed above (the ones with
> built-in defaults in [`config.go`](../internal/config/config.go)). For options without a
> built-in default — `seed`, `output.path`, `crawl.allow_subdomains`, `crawl.follow_external`,
> `crawl.follow_nofollow`, `crawl.include`, `crawl.exclude`, and the analyzer lists — set them
> in the YAML file or via CLI flags rather than the environment.

## Example config file

`gocrawl init` writes the fully-commented template below (also at
[`configs/example.yaml`](../configs/example.yaml)):

```yaml
# gocrawl configuration file
seed: "https://example.com"
render: "raw"            # "raw" or "headless" (stub)

crawl:
  max_depth: 2           # link hops from the seed (0 = only the seed page)
  max_pages: 500         # hard cap on the number of pages crawled
  concurrency: 4         # number of parallel fetch workers
  rate_per_second: 0     # max requests/second across the crawl (0 = unlimited)
  user_agent: "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)"
  timeout: "15s"         # per-request timeout
  max_body_bytes: 5242880  # 5 MiB cap on a single response body
  respect_robots: true   # obey robots.txt while crawling
  allow_subdomains: false  # follow links to subdomains of the seed host
  follow_external: false   # crawl links that leave the seed host
  follow_nofollow: false   # follow links marked rel="nofollow"
  include: []            # only crawl URLs matching at least one of these regexes
  exclude:               # never crawl URLs matching any of these regexes
    - "\\.(?:png|jpe?g|gif|svg|webp|ico|css|js|pdf|zip)(?:\\?|$)"

output:
  format: "json"         # "json", "csv", or "html"
  path: ""               # file to write to; empty = stdout

analyzers:
  enabled: []            # if non-empty, only these run
  disabled: []           # otherwise all run except these
```

Run it with:

```sh
gocrawl crawl https://example.com --config gocrawl.yaml
```

## See also

- [Analyzer reference](analyzers.md) — what each analyzer checks and emits.
- [Output reference](output.md) — the report schema.
- [MCP server guide](mcp.md) — the same options exposed as MCP tool arguments.

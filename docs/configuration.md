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
| `crawl.max_depth` | `--depth` / `-d` | int | `0` | Link hops from the seed (`0` = unlimited). By default the crawl is bounded by `max_pages`, not depth, so it walks the whole site rather than stopping shallow. |
| `crawl.max_pages` | `--max-pages` | int | `500` | Hard cap on pages crawled (`0` = unlimited) — the primary bound on crawl size. When it (or a depth limit) leaves in-scope URLs un-fetched, the report flags **partial coverage** (see [Output](output.md#coverage)). |
| `crawl.concurrency` | `--concurrency` | int | `4` | Parallel fetch workers (`<=0` is treated as 1). |
| `crawl.rate_per_second` | `--rate` | float | `0` | Max requests/second across the crawl (`0` = unlimited). |
| `crawl.adaptive_delay` | `--adaptive-delay` | bool | `true` | Automatically slow the crawl when the server responds with HTTP 429/503: the requests-per-second rate is halved on each trigger (down to one request every 10s), and any `Retry-After` header is honored. See below. |
| `crawl.verbose` | `--verbose` / `-v` | bool | `false` | Log each fetch (URL, status, duration) and every rate-limit change to stderr while crawling. |
| `crawl.user_agent` | `--user-agent` | string | `gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)` | `User-Agent` header sent on every request. Useful when a site allow-lists a specific UA to exempt the crawler from a CAPTCHA. The bare `gocrawl` command also accepts `--user-agent` to pre-fill the interactive menu, and the menu has a User-Agent field. |
| `crawl.user_agents` | `--user-agents` | list of string | *(none)* | Pool of `User-Agent` strings to rotate across; supersedes `user_agent` when set. See [Rotating proxies and User-Agents](#rotating-proxies-and-user-agents). |
| `crawl.user_agent_rotation` | `--user-agent-rotation` | string | `round-robin` | How a multi-entry `user_agents` pool is picked: `off`, `round-robin`, or `random`. |
| `crawl.proxy` | `--proxy` | string | *(none)* | Route requests through this proxy URL (`http(s)://` or `socks5://`; supports `user:pass@host`). Prepended to `proxies` when both are set. |
| `crawl.proxies` | `--proxies` | list of string | *(none)* | Pool of proxy URLs to rotate across. See [Rotating proxies and User-Agents](#rotating-proxies-and-user-agents). |
| `crawl.proxy_rotation` | `--proxy-rotation` | string | `round-robin` | How a multi-entry proxy pool is picked: `off`, `round-robin`, `random`, or `sticky-host`. |
| `crawl.basic_auth` | `--basic-auth` | string | *(none)* | HTTP Basic Auth credentials as `user:pass`, for sites gated by server-level Basic Auth (common on staging/acceptance environments). See [HTTP Basic Auth](#http-basic-auth). The interactive menu has separate username/password fields for this. |
| `crawl.timeout` | — | duration | `15s` | Per-request timeout (e.g. `"10s"`, `"500ms"`). |
| `crawl.max_duration` | `--max-duration` | duration | `0` (unlimited) | Wall-clock budget for the whole crawl (e.g. `"90m"`). On expiry the crawl stops early and still writes a report from whatever was fetched, flagged as **partial coverage** (see [Output](output.md#coverage)) — the same mechanism a Ctrl-C interruption uses. |
| `crawl.max_body_bytes` | — | int | `5242880` (5 MiB) | Cap on a single response body. |
| `crawl.respect_robots` | `--respect-robots` | bool | `true` | Obey `robots.txt` while crawling. |
| `crawl.allow_subdomains` | `--subdomains` | bool | `false` | Follow links to subdomains of the seed host. |
| `crawl.follow_external` | `--external` | bool | `false` | Crawl links that leave the seed host. |
| `crawl.follow_nofollow` | — | bool | `false` | Follow links marked `rel="nofollow"`. |
| `crawl.strip_query` | `--strip-query` | bool | `false` | Ignore query strings (treat `?a=1` and `?a=2` as one URL). Skips the query-dependent analyzers — see below. |
| `crawl.include` | `--include` | list of regex | *(none)* | Only crawl URLs matching at least one pattern. |
| `crawl.exclude` | `--exclude` | list of regex | *(none)* | Skip URLs matching any pattern. |
| `output.format` | `--format` / `-f` | string | `json` | `json`, `csv`, or `html`. |
| `output.path` | `--out` / `-o` | string | *(empty = stdout)* | File to write the report to. |
| `output.sitemap_path` | `--sitemap` | string | *(none)* | Also write a standard `sitemap.xml` of crawled pages here. The HTML report always includes a [Site map tab](output.md#site-map). |
| `analyzers.enabled` | `--analyzers` | list | *(empty)* | Allow-list of analyzers (see below). |
| `analyzers.disabled` | — | list | *(empty)* | Deny-list of analyzers (see below). |
| `analyzers.specialized` | `--specialized` | bool | `false` | Enable the opt-in specialized checks: AI-search heuristics and WordPress security probes (see below). |

### A note on flag defaults

The `--depth`, `--max-pages`, and `--concurrency` flags show `0` as their flag default, but
a flag is only applied when you actually pass it. If you omit the flag, the value comes from
the config file or the built-in defaults in the table above (depth `0` = unlimited, max-pages
`500`, concurrency `4`). Passing `--max-pages 0` or `--depth 0` explicitly means *unlimited*.

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

### Rotating proxies and User-Agents

For large audits — or sites that throttle or block a single client IP / `User-Agent` — gocrawl
can spread requests across a pool of proxies and/or `User-Agent` strings. This is meant for
crawling sites you own or are authorized to audit; it does not change robots.txt or rate-limit
behavior (see the note at the end).

**User-Agents.** Set a pool with `user_agents` (or `--user-agents`). When non-empty it
supersedes the single `user_agent`, and `user_agent_rotation` decides how one is chosen per
request:

- `off` — always use the first entry.
- `round-robin` (default) — cycle through the list in order.
- `random` — pick a uniformly random entry per request.

```sh
gocrawl crawl https://example.com \
  --user-agents "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ...,Mozilla/5.0 (Macintosh; ...)" \
  --user-agent-rotation random
```

A common audit use is **cloaking detection** — crawl once with a Googlebot `User-Agent` and
once with a browser one, then diff the reports to spot UA-conditional serving.

**Proxies.** Set a single proxy with `proxy` / `--proxy`, or a pool with `proxies` /
`--proxies` (the single `proxy`, if also set, is prepended to the pool). Each entry is an
`http://`, `https://`, or `socks5://` URL; a bare `host:port` is treated as `http://`.
Credentials may be embedded as `user:pass@host` (raw mode only). `proxy_rotation` selects one
per request:

- `off` — always use the first proxy.
- `round-robin` (default) — cycle through the pool in order.
- `random` — pick a uniformly random proxy per request.
- `sticky-host` — map each target host to a fixed proxy (by hash), so every request to one host
  leaves from the same IP. Useful to avoid a server seeing one session arrive from several IPs.

```sh
gocrawl crawl https://example.com \
  --proxies "http://user:pass@gw1.proxy.net:8080,socks5://gw2.proxy.net:1080" \
  --proxy-rotation sticky-host
```

When no proxy is configured, gocrawl honors the standard `HTTP_PROXY` / `HTTPS_PROXY` /
`NO_PROXY` environment variables (Go's default behavior).

**Headless mode caveats.** Under `--render headless`, `User-Agent` rotation still applies
per page (via a CDP override), but Chromium accepts only one proxy per browser process, so
headless mode uses the **first** proxy in the pool and proxy credentials are not sent. Run raw
mode (the default) for full per-request proxy rotation.

**This is not an evasion switch.** Rotation is independent of `respect_robots` and the rate
limiter / adaptive delay, which remain per-crawl and stay in force. The intended use is keeping
authorized audits from tripping WAF false-positives and testing UA/IP-conditional serving — not
bypassing access controls.

### HTTP Basic Auth

Some sites — most often staging/acceptance environments — sit behind a reverse-proxy realm
challenge that gates the entire host with HTTP Basic Auth, independent of any app-level login.
Set `basic_auth` (or `--basic-auth`) to `user:pass` to send credentials on every request to that
host:

```sh
gocrawl crawl https://staging.example.com --basic-auth "svc-crawler:s3cret"
```

**Scoping.** The `Authorization` header is only sent to the host you asked gocrawl to crawl,
over the scheme you requested. If a page redirects to a *different* host, or downgrades from
`https` to plain `http`, the header is not carried along — so credentials for the protected site
can't leak to a redirect target on another domain or in cleartext.

**Not supported with `--render headless`.** Chromium's extra-headers mechanism has no per-host
equivalent: it would attach the `Authorization` header to every request the page makes,
including third-party subresources (fonts, analytics, ads) on other hosts. `--basic-auth` with
`--render headless` is rejected with an error; use raw mode (the default) for auth-gated sites.

## Selecting analyzers

Which analyzers run is decided by `analyzers.enabled` / `analyzers.disabled` (see
[`Registry.Select`](../internal/analyze/analyze.go)):

- If **`enabled` is non-empty**, only those analyzers run (any unknown names are ignored).
- Otherwise **all** analyzers run **except** any listed in `disabled`.

The `--analyzers` CLI flag sets `enabled`. Analyzer names: `seo`, `redirects`, `links`,
`robots`, `sitemap`, `structured`, `perf`, `images`, `urls`, `security`, `pagination`,
`hreflang`, `amp`, `duplicates`, `content`, the CMS-specific `wordpress`, the SEA analyzers
`utm`, `tracking`, `landing`, and the AI-search analyzers `aeo`, `geo`. See the
[Analyzer reference](analyzers.md).

```sh
# Only SEO, links, and redirects
gocrawl crawl https://example.com --analyzers seo,links,redirects
```

```yaml
analyzers:
  disabled: ["perf"]   # run everything except perf
```

### strip_query and query-dependent analyzers

`crawl.strip_query` drops the query string while crawling, so URLs that differ only by their
query collapse to one. Because of that, analyzers that read query parameters — `utm` (UTM tags
on outbound links), `landing` (campaign `utm_*` params identify landing pages), and `wordpress`
(the `?p=`, `?attachment_id=`, `?s=` URL checks) — would have nothing to inspect. When
`strip_query` is on they are **automatically skipped** rather than run with empty input, and the
report records a `notes` entry (printed as a `note:` line on the CLI) listing what was skipped.
This is independent of the `enabled`/`disabled` lists: a query-dependent analyzer is skipped even
if you explicitly enable it alongside `strip_query`.

### Adaptive crawl delay (HTTP 429/503)

`crawl.adaptive_delay` (on by default) makes the crawl back off automatically when a server
signals it is overloaded. On any HTTP **429 Too Many Requests** or **503 Service Unavailable**
response, the engine halves its effective requests-per-second and applies that new rate to all
subsequent fetches. Repeated triggers keep halving the rate down to a floor of one request every
10 seconds. If the crawl was running unrestricted (`rate_per_second: 0`), the first trigger drops
it to 1 req/s before halving from there.

A burst of 429s arriving close together (e.g. several concurrent workers) backs the crawl off
once rather than collapsing straight to the floor — adjustments within 2 seconds of the last one
are ignored. When the response carries a `Retry-After` header (delay-seconds or HTTP-date), the
crawl never goes faster than it asks, even below the heuristic floor.

Turn it off with `--adaptive-delay=false` if you want a fixed rate regardless of server
responses. Pair it with `--verbose` to see each rate change logged as it happens.

### Specialized checks

`analyzers.specialized` (or the `--specialized` flag) is independent of the allow/deny lists.
It turns on opt-in checks that are off by default: two lower-confidence AI-search heuristics
(`aeo-no-answer-lead` and `geo-low-quotable-density`) and the `wordpress` analyzer's active
security-endpoint probes (`wp-xmlrpc-enabled`, `wp-user-enumeration-rest`,
`wp-user-enumeration-author`, `wp-directory-listing`, `wp-readme-exposed`). The affected
analyzers always run their other checks; this toggle only adds these. See the
[Specialized AI-search checks](analyzers.md#specialized-ai-search-checks) note for details.

```sh
gocrawl crawl https://example.com --specialized
```

### Crawl store

`store.dir` (or the `--store-dir` flag, or `GOCRAWL_STORE_DIR`) sets where `gocrawl crawl
--save` writes crawls and where `gocrawl history` / `gocrawl compare` read them. Empty means
`~/.gocrawl/crawls`. See [Storage & comparison](storage.md) for the full workflow.

## Environment variables

Environment variables use the prefix `GOCRAWL_`, with the config key uppercased and dots
replaced by underscores. They override the YAML file and are overridden by CLI flags.

| Config key | Environment variable |
| --- | --- |
| `crawl.max_depth` | `GOCRAWL_CRAWL_MAX_DEPTH` |
| `crawl.max_pages` | `GOCRAWL_CRAWL_MAX_PAGES` |
| `crawl.concurrency` | `GOCRAWL_CRAWL_CONCURRENCY` |
| `crawl.rate_per_second` | `GOCRAWL_CRAWL_RATE_PER_SECOND` |
| `crawl.adaptive_delay` | `GOCRAWL_CRAWL_ADAPTIVE_DELAY` |
| `crawl.user_agent` | `GOCRAWL_CRAWL_USER_AGENT` |
| `crawl.timeout` | `GOCRAWL_CRAWL_TIMEOUT` |
| `crawl.max_duration` | `GOCRAWL_CRAWL_MAX_DURATION` |
| `crawl.max_body_bytes` | `GOCRAWL_CRAWL_MAX_BODY_BYTES` |
| `crawl.respect_robots` | `GOCRAWL_CRAWL_RESPECT_ROBOTS` |
| `render` | `GOCRAWL_RENDER` |
| `output.format` | `GOCRAWL_OUTPUT_FORMAT` |
| `store.dir` | `GOCRAWL_STORE_DIR` |

```sh
GOCRAWL_CRAWL_MAX_DEPTH=1 GOCRAWL_CRAWL_CONCURRENCY=8 gocrawl crawl https://example.com
```

> Environment overrides are wired for the core crawl options listed above (the ones with
> built-in defaults in [`config.go`](../internal/config/config.go)). For options without a
> built-in default — `seed`, `output.path`, `output.sitemap_path`, `crawl.allow_subdomains`,
> `crawl.follow_external`, `crawl.follow_nofollow`, `crawl.include`, `crawl.exclude`, and the
> analyzer lists — set them in the YAML file or via CLI flags rather than the environment.

## Example config file

`gocrawl init` writes the fully-commented template below (also at
[`configs/example.yaml`](../configs/example.yaml)):

```yaml
# gocrawl configuration file
seed: "https://example.com"
render: "raw"            # "raw" or "headless" (stub)

crawl:
  max_depth: 0           # link hops from the seed (0 = unlimited; bounded by max_pages)
  max_pages: 500         # hard cap on the number of pages crawled (the primary bound)
  concurrency: 4         # number of parallel fetch workers
  rate_per_second: 0     # max requests/second across the crawl (0 = unlimited)
  adaptive_delay: true   # slow down automatically on HTTP 429/503 responses
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
  sitemap_path: ""       # also write a standard sitemap.xml here

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

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
| `analyzers.security_audit` | `--security-audit` | bool | `false` | Enable the opt-in security audit: TLS/certificate, cookie, and response-header checks (see below). |

### A note on flag defaults

The `--depth`, `--max-pages`, and `--concurrency` flags show `0` as their flag default, but
a flag is only applied when you actually pass it. If you omit the flag, the value comes from
the config file or the built-in defaults in the table above (depth `0` = unlimited, max-pages
`500`, concurrency `4`). Passing `--max-pages 0` or `--depth 0` explicitly means *unlimited*.

### include / exclude patterns

`include` and `exclude` are [Go regular expressions](https://pkg.go.dev/regexp/syntax)
matched against the full URL string. `exclude` is evaluated first; if `include` is non-empty,
a URL must match at least one include pattern to be crawled.

> **The seed is filtered too, and URLs are normalized before matching.** Two things trip
> people up here, and both fail the same silent way — the seed is rejected, so the crawl
> finishes with **zero pages**, no error, and no note explaining why:
>
> 1. `include` is applied to every URL the crawl considers, *including the seed*. So
>    `--include '/blog'` on a seed of `https://example.com/` rejects the seed itself.
> 2. Matching happens against the **normalized** URL, which has any trailing slash stripped
>    from a non-root path. So `--include '/blog/'` never matches a seed of
>    `https://example.com/blog/` — that URL is normalized to `.../blog` first.
>
> Write include patterns without a trailing slash, and either seed inside the included
> section or alternate the seed in explicitly:
>
> ```sh
> gocrawl crawl https://example.com/blog/ --include '/blog'                     # ✅ crawls /blog + /blog/*
> gocrawl crawl https://example.com --include '/blog|^https://example\.com/?$'  # ✅ root seed + /blog/*
> gocrawl crawl https://example.com --include '/blog'                           # ❌ 0 pages: seed rejected
> gocrawl crawl https://example.com/blog/ --include '/blog/'                    # ❌ 0 pages: trailing slash
> ```

The example config excludes common asset types:

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

**Scoping.** The `Authorization` header is sent only to the **seed host** — plus its
subdomains when `allow_subdomains` / `--subdomains` is set — and never over a scheme
downgrade from `https` to plain `http` (an `http` → `https` upgrade is fine, since that's not
a downgrade). Any request outside that scope is made without credentials, so they can't leak
to another domain or travel in cleartext. Concretely:

- Both checks are re-evaluated on **every redirect hop**, not just the first request, so a
  redirect to a host outside the credential scope is followed without the header.
- With `--subdomains`, a redirect from the seed host to a *sibling* subdomain of it (say
  `example.com` → `www.example.com`) **does** carry the header, because that subdomain is
  inside the credential scope.
- Credential scope is deliberately *narrower* than crawl scope: `--external` widens what
  gocrawl will crawl, but never what it will authenticate to. A crawl with `--external
  --basic-auth` does not send the seed's credentials to the third-party hosts it follows.
- The analyzers that fetch a few extra resources (`sitemap`, `geo`, `wordpress`) use the same
  credential scope. So a `Sitemap:` directive in `robots.txt` pointing at another host — a CDN
  or a separate subdomain without `--subdomains` — is fetched **anonymously**, and on a site
  whose Basic Auth realm also covers that host the sitemap will come back `401` and be
  reported as unreachable. Crawl the host that serves the sitemap directly, or add
  `--subdomains` if it's a subdomain of the seed.

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

### Security audit

`analyzers.security_audit` (or the `--security-audit` flag) is likewise independent of the
allow/deny lists. It adds a second pass to the `security` analyzer covering transport and
cookie configuration: TLS protocol version and cipher suite, certificate expiry and chain
health, `Set-Cookie` attributes, and response-header policy (HSTS quality, framing protection,
`Referrer-Policy`, version disclosure). The analyzer's baseline checks run either way.

Unlike the WordPress probes above, the audit is **passive** — it reads the responses the crawl
already made and opens no extra connections. Its findings are host-wide server configuration,
so they are reported once per host rather than per page. The TLS and certificate checks need
`--render raw` (the default); headless rendering does not expose the handshake, and the report
notes when they were skipped for that reason.

```sh
gocrawl crawl https://example.com --security-audit
```

See the [`security` analyzer reference](analyzers.md#security--security-headers-insecure-forms-and-the-opt-in-tlscookie-audit)
for every issue code and threshold.

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
#
# All values are optional and fall back to built-in defaults. Command-line flags and
# GOCRAWL_* environment variables override anything set here.
#
# Run a crawl with:  gocrawl crawl https://example.com --config gocrawl.yaml

# Seed URL to start from (the positional CLI argument overrides this).
seed: "https://example.com"

# Rendering mode: "raw" (HTTP fetch, fast) or "headless" (chromedp — renders JS and captures
# Core Web Vitals; needs a Chromium-class browser on PATH).
render: "raw"

crawl:
  max_depth: 2          # link hops from the seed; 0 = unlimited (bounded by max_pages instead).
                        # 2 is a conservative starter, not the built-in default (which is 0).
  max_pages: 500        # hard cap on pages crawled — the primary bound on crawl size
  concurrency: 4        # number of parallel fetch workers
  rate_per_second: 0    # max requests/second across the crawl (0 = unlimited)
  adaptive_delay: true  # slow down automatically on HTTP 429/503 responses
  verbose: false        # log each fetch and every rate change to stderr while crawling
  user_agent: "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)"
  # Optional User-Agent rotation. When user_agents is non-empty it supersedes user_agent, and
  # one is picked per request by user_agent_rotation: off, round-robin, or random.
  user_agents: []
  user_agent_rotation: "round-robin"
  # Optional proxy / IP rotation. Set a single proxy with "proxy", or a pool with "proxies"
  # (a single "proxy", if set, is prepended to the pool). Schemes: http, https, socks5;
  # credentials may be embedded as user:pass@host (raw mode only). proxy_rotation is off,
  # round-robin, random, or sticky-host (all requests to one host reuse the same proxy).
  # Headless mode uses only the first proxy. Leave empty to honor HTTP(S)_PROXY env vars.
  proxy: ""
  proxies: []
  proxy_rotation: "round-robin"
  # HTTP Basic Auth as "user:pass", for sites gated by server-level Basic Auth (common on
  # staging/acceptance environments). The Authorization header is sent only to the seed host —
  # plus its subdomains when allow_subdomains is set — and never over an https -> http
  # downgrade; both checks are re-applied on every redirect hop. Not supported with
  # render: "headless" (Chromium cannot scope the header per host), which errors out.
  basic_auth: ""
  timeout: "15s"        # per-request timeout
  max_duration: "0s"    # wall-clock budget for the whole crawl (0 = unlimited); on expiry the
                        # crawl stops early and still writes a partial report
  max_body_bytes: 5242880  # 5 MiB cap on a single response body
  respect_robots: true  # obey robots.txt while crawling
  allow_subdomains: false  # follow links to subdomains of the seed host
  follow_external: false   # crawl links that leave the seed host
  follow_nofollow: false   # follow links marked rel="nofollow"
  strip_query: false       # ignore query strings (treat ?a=1 and ?a=2 as one URL).
                           # NOTE: this drops query params, so the query-dependent analyzers
                           # (utm, landing, wordpress) are automatically skipped while it is on.
  # include/exclude are Go regexes matched against the full *normalized* URL — a trailing slash
  # is stripped from a non-root path, so "/blog/" never matches a page at /blog. exclude is
  # evaluated first. NOTE: include is applied to the seed as well, so a pattern the seed itself
  # doesn't match yields a crawl of zero pages with no error: either seed inside the included
  # section, or alternate the seed into the pattern.
  include: []           # only crawl URLs matching at least one of these regexes
  exclude:              # never crawl URLs matching any of these regexes
    - "\\.(?:png|jpe?g|gif|svg|webp|ico|css|js|pdf|zip)(?:\\?|$)"

output:
  format: "json"        # "json", "csv", or "html"
  path: ""              # file to write to; empty = stdout
  sitemap_path: ""      # also write a standard sitemap.xml of the crawled pages here

analyzers:
  # If "enabled" is non-empty, only those analyzers run. Otherwise all run except those
  # listed in "disabled". Names: seo, redirects, links, robots, sitemap, structured, perf,
  # images, urls, security, pagination, hreflang, amp, duplicates, content, botwall,
  # wordpress, the SEA analyzers utm, tracking, datalayer, landing, consent, and the
  # AI-search analyzers aeo, geo.
  enabled: []
  disabled: []
  # Turn on the opt-in specialized checks (off by default): the lower-confidence AI-search
  # heuristics (aeo-no-answer-lead, geo-low-quotable-density) and the WordPress
  # security-endpoint probes.
  specialized: false
  # Turn on the opt-in security audit (off by default): TLS protocol and certificate checks,
  # Set-Cookie attribute hygiene, and response-header policy. Passive — it reads the crawl's
  # own responses and opens no extra connections.
  security_audit: false

store:
  # Where 'gocrawl crawl --save' writes crawls and where 'gocrawl history' / 'gocrawl
  # compare' read them from. Empty = ~/.gocrawl/crawls. Saved crawls are addressable by
  # their "<host>/<timestamp>" ID, or by "latest" / a bare host name.
  dir: ""
```

Run it with:

```sh
gocrawl crawl https://example.com --config gocrawl.yaml
```

## See also

- [Analyzer reference](analyzers.md) — what each analyzer checks and emits.
- [Output reference](output.md) — the report schema.
- [MCP server guide](mcp.md) — the same options exposed as MCP tool arguments.

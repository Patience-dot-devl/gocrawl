# Ignore External UTM Tagging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Suppress the `utm` analyzer's 4 per-link tagging-quality warnings for links leaving
the crawled domain, on by default, with a toggle to restore them — wired through the CLI,
config file, MCP/web API, and web UI the same way `--specialized`/`--security-audit` are.

**Architecture:** A new `utm.Option` (`WithIgnoreExternalTagging`, default `true` inside
`New()`) gates the 4 quality-issue checks per link; everything else (config field, CLI flag,
`runner.RegistryOptions`, `crawlrequest.Params`, web form) is plumbing that mirrors the
existing `Specialized`/`SecurityAudit` fields exactly, just defaulted to `true`.

**Tech Stack:** Go (cobra/viper config layer), TypeScript/React (web form).

## Global Constraints

- Match the existing `Specialized`/`SecurityAudit` wiring pattern in every layer — same field
  ordering, same `f.Changed(...)` guard style, same jsonschema comment style.
- `utm-summary` counts and `utm-internal-tagged` must NOT change — only the 4 per-link quality
  codes (`utm-partial-tagging`, `utm-empty-value`, `utm-duplicate-param`,
  `utm-inconsistent-casing`) are suppressed, and only for `link.External == true`.
- Full spec: `docs/superpowers/specs/2026-07-29-ignore-external-utm-tagging-design.md`.

---

### Task 1: Analyzer toggle in `internal/analyze/utm`

**Files:**
- Modify: `internal/analyze/utm/utm.go`
- Test: `internal/analyze/utm/utm_test.go`

**Interfaces:**
- Produces: `utm.Option`, `utm.WithIgnoreExternalTagging(on bool) Option`, `utm.New(opts ...Option) *Analyzer` (was `New() *Analyzer`) — callers with no options still get `ignoreExternal == true`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/analyze/utm/utm_test.go`:

```go
func TestExternalPartialTaggingIgnoredByDefault(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://ads.example.net/?utm_source=g", External: true})
	issues := analyze1(res)
	if _, ok := find(issues, "utm-partial-tagging"); ok {
		t.Error("expected utm-partial-tagging to be suppressed for an external link by default")
	}
	sum, ok := find(issues, "utm-summary")
	if !ok {
		t.Fatal("expected utm-summary")
	}
	if sum.Data["external_tagged"] != 1 {
		t.Errorf("external_tagged = %v, want 1", sum.Data["external_tagged"])
	}
}

func TestExternalTaggingWarningsRestoredWhenIncluded(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://ads.example.net/?utm_source=g", External: true})
	issues := utm.New(utm.WithIgnoreExternalTagging(false)).Analyze(context.Background(), res)
	if _, ok := find(issues, "utm-partial-tagging"); !ok {
		t.Error("expected utm-partial-tagging when external tagging checks are included")
	}
}

func TestInternalPartialTaggingAlwaysChecked(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://example.com/page?utm_source=g", External: false})
	issues := analyze1(res)
	if _, ok := find(issues, "utm-partial-tagging"); !ok {
		t.Error("expected utm-partial-tagging for an internal link regardless of the toggle")
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/analyze/utm/... -run 'TestExternalPartialTaggingIgnoredByDefault|TestExternalTaggingWarningsRestoredWhenIncluded|TestInternalPartialTaggingAlwaysChecked' -v`
Expected: `TestExternalPartialTaggingIgnoredByDefault` FAILs (today's code still emits the warning for external links); `TestExternalTaggingWarningsRestoredWhenIncluded` fails to compile (`utm.WithIgnoreExternalTagging` doesn't exist yet); `TestInternalPartialTaggingAlwaysChecked` passes already (that's fine — it's a regression guard, not new behavior).

- [ ] **Step 3: Implement the toggle**

In `internal/analyze/utm/utm.go`, replace:

```go
// Analyzer inspects outbound links for UTM campaign tagging.
type Analyzer struct{}

// New returns a UTM analyzer.
func New() *Analyzer { return &Analyzer{} }
```

with:

```go
// Option configures the analyzer.
type Option func(*Analyzer)

// WithIgnoreExternalTagging controls whether the 4 per-link tagging-quality warnings
// (utm-partial-tagging, utm-empty-value, utm-duplicate-param, utm-inconsistent-casing) are
// suppressed for links leaving the crawled domain — the site owner doesn't control tagging on
// links it doesn't own (e.g. a third-party widget's own UTM-tagged badge link). On by default.
func WithIgnoreExternalTagging(on bool) Option { return func(a *Analyzer) { a.ignoreExternal = on } }

// Analyzer inspects outbound links for UTM campaign tagging.
type Analyzer struct {
	ignoreExternal bool
}

// New returns a UTM analyzer. External-link tagging-quality warnings are suppressed by
// default; pass WithIgnoreExternalTagging(false) to include them.
func New(opts ...Option) *Analyzer {
	a := &Analyzer{ignoreExternal: true}
	for _, opt := range opts {
		opt(a)
	}
	return a
}
```

Then in `analyzePage`, immediately after the `tagged++` / `externalTagged`/`internalTagged`/
`utm-internal-tagged` block and before `present := u.PresentKeys()`, add:

```go
		if link.External && a.ignoreExternal {
			continue
		}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./internal/analyze/utm/... -v`
Expected: PASS, including the pre-existing tests (`TestCompleteTaggingNoWarnings`, `TestPartialTagging`, etc.).

- [ ] **Step 5: Commit**

```bash
git add internal/analyze/utm/utm.go internal/analyze/utm/utm_test.go
git commit -m "utm: suppress tagging-quality warnings on external links by default"
```

---

### Task 2: Wire the toggle through config, CLI, runner, and MCP/web API

**Files:**
- Modify: `internal/runner/runner.go:51-57,62-86,163-166`
- Modify: `internal/config/config.go:82-95,100-116,176-179`
- Modify: `cmd/gocrawl/crawl.go` (flag registration near line 53, apply near line 189)
- Modify: `internal/crawlrequest/crawlrequest.go:17-41,86-92`
- Test: `internal/config/config_test.go`, `internal/crawlrequest/crawlrequest_test.go`

**Interfaces:**
- Consumes: `utm.WithIgnoreExternalTagging(bool)` from Task 1.
- Produces: `config.AnalyzersConfig.IgnoreExternalTagging bool`, `runner.RegistryOptions.IgnoreExternalTagging bool`, `crawlrequest.Params.IgnoreExternalTagging *bool`, CLI flag `--ignore-external-tagging`.

- [ ] **Step 1: Write the failing config tests**

Append to `internal/config/config_test.go`:

```go
func TestDefaultIgnoresExternalUTMTagging(t *testing.T) {
	if !Default().Analyzers.IgnoreExternalTagging {
		t.Error("Default().Analyzers.IgnoreExternalTagging = false, want true")
	}
}
```

Add this line inside `TestLoadPicksUpEnvVarsForEveryField`, next to the other `t.Setenv` calls:

```go
	t.Setenv("GOCRAWL_ANALYZERS_IGNORE_EXTERNAL_TAGGING", "false")
```

and this assertion next to the `Analyzers.Specialized` check:

```go
	if cfg.Analyzers.IgnoreExternalTagging {
		t.Error("Analyzers.IgnoreExternalTagging = true, want false (env override)")
	}
```

Append to `internal/crawlrequest/crawlrequest_test.go`:

```go
func TestToConfig_IgnoreExternalTaggingDefaultsTrue(t *testing.T) {
	cfg, _, err := Params{URL: "https://example.com"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if !cfg.Analyzers.IgnoreExternalTagging {
		t.Error("IgnoreExternalTagging = false, want true (default)")
	}
}

func TestToConfig_IgnoreExternalTaggingOverride(t *testing.T) {
	ignore := false
	cfg, _, err := Params{URL: "https://example.com", IgnoreExternalTagging: &ignore}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if cfg.Analyzers.IgnoreExternalTagging {
		t.Error("IgnoreExternalTagging = true, want false (explicit override)")
	}
}
```

- [ ] **Step 2: Run the new tests, verify they fail**

Run: `go test ./internal/config/... ./internal/crawlrequest/... -v`
Expected: FAIL to compile — `Analyzers.IgnoreExternalTagging` and `Params.IgnoreExternalTagging` don't exist yet.

- [ ] **Step 3: Add the config field**

In `internal/config/config.go`, add to `AnalyzersConfig` (after the `SecurityAudit` field, line 94):

```go
	// IgnoreExternalTagging suppresses the utm analyzer's per-link tagging-quality warnings
	// (partial/empty/duplicate/casing) for links leaving the crawled domain, since the site
	// owner doesn't control tagging on links it doesn't own. On by default; the utm-summary
	// rollup and utm-internal-tagged are unaffected.
	IgnoreExternalTagging bool `mapstructure:"ignore_external_tagging"`
```

In `Default()`, add an `Analyzers:` field to the returned `Config` literal (it currently has none):

```go
		Analyzers: AnalyzersConfig{
			IgnoreExternalTagging: true,
		},
```

In `setDefaults`, add after the `analyzers.security_audit` line (179):

```go
	v.SetDefault("analyzers.ignore_external_tagging", d.Analyzers.IgnoreExternalTagging)
```

- [ ] **Step 4: Add the `crawlrequest.Params` field**

In `internal/crawlrequest/crawlrequest.go`, add to `Params` (after `SecurityAudit`, line 27):

```go
	IgnoreExternalTagging *bool `json:"ignore_external_tagging,omitempty" jsonschema:"Suppress the utm analyzer's tagging-quality warnings for links leaving the domain (default true — the site doesn't control third-party tagging)"`
```

In `ToConfig`, add after the `SecurityAudit` block (line 92):

```go
	if p.IgnoreExternalTagging != nil {
		cfg.Analyzers.IgnoreExternalTagging = *p.IgnoreExternalTagging
	}
```

- [ ] **Step 5: Run tests, verify they pass**

Run: `go test ./internal/config/... ./internal/crawlrequest/... -v`
Expected: PASS.

- [ ] **Step 6: Wire `runner.RegistryOptions` and the CLI flag**

In `internal/runner/runner.go`, add to `RegistryOptions` (after `SecurityAudit`, line 56):

```go
	// IgnoreExternalTagging suppresses the utm analyzer's tagging-quality warnings for
	// links leaving the crawled domain. On by default.
	IgnoreExternalTagging bool
```

Change the `utm.New()` call (line 86) to:

```go
	r.Register(utm.New(utm.WithIgnoreExternalTagging(opts.IgnoreExternalTagging)))
```

Note: `ListAnalyzers` (line 108) calls `BuildRegistry(..., RegistryOptions{})` — its zero
value now means `IgnoreExternalTagging: false`, which would flip `utm.New`'s effective default
to "include everything" for that one metadata-only call. That's fine (`ListAnalyzers` only
reads `Name()`/`Description()`, never `Analyze()`), but set it explicitly there for clarity:

```go
	reg := BuildRegistry(crawler.NewHTTPFetcher(crawler.DefaultOptions()), RegistryOptions{IgnoreExternalTagging: true})
```

In `Run` (line 163), add to the `RegistryOptions{}` literal:

```go
		IgnoreExternalTagging: cfg.Analyzers.IgnoreExternalTagging,
```

In `cmd/gocrawl/crawl.go`, add the flag next to `security-audit` (after line 53):

```go
	f.Bool("ignore-external-tagging", true, "ignore the utm analyzer's tagging-quality warnings for links leaving the domain")
```

and the apply block next to the `security-audit` one:

```go
	if f.Changed("ignore-external-tagging") {
		cfg.Analyzers.IgnoreExternalTagging, _ = f.GetBool("ignore-external-tagging")
	}
```

- [ ] **Step 7: Build and run the full Go test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: builds clean, all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/runner/runner.go internal/config/config.go internal/config/config_test.go \
        internal/crawlrequest/crawlrequest.go internal/crawlrequest/crawlrequest_test.go \
        cmd/gocrawl/crawl.go
git commit -m "config/cli/mcp: wire ignore-external-tagging toggle for the utm analyzer"
```

---

### Task 3: Web UI checkbox

**Files:**
- Modify: `web/src/types.ts:77-101`
- Modify: `web/src/components/CrawlForm.tsx:17-18,64-73,159-166`

**Interfaces:**
- Consumes: `ignore_external_tagging` JSON field from Task 2 (`crawlrequest.Params`).
- Produces: `StartCrawlParams.ignore_external_tagging?: boolean`.

- [ ] **Step 1: Add the field to `StartCrawlParams`**

In `web/src/types.ts`, add after `security_audit?: boolean` (line 87):

```ts
  ignore_external_tagging?: boolean
```

- [ ] **Step 2: Add form state**

In `web/src/components/CrawlForm.tsx`, add after `const [securityAudit, setSecurityAudit] = useState(false)` (line 18):

```ts
  const [ignoreExternalTagging, setIgnoreExternalTagging] = useState(true)
```

- [ ] **Step 3: Include it in the submitted params**

In the `params` object literal in `handleSubmit` (after `security_audit: securityAudit,`, line 71):

```ts
      ignore_external_tagging: ignoreExternalTagging,
```

- [ ] **Step 4: Add the checkbox**

After the `security_audit` checkbox block (line 166):

```tsx
      <label className="checkbox">
        <input
          type="checkbox"
          checked={ignoreExternalTagging}
          onChange={(e) => setIgnoreExternalTagging(e.target.checked)}
        />
        Ignore UTM tagging issues on links leaving the domain
      </label>
```

- [ ] **Step 5: Build the frontend**

Run: `cd web && npm ci && npm run build`
Expected: builds with no TypeScript errors.

- [ ] **Step 6: Manual check**

Run: `make build && ./gocrawl serve`, open `http://localhost:8080`, confirm the new checkbox
appears checked by default next to "Enable security audit," and that starting a crawl against
a site with partially-UTM-tagged outbound links (or re-running the `chainfill.ai` crawl from
earlier) no longer reports `utm-partial-tagging` for those external links unless the box is
unchecked.

- [ ] **Step 7: Commit**

```bash
git add web/src/types.ts web/src/components/CrawlForm.tsx internal/webserver/webui/dist
git commit -m "web: add toggle for ignoring external-link UTM tagging warnings"
```

---

### Task 4: Docs and changelog

**Files:**
- Modify: `docs/configuration.md` (table at line 56, new subsection after "Security audit" at line 277, YAML example at line 402)
- Modify: `docs/analyzers.md` (utm section, around line 522)
- Modify: `docs/mcp.md` (param table at line 62)
- Modify: `docs/web.md` (line 46)
- Modify: `CHANGELOG.md` (`## [Unreleased]`)

- [ ] **Step 1: `docs/configuration.md` table row**

After the `analyzers.security_audit` row (line 56):

```markdown
| `analyzers.ignore_external_tagging` | `--ignore-external-tagging` | bool | `true` | Suppress the `utm` analyzer's tagging-quality warnings for links leaving the domain (see below). |
```

- [ ] **Step 2: `docs/configuration.md` subsection**

After the "Security audit" section (after line 277, before "### Crawl store"):

```markdown
### Ignoring external-link UTM tagging

`analyzers.ignore_external_tagging` (or `--ignore-external-tagging`) is on by default. The
`utm` analyzer's 4 per-link tagging-quality checks (`utm-partial-tagging`, `utm-empty-value`,
`utm-duplicate-param`, `utm-inconsistent-casing`) are skipped for links where
`crawler.Link.External` is true — a page's own outbound links to a third party (e.g. a widget's
"powered by" badge) often carry tagging the site owner doesn't control and can't fix. The
`utm-summary` rollup and `utm-internal-tagged` are unaffected either way.

```sh
gocrawl crawl https://example.com --ignore-external-tagging=false
```
```

- [ ] **Step 3: `docs/configuration.md` YAML example**

After the `security_audit: false` line (line 402):

```yaml
  # Suppress the utm analyzer's tagging-quality warnings for links leaving the domain
  # (on by default — the site doesn't control third-party tagging).
  ignore_external_tagging: true
```

- [ ] **Step 4: `docs/analyzers.md` utm section**

After the issue-code table (line 522), add a short paragraph:

```markdown
By default (`analyzers.ignore_external_tagging`, `--ignore-external-tagging`), the 4
tagging-quality codes above (`utm-partial-tagging`, `utm-empty-value`, `utm-duplicate-param`,
`utm-inconsistent-casing`) are not emitted for links where the target leaves the crawled
domain; `utm-internal-tagged` and `utm-summary` are unaffected.
```

- [ ] **Step 5: `docs/mcp.md` param table**

After the `security_audit` row (line 62):

```markdown
| `ignore_external_tagging` | bool | `true` | Suppress the `utm` analyzer's tagging-quality warnings for links leaving the domain. |
```

- [ ] **Step 6: `docs/web.md`**

Update line 46 to mention the new toggle, e.g. append ", and an option to ignore UTM tagging
issues on external links" to the existing sentence about specialized checks/security audit.

- [ ] **Step 7: `CHANGELOG.md`**

Under `## [Unreleased]`, add:

```markdown
### Added

- **Ignore external-link UTM tagging by default.** The `utm` analyzer's tagging-quality
  warnings (partial/empty/duplicate/casing) no longer fire for outbound links to other
  domains, since the site owner doesn't control third-party tagging (e.g. a widget's own
  "powered by" badge link). Toggle with `--ignore-external-tagging=false` / `analyzers.
  ignore_external_tagging: false` / `ignore_external_tagging: false` (MCP/web API) to restore
  them.
```

- [ ] **Step 8: Commit**

```bash
git add docs/configuration.md docs/analyzers.md docs/mcp.md docs/web.md CHANGELOG.md
git commit -m "docs: document the ignore-external-tagging toggle"
```

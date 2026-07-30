# Ignore UTM tagging-quality warnings on external links, by default

Status: approved (design)
Date: 2026-07-29

## Problem

The `utm` analyzer flags outbound links with incomplete/empty/duplicate/inconsistently-cased
UTM parameters, even when the link points off-site (e.g. a third-party widget's own
"powered by" link with its own partial UTM tagging, like Cookiebot's). The site owner doesn't
control tagging on links they don't own, so these are noise. Confirmed example:
`chainfill.ai` → `cookiebot.com` flagged `utm-partial-tagging` for a Cookiebot badge link.

## Goal

A toggle, **on by default**, that suppresses the 4 per-link quality warnings
(`utm-partial-tagging`, `utm-empty-value`, `utm-duplicate-param`, `utm-inconsistent-casing`)
for links where `crawler.Link.External == true`. The per-page `utm-summary` rollup (counts of
`tagged_links` / `external_tagged` / `internal_tagged`) and `utm-internal-tagged` are
unaffected — internal links keep full checks regardless of the toggle.

## Design

Mirrors the existing `Specialized` / `SecurityAudit` opt-in wiring end to end
(CLI flag → `config.AnalyzersConfig` → `runner.RegistryOptions` → analyzer functional option
→ `crawlrequest.Params` for MCP/web parity → web UI checkbox), with the default flipped to
`true`.

| Layer | Name | Default |
|---|---|---|
| `internal/analyze/utm` | `Option` `WithIgnoreExternalTagging(bool)`, field `ignoreExternal` | `true`, set inside `New()` before applying opts |
| `internal/runner.RegistryOptions` | `IgnoreExternalTagging bool` | passed straight through to `utm.New` |
| `internal/config.AnalyzersConfig` | `IgnoreExternalTagging bool` (`mapstructure:"ignore_external_tagging"`) | explicitly `true` in `config.Default()` + registered as a viper default |
| CLI (`cmd/gocrawl/crawl.go`) | `--ignore-external-tagging` | `true`; applied only `if f.Changed(...)` |
| `internal/crawlrequest.Params` | `IgnoreExternalTagging *bool` (`json:"ignore_external_tagging,omitempty"`) | `nil` = inherit config default (`true`) |
| Web UI (`CrawlForm.tsx`) | new checkbox next to `specialized`/`security_audit` | `useState(true)` (checked by default) |

### Analyzer logic (`internal/analyze/utm/utm.go`)

In `analyzePage`, right after the existing `tagged`/`externalTagged`/`internalTagged`
counting and `utm-internal-tagged` emission, add:

```go
if link.External && a.ignoreExternal {
    continue
}
```

before the block that runs the 4 quality checks. Nothing else changes — summary counts and
`utm-internal-tagged` already run before this point.

## Testing

Add to `utm_test.go`:
- Default (`utm.New()`): an external link with partial tagging produces no
  `utm-partial-tagging`, but `utm-summary`'s `external_tagged` count still includes it.
- `utm.New(utm.WithIgnoreExternalTagging(false))`: the same external link produces the warning
  (restores today's behavior).
- An internal link with partial tagging still produces the warning regardless of the toggle.

## Docs

Update `docs/configuration.md` (analyzer options table), `docs/analyzers.md` (utm section),
`docs/mcp.md` (param table), `docs/web.md` if it enumerates form fields, and add a
`CHANGELOG.md` entry under `## [Unreleased]`.

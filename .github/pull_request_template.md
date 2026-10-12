## What changed

<!-- One or two sentences: what this PR does and why. -->

## How it was verified

<!-- Tests added, a crawl you ran, before/after output. -->

## Checklist

- [ ] `make check` passes (gofmt, vet, build, `test -race`, golangci-lint, docs-drift tests)
- [ ] New behavior has tests; new analyzer checks have an HTML fixture under `testdata/`
- [ ] New or changed issue codes have an entry in `internal/report/explanations.go` and a row in `docs/analyzers.md`
- [ ] Relevant docs updated (`docs/`, `README.md`, `AGENTS.md` package map or CLI list)
- [ ] User-visible change noted under `## [Unreleased]` in `CHANGELOG.md`
- [ ] No credentials, tokens, or personal data added

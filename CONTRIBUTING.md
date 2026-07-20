# Contributing to gocrawl

Thanks for your interest in improving gocrawl! Contributions of all kinds are welcome:
bug reports, feature ideas, documentation, and code.

## Development setup

```sh
git clone https://github.com/Patience-dot-devl/gocrawl
cd gocrawl
go build ./...
go test ./...
```

Useful Make targets: `make build`, `make test`, `make vet`, `make lint`.

## Adding a new analyzer

gocrawl's extensibility lives in the **analyzer pipeline**. Every SEO/SEA check is a
self-contained type that implements `analyze.Analyzer`:

```go
type Analyzer interface {
    Name() string
    Description() string
    Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue
}
```

To add a check:

1. Create a package under `internal/analyze/<yourcheck>/`.
2. Implement the `Analyzer` interface. For per-page checks, use the
   `analyze.EachPage(result, fn)` helper.
3. Register it in `internal/runner/runner.go` (`BuildRegistry`).
4. Add a unit test with an HTML fixture under `testdata/`.

That's it — no changes to the crawl engine are needed. This is exactly how the SEA analyzers
(`utm` for UTM auditing, `tracking` for GTM/GA4/Meta-Pixel detection, `landing` for
landing-page relevance) were added: each a new package under `internal/analyze/` registered in
`BuildRegistry`, no engine changes.

## Releasing

Pushing a tag matching `v*.*.*` triggers [`.github/workflows/release.yml`](.github/workflows/release.yml),
which runs [GoReleaser](https://goreleaser.com) (config: [`.goreleaser.yaml`](.goreleaser.yaml)) to
build the web UI, cross-compile binaries for linux/darwin/windows (amd64/arm64) with it embedded,
and publish them to a GitHub Release. The release body is pulled straight from the tagged
version's `## [x.y.z]` section in [`CHANGELOG.md`](CHANGELOG.md) — update the changelog before
tagging, not after. To cut a release:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

## Guidelines

- Run `go vet ./...` and `gofmt` before opening a PR.
- Keep analyzers focused and side-effect free; emit `analyze.Issue` values rather than
  printing.
- Add tests for new behavior.

## License

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE).

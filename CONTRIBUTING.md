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

Useful Make targets: `make build`, `make test`, `make vet`, `make lint`. Before opening a PR,
run `make check`: it runs everything CI runs on the Go code, including tests that fail when
the docs drift from the code. AI coding agents: see [`AGENTS.md`](AGENTS.md).

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
and publish them to a GitHub Release. The same workflow builds the [`Dockerfile`](Dockerfile) for
linux/amd64 and linux/arm64 and pushes it to `ghcr.io/patience-dot-devl/gocrawl`. GHCR makes a
new package private, so after the first image push, set the package's visibility to public in
its GitHub package settings, or `docker pull` needs a login. GitHub cannot make a public
package private again. The release body is pulled straight from the tagged
version's `## [x.y.z]` section in [`CHANGELOG.md`](CHANGELOG.md) — update the changelog before
tagging, not after. There is no version string to bump in the code: GoReleaser and the
Docker build stamp the tag into `main.version` with `-ldflags`, and a `go install ...@vX.Y.Z`
binary reads the version from the module's build info. To cut a release:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

A tag with a pre-release suffix (`v1.0.0-beta.3`, `v2.0.0-rc.1`) is published as a GitHub
pre-release; only a plain `vX.Y.Z` becomes the "Latest" release.

### Homebrew and Scoop

`.goreleaser.yaml` also builds a Homebrew cask and a Scoop manifest on every release, but only
pushes them once these exist; until then a release skips them and still succeeds:

1. Two public repositories under the same owner: `homebrew-tap` (Homebrew finds a tap by that
   name) and `scoop-bucket`. Each needs at least one commit, for example a README.
2. A fine-grained personal access token with **Contents: read and write** on those two
   repositories only.
3. That token as the `TAP_GITHUB_TOKEN` repository secret of this repository (Settings →
   Secrets and variables → Actions).

From the next release on, users can then install with
`brew install patience-dot-devl/tap/gocrawl` or
`scoop bucket add gocrawl https://github.com/Patience-dot-devl/scoop-bucket` followed by
`scoop install gocrawl`. Add those to `docs/install.md` once the first release has published
them. The binaries are not notarized, so the cask clears macOS's quarantine flag after
installing; signing and notarizing would make that unnecessary.

## Guidelines

- Run `make check` before opening a PR.
- Keep analyzers focused and side-effect free; emit `analyze.Issue` values rather than
  printing.
- Add tests for new behavior.

## License

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE).

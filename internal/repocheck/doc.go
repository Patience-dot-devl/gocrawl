// Package repocheck holds no production code. Its tests check that the repository's docs and
// agent instructions agree with the code: every registered analyzer is documented, every
// top-level internal package and CLI command is in AGENTS.md, every relative Markdown
// link resolves, and every document under docs/ has valid frontmatter that matches its links
// and the generated index in docs/README.md (`make docs` rewrites the generated parts). They run with `go test ./...`, so CI and `make check` catch docs drift the
// same way they catch a broken test.
package repocheck

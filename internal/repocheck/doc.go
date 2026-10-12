// Package repocheck holds no production code. Its tests check that the repository's docs and
// agent instructions agree with the code: every registered analyzer is documented, every
// top-level internal package and CLI command is in AGENTS.md, and every relative Markdown
// link resolves. They run with `go test ./...`, so CI and `make check` catch docs drift the
// same way they catch a broken test.
package repocheck

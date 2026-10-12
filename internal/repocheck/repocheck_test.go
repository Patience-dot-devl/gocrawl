package repocheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// root is the repository root, relative to this package's directory (go test runs each
// package's tests from that package's directory).
const root = "../.."

func readFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// TestAnalyzersDocumented asserts every registered analyzer has a section in the analyzer
// reference and is named in README.md and AGENTS.md.
func TestAnalyzersDocumented(t *testing.T) {
	reference := readFile(t, "docs/analyzers.md")
	readme := readFile(t, "README.md")
	agents := readFile(t, "AGENTS.md")

	analyzers := runner.ListAnalyzers()
	if len(analyzers) == 0 {
		t.Fatal("no analyzers registered; the registry lookup is likely broken")
	}
	for _, a := range analyzers {
		name := "`" + a.Name + "`"
		if !strings.Contains(reference, "## "+name) {
			t.Errorf("analyzer %s has no \"## %s\" section in docs/analyzers.md", a.Name, name)
		}
		if !strings.Contains(readme, name) {
			t.Errorf("analyzer %s is not named in README.md", a.Name)
		}
		if !strings.Contains(agents, name) {
			t.Errorf("analyzer %s is not named in AGENTS.md", a.Name)
		}
	}
}

// TestPackageMapComplete asserts every top-level package under internal/ has a row in the
// AGENTS.md package map. Analyzer subpackages are covered by TestAnalyzersDocumented.
func TestPackageMapComplete(t *testing.T) {
	agents := readFile(t, "AGENTS.md")
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatalf("listing internal/: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		row := "| `internal/" + e.Name() + "` |"
		if !strings.Contains(agents, row) {
			t.Errorf("internal/%s has no row in the AGENTS.md package map (want a line starting %q)", e.Name(), row)
		}
	}
}

var cobraUse = regexp.MustCompile(`Use:\s*"([a-z][a-z-]*)`)

// TestCommandsListed asserts every Cobra command in cmd/gocrawl is named in the AGENTS.md
// package-map row for cmd/gocrawl.
func TestCommandsListed(t *testing.T) {
	var row string
	for _, line := range strings.Split(readFile(t, "AGENTS.md"), "\n") {
		if strings.HasPrefix(line, "| `cmd/gocrawl` |") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("AGENTS.md package map has no `cmd/gocrawl` row")
	}

	files, err := filepath.Glob(filepath.Join(root, "cmd", "gocrawl", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var commands []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range cobraUse.FindAllStringSubmatch(string(src), -1) {
			if m[1] != "gocrawl" { // the root command
				commands = append(commands, m[1])
			}
		}
	}
	if len(commands) == 0 {
		t.Fatal("found no Cobra commands; the scan is likely broken")
	}
	for _, c := range commands {
		// Match the command as a word inside a code span, so `analyzers list` covers "list".
		inSpan := regexp.MustCompile("`[^`]*\\b" + regexp.QuoteMeta(c) + "\\b[^`]*`")
		if !inSpan.MatchString(row) {
			t.Errorf("command %q is not named in the AGENTS.md `cmd/gocrawl` row", c)
		}
	}
}

// skipDirs are never scanned for Markdown. docs/superpowers holds dated specs and plans, a
// historical record whose links point at paths as they were (or were planned) at the time.
var skipDirs = map[string]bool{
	".git":             true,
	".context":         true,
	"node_modules":     true,
	"docs/superpowers": true,
}

var (
	mdLink     = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	hasScheme  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	fenceStart = regexp.MustCompile("^\\s*(```|~~~)")
)

// TestRelativeLinksResolve asserts every relative Markdown link points at a file or directory
// that exists. External URLs and same-page anchors are not checked.
func TestRelativeLinksResolve(t *testing.T) {
	var broken []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDirs[rel] || skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		inFence := false
		for i, line := range strings.Split(string(src), "\n") {
			if fenceStart.MatchString(line) {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
				target, _, _ := strings.Cut(m[1], "#")
				if target == "" || hasScheme.MatchString(target) {
					continue
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), target)); err != nil {
					broken = append(broken, fmt.Sprintf("%s:%d: %s", rel, i+1, m[1]))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository: %v", err)
	}
	sort.Strings(broken)
	for _, b := range broken {
		t.Errorf("broken relative link: %s", b)
	}
}

package repocheck

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// update rewrites the generated parts of the docs instead of checking them: each document's
// depends_on and sources, and the index in docs/README.md. Run it with `make docs`.
var update = flag.Bool("update", false, "rewrite generated docs frontmatter fields and the docs/README.md index")

const docsIndex = "docs/README.md"

var (
	isoDate      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	statusWords  = map[string]bool{"current": true, "superseded": true, "archived": true}
	topicForDir  = map[string]string{"docs": "reference", "docs/superpowers/specs": "spec", "docs/superpowers/plans": "plan", "docs/superpowers/findings": "findings"}
	indexStart   = "<!-- index:start -->"
	indexEnd     = "<!-- index:end -->"
	recordTopics = []string{"spec", "plan", "findings"}
)

// referenceOrder is the reading order of the reference guides in the docs/README.md index.
// A new reference doc must be added here; the test says so if it is missing.
var referenceOrder = []string{
	"install", "configuration", "analyzers", "output", "storage", "redirect-check",
	"mcp", "web", "architecture", "roadmap",
}

type doc struct {
	rel  string // repository-relative slash path, e.g. docs/analyzers.md
	src  string
	fm   frontmatter
	body string
}

// loadDocs reads every Markdown file under docs/ except the index itself.
func loadDocs(t *testing.T) []*doc {
	t.Helper()
	var docs []*doc
	err := filepath.WalkDir(filepath.Join(root, "docs"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == docsIndex {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		docs = append(docs, &doc{rel: rel, src: string(src)})
		return nil
	})
	if err != nil {
		t.Fatalf("walking docs/: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("found no documents under docs/")
	}
	return docs
}

// TestDocsFrontmatter checks every document under docs/ against the frontmatter scheme:
// all keys present, no unknown keys, valid status, topic, and date, unique ids, existing
// superseded_by targets, and depends_on / sources equal to the links in the body.
func TestDocsFrontmatter(t *testing.T) {
	docs := loadDocs(t)
	paths := map[string]bool{}
	for _, d := range docs {
		paths[d.rel] = true
	}

	ids := map[string]string{}
	for _, d := range docs {
		block, body, ok := splitFrontmatter(d.src)
		if !ok {
			t.Errorf("%s: no frontmatter block (see AGENTS.md, Documentation)", d.rel)
			continue
		}
		fm, err := parseFrontmatter(block)
		if err != nil {
			t.Errorf("%s: frontmatter: %v", d.rel, err)
			continue
		}
		d.fm, d.body = fm, body

		known := map[string]bool{}
		for _, k := range fmKeys {
			known[k] = true
			if !fm.has(k) {
				t.Errorf("%s: frontmatter is missing %q", d.rel, k)
			}
		}
		for _, k := range fm.order {
			if !known[k] {
				t.Errorf("%s: unknown frontmatter key %q", d.rel, k)
			}
		}

		id := fm.scalars["id"]
		switch {
		case id == "":
			t.Errorf("%s: empty id", d.rel)
		case ids[id] != "":
			t.Errorf("%s: id %q is already used by %s", d.rel, id, ids[id])
		default:
			ids[id] = d.rel
		}
		for _, k := range []string{"title", "scope"} {
			if fm.scalars[k] == "" {
				t.Errorf("%s: empty %s", d.rel, k)
			}
		}
		if want := topicForDir[path.Dir(d.rel)]; fm.scalars["topic"] != want {
			t.Errorf("%s: topic is %q, want %q for a document in %s/", d.rel, fm.scalars["topic"], want, path.Dir(d.rel))
		}
		status := fm.scalars["status"]
		if !statusWords[status] {
			t.Errorf("%s: status %q is not one of current, superseded, archived", d.rel, status)
		}
		if (status == "superseded") != (len(fm.lists["superseded_by"]) > 0) {
			t.Errorf("%s: superseded_by must be set exactly when status is superseded", d.rel)
		}
		for _, s := range fm.lists["superseded_by"] {
			if !paths[path.Clean(path.Join(path.Dir(d.rel), s))] {
				t.Errorf("%s: superseded_by %q is not a document under docs/", d.rel, s)
			}
		}
		if !isoDate.MatchString(fm.scalars["updated"]) {
			t.Errorf("%s: updated %q is not a YYYY-MM-DD date", d.rel, fm.scalars["updated"])
		}

		deps, sources := deriveLinks(body, d.rel, paths)
		if *update {
			fm.lists["depends_on"], fm.lists["sources"] = deps, sources
			if err := os.WriteFile(filepath.Join(root, d.rel), []byte(emitFrontmatter(fm)+body), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		for _, k := range []struct {
			key     string
			derived []string
		}{{"depends_on", deps}, {"sources", sources}} {
			if missing, extra := linkDrift(d.rel, fm.lists[k.key], k.derived); missing != nil || extra != nil {
				t.Errorf("%s: %s out of date: missing %q, extra %q (run `make docs`)", d.rel, k.key, missing, extra)
			}
		}
	}
}

// linkDrift compares a frontmatter list with the derived links, order-insensitively and as
// resolved paths, so `./x.md` and `x.md` are the same document.
func linkDrift(self string, listed, derived []string) (missing, extra []string) {
	resolve := func(t string) string { return path.Clean(path.Join(path.Dir(self), t)) }
	have, want := map[string]bool{}, map[string]bool{}
	for _, l := range listed {
		have[resolve(l)] = true
	}
	for _, l := range derived {
		want[resolve(l)] = true
		if !have[resolve(l)] {
			missing = append(missing, l)
		}
	}
	for _, l := range listed {
		if !want[resolve(l)] {
			extra = append(extra, l)
		}
	}
	return missing, extra
}

// TestDocsIndex checks that the index in docs/README.md matches what the frontmatter says.
func TestDocsIndex(t *testing.T) {
	docs := loadDocs(t)
	for _, d := range docs {
		block, _, ok := splitFrontmatter(d.src)
		if !ok {
			t.Fatalf("%s: no frontmatter block; TestDocsFrontmatter has the details", d.rel)
		}
		fm, err := parseFrontmatter(block)
		if err != nil {
			t.Fatalf("%s: frontmatter: %v", d.rel, err)
		}
		d.fm = fm
	}
	want, err := renderIndex(docs)
	if err != nil {
		t.Fatal(err)
	}

	readme := readFile(t, docsIndex)
	start, end := strings.Index(readme, indexStart), strings.Index(readme, indexEnd)
	if start < 0 || end < start {
		t.Fatalf("%s has no %s ... %s block", docsIndex, indexStart, indexEnd)
	}
	got := readme[start+len(indexStart) : end]
	if got == want {
		return
	}
	if *update {
		out := readme[:start+len(indexStart)] + want + readme[end:]
		if err := os.WriteFile(filepath.Join(root, docsIndex), []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Errorf("%s index is out of date with the docs' frontmatter (run `make docs`)", docsIndex)
}

// renderIndex builds the text between the index markers: the reference guides in reading
// order, then the design records grouped by topic, oldest first.
func renderIndex(docs []*doc) (string, error) {
	link := func(d *doc) string {
		return fmt.Sprintf("[%s](%s)", d.fm.scalars["title"], strings.TrimPrefix(d.rel, "docs/"))
	}
	byID := map[string]*doc{}
	records := map[string][]*doc{}
	for _, d := range docs {
		if topic := d.fm.scalars["topic"]; topic == "reference" {
			byID[d.fm.scalars["id"]] = d
		} else {
			records[topic] = append(records[topic], d)
		}
	}

	var b strings.Builder
	b.WriteString("\n| Guide | What's in it |\n| --- | --- |\n")
	for _, id := range referenceOrder {
		d, ok := byID[id]
		if !ok {
			return "", fmt.Errorf("referenceOrder lists %q, but no reference doc has that id", id)
		}
		fmt.Fprintf(&b, "| %s | %s |\n", link(d), d.fm.scalars["scope"])
		delete(byID, id)
	}
	for id := range byID {
		return "", fmt.Errorf("reference doc %q is missing from referenceOrder in internal/repocheck/docs_test.go", id)
	}

	b.WriteString("\n### Design records\n\n" +
		"Dated specs, plans, and findings written while building gocrawl. They are a historical\n" +
		"record: `archived` ones describe the code as it was planned, not as it is now.\n")
	headings := map[string]string{"spec": "Specs", "plan": "Plans", "findings": "Findings"}
	for _, topic := range recordTopics {
		list := records[topic]
		if len(list) == 0 {
			continue
		}
		sort.Slice(list, func(i, j int) bool { return list[i].fm.scalars["id"] < list[j].fm.scalars["id"] })
		fmt.Fprintf(&b, "\n**%s**\n\n| Record | Status | Updated |\n| --- | --- | --- |\n", headings[topic])
		for _, d := range list {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", link(d), d.fm.scalars["status"], d.fm.scalars["updated"])
		}
	}
	return b.String(), nil
}

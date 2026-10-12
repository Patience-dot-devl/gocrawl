package repocheck

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// The docs/ frontmatter follows the scheme used in the conductor-playground repository
// (dermatuer/tools/lib/frontmatter.py), minus the client-specific keys (aliases, markets,
// channels). Values are a small YAML subset — strings, dates, and lists, in the forms a person
// or Obsidian's Properties panel writes them — so no YAML dependency is needed.

// fmKeys lists every frontmatter key, in the order emitFrontmatter writes them. All are
// required; list keys may be empty.
var fmKeys = []string{"id", "title", "scope", "topic", "status", "superseded_by", "updated", "depends_on", "sources"}

var fmListKeys = map[string]bool{"superseded_by": true, "depends_on": true, "sources": true}

var (
	fmKeyLine  = regexp.MustCompile(`^([A-Za-z_][\w-]*):(?:[ \t]+(.*))?$`)
	fmItemLine = regexp.MustCompile(`^[ \t]*-(?:[ \t]+(.*))?$`)
)

// frontmatter holds a parsed block: scalars as strings, list keys as []string.
type frontmatter struct {
	scalars map[string]string
	lists   map[string][]string
	order   []string // keys in the order they appeared, for unknown-key reporting
}

func (f frontmatter) has(key string) bool {
	if fmListKeys[key] {
		_, ok := f.lists[key]
		return ok
	}
	_, ok := f.scalars[key]
	return ok
}

// splitFrontmatter separates a leading "---" block from the body. ok is false when the
// document has no frontmatter.
func splitFrontmatter(src string) (block, body string, ok bool) {
	if !strings.HasPrefix(src, "---\n") {
		return "", src, false
	}
	rest := src[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", src, false
	}
	return rest[:end], rest[end+len("\n---\n"):], true
}

// parseFrontmatter reads the block between the --- fences.
func parseFrontmatter(block string) (frontmatter, error) {
	f := frontmatter{scalars: map[string]string{}, lists: map[string][]string{}}
	var listKey string // the key a run of "- item" lines belongs to
	for i, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := fmItemLine.FindStringSubmatch(line); m != nil && listKey != "" {
			v, err := fmScalar(m[1])
			if err != nil {
				return f, fmt.Errorf("line %d: %w", i+2, err)
			}
			f.lists[listKey] = append(f.lists[listKey], v)
			continue
		}
		m := fmKeyLine.FindStringSubmatch(line)
		if m == nil {
			return f, fmt.Errorf("line %d: not a `key: value` line: %q", i+2, line)
		}
		key, val := m[1], strings.TrimSpace(m[2])
		if f.has(key) {
			return f, fmt.Errorf("line %d: duplicate key %q", i+2, key)
		}
		f.order = append(f.order, key)
		listKey = ""
		switch {
		case strings.HasPrefix(val, "["):
			items, err := fmFlowList(val)
			if err != nil {
				return f, fmt.Errorf("line %d: %w", i+2, err)
			}
			f.lists[key] = items
		case val == "" && fmListKeys[key]:
			f.lists[key] = []string{}
			listKey = key
		default:
			v, err := fmScalar(val)
			if err != nil {
				return f, fmt.Errorf("line %d: %w", i+2, err)
			}
			if fmListKeys[key] {
				f.lists[key] = []string{v}
			} else {
				f.scalars[key] = v
			}
		}
	}
	return f, nil
}

func fmScalar(v string) (string, error) {
	v = strings.TrimSpace(v)
	switch {
	case strings.HasPrefix(v, `"`):
		var s string
		if err := json.Unmarshal([]byte(v), &s); err != nil {
			return "", fmt.Errorf("bad double-quoted string %s", v)
		}
		return s, nil
	case strings.HasPrefix(v, "'"):
		if len(v) < 2 || !strings.HasSuffix(v, "'") {
			return "", fmt.Errorf("bad single-quoted string %s", v)
		}
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'"), nil
	case v != "" && strings.ContainsAny(v[:1], "[]{}&*!|>%@`"):
		return "", fmt.Errorf("unsupported value %s", v)
	}
	return v, nil
}

// fmFlowList parses a one-line `[a, "b", 'c']` list.
func fmFlowList(v string) ([]string, error) {
	if !strings.HasSuffix(v, "]") {
		return nil, fmt.Errorf("flow list not closed on its line: %s", v)
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	items := []string{}
	if inner == "" {
		return items, nil
	}
	var cur strings.Builder
	var quote rune
	flush := func() error {
		s, err := fmScalar(cur.String())
		if err != nil {
			return err
		}
		items = append(items, s)
		cur.Reset()
		return nil
	}
	prev := rune(0)
	for _, c := range inner {
		switch {
		case quote != 0:
			cur.WriteRune(c)
			if c == quote && (quote != '"' || prev != '\\') {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			cur.WriteRune(c)
		case c == ',':
			if err := flush(); err != nil {
				return nil, err
			}
		default:
			cur.WriteRune(c)
		}
		prev = c
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote in %s", v)
	}
	return items, flush()
}

// emitFrontmatter writes f in canonical form: keys in fmKeys order, values JSON-quoted (valid
// YAML, and what Obsidian renders as properties), the date bare.
func emitFrontmatter(f frontmatter) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, k := range fmKeys {
		var v []byte
		switch {
		case k == "updated":
			b.WriteString(k + ": " + f.scalars[k] + "\n")
			continue
		case fmListKeys[k]:
			items := f.lists[k]
			if items == nil {
				items = []string{}
			}
			v, _ = json.Marshal(items)
			v = []byte(strings.ReplaceAll(string(v), `","`, `", "`))
		default:
			v, _ = json.Marshal(f.scalars[k])
		}
		b.WriteString(k + ": " + string(v) + "\n")
	}
	b.WriteString("---\n")
	return b.String()
}

// deriveLinks returns the links a document's body makes, split the way conductor-playground
// splits them: links to another indexed document are depends_on, links to anything else in
// the repository (source files, directories, images) are sources. Links to Markdown files
// outside the index (README.md, CHANGELOG.md), external URLs, anchors, and links inside
// fenced code blocks are ignored. self and docs are repository-relative slash paths.
func deriveLinks(body, self string, docs map[string]bool) (deps, sources []string) {
	dir := path.Dir(self)
	seen := map[string]bool{}
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if fenceStart.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
			target, _, _ := strings.Cut(m[1], "#")
			if target == "" || hasScheme.MatchString(target) || seen[target] {
				continue
			}
			resolved := path.Clean(path.Join(dir, target))
			if resolved == self {
				continue
			}
			if strings.HasSuffix(target, ".md") {
				if !docs[resolved] {
					continue
				}
				deps = append(deps, target)
			} else {
				sources = append(sources, target)
			}
			seen[target] = true
		}
	}
	return deps, sources
}

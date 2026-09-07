// Package images implements image checks: alt-text state (missing, empty, present),
// duplicate and filename-derived alt text, non-descriptive filenames, and missing
// width/height dimension attributes.
package images

import (
	"context"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// maxSources caps how many image URLs are listed per finding, so an image-heavy page
// cannot blow up the report. sampleSize is the shorter list kept for readability.
const (
	maxSources = 100
	sampleSize = 5
)

// Analyzer performs image alt-text and dimension checks.
type Analyzer struct{}

// New returns a new images analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "images" }
func (Analyzer) Description() string {
	return "Image alt text (missing, empty, duplicate, filename-derived) and width/height dimension checks"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

// img is one <img> on a page, reduced to the fields the checks need.
type img struct {
	src      string // resolved against the page URL when possible
	filename string // base name of the src path, without query
	stem     string // filename without its extension
	alt      string // trimmed alt value; only meaningful when altState is altPresent
	altState altState
	hasDim   bool
}

type altState int

const (
	altMissing altState = iota // no alt attribute at all
	altEmpty                   // alt="" (or whitespace only) — valid for decorative images
	altPresent                 // non-empty alt
)

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	sel := p.Doc.Find("img")
	if sel.Length() == 0 {
		return nil
	}
	pageURL := p.FinalURL
	base, _ := url.Parse(pageURL)

	imgs := make([]img, 0, sel.Length())
	sel.Each(func(_ int, s *goquery.Selection) {
		imgs = append(imgs, parseImg(s, base))
	})

	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "images", URL: pageURL, Severity: sev, Code: code, Message: msg, Data: data})
	}

	var missing, empty, missingDim []string
	for _, im := range imgs {
		switch im.altState {
		case altMissing:
			missing = append(missing, im.src)
		case altEmpty:
			empty = append(empty, im.src)
		}
		if !im.hasDim {
			missingDim = append(missingDim, im.src)
		}
	}

	if len(missing) > 0 {
		add(analyze.Warning, "img-missing-alt", "Images without an alt attribute", sourceData(missing, nil))
	}
	// An empty alt is the correct markup for decorative images, so this is reported as
	// info rather than flagged as a defect — but it is reported, because a CMS or theme
	// emitting alt="" for every content image looks identical to a correctly decorated
	// page unless you can see the list.
	if len(empty) > 0 {
		add(analyze.Info, "img-empty-alt", "Images with an empty alt attribute (treated as decorative)", sourceData(empty, map[string]any{"images_total": len(imgs)}))
	}
	if len(missingDim) > 0 {
		add(analyze.Info, "img-missing-dimensions", "Images missing width or height attribute", sourceData(missingDim, nil))
	}

	if dups := duplicateAlts(imgs); len(dups) > 0 {
		n := 0
		for _, d := range dups {
			n += d.count
		}
		add(analyze.Warning, "img-duplicate-alt", "The same alt text is reused by several images on this page", map[string]any{
			"count":      n,
			"duplicates": dupData(dups),
		})
	}

	var altIsFilename []string
	for _, im := range imgs {
		if im.altState == altPresent && altMatchesFilename(im.alt, im.stem) {
			altIsFilename = append(altIsFilename, im.src)
		}
	}
	if len(altIsFilename) > 0 {
		add(analyze.Warning, "img-alt-is-filename", "Alt text is just the image filename", sourceData(altIsFilename, nil))
	}

	var badNames []string
	for _, im := range imgs {
		if nonDescriptiveFilename(im.stem) {
			badNames = append(badNames, im.src)
		}
	}
	if len(badNames) > 0 {
		add(analyze.Info, "img-nondescriptive-filename", "Image filenames carry no meaning (numbers, camera defaults, hashes)", sourceData(badNames, nil))
	}

	return issues
}

// parseImg reduces one <img> to the fields the checks need. src falls back to the common
// lazy-loading attributes so themes that keep a placeholder in src still report a usable URL.
func parseImg(s *goquery.Selection, base *url.URL) img {
	raw := firstAttr(s, "src", "data-src", "data-original")
	if raw == "" {
		raw = firstSrcsetURL(firstAttr(s, "srcset", "data-srcset"))
	}
	out := img{src: resolve(base, raw)}
	out.filename, out.stem = filenameOf(out.src)

	if v, ok := s.Attr("alt"); !ok {
		out.altState = altMissing
	} else if strings.TrimSpace(v) == "" {
		out.altState = altEmpty
	} else {
		out.altState = altPresent
		out.alt = strings.TrimSpace(v)
	}

	_, hasW := s.Attr("width")
	_, hasH := s.Attr("height")
	out.hasDim = hasW && hasH
	return out
}

func firstAttr(s *goquery.Selection, names ...string) string {
	for _, n := range names {
		if v, ok := s.Attr(n); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// firstSrcsetURL takes the first candidate of a srcset list ("a.jpg 1x, b.jpg 2x").
func firstSrcsetURL(srcset string) string {
	first, _, _ := strings.Cut(srcset, ",")
	fields := strings.Fields(first)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func resolve(base *url.URL, raw string) string {
	if raw == "" || base == nil {
		return raw
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

// filenameOf returns the base name of a URL path and that name without its extension.
func filenameOf(raw string) (filename, stem string) {
	if raw == "" {
		return "", ""
	}
	p := raw
	if u, err := url.Parse(raw); err == nil {
		p = u.Path
	}
	filename = path.Base(p)
	if filename == "." || filename == "/" {
		return "", ""
	}
	stem = strings.TrimSuffix(filename, path.Ext(filename))
	return filename, stem
}

// sourceData builds the Data map shared by the per-image findings: a count, a short sample
// for readers, and the full (capped) list for tooling that needs to act on every image.
func sourceData(srcs []string, extra map[string]any) map[string]any {
	srcs = dedupe(srcs)
	data := map[string]any{"count": len(srcs)}
	for k, v := range extra {
		data[k] = v
	}
	sample := srcs
	if len(sample) > sampleSize {
		sample = sample[:sampleSize]
	}
	data["sample"] = sample
	sources := srcs
	if len(sources) > maxSources {
		sources = sources[:maxSources]
		data["truncated"] = true
	}
	data["sources"] = sources
	return data
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

type dupAlt struct {
	alt   string
	count int
	srcs  []string
}

// duplicateAlts groups the page's non-empty alt values and returns those used by more than
// one image, most-repeated first.
func duplicateAlts(imgs []img) []dupAlt {
	order := []string{}
	byAlt := map[string]*dupAlt{}
	for _, im := range imgs {
		if im.altState != altPresent {
			continue
		}
		key := strings.ToLower(im.alt)
		d, ok := byAlt[key]
		if !ok {
			d = &dupAlt{alt: im.alt}
			byAlt[key] = d
			order = append(order, key)
		}
		d.count++
		d.srcs = append(d.srcs, im.src)
	}
	var out []dupAlt
	for _, key := range order {
		if d := byAlt[key]; d.count > 1 {
			out = append(out, *d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].count > out[j].count })
	return out
}

func dupData(dups []dupAlt) []map[string]any {
	out := make([]map[string]any, 0, len(dups))
	for _, d := range dups {
		srcs := dedupe(d.srcs)
		if len(srcs) > sampleSize {
			srcs = srcs[:sampleSize]
		}
		out = append(out, map[string]any{"alt": d.alt, "count": d.count, "sample": srcs})
	}
	return out
}

var wordSplit = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeWords lowercases and collapses everything that is not a letter or digit into
// single spaces, so "kantoor-2.jpg" and "Kantoor 2" compare equal.
func normalizeWords(s string) string {
	return strings.TrimSpace(wordSplit.ReplaceAllString(strings.ToLower(s), " "))
}

// altMatchesFilename reports whether the alt text is just the filename restated — either
// the stem verbatim, or the stem with the extension left on.
func altMatchesFilename(alt, stem string) bool {
	if stem == "" {
		return false
	}
	want := normalizeWords(stem)
	if want == "" {
		return false
	}
	got := normalizeWords(alt)
	if got == "" {
		return false
	}
	if got == want {
		return true
	}
	// alt="kantoor.jpg" — strip a trailing extension-looking word before comparing.
	return normalizeWords(strings.TrimSuffix(alt, path.Ext(alt))) == want && path.Ext(alt) != ""
}

var (
	digitsOnly      = regexp.MustCompile(`^[0-9]+$`)
	cameraDefault   = regexp.MustCompile(`^(img|dsc|dscn|pxl|mvimg|photo|image|pic|screenshot|screen shot|untitled|download|unnamed|final|copy of|copy)[ 0-9]*$`)
	hexHash         = regexp.MustCompile(`^[0-9a-f]{16,}$`)
	genericFilename = regexp.MustCompile(`^(logo|banner|hero|slide|asset|file|temp|test|new|default|placeholder)[ 0-9]*$`)
)

// nonDescriptiveFilename reports whether a filename tells a search engine nothing about the
// image: pure digits ("7.jpg"), a camera or export default ("IMG_1234", "unnamed"), a content
// hash, a bare generic word, or a stem too short to carry meaning ("hs1.png").
func nonDescriptiveFilename(stem string) bool {
	if stem == "" {
		return false
	}
	n := normalizeWords(stem)
	if n == "" {
		return false
	}
	if digitsOnly.MatchString(n) || hexHash.MatchString(strings.ReplaceAll(n, " ", "")) {
		return true
	}
	if cameraDefault.MatchString(n) || genericFilename.MatchString(n) {
		return true
	}
	// "hs1", "a2" — too short to be a word, even before the digits are stripped.
	return len(strings.ReplaceAll(n, " ", "")) <= 3
}

package sitemapgen

import (
	"embed"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
)

//go:embed sitemap.html.tmpl
var htmlTemplateFS embed.FS

// xmlURLSet mirrors the sitemaps.org 0.9 urlset schema.
type xmlURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	NS      string   `xml:"xmlns,attr"`
	URLs    []xmlURL `xml:"url"`
}

type xmlURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

const sitemapNS = "http://www.sitemaps.org/schemas/sitemap/0.9"

// WriteXML writes a standard sitemap.xml (sitemaps.org urlset) for the map's entries.
func WriteXML(w io.Writer, m Map) error {
	set := xmlURLSet{NS: sitemapNS}
	for _, e := range m.Entries {
		set.URLs = append(set.URLs, xmlURL(e))
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(set); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// WriteHTML writes a self-contained HTML page (inline CSS, no external assets) that renders
// the crawled site as a collapsible tree.
func WriteHTML(w io.Writer, m Map) error {
	tmpl, err := template.New("sitemap.html.tmpl").Funcs(template.FuncMap{
		"statusClass":   statusClass,
		"severityClass": severityClass,
	}).ParseFS(htmlTemplateFS, "sitemap.html.tmpl")
	if err != nil {
		return fmt.Errorf("parse sitemap template: %w", err)
	}
	return tmpl.Execute(w, m)
}

func severityClass(sev string) string {
	switch sev {
	case "error":
		return "sev-error"
	case "warning":
		return "sev-warning"
	default:
		return "sev-info"
	}
}

func statusClass(status int) string {
	switch {
	case status == 0:
		return "st-none"
	case status >= 200 && status < 300:
		return "st-ok"
	case status >= 300 && status < 400:
		return "st-redirect"
	default:
		return "st-error"
	}
}

// WriteXMLFile writes the sitemap.xml to path, creating parent directories as needed.
func WriteXMLFile(path string, m Map) error {
	return writeFile(path, func(w io.Writer) error { return WriteXML(w, m) })
}

// WriteHTMLFile writes the HTML site-tree to path, creating parent directories as needed.
func WriteHTMLFile(path string, m Map) error {
	return writeFile(path, func(w io.Writer) error { return WriteHTML(w, m) })
}

func writeFile(path string, write func(io.Writer) error) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating output directory %q: %w", dir, err)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	writeErr := write(f)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

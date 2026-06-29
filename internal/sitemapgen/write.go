package sitemapgen

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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

// WriteXMLFile writes the sitemap.xml to path, creating parent directories as needed.
func WriteXMLFile(path string, m Map) error {
	return writeFile(path, func(w io.Writer) error { return WriteXML(w, m) })
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

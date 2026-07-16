package sitemapgen

import (
	"encoding/xml"
	"io"

	"github.com/Patience-dot-devl/gocrawl/internal/atomicfile"
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

// WriteXMLFile writes the sitemap.xml to path, creating parent directories as needed. The
// write is atomic: a failure partway through leaves any previous sitemap.xml at path intact.
func WriteXMLFile(path string, m Map) error {
	return atomicfile.Write(path, 0o644, func(w io.Writer) error { return WriteXML(w, m) })
}

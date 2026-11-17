package schemaorg

import (
	"html"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// FuzzParse runs arbitrary JSON-LD blocks, as found on crawled pages, through the graph
// parser and its accessors. Malformed or adversarial markup must yield parse errors, not a
// panic. Every node's path must also be non-empty, since reports address nodes by it.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		`{"@context":"https://schema.org","@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"9.99"}}`,
		`{"@graph":[{"@type":["Organization","Brand"],"@id":"#org"},{"@type":"WebSite","publisher":{"@id":"#org"}}]}`,
		`[{"@type":"BreadcrumbList","itemListElement":[{"@type":"ListItem","position":1}]}]`,
		`{"@type":null,"@id":5,"@graph":"x"}`,
		`{"@type":"Thing",`,
		`[[[[[[[[[[{"@type":"Deep"}]]]]]]]]]]`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, jsonld string) {
		page := `<html><head><script type="application/ld+json">` +
			strings.ReplaceAll(jsonld, "</", `<\/`) + `</script></head><body>` +
			html.EscapeString(jsonld) + `</body></html>`
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
		if err != nil {
			t.Skip()
		}
		g, _ := Parse(doc)
		for _, n := range g.Nodes {
			if n.Path == "" {
				t.Fatalf("node %+v has an empty path", n)
			}
			for _, typ := range n.Types {
				_ = g.OfType(typ)
			}
			if n.ID != "" {
				_, _ = g.Resolve(n.ID)
			}
		}
		_ = g.Types()
	})
}

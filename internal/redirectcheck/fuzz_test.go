package redirectcheck

import (
	"strings"
	"testing"
)

// FuzzParseCSV feeds arbitrary uploads through the redirect-rule CSV parser, which the web
// UI exposes to whatever file a user picks. It must return rules or an error, never panic.
func FuzzParseCSV(f *testing.F) {
	f.Add("source,target\n/old,/new\n")
	f.Add("Source URL;Target URL;Status\n/a;https://example.com/b;301\n")
	f.Add("\xef\xbb\xbf\"source\",\"target\"\n\"/a\"\"b\",/c\n")
	f.Add("source\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, data string) {
		_, _ = ParseCSV(strings.NewReader(data))
	})
}

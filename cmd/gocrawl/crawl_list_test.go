package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

func TestReadURLList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "urls.txt")
	body := "# landing pages\nhttps://example.com/a\n\n  example.com/b  \r\n# end\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readURLList(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "https://example.com/a" || got[1] != "example.com/b" {
		t.Errorf("urls = %q", got)
	}

	got, err = readURLList("-", strings.NewReader("https://example.com/x\n"))
	if err != nil || len(got) != 1 {
		t.Errorf("stdin: urls = %q, err = %v", got, err)
	}
	if _, err := readURLList("-", strings.NewReader("# nothing\n\n")); err == nil {
		t.Error("an empty list was accepted")
	}
	if _, err := readURLList(filepath.Join(t.TempDir(), "missing.txt"), nil); err == nil {
		t.Error("a missing file was accepted")
	}
}

func TestWithScheme(t *testing.T) {
	for in, want := range map[string]string{
		"example.com/a":        "https://example.com/a",
		"http://example.com/a": "http://example.com/a",
	} {
		if got := withScheme(in); got != want {
			t.Errorf("withScheme(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFailOn(t *testing.T) {
	rep := &report.Report{Summary: report.Summary{BySeverity: map[string]int{
		string(analyze.Warning): 1, string(analyze.Info): 1,
	}}}
	for sev, wantFail := range map[string]bool{"": false, "error": false, "warning": true, "info": true} {
		err := failOn(sev, rep)
		if (err != nil) != wantFail {
			t.Errorf("failOn(%q) = %v, want failure %v", sev, err, wantFail)
		}
	}
	if err := failOn("info", rep); err == nil || !strings.Contains(err.Error(), "2 issue(s)") {
		t.Errorf("failOn(info) = %v, want it to count both issues", err)
	}
}

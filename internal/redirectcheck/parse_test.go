package redirectcheck_test

import (
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

const sampleCSV = `"Original URL","Redirect to","Redirect type","Redirect style","Priority","Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"
"/old-page","/new-page","STANDARD","301","1000000001","FALSE","TRUE","TRUE","TRUE",""
"https?://example.com/cases(?P<page_slug>/.*)?$","https://example.com/en/cases{page_slug}","STANDARD","301","2000000000","FALSE","TRUE","TRUE","TRUE",""
`

func TestParseCSV(t *testing.T) {
	rules, err := redirectcheck.ParseCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].Original != "/old-page" || rules[0].Target != "/new-page" {
		t.Errorf("row 0 = %+v, want Original=/old-page Target=/new-page", rules[0])
	}
	if rules[0].DisableIfPageExists != "TRUE" {
		t.Errorf("row 0 DisableIfPageExists = %q, want TRUE", rules[0].DisableIfPageExists)
	}
	if !strings.Contains(rules[1].Original, "(?P<") {
		t.Errorf("row 1 Original should retain regex syntax, got %q", rules[1].Original)
	}
}

func TestParseCSVRejectsWrongSchema(t *testing.T) {
	bad := "\"Original URL\",\"Redirect to\"\n\"/a\",\"/b\"\n"
	if _, err := redirectcheck.ParseCSV(strings.NewReader(bad)); err == nil {
		t.Fatal("expected an error for a CSV with the wrong column schema")
	}
}

package redirectcheck_test

import (
	"encoding/json"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestRowResultJSONUsesSnakeCase(t *testing.T) {
	b, err := json.Marshal(redirectcheck.RowResult{
		Scope: redirectcheck.ScopeInScope, SourceStatus: 301, SourceFinalURL: "https://example.com/a",
		SourceMatchesTarget: true, TargetStatus: 200, OriginalInSitemap: true, TargetInSitemap: true,
		Verdict: redirectcheck.VerdictOK, Notes: []string{"n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"scope":"in-scope","source_status":301,"source_final_url":"https://example.com/a",` +
		`"source_matches_target":true,"target_status":200,"original_in_sitemap":true,` +
		`"target_in_sitemap":true,"verdict":"ok","notes":["n"]}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}

	b, err = json.Marshal(redirectcheck.RowResult{Verdict: redirectcheck.VerdictSkippedExternal, Scope: redirectcheck.ScopeExternal})
	if err != nil {
		t.Fatal(err)
	}
	want = `{"scope":"external","source_status":0,"source_final_url":"","source_matches_target":false,` +
		`"target_status":0,"original_in_sitemap":false,"target_in_sitemap":false,"verdict":"skipped-external"}`
	if string(b) != want {
		t.Fatalf("notes must be omitted when empty\ngot  %s\nwant %s", b, want)
	}
}

func TestRuleJSONUsesSnakeCase(t *testing.T) {
	b, err := json.Marshal(redirectcheck.Rule{
		Original: "/old", Target: "/new", RedirectType: "301", RedirectStyle: "STANDARD", Priority: "1",
		MatchQueryStrings: "FALSE", IgnoreTrailingSlash: "TRUE", IgnoreProtocol: "TRUE",
		DisableIfPageExists: "FALSE", Note: "moved",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"original":"/old","target":"/new","redirect_type":"301","redirect_style":"STANDARD",` +
		`"priority":"1","match_query_strings":"FALSE","ignore_trailing_slash":"TRUE",` +
		`"ignore_protocol":"TRUE","disable_if_page_exists":"FALSE","note":"moved"}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
}

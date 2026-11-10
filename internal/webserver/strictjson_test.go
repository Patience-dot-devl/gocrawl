package webserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRequestBodiesRefuseTrailingData locks the rule docs/web.md states: data after the JSON
// value is a 400, for the handlers that use json.UnmarshalRead and for resume, whose body is
// optional and goes through decodeOptionalBody.
func TestRequestBodiesRefuseTrailingData(t *testing.T) {
	srv := newTestServer(t)
	post := func(path, body string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Host = "localhost:8080"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	for _, tc := range []struct {
		name, path, body string
		want             int
	}{
		{"crawl, trailing value", "/api/crawls", `{"url":"https://example.com"} {}`, http.StatusBadRequest},
		{"resume, trailing value", "/api/crawls/nope/resume", `{"rate":5} {}`, http.StatusBadRequest},
		{"resume, trailing junk", "/api/crawls/nope/resume", `{} junk`, http.StatusBadRequest},
		// A body that decodes cleanly gets past decoding to the job lookup.
		{"resume, empty body", "/api/crawls/nope/resume", ``, http.StatusNotFound},
		{"resume, trailing whitespace", "/api/crawls/nope/resume", "{}\n  ", http.StatusNotFound},
	} {
		if got := post(tc.path, tc.body); got != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, got, tc.want)
		}
	}
}

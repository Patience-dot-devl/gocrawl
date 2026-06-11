package httpx

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

func TestClientErrorReportsReferrer(t *testing.T) {
	result := &crawler.Result{Pages: []*crawler.Page{
		{RequestedURL: "https://example.com/missing", StatusCode: 404, Referrer: "https://example.com/blog"},
	}}

	var foundOn any
	gotClientError := false
	for _, iss := range New().Analyze(context.Background(), result) {
		if iss.Code == "client-error" {
			gotClientError = true
			foundOn = iss.Data["found_on"]
		}
	}
	if !gotClientError {
		t.Fatal("expected a client-error issue for the 404 page")
	}
	if foundOn != "https://example.com/blog" {
		t.Errorf("client-error found_on = %v, want %q", foundOn, "https://example.com/blog")
	}
}

func TestErrorWithoutReferrerOmitsFoundOn(t *testing.T) {
	result := &crawler.Result{Pages: []*crawler.Page{
		{RequestedURL: "https://example.com/missing", StatusCode: 404},
	}}

	for _, iss := range New().Analyze(context.Background(), result) {
		if iss.Code == "client-error" {
			if _, ok := iss.Data["found_on"]; ok {
				t.Error("found_on should be absent when the page has no referrer")
			}
		}
	}
}

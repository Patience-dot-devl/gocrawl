package datalayer_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/datalayer"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// staticPage builds a non-rendered HTML page (static tier only).
func staticPage(t *testing.T, html string) *crawler.Result {
	t.Helper()
	return &crawler.Result{Seed: "https://example.com/", Pages: []*crawler.Page{htmlPage(t, html, nil)}}
}

// renderedPage builds a headless-rendered page with the given dataLayer entries and
// captured request URLs.
func renderedPage(t *testing.T, html string, entries []string, requests []string) *crawler.Result {
	t.Helper()
	raw := make([]json.RawMessage, len(entries))
	for i, e := range entries {
		raw[i] = json.RawMessage(e)
	}
	render := &crawler.RenderResult{
		Implemented:      true,
		DataLayerPresent: true,
		DataLayer:        raw,
		Requests:         requests,
	}
	return &crawler.Result{Seed: "https://example.com/", Pages: []*crawler.Page{htmlPage(t, html, render)}}
}

func htmlPage(t *testing.T, html string, r *crawler.RenderResult) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &crawler.Page{FinalURL: "https://example.com/", StatusCode: 200, ContentType: "text/html", Doc: doc, Render: r}
}

func run(res *crawler.Result) []analyze.Issue {
	return datalayer.New().Analyze(context.Background(), res)
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func mustNot(t *testing.T, issues []analyze.Issue, code string) {
	t.Helper()
	if _, ok := find(issues, code); ok {
		t.Errorf("did not expect issue %q", code)
	}
}

func must(t *testing.T, issues []analyze.Issue, code string) analyze.Issue {
	t.Helper()
	is, ok := find(issues, code)
	if !ok {
		t.Fatalf("expected issue %q in %v", code, codes(issues))
	}
	return is
}

func codes(issues []analyze.Issue) []string {
	out := make([]string, len(issues))
	for i, is := range issues {
		out[i] = is.Code
	}
	return out
}

// --- static tier ---

const gtmSnippet = `<script>(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0];})(window,document,'script','dataLayer','GTM-ABC123');</script>`

func TestGTMNoscriptMissing(t *testing.T) {
	res := staticPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`)
	is := must(t, run(res), "datalayer-gtm-noscript-missing")
	if is.Severity != analyze.Warning {
		t.Errorf("severity = %v, want warning", is.Severity)
	}
}

func TestGTMNoscriptPresent(t *testing.T) {
	res := staticPage(t, `<html><head>`+gtmSnippet+`</head><body>
		<noscript><iframe src="https://www.googletagmanager.com/ns.html?id=GTM-ABC123"></iframe></noscript>
	</body></html>`)
	mustNot(t, run(res), "datalayer-gtm-noscript-missing")
}

func TestPushBeforeInit(t *testing.T) {
	res := staticPage(t, `<html><head>
		<script>dataLayer.push({event:'early'});</script>
		`+gtmSnippet+`
	</head><body></body></html>`)
	must(t, run(res), "datalayer-push-before-init")
}

func TestPushAfterInitNoWarning(t *testing.T) {
	res := staticPage(t, `<html><head>
		`+gtmSnippet+`
		<script>dataLayer.push({event:'ok'});</script>
	</head><body></body></html>`)
	mustNot(t, run(res), "datalayer-push-before-init")
}

func TestInitMissing(t *testing.T) {
	res := staticPage(t, `<html><head>
		<script async src="https://www.googletagmanager.com/gtag/js?id=G-XXXX1234"></script>
	</head><body></body></html>`)
	must(t, run(res), "datalayer-init-missing")
}

func TestGtagConfigIDMismatch(t *testing.T) {
	res := staticPage(t, `<html><head>
		<script async src="https://www.googletagmanager.com/gtag/js?id=G-AAAA1111"></script>
		<script>window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}gtag('config','G-BBBB2222');</script>
	</head><body></body></html>`)
	is := must(t, run(res), "datalayer-gtag-config-id-mismatch")
	if is.Data["config_id"] != "G-BBBB2222" {
		t.Errorf("config_id = %v", is.Data["config_id"])
	}
}

func TestGtagConfigIDMatchNoWarning(t *testing.T) {
	res := staticPage(t, `<html><head>
		<script async src="https://www.googletagmanager.com/gtag/js?id=G-AAAA1111"></script>
		<script>window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}gtag('config','G-AAAA1111');</script>
	</head><body></body></html>`)
	mustNot(t, run(res), "datalayer-gtag-config-id-mismatch")
}

func TestConsentModeMissingAndPresent(t *testing.T) {
	missing := staticPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`)
	must(t, run(missing), "datalayer-consent-mode-missing")

	present := staticPage(t, `<html><head>
		<script>gtag('consent','default',{ad_storage:'denied'});</script>
		`+gtmSnippet+`
	</head><body></body></html>`)
	got := run(present)
	must(t, got, "datalayer-consent-mode-present")
	mustNot(t, got, "datalayer-consent-mode-missing")
}

func TestStaticTierRunsWithoutRender(t *testing.T) {
	res := staticPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`)
	got := run(res)
	must(t, got, "datalayer-not-collected")        // runtime checks need headless
	must(t, got, "datalayer-gtm-noscript-missing") // static checks still ran
}

// --- runtime tier ---

func TestEventInventoryAndPageView(t *testing.T) {
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{`{"event":"gtm.js"}`, `{"event":"page_view"}`, `{"event":"add_to_cart","currency":"EUR","value":9.99,"items":[{"id":"x"}]}`},
		nil)
	got := run(res)
	inv := must(t, got, "datalayer-events")
	events, _ := inv.Data["events"].([]map[string]any)
	if len(events) != 3 {
		t.Errorf("event count = %d, want 3 (%v)", len(events), events)
	}
	mustNot(t, got, "datalayer-page-view-missing")
}

func TestPageViewMissing(t *testing.T) {
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{`{"event":"custom_click"}`}, nil)
	must(t, run(res), "datalayer-page-view-missing")
}

func TestDataLayerEmpty(t *testing.T) {
	html := `<html><head>` + gtmSnippet + `</head><body></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	res := &crawler.Result{Seed: "https://example.com/", Pages: []*crawler.Page{{
		FinalURL: "https://example.com/", StatusCode: 200, ContentType: "text/html", Doc: doc,
		Render: &crawler.RenderResult{Implemented: true, DataLayerPresent: false},
	}}}
	must(t, run(res), "datalayer-empty")
}

func TestEcommercePurchaseMissingParams(t *testing.T) {
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{`{"event":"purchase","ecommerce":{"value":10,"currency":"USD"}}`}, nil)
	is := must(t, run(res), "datalayer-ecommerce-event-invalid")
	missing, _ := is.Data["missing"].([]string)
	if !contains(missing, "transaction_id") || !contains(missing, "items") {
		t.Errorf("missing = %v, want transaction_id and items", missing)
	}
}

func TestEcommerceValidPurchase(t *testing.T) {
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{`{"event":"purchase","ecommerce":{"transaction_id":"T1","value":10.5,"currency":"USD","items":[{"id":"a"}]}}`}, nil)
	mustNot(t, run(res), "datalayer-ecommerce-event-invalid")
}

func TestParamTypeStringValue(t *testing.T) {
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{`{"event":"add_to_cart","currency":"usd","value":"9.99","items":[{"id":"a"}]}`}, nil)
	got := run(res)
	// value is a string, currency is lowercase: both flagged as param-type.
	is := must(t, got, "datalayer-param-type")
	_ = is
	var params []string
	for _, i := range got {
		if i.Code == "datalayer-param-type" {
			if p, ok := i.Data["param"].(string); ok {
				params = append(params, p)
			}
		}
	}
	if !contains(params, "value") || !contains(params, "currency") {
		t.Errorf("flagged params = %v, want value and currency", params)
	}
}

func TestGtagArgumentsEvent(t *testing.T) {
	// gtag('event','purchase',{...}) serializes as an arguments object.
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{`{"event":"gtm.js"}`, `{"0":"event","1":"purchase","2":{"value":5,"currency":"EUR"}}`}, nil)
	is := must(t, run(res), "datalayer-ecommerce-event-invalid")
	if is.Data["event"] != "purchase" {
		t.Errorf("event = %v, want purchase", is.Data["event"])
	}
}

func TestDuplicateConversion(t *testing.T) {
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{
			`{"event":"purchase","ecommerce":{"transaction_id":"T1","value":1,"currency":"USD","items":[{"id":"a"}]}}`,
			`{"event":"purchase","ecommerce":{"transaction_id":"T1","value":1,"currency":"USD","items":[{"id":"a"}]}}`,
		}, nil)
	got := run(res)
	must(t, got, "datalayer-duplicate-event")
	must(t, got, "datalayer-duplicate-transaction")
}

func TestPIIEmailAndPhone(t *testing.T) {
	res := renderedPage(t, `<html><head>`+gtmSnippet+`</head><body></body></html>`,
		[]string{`{"event":"login","user":{"email":"jane.doe@example.com","phone":"+1 (555) 123-4567"}}`}, nil)
	got := run(res)
	var kinds []string
	for _, i := range got {
		if i.Code == "datalayer-pii" {
			if k, ok := i.Data["kind"].(string); ok {
				kinds = append(kinds, k)
			}
		}
	}
	if !contains(kinds, "email") || !contains(kinds, "phone") {
		t.Errorf("PII kinds = %v, want email and phone", kinds)
	}
}

func TestTagFiringAndNotFiring(t *testing.T) {
	html := `<html><head>
		<script async src="https://www.googletagmanager.com/gtag/js?id=G-XXXX1234"></script>
		<script>window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}gtag('config','G-XXXX1234');</script>
	</head><body></body></html>`
	// GA4 configured but no /g/collect beacon -> tag-not-firing.
	notFiring := renderedPage(t, html, []string{`{"0":"config","1":"G-XXXX1234"}`}, []string{"https://example.com/style.css"})
	must(t, run(notFiring), "datalayer-tag-not-firing")

	// With the beacon present -> tags-firing, no tag-not-firing.
	firing := renderedPage(t, html, []string{`{"0":"config","1":"G-XXXX1234"}`},
		[]string{"https://www.google-analytics.com/g/collect?v=2&tid=G-XXXX1234"})
	got := run(firing)
	must(t, got, "datalayer-tags-firing")
	mustNot(t, got, "datalayer-tag-not-firing")
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

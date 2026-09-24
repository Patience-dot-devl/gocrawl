package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connect wires a client to New's server over an in-memory transport, so the tools are
// exercised through the real MCP request path rather than by calling the handlers directly.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := New("test").Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestListToolsExposesCrawlAndListAnalyzers(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"crawl", "list_analyzers"} {
		if !got[want] {
			t.Errorf("tool %q not listed; got %v", want, got)
		}
	}
}

func TestListAnalyzersTool(t *testing.T) {
	cs := connect(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_analyzers"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %v", res.Content)
	}
	var out ListAnalyzersOutput
	decode(t, res.StructuredContent, &out)
	names := map[string]bool{}
	for _, a := range out.Analyzers {
		names[a.Name] = true
	}
	if !names["seo"] || !names["redirects"] {
		t.Fatalf("analyzers = %v, want at least seo and redirects", names)
	}
}

func TestCrawlToolReturnsReport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
			return
		}
		fmt.Fprint(w, `<html><head></head><body><a href="/about">about</a></body></html>`)
	}))
	defer ts.Close()

	cs := connect(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "crawl",
		Arguments: map[string]any{"url": ts.URL, "depth": 1, "max_pages": 5, "analyzers": []string{"seo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %v", res.Content)
	}
	var out CrawlOutput
	decode(t, res.StructuredContent, &out)
	if out.Report.PagesCrawled < 2 {
		t.Fatalf("pages_crawled = %d, want >= 2 (seed and /about)", out.Report.PagesCrawled)
	}
	if out.Report.Seed != ts.URL+"/" {
		t.Errorf("seed = %q, want normalized %q", out.Report.Seed, ts.URL+"/")
	}
	sawMissingTitle := false
	for _, is := range out.Report.Issues {
		if is.Analyzer != "seo" {
			t.Errorf("issue from %q leaked past the analyzers filter", is.Analyzer)
		}
		if is.Code == "seo-missing-title" {
			sawMissingTitle = true
		}
	}
	if !sawMissingTitle {
		t.Error("expected seo-missing-title for a page with no <title>")
	}
}

func TestCrawlToolRejectsMissingURL(t *testing.T) {
	cs := connect(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "crawl", Arguments: map[string]any{}})
	if err == nil && !res.IsError {
		t.Fatalf("crawl without url succeeded: %+v", res.StructuredContent)
	}
}

func decode(t *testing.T, structured any, into any) {
	t.Helper()
	b, err := json.Marshal(structured)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("decode structured content %s: %v", b, err)
	}
}

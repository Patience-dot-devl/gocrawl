package mcpserver

import "testing"

// New builds every tool's input/output JSON schema via reflection (go-sdk's ForType). A
// self-referential field anywhere in CrawlOutput (e.g. embedding the site-map tree, whose
// Node.Children []*Node cycles) makes ForType panic at startup — this guards against that
// regressing silently, since AddTool only runs when the server actually starts.
func TestNewDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New panicked: %v", r)
		}
	}()
	New("test")
}

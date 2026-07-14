package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleYAMLParses ensures the template `gocrawl init` writes is itself valid, loadable
// configuration — a regression here would mean every fresh `gocrawl init` produces a file that
// fails on the first crawl.
func TestExampleYAMLParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gocrawl.yaml")
	if err := os.WriteFile(path, []byte(ExampleYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(example config): %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("example config fails Validate: %v", err)
	}
}

// TestExampleYAMLNotStale guards against the template drifting from reality: it previously
// described headless rendering as "stubbed" (it's fully implemented), omitted "html" from the
// documented output formats, omitted the botwall/datalayer analyzers from the documented
// analyzer list, and described max_depth: 0 as "seed only" rather than unlimited.
func TestExampleYAMLNotStale(t *testing.T) {
	if strings.Contains(ExampleYAML, "stubbed") {
		t.Error(`ExampleYAML still describes headless rendering as "stubbed"; it's implemented`)
	}
	if !strings.Contains(ExampleYAML, "html") {
		t.Error("ExampleYAML doesn't mention the html output format")
	}
	if !strings.Contains(ExampleYAML, "botwall") {
		t.Error("ExampleYAML's analyzer list omits botwall")
	}
	if !strings.Contains(ExampleYAML, "datalayer") {
		t.Error("ExampleYAML's analyzer list omits datalayer")
	}
	if strings.Contains(ExampleYAML, "only the seed page") {
		t.Error("ExampleYAML still documents max_depth: 0 as \"seed only\"; 0 means unlimited")
	}
}

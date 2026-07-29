package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

// TestExampleYAMLCoversEveryOption walks Config's mapstructure tags and requires each one to
// appear as a key in the template. This is the check that would have caught the template
// silently omitting basic_auth, the proxy/User-Agent rotation options, verbose, and
// output.sitemap_path — a starter config that doesn't mention an option is how users end up
// believing it doesn't exist.
func TestExampleYAMLCoversEveryOption(t *testing.T) {
	for _, key := range mapstructureKeys(reflect.TypeOf(Config{})) {
		if !strings.Contains(ExampleYAML, key+":") {
			t.Errorf("ExampleYAML has no %q key; every config option belongs in the starter template", key)
		}
	}
}

// mapstructureKeys returns the leaf mapstructure tag names of t, descending into nested
// structs (but not into time.Duration, which is a named int64 with no fields worth walking).
func mapstructureKeys(t reflect.Type) []string {
	var keys []string
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("mapstructure")
		if tag == "" {
			continue
		}
		if f.Type.Kind() == reflect.Struct && f.Type != reflect.TypeOf(time.Duration(0)) {
			keys = append(keys, mapstructureKeys(f.Type)...)
			continue
		}
		keys = append(keys, tag)
	}
	return keys
}

// TestExampleYAMLCopiesAreInSync keeps the two checked-in copies of the starter template
// byte-identical to ExampleYAML. Both had drifted badly before this test existed: they
// disagreed with each other and with the constant on whether headless rendering was
// implemented, how Basic Auth is scoped, and which options exist at all — while
// docs/configuration.md claimed to show "the template below" that `gocrawl init` writes.
func TestExampleYAMLCopiesAreInSync(t *testing.T) {
	root := filepath.Join("..", "..")

	yamlCopy, err := os.ReadFile(filepath.Join(root, "configs", "example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(yamlCopy) != ExampleYAML {
		t.Error("configs/example.yaml has drifted from config.ExampleYAML; re-copy the constant into it verbatim")
	}

	doc, err := os.ReadFile(filepath.Join(root, "docs", "configuration.md"))
	if err != nil {
		t.Fatal(err)
	}
	block := docExampleBlock(string(doc))
	if block == "" {
		t.Fatal("could not find the fenced yaml example-config block in docs/configuration.md")
	}
	if block != ExampleYAML {
		t.Error("the example-config block in docs/configuration.md has drifted from config.ExampleYAML; re-copy the constant into it verbatim")
	}
}

// docExampleBlock extracts the fenced YAML block that follows the "Example config file"
// heading in docs/configuration.md, or "" if it isn't there.
func docExampleBlock(doc string) string {
	_, after, ok := strings.Cut(doc, "## Example config file")
	if !ok {
		return ""
	}
	_, after, ok = strings.Cut(after, "```yaml\n")
	if !ok {
		return ""
	}
	block, _, ok := strings.Cut(after, "```")
	if !ok {
		return ""
	}
	return block
}

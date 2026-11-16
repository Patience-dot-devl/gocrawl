package main

import "testing"

func TestResolveVersionPrefersLdflags(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })

	version = "1.2.3"
	if got := resolveVersion(); got != "1.2.3" {
		t.Fatalf("resolveVersion() = %q, want the ldflags value 1.2.3", got)
	}
}

func TestResolveVersionFallsBack(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })

	version = ""
	// A test binary has no module version, so the fallback must not be empty or "(devel)".
	if got := resolveVersion(); got == "" || got == "(devel)" {
		t.Fatalf("resolveVersion() = %q, want a usable version", got)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRedirectsRequiresInputAndDomain(t *testing.T) {
	cmd := newCheckRedirectsCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error when --input and --domain are missing")
	}
}

func TestCheckRedirectsErrorsOnMissingInputFile(t *testing.T) {
	cmd := newCheckRedirectsCmd()
	cmd.SetArgs([]string{"--input", filepath.Join(t.TempDir(), "does-not-exist.csv"), "--domain", "example.com"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a nonexistent input file")
	}
	if !strings.Contains(err.Error(), "opening input") {
		t.Errorf("error = %v, want it to mention opening the input file", err)
	}
}

func TestCheckRedirectsErrorsOnBadCSVSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(path, []byte("\"Original URL\",\"Redirect to\"\n\"/a\",\"/b\"\n"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	cmd := newCheckRedirectsCmd()
	cmd.SetArgs([]string{"--input", path, "--domain", "example.com"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a CSV with the wrong column schema")
	}
	if !strings.Contains(err.Error(), "parsing input CSV") {
		t.Errorf("error = %v, want it to mention parsing the input CSV", err)
	}
}

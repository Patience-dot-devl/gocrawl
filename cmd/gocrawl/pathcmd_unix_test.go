//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirOnPath(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	t.Setenv("PATH", strings.Join([]string{other, dir, "/usr/bin"}, string(os.PathListSeparator)))

	if !dirOnPath(dir) {
		t.Errorf("dirOnPath(%q) = false, want true", dir)
	}
	// A clean-equivalent form (trailing slash) should still match.
	if !dirOnPath(dir + "/") {
		t.Errorf("dirOnPath(%q) = false, want true (trailing slash)", dir+"/")
	}
	if dirOnPath(filepath.Join(dir, "nope")) {
		t.Errorf("dirOnPath of an absent dir = true, want false")
	}
}

func TestDetectShell(t *testing.T) {
	for _, tc := range []struct{ shell, want string }{
		{"/bin/zsh", "zsh"},
		{"/usr/bin/bash", "bash"},
		{"/usr/local/bin/fish", "fish"},
		{"/bin/dash", ""}, // unrecognized → platform default
	} {
		t.Setenv("SHELL", tc.shell)
		got := detectShell()
		if tc.want != "" && got != tc.want {
			t.Errorf("detectShell(%q) = %q, want %q", tc.shell, got, tc.want)
		}
		if tc.want == "" && got != "zsh" && got != "bash" {
			t.Errorf("detectShell(%q) = %q, want a platform default (zsh/bash)", tc.shell, got)
		}
	}
}

func TestProfileAndLine(t *testing.T) {
	dir := "/Users/you/go/bin"
	if _, line := profileAndLine("zsh", dir); !strings.Contains(line, "export PATH=") || !strings.Contains(line, dir) {
		t.Errorf("zsh line = %q, want an export containing %q", line, dir)
	}
	if _, line := profileAndLine("fish", dir); !strings.HasPrefix(line, "fish_add_path") {
		t.Errorf("fish line = %q, want a fish_add_path command", line)
	}
}

func TestAppendProfileLineIdempotent(t *testing.T) {
	profile := filepath.Join(t.TempDir(), ".zshrc")
	dir := "/Users/you/go/bin"
	line := exportLine(dir)

	already, err := appendProfileLine(profile, line, dir)
	if err != nil {
		t.Fatalf("first append: %v", err)
	}
	if already {
		t.Fatalf("first append reported alreadyPresent, want false")
	}

	already, err = appendProfileLine(profile, line, dir)
	if err != nil {
		t.Fatalf("second append: %v", err)
	}
	if !already {
		t.Fatalf("second append reported alreadyPresent=false, want true")
	}

	data, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), dir); n != 1 {
		t.Errorf("profile contains dir %d times, want exactly 1:\n%s", n, data)
	}
}

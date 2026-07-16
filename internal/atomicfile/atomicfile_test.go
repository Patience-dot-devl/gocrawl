package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := Write(path, 0o644, func(w io.Writer) error {
		_, err := io.WriteString(w, "hello")
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

func TestWriteCreatesParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "out.txt")
	if err := Write(path, 0o644, func(w io.Writer) error {
		_, err := io.WriteString(w, "hi")
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

// TestWriteDoesNotTruncateOnFailure guards against the real bug: os.Create truncates the
// destination immediately, before any new content is written, so a write that fails partway
// through destroys the previous good artifact. Write must leave the original file untouched
// when the callback errors.
func TestWriteDoesNotTruncateOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(path, []byte("previous good content"), 0o644); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("boom")
	err := Write(path, 0o644, func(w io.Writer) error {
		_, _ = io.WriteString(w, "partial garbage")
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Write error = %v, want %v", err, wantErr)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "previous good content" {
		t.Errorf("destination was modified on failure: %q", got)
	}
}

func TestWriteLeavesNoTempFileBehindOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	_ = Write(path, 0o644, func(w io.Writer) error {
		return errors.New("boom")
	})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no leftover files, got %v", entries)
	}
}

func TestWriteSetsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := Write(path, 0o600, func(w io.Writer) error {
		_, err := io.WriteString(w, "secret")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteOverwritesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, 0o644, func(w io.Writer) error {
		_, err := io.WriteString(w, "new")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("content = %q, want %q", got, "new")
	}
}

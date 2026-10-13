package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestHeaderProfilesSaveListReadDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	names, err := ListHeaderProfiles()
	if err != nil {
		t.Fatalf("ListHeaderProfiles on a fresh home: %v", err)
	}
	if names == nil || len(names) != 0 {
		t.Fatalf("names = %#v, want an empty, non-nil list", names)
	}

	if err := SaveHeaderProfile("uk", "# UK store\r\nSignature: abc\r\nSignature-Agent: https://shopify.com"); err != nil {
		t.Fatalf("SaveHeaderProfile(uk): %v", err)
	}
	if err := SaveHeaderProfile("de", "X-Token: 1\n"); err != nil {
		t.Fatalf("SaveHeaderProfile(de): %v", err)
	}

	names, err = ListHeaderProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"de", "uk"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}

	body, err := ReadHeaderProfile("uk")
	if err != nil {
		t.Fatal(err)
	}
	if want := "# UK store\nSignature: abc\nSignature-Agent: https://shopify.com\n"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}

	if runtime.GOOS != "windows" {
		path, _ := HeaderProfilePath("uk")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("profile mode = %o, want 600", perm)
		}
		dirInfo, err := os.Stat(HeaderProfileDir())
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Errorf("profile dir mode = %o, want 700", perm)
		}
	}

	if err := DeleteHeaderProfile("uk"); err != nil {
		t.Fatal(err)
	}
	names, _ = ListHeaderProfiles()
	if want := []string{"de"}; !reflect.DeepEqual(names, want) {
		t.Errorf("after delete names = %v, want %v", names, want)
	}
	if _, err := ReadHeaderProfile("uk"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadHeaderProfile(deleted) err = %v, want fs.ErrNotExist", err)
	}
	if err := DeleteHeaderProfile("uk"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("DeleteHeaderProfile(deleted) err = %v, want fs.ErrNotExist", err)
	}
}

func TestSaveHeaderProfileValidates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cases := []struct{ label, name, body string }{
		{"traversal", "../evil", "X-Token: 1"},
		{"hidden name", ".hidden", "X-Token: 1"},
		{"no colon", "uk", "just some text"},
		{"reserved header", "uk", "Cookie: a=b"},
		{"no headers", "uk", "# only a comment\n\n"},
		{"control character", "uk", "X-Token: a\x00b"},
	}
	for _, tc := range cases {
		if err := SaveHeaderProfile(tc.name, tc.body); !errors.Is(err, ErrInvalidHeaderProfile) {
			t.Errorf("%s: err = %v, want ErrInvalidHeaderProfile", tc.label, err)
		}
	}
	if names, _ := ListHeaderProfiles(); len(names) != 0 {
		t.Errorf("invalid saves wrote profiles: %v", names)
	}
}

// A rejected body names the offending line, never its content: profile values are secrets.
func TestSaveHeaderProfileErrorsOmitBodyContent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const secret = "s3cr3t-token-value"
	cases := []struct{ label, body string }{
		{"no colon", "X-Ok: 1\n" + secret},
		{"reserved header", "Cookie: session=" + secret},
		{"control character", "X-Token: " + secret + "\x00"},
		{"invalid name", "Bad Name " + secret + ": 1"},
	}
	for _, tc := range cases {
		err := SaveHeaderProfile("uk", tc.body)
		if !errors.Is(err, ErrInvalidHeaderProfile) {
			t.Errorf("%s: err = %v, want ErrInvalidHeaderProfile", tc.label, err)
			continue
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("%s: error %q contains the body's secret value", tc.label, err)
		}
	}
}

func TestHeaderProfileFuncsRejectBadNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := ReadHeaderProfile("../x"); !errors.Is(err, ErrInvalidHeaderProfile) {
		t.Errorf("ReadHeaderProfile(../x) err = %v, want ErrInvalidHeaderProfile", err)
	}
	if err := DeleteHeaderProfile("a/b"); !errors.Is(err, ErrInvalidHeaderProfile) {
		t.Errorf("DeleteHeaderProfile(a/b) err = %v, want ErrInvalidHeaderProfile", err)
	}
}

func TestListHeaderProfilesSkipsOtherEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".gocrawl", "headers")
	writeFile(t, filepath.Join(dir, "uk.headers"), "X-A: 1\n")
	writeFile(t, filepath.Join(dir, "notes.txt"), "x")
	writeFile(t, filepath.Join(dir, ".hidden.headers"), "X-A: 1\n")
	if err := os.MkdirAll(filepath.Join(dir, "sub.headers"), 0o700); err != nil {
		t.Fatal(err)
	}
	names, err := ListHeaderProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"uk"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

// A profile saved through the API is one a crawl can use by name.
func TestSavedHeaderProfileFeedsToOptions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHeaderProfile("uk", "Signature: abc\n"); err != nil {
		t.Fatal(err)
	}
	c := Default()
	c.Crawl.HeaderProfile = "uk"
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if got := o.Headers.Get("Signature"); got != "abc" {
		t.Errorf("Signature = %q, want abc", got)
	}
}

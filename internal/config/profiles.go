package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/atomicfile"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// ErrInvalidHeaderProfile marks a header profile name or body that can't be used. The web
// API maps it to 400; a profile that doesn't exist wraps fs.ErrNotExist instead.
var ErrInvalidHeaderProfile = errors.New("invalid header profile")

const headerProfileExt = ".headers"

// ListHeaderProfiles returns the names of the profiles in HeaderProfileDir, sorted. A missing
// directory is not an error: no profile has been saved yet.
func ListHeaderProfiles() ([]string, error) {
	names := []string{}
	entries, err := os.ReadDir(HeaderProfileDir())
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name, ok := strings.CutSuffix(e.Name(), headerProfileExt)
		if !ok || !headerProfileName.MatchString(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// ReadHeaderProfile returns a profile's file content.
func ReadHeaderProfile(name string) (string, error) {
	path, err := HeaderProfilePath(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("header profile %q: %w", name, fs.ErrNotExist)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveHeaderProfile validates body with crawler.ParseHeaderLines and writes it atomically to
// the profile's file with mode 0600, in a 0700 directory: profiles hold access secrets.
// Windows line endings are normalized and a final newline is added. A rejected body is
// reported by line number only; its content never appears in the error.
func SaveHeaderProfile(name, body string) error {
	path, err := HeaderProfilePath(name)
	if err != nil {
		return err
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	h, err := crawler.ParseHeaderLines(strings.Split(body, "\n"))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidHeaderProfile, err)
	}
	if len(h) == 0 {
		return fmt.Errorf(`%w: no "Name: value" lines`, ErrInvalidHeaderProfile)
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.Write(path, 0o600, func(w io.Writer) error {
		_, err := io.WriteString(w, body)
		return err
	})
}

// DeleteHeaderProfile removes a profile's file.
func DeleteHeaderProfile(name string) error {
	path, err := HeaderProfilePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("header profile %q: %w", name, fs.ErrNotExist)
		}
		return err
	}
	return nil
}

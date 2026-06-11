//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// addBinDirToPath appends a PATH line for dir to the user's shell profile, choosing the
// profile file and syntax based on the active shell.
func addBinDirToPath(dir string) (pathUpdate, error) {
	profile, line := profileAndLine(detectShell(), dir)

	already, err := appendProfileLine(profile, line, dir)
	if err != nil {
		return pathUpdate{}, err
	}
	return pathUpdate{
		Target:         tildeAbbrev(profile),
		Reload:         fmt.Sprintf("Restart your terminal or run: source %s", tildeAbbrev(profile)),
		AlreadyPresent: already,
	}, nil
}

// manualInstructions returns copy-pasteable steps for adding dir to PATH by hand, used when
// we can't or shouldn't modify the profile automatically.
func manualInstructions(dir string) string {
	profile, line := profileAndLine(detectShell(), dir)
	return fmt.Sprintf(
		"To run `gocrawl` from anywhere, add its directory to your PATH:\n\n"+
			"  echo '%s' >> %s\n"+
			"  source %s\n",
		line, tildeAbbrev(profile), tildeAbbrev(profile))
}

// detectShell identifies the user's login shell from $SHELL, falling back to the platform
// default (zsh on macOS, bash elsewhere) when it is unset or unrecognized.
func detectShell() string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		return "zsh"
	case "bash":
		return "bash"
	case "fish":
		return "fish"
	}
	if runtime.GOOS == "darwin" {
		return "zsh"
	}
	return "bash"
}

// profileAndLine returns the profile file to edit and the line to append for the given shell.
func profileAndLine(shell, dir string) (profile, line string) {
	home, _ := os.UserHomeDir()
	switch shell {
	case "fish":
		// fish doesn't use `export`; fish_add_path is idempotent and the documented way.
		return filepath.Join(home, ".config", "fish", "config.fish"),
			fmt.Sprintf("fish_add_path %q", dir)
	case "zsh":
		return filepath.Join(home, ".zshrc"), exportLine(dir)
	case "bash":
		// macOS terminals start login shells (read ~/.bash_profile); Linux terminals start
		// interactive non-login shells (read ~/.bashrc).
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile"), exportLine(dir)
		}
		return filepath.Join(home, ".bashrc"), exportLine(dir)
	default:
		return filepath.Join(home, ".profile"), exportLine(dir)
	}
}

// exportLine builds a POSIX export that appends dir to PATH (appended, not prepended, so it
// never shadows system binaries).
func exportLine(dir string) string {
	return fmt.Sprintf("export PATH=\"$PATH:%s\"", dir)
}

// appendProfileLine appends a marked block setting PATH to profile. It is idempotent: if dir
// already appears anywhere in the file it makes no change and reports alreadyPresent=true.
func appendProfileLine(profile, line, dir string) (alreadyPresent bool, err error) {
	if data, readErr := os.ReadFile(profile); readErr == nil && strings.Contains(string(data), dir) {
		return true, nil
	}

	if mkErr := os.MkdirAll(filepath.Dir(profile), 0o755); mkErr != nil {
		return false, fmt.Errorf("preparing %s: %w", tildeAbbrev(profile), mkErr)
	}
	f, err := os.OpenFile(profile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, fmt.Errorf("opening %s: %w", tildeAbbrev(profile), err)
	}
	// Surface a flush error from Close, but don't let it mask an earlier write error.
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing %s: %w", tildeAbbrev(profile), cerr)
		}
	}()

	block := fmt.Sprintf("\n# Added by gocrawl — make the gocrawl binary available on PATH\n%s\n", line)
	if _, err := f.WriteString(block); err != nil {
		return false, fmt.Errorf("writing to %s: %w", tildeAbbrev(profile), err)
	}
	return false, nil
}

// tildeAbbrev shortens a path under the home directory to a ~-prefixed form for display.
func tildeAbbrev(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p == home {
			return "~"
		}
		if strings.HasPrefix(p, home+string(os.PathSeparator)) {
			return "~" + p[len(home):]
		}
	}
	return p
}

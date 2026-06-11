package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// pathUpdate describes the result of adding the binary directory to PATH so the caller can
// print an accurate message regardless of platform.
type pathUpdate struct {
	Target         string // human description of what changed, e.g. "~/.zshrc" or "your user PATH"
	Reload         string // how to make the change take effect
	AlreadyPresent bool   // the directory was already configured in Target, just not yet loaded
}

func newPathCmd() *cobra.Command {
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Add the gocrawl binary's directory to your PATH",
		Long: "Detects where the running gocrawl binary lives and, if that directory is not\n" +
			"already on your PATH, offers to add it — to your shell profile on macOS/Linux\n" +
			"(zsh, bash, or fish) or to your user PATH on Windows — so you can run `gocrawl`\n" +
			"from any terminal.\n\n" +
			"On a non-interactive terminal (or when you answer no) it changes nothing and just\n" +
			"prints the manual instructions instead.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPath(cmd, assumeYes)
		},
	}
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "skip the confirmation prompt and update PATH directly")
	return cmd
}

func runPath(cmd *cobra.Command, assumeYes bool) error {
	out := cmd.OutOrStdout()

	dir, err := binaryDir()
	if err != nil {
		return err
	}

	if dirOnPath(dir) {
		_, err := fmt.Fprintf(out, "%s is already on your PATH — nothing to do.\n", dir)
		return err
	}

	// Without an interactive terminal we can't safely prompt, so don't touch anything —
	// print the manual steps and exit cleanly, mirroring the bare-invocation fallback.
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	if !assumeYes && !interactive {
		_, err := fmt.Fprint(out, manualInstructions(dir))
		return err
	}

	if !assumeYes {
		confirm := true
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Add %s to your PATH?", dir)).
				Description("Lets you run `gocrawl` from any terminal.").
				Affirmative("Yes, add it").
				Negative("No").
				Value(&confirm),
		))
		if err := runForm(form); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil // clean exit on Ctrl-C / Esc
			}
			return err
		}
		if !confirm {
			_, err := fmt.Fprint(out, manualInstructions(dir))
			return err
		}
	}

	upd, err := addBinDirToPath(dir)
	if err != nil {
		return err
	}
	if upd.AlreadyPresent {
		_, err := fmt.Fprintf(out, "%s is already configured in %s but not loaded yet.\n%s\n", dir, upd.Target, upd.Reload)
		return err
	}
	_, err = fmt.Fprintf(out, "Added %s to %s.\n%s\n", dir, upd.Target, upd.Reload)
	return err
}

// binaryDir returns the absolute directory containing the running gocrawl binary, resolving
// any symlink (`go install` and Homebrew-style layouts commonly symlink the binary).
func binaryDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating the gocrawl binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

// dirOnPath reports whether dir is already one of the entries in the PATH environment
// variable, comparing in a platform-appropriate way.
func dirOnPath(dir string) bool {
	target := cleanPathEntry(dir)
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == "" {
			continue
		}
		if cleanPathEntry(entry) == target {
			return true
		}
	}
	return false
}

// cleanPathEntry normalizes a PATH entry for comparison: symlinks resolved (so e.g. macOS's
// /tmp vs /private/tmp, or a symlinked bin dir, compare equal to the resolved binaryDir),
// cleaned, and case-folded on Windows where the filesystem is case-insensitive.
func cleanPathEntry(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		return strings.ToLower(p)
	}
	return p
}

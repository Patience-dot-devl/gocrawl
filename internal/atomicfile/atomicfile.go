// Package atomicfile writes files atomically: content is written to a temporary file in the
// same directory as the destination, then renamed into place. A write that fails partway
// through (a full disk, a killed process, a crash) leaves the temp file incomplete but never
// touches the destination, so a good previous artifact is never truncated or corrupted by a
// failed overwrite — os.Create alone truncates the destination immediately, before a single
// byte of the new content is written.
package atomicfile

import (
	"io"
	"os"
	"path/filepath"
)

// Write creates parent directories for path as needed, calls write with a temporary file in
// the same directory, and renames it into place at path only if write and the subsequent close
// both succeed. perm sets the final file's permissions. A same-directory temp file guarantees
// the rename is on the same filesystem, so it's atomic.
func Write(path string, perm os.FileMode, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	tmp, err := os.CreateTemp(dir, ".gocrawl-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpPath)
		}
	}()

	writeErr := write(tmp)
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	renamed = true
	return nil
}

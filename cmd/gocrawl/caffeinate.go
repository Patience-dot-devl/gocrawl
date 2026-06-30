package main

import (
	"os/exec"
	"runtime"
)

// caffeinateSupported reports whether a keep-awake toggle is meaningful on this OS. Only macOS
// ships the caffeinate(8) utility we shell out to, so the menu hides the toggle elsewhere.
func caffeinateSupported() bool {
	return runtime.GOOS == "darwin"
}

// startCaffeinate raises a macOS power assertion that prevents idle system sleep for as long as
// the returned stop function is uncalled, so a locked screen or idle timer doesn't pause a long
// crawl. It shells out to caffeinate(8) with -i (prevent idle sleep); if caffeinate is missing
// or fails to start it degrades to a no-op. The returned stop function is always safe to call.
func startCaffeinate() (stop func()) {
	noop := func() {}
	if runtime.GOOS != "darwin" {
		return noop
	}
	path, err := exec.LookPath("caffeinate")
	if err != nil {
		return noop
	}
	cmd := exec.Command(path, "-i")
	if err := cmd.Start(); err != nil {
		return noop
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

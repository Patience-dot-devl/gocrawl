//go:build windows

package main

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// addBinDirToPath appends dir to the user's PATH in the registry (HKCU\Environment) and
// broadcasts the change so newly launched terminals pick it up.
func addBinDirToPath(dir string) (pathUpdate, error) {
	const reload = "Open a new terminal for the change to take effect."

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return pathUpdate{}, fmt.Errorf("opening the user environment registry key: %w", err)
	}
	defer k.Close()

	current, valType, getErr := k.GetStringValue("Path")
	if getErr != nil && getErr != registry.ErrNotExist {
		return pathUpdate{}, fmt.Errorf("reading your user PATH: %w", getErr)
	}

	// Don't double-add if a manual edit already lists the directory.
	for _, entry := range strings.Split(current, ";") {
		if strings.EqualFold(strings.TrimRight(entry, `\`), strings.TrimRight(dir, `\`)) {
			return pathUpdate{Target: "your user PATH", Reload: reload, AlreadyPresent: true}, nil
		}
	}

	updated := dir
	if current != "" {
		updated = strings.TrimRight(current, ";") + ";" + dir
	}

	// Preserve REG_EXPAND_SZ (Path normally uses it so entries like %USERPROFILE% expand);
	// default new values to it too, matching the Windows convention. Our dir is literal, so
	// either type is safe for it.
	if valType == registry.EXPAND_SZ || getErr == registry.ErrNotExist {
		err = k.SetExpandStringValue("Path", updated)
	} else {
		err = k.SetStringValue("Path", updated)
	}
	if err != nil {
		return pathUpdate{}, fmt.Errorf("updating your user PATH: %w", err)
	}

	broadcastEnvChange()
	return pathUpdate{Target: "your user PATH", Reload: reload}, nil
}

// manualInstructions returns PowerShell steps for adding dir to the user PATH by hand.
func manualInstructions(dir string) string {
	return fmt.Sprintf(
		"To run `gocrawl` from anywhere, add its directory to your user PATH in PowerShell:\n\n"+
			"  $gobin = \"%s\"\n"+
			"  [Environment]::SetEnvironmentVariable(\"Path\",\n"+
			"    \"$([Environment]::GetEnvironmentVariable('Path','User'));$gobin\", \"User\")\n\n"+
			"Then open a new terminal.\n", dir)
}

// broadcastEnvChange notifies other processes that the environment changed so freshly
// launched terminals see the new PATH without a sign-out. Best effort: failures are ignored.
func broadcastEnvChange() {
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")
	var result uintptr
	proc.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(env)),
		uintptr(smtoAbortIfHung),
		uintptr(5000),
		uintptr(unsafe.Pointer(&result)),
	)
}

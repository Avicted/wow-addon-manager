//go:build unix

package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// crossDevice reports whether a rename failed because src and dst are on different filesystems.
func crossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }

// WowRunning looks for a Wow.exe process (under Wine it shows up in the command line).
// Inside a container it cannot see the host's processes and reports false.
func WowRunning() bool {
	procs, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, p := range procs {
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(strings.ToLower(string(b)), "wow.exe") {
			return true
		}
	}
	return false
}

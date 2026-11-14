//go:build unix

package install

import (
	"os"
	"path/filepath"
	"strings"
)

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

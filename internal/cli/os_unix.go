//go:build unix

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

const (
	addonsDirHint = "~/Games/battlenet/drive_c/Program Files (x86)/World of Warcraft/_forever_/Interface/AddOns"
	dataDirHint   = "$XDG_DATA_HOME/wam or ~/.local/share/wam"
)

// defaultAddonsDir is where a Battle.net-under-Wine install keeps WoW Forever's addons.
func defaultAddonsDir(home string) string {
	return filepath.Join(home, "Games/battlenet/drive_c/Program Files (x86)/World of Warcraft/_forever_/Interface/AddOns")
}

func defaultDataDir(home string) string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "wam")
	}
	return filepath.Join(home, ".local", "share", "wam")
}

// inUse reports whether err means a file is open in another program. Unix lets wam replace
// folders whose files are open, so this never happens there.
func inUse(error) bool { return false }

func diskFull(err error) bool { return errors.Is(err, syscall.ENOSPC) }

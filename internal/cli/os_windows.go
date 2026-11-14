package cli

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

const (
	addonsDirHint = `C:\Program Files (x86)\World of Warcraft\_forever_\Interface\AddOns`
	dataDirHint   = `%LOCALAPPDATA%\wam`
)

// defaultAddonsDir is where the Battle.net installer puts WoW Forever's addons.
func defaultAddonsDir(string) string {
	base := os.Getenv("ProgramFiles(x86)")
	if base == "" {
		base = `C:\Program Files (x86)`
	}
	return filepath.Join(base, "World of Warcraft", "_forever_", "Interface", "AddOns")
}

func defaultDataDir(home string) string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "wam")
	}
	return filepath.Join(home, "AppData", "Local", "wam")
}

// Windows error codes the GUI explains in plain words.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
	errorHandleDiskFull   syscall.Errno = 39
	errorDiskFull         syscall.Errno = 112
)

// inUse reports whether err means a file is open in another program, usually the game.
// Windows refuses to move a folder holding an open file, often with "access denied": the
// folder was writable enough to stage the download, so a refused move means an open file.
func inUse(err error) bool {
	if errors.Is(err, errorSharingViolation) || errors.Is(err, errorLockViolation) {
		return true
	}
	var le *os.LinkError
	return errors.As(err, &le) && errors.Is(le.Err, errorAccessDenied)
}

func diskFull(err error) bool {
	return errors.Is(err, errorDiskFull) || errors.Is(err, errorHandleDiskFull)
}

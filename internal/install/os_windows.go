package install

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

// errorNotSameDevice is ERROR_NOT_SAME_DEVICE: a rename across drives (AddOns on D:,
// backups on C:).
const errorNotSameDevice syscall.Errno = 17

// crossDevice reports whether a rename failed because src and dst are on different drives.
// EXDEV is accepted too, so the tests' simulated cross-device error works here as well.
func crossDevice(err error) bool {
	return errors.Is(err, errorNotSameDevice) || errors.Is(err, syscall.EXDEV)
}

// WowRunning looks for a Wow.exe process.
func WowRunning() bool {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer func() { _ = syscall.CloseHandle(snap) }()
	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = syscall.Process32First(snap, &pe); err == nil; err = syscall.Process32Next(snap, &pe) {
		if strings.EqualFold(syscall.UTF16ToString(pe.ExeFile[:]), "wow.exe") {
			return true
		}
	}
	return false
}

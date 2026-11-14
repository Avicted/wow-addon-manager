package state

import (
	"errors"
	"os"
	"syscall"
)

// errorSharingViolation is ERROR_SHARING_VIOLATION: another process has the file open.
const errorSharingViolation syscall.Errno = 32

// lockFile opens path with no sharing allowed, so a second open fails until this handle
// is closed (or the process exits).
func lockFile(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if errors.Is(err, errorSharingViolation) {
		return nil, errLocked
	}
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

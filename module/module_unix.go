//go:build linux || darwin

package module

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
)

// MakeSelfOwnedFilesFunc calls the given function such that any files made will
// be self owned.
func MakeSelfOwnedFilesFunc(f func() error) error {
	oldMask := syscall.Umask(0o077)
	defer syscall.Umask(oldMask)
	return f()
}

// CheckSocketOwner verifies that UID of a filepath/socket matches the current process's UID.
func CheckSocketOwner(address string) error {
	// check that the module socket has the same ownership as our process
	info, err := os.Stat(address)
	if err != nil {
		return err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if os.Getuid() != int(stat.Uid) {
		return errors.New("socket ownership doesn't match current process UID")
	}
	return nil
}

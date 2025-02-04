//go:build linux || darwin

package module

import (
	"os"
	"os/user"
	"strconv"
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
	sockUID := int(info.Sys().(*syscall.Stat_t).Uid)
	if serverUID := os.Getuid(); serverUID != sockUID {
		sockUser, err := user.LookupId(strconv.Itoa(sockUID))
		if err != nil {
			return errors.Wrap(err, "error looking up user")
		}
		serverUser, err := user.LookupId(strconv.Itoa(serverUID))
		if err != nil {
			return errors.Wrap(err, "error looking up user")
		}
		return errors.Errorf("socket owned by %s while process is owned by %s", sockUser.Name, serverUser.Name)
	}
	return nil
}

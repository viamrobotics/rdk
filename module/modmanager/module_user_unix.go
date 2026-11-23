//go:build !windows

package modmanager

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
)

type moduleUser struct {
	name    string
	homeDir string
	uid     int
	gid     int
}

func lookupModuleUser(username string) (*moduleUser, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot run module as user %q", username)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, err
	}
	return &moduleUser{name: u.Username, homeDir: u.HomeDir, uid: uid, gid: gid}, nil
}

// chownTree gives the user ownership of everything under root, including files a first_run script created as
// viam-server's user.
func (u *moduleUser) chownTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == u.uid && int(st.Gid) == u.gid {
			return nil
		}
		return os.Lchown(path, u.uid, u.gid)
	})
}

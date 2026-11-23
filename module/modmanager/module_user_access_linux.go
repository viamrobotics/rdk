package modmanager

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const aclAccessXattr = "system.posix_acl_access"

// grantPerms adds an ACL entry giving the user perm on path, unless the user already has it through
// ownership or the "other" bits.
func (u *moduleUser) grantPerms(path string, perm uint16) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	mode := uint32(info.Mode().Perm())
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == u.uid && uint16(mode>>6)&perm == perm {
		return nil
	}
	if uint16(mode)&perm == perm {
		return nil
	}

	var acl posixACL
	buf := make([]byte, 1024)
	n, err := unix.Getxattr(path, aclAccessXattr, buf)
	switch {
	case errors.Is(err, unix.ENODATA):
		acl = aclFromMode(mode)
	case err != nil:
		return errors.Wrapf(err, "reading ACL of %q", path)
	default:
		if acl, err = decodePosixACL(buf[:n]); err != nil {
			return errors.Wrapf(err, "reading ACL of %q", path)
		}
	}

	updated, changed := acl.withUserPerms(uint32(u.uid), perm)
	if !changed {
		return nil
	}
	if err := unix.Setxattr(path, aclAccessXattr, updated.encode(), 0); err != nil {
		return errors.Wrapf(err, "granting user %q access to %q (does the filesystem support ACLs?)", u.name, path)
	}
	return nil
}

func (u *moduleUser) grantTraverseToAncestors(path string) error {
	for dir := filepath.Dir(filepath.Clean(path)); ; dir = filepath.Dir(dir) {
		if err := u.grantPerms(dir, aclPermExecute); err != nil {
			return err
		}
		if dir == filepath.Dir(dir) {
			return nil
		}
	}
}

func (u *moduleUser) grantReadTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return nil
		case d.IsDir():
			return u.grantPerms(path, aclPermRead|aclPermExecute)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		perm := uint16(aclPermRead)
		if info.Mode().Perm()&0o111 != 0 {
			perm |= aclPermExecute
		}
		return u.grantPerms(path, perm)
	})
}

// grantModuleAccess lets the user run a module: read access to its package directory when viam-server
// manages it, and traverse access to that directory's and the data directory's ancestors. Executables
// outside the packages directories belong to the person who configured them and are left untouched.
func (u *moduleUser) grantModuleAccess(exePath, moduleRoot string, packagesDirs []string, dataDir string) error {
	codeDir := filepath.Dir(exePath)
	if moduleRoot != "" && isWithinDir(exePath, moduleRoot) {
		codeDir = moduleRoot
	}
	for _, packagesDir := range packagesDirs {
		if packagesDir == "" || !isWithinDir(codeDir, packagesDir) || filepath.Clean(codeDir) == filepath.Clean(packagesDir) {
			continue
		}
		if err := u.grantReadTree(codeDir); err != nil {
			return err
		}
		if err := u.grantTraverseToAncestors(codeDir); err != nil {
			return err
		}
		break
	}

	if dataDir != "" {
		if err := u.chownTree(dataDir); err != nil {
			return errors.WithMessagef(err, "giving user %q ownership of module data directory", u.name)
		}
		return u.grantTraverseToAncestors(dataDir)
	}
	return nil
}

func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

//go:build windows

package module

import "os"

// MakeSelfOwnedFilesFunc calls the given function such that any files made will
// be self owned.
// TODO(RSDK-1775): verify security
func MakeSelfOwnedFilesFunc(f func() error) error {
	// new files should inherit the permissions of the directory they are created in
	return f()
}

// CheckSocketOwner on Windows only returns errors returned by os.Stat, if any. This can be used to check if the file exists.
// TODO(RSDK-1775): verify security
func CheckSocketOwner(address string) error {
	_, err := os.Stat(address)
	if err != nil {
		return err
	}
	return nil
}

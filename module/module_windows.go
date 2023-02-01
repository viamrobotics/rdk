//go:build windows

package module

// MakeSelfOwnedFilesFunc calls the given function such that any files made will
// be self owned.
// TODO(RSDK-1775): verify security
func MakeSelfOwnedFilesFunc(f func() error) error {
	// new files should inherit the permissions of the directory they are created in
	return f()
}

// CheckSocketOwner is ignored on Windows for now and assumes hierarchy permissions
// are set up correctly.
// TODO(RSDK-1775): verify security
func CheckSocketOwner(address string) error {
	return nil
}

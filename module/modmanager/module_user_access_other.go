//go:build !linux && !windows

package modmanager

func (u *moduleUser) grantModuleAccess(_, _ string, _ []string, dataDir string) error {
	if dataDir == "" {
		return nil
	}
	return u.chownTree(dataDir)
}

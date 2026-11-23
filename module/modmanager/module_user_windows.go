package modmanager

import "github.com/pkg/errors"

type moduleUser struct {
	name    string
	homeDir string
}

func lookupModuleUser(username string) (*moduleUser, error) {
	return nil, errors.Errorf("cannot run module as user %q: running modules as another user is not supported on Windows", username)
}

func (u *moduleUser) chownTree(string) error {
	return nil
}

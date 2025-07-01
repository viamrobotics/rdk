// Package shared is for any variables/functions shared amongst the datamanager builtin package and its subpackages.
package shared

import (
	"os"
	"path/filepath"

	"go.viam.com/rdk/utils"
)

var (
	// ViamCaptureDotDir is the default directory for capturing and syncing data.
	ViamCaptureDotDir = filepath.Join(utils.ViamDotDir, "capture")
	// OldViamCaptureDotDir is the old default directory for capturing and syncing data.
	// We will continue syncing data from this directory for backwards compatibility.
	OldViamCaptureDotDir = filepath.Join(os.Getenv("HOME"), ".viam", "capture")
	// DefaultCaptureDirChanged is true if the default capture directory has changed since fixing the capture directory location.
	DefaultCaptureDirChanged = ViamCaptureDotDir != OldViamCaptureDotDir
)

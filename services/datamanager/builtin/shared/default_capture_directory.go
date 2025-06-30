package shared

import (
	"os"
	"path/filepath"

	"go.viam.com/rdk/utils"
)

var ViamCaptureDotDir = filepath.Join(utils.ViamDotDir, "capture")
var OldViamCaptureDotDir = filepath.Join(os.Getenv("HOME"), ".viam", "capture")
var CaptureDirChanged = ViamCaptureDotDir != OldViamCaptureDotDir

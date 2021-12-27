package rimage

import (
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/rlog"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "rimage")
	if err != nil {
		panic(err)
	}
	rlog.Logger.Debugf("out dir: %q", outDir)
}

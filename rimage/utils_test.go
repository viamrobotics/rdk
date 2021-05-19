package rimage

import (
	"go.viam.com/core/rlog"
	"go.viam.com/core/testutils"
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

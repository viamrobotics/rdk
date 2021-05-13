package rimage

import (
	"io/ioutil"

	"go.viam.com/core/rlog"
)

var outDir string

func init() {
	var err error
	outDir, err = ioutil.TempDir("", "rimage")
	if err != nil {
		panic(err)
	}
	rlog.Logger.Debugf("out dir: %q", outDir)
}

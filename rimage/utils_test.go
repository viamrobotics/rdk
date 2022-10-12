package rimage

import (
	"github.com/edaniels/golog"
	"go.viam.com/utils/testutils"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "rimage")
	if err != nil {
		panic(err)
	}
	golog.Global().Debugf("out dir: %q", outDir)
}

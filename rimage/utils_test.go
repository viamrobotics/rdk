package rimage

import (
	"io/ioutil"

	"github.com/edaniels/golog"
)

var outDir string

func init() {
	var err error
	outDir, err = ioutil.TempDir("", "rimage")
	if err != nil {
		panic(err)
	}
	golog.Global.Debugf("out dir: %q", outDir)
}

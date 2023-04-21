package rimage

import (
	"go.viam.com/utils/testutils"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "rimage")
	if err != nil {
		panic(err)
	}
}

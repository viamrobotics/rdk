package main

import (
	"os"
	"testing"

	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/testutils"

	"github.com/stretchr/testify/assert"
)

func TestBothMain(t *testing.T) {
	assert.Error(t, realMain([]string{}))
	assert.Error(t, realMain([]string{"merge"}))
	assert.Error(t, realMain([]string{"merge", "x"}))
	assert.Error(t, realMain([]string{"to-las"}))
	assert.Error(t, realMain([]string{"to-las", "x"}))
	assert.Error(t, realMain([]string{"xxx"}))

	outDir := testutils.TempDir(t, "", "rimage_cmd_both")

	out := outDir + "/board1.both.gz"
	err := realMain([]string{"merge", artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), out, "-aligned"})
	if err != nil {
		t.Fatal(err)
	}

	out3 := outDir + "/shelf.both.gz"
	err = realMain([]string{"combineRGBAndZ16", artifact.MustPath("rimage/shelf_color.png"), artifact.MustPath("rimage/shelf_grayscale.png"), out3})
	if err != nil {
		t.Fatal(err)
	}

	out2 := outDir + "/shelf.las"
	jsonFilePath := "../../../robots/configs/intel515_parameters.json"
	err = realMain([]string{"to-las", artifact.MustPath("align/intel515/shelf.both.gz"), jsonFilePath, out2})
	if err != nil {
		t.Fatal(err)
	}

	s, err := os.Stat(out2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Less(t, int64(0), s.Size())
}

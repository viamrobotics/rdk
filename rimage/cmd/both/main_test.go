package main

import (
	"os"
	"testing"

	"go.viam.com/core/artifact"
	"go.viam.com/core/testutils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestBothMain(t *testing.T) {
	test.That(t, realMain([]string{}), test.ShouldBeError)
	test.That(t, realMain([]string{"merge"}), test.ShouldBeError)
	test.That(t, realMain([]string{"merge", "x"}), test.ShouldBeError)
	test.That(t, realMain([]string{"to-las"}), test.ShouldBeError)
	test.That(t, realMain([]string{"to-las", "x"}), test.ShouldBeError)
	test.That(t, realMain([]string{"xxx"}), test.ShouldBeError)

	outDir := testutils.TempDir(t, "", "rimage_cmd_both")
	golog.NewTestLogger(t).Debugf("out dir: %q", outDir)

	out := outDir + "/board1.both.gz"
	err := realMain([]string{"merge", artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), out, "-aligned"})
	test.That(t, err, test.ShouldBeNil)

	out3 := outDir + "/shelf.both.gz"
	err = realMain([]string{"combineRGBAndZ16", artifact.MustPath("rimage/shelf_color.png"), artifact.MustPath("rimage/shelf_grayscale.png"), out3})
	test.That(t, err, test.ShouldBeNil)

	out2 := outDir + "/shelf.las"
	jsonFilePath := "../../../robots/configs/intel515_parameters.json"
	err = realMain([]string{"to-las", artifact.MustPath("align/intel515/shelf.both.gz"), jsonFilePath, out2})
	test.That(t, err, test.ShouldBeNil)

	s, err := os.Stat(out2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.Size(), test.ShouldBeGreaterThan, int64(0))
}

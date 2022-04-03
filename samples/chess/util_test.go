package main

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/tonyOreglia/glee/pkg/position"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/vision/chess"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "samples_chess")
	if err != nil {
		panic(err)
	}
	rlog.Logger.Debugf("out dir: %q", outDir)
}

/* TODO(erh): put back
func TestInit(t *testing.T) {
	state := boardStateGuesser{}

	fns, err := filepath.Glob(artifact.MustPath("samples/chess/init/board-*.png"))
	test.That(t, err, test.ShouldBeNil)
	sort.Strings(fns)

	for idx, fn := range fns {
		rlog.Logger.Info(fn)
		depthDN := strings.Replace(fn, ".png", ".dat.gz", 1)

		board, err := chess.FindAndWarpBoardFromFiles(fn, depthDN, true)
		test.That(t, err, test.ShouldBeNil)

		_, err = state.newData(board)
		test.That(t, err, test.ShouldBeNil)

		pcs, err := state.game.GetSquaresWithPieces(board)
		if err != nil {
			err2 := board.WriteDebugImages(fmt.Sprintf("%s/init_foo", outDir))
			test.That(t, err2, test.ShouldBeNil)
		}
		t.Logf("\t%s\n", pcs)
		if len(pcs) != 32 {
			temp := board.Annotate()
			tempfn := fmt.Sprintf(outDir + "/init-%d.png", idx)

			utils.WriteImageToFile(tempfn, temp)
			t.Logf("\t annotated -> %s\n", tempfn)
		}

		if state.Ready() {
			squares, err := state.GetSquaresWithPieces()
			test.That(t, err, test.ShouldBeNil)

			test.That(t, squares, test.ShouldHaveLength, 32)

			for x := 'a'; x <= 'h'; x++ {
				for _, y := range []string{"1", "2", "7", "8"} {
					sq := string(x) + y
					test.That(t, squares[sq], test.ShouldBeTrue)
				}
			}
		}
	}

	p := position.StartingPosition()
	m, err := state.GetPrevMove(p)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m, test.ShouldBeNil)
}.
*/
func TestOneMove(t *testing.T) {
	t.Skip()
	logger := golog.NewTestLogger(t)
	state := boardStateGuesser{}

	e2e4Path := artifact.MustPath("samples/chess/e2e4")
	fns, err := filepath.Glob(e2e4Path + "/board-*.png")
	test.That(t, err, test.ShouldBeNil)
	sort.Strings(fns)

	for idx, fn := range fns {
		rlog.Logger.Info(fn)
		depthDN := strings.Replace(fn, ".png", ".dat.gz", 1)

		board, err := chess.FindAndWarpBoardFromFiles(fn, depthDN, true, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = state.newData(board)
		test.That(t, err, test.ShouldBeNil)

		board.WriteDebugImages(fmt.Sprintf("%s/%d", outDir, idx))
	}

	bb, err := state.GetBitBoard()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, bb.Value(), test.ShouldEqual, uint64(18441959067825012735))

	p := position.StartingPosition()
	m, err := state.GetPrevMove(p)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m, test.ShouldNotBeNil)
	test.That(t, m.String(), test.ShouldEqual, "e2e4")
}

func TestWristDepth1(t *testing.T) {
	t.Skip()
	dm, err := rimage.ParseDepthMap(artifact.MustPath("samples/chess/wristdepth1.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	ppr := dm.ToPrettyPicture(0, 1000)
	pp := rimage.ConvertImage(ppr)

	center := image.Point{dm.Width() / 2, dm.Height() / 2}
	pp.Circle(center, 30, rimage.Red)

	lowest, lowestValue, highest, highestValue := findDepthPeaks(dm, center, 30)
	t.Logf("lowest: %v highest: %v\n", lowest, highest)
	t.Logf("lowestValue: %v highestValue: %v\n", lowestValue, highestValue)

	pp.Circle(lowest, 5, rimage.Blue)
	pp.Circle(highest, 5, rimage.Red)

	err = pp.WriteTo("/tmp/x.png")
	test.That(t, err, test.ShouldBeNil)
}

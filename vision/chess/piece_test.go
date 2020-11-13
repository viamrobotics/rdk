package chess

import (
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

func TestPiece1(t *testing.T) {
	img := gocv.IMRead("data/board1.png", gocv.IMReadUnchanged)
	dm, err := vision.ParseDepthMap("data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	theBoard, err := FindAndWarpBoard(img, dm.ToMat())
	if err != nil {
		t.Fatal(err)
	}

	theBoard.Piece("E1")
	theBoard.Piece("E3")

}

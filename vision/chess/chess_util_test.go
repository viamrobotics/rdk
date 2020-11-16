package chess

import (
	"os"
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

func TestGetMinChessCorner(t *testing.T) {
	x := getMinChessCorner("a8")
	if x.X != 0 {
		t.Errorf("x is wrong for a8")
	}
	if x.Y != 0 {
		t.Errorf("y is wrong for a8")
	}

	x = getMinChessCorner("h1")
	if x.X != 700 {
		t.Errorf("x is wrong for h1")
	}
	if x.Y != 700 {
		t.Errorf("y is wrong for h1")
	}

}

func TestWarpColorAndDepthToChess1(t *testing.T) {
	img := gocv.IMRead("data/board1.png", gocv.IMReadUnchanged)
	dm, err := vision.ParseDepthMap("data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	debugOut := gocv.NewMat()
	defer debugOut.Close()
	corners, err := findChessCorners(img, &debugOut)
	if err != nil {
		t.Fatal(err)
	}

	gocv.IMWrite("out/board1_corners.png", debugOut)

	a, b, err := warpColorAndDepthToChess(img, dm.ToMat(), corners)
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)
	gocv.IMWrite("out/board1_warped.png", a)

	theBoard := Board{a, b}

	x := theBoard.PieceHeight("b1")
	if x < 40 || x > 58 {
		t.Errorf("height for b1 is wrong %f", x)
	}

	x = theBoard.PieceHeight("e1")
	if x < 70 || x > 100 {
		t.Errorf("height for e1 is wrong %f", x)
	}

	x = theBoard.PieceHeight("c1")
	if x < 50 || x > 71 {
		t.Errorf("height for c1 is wrong %f", x)
	}

	annotated := theBoard.Annotate()
	gocv.IMWrite("out/board1_annotated.png", annotated)
}

func TestWarpColorAndDepthToChess2(t *testing.T) {
	img := gocv.IMRead("data/board2.png", gocv.IMReadUnchanged)
	dm, err := vision.ParseDepthMap("data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	debugOut := gocv.NewMat()
	defer debugOut.Close()
	corners, err := findChessCorners(img, &debugOut)
	if err != nil {
		t.Fatal(err)
	}

	gocv.IMWrite("out/board2_corners.png", debugOut)

	a, b, err := warpColorAndDepthToChess(img, dm.ToMat(), corners)
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)
	gocv.IMWrite("out/board2_warped.png", a)

	theBoard := Board{a, b}

	x := theBoard.PieceHeight("b1")
	if x < 45 || x > 58 {
		t.Errorf("height for b1 is wrong %f", x)
	}

	x = theBoard.PieceHeight("e1")
	if x < 72 || x > 100 {
		t.Errorf("height for e1 is wrong %f", x)
	}

	x = theBoard.PieceHeight("c1")
	if x < 50 || x > 71 {
		t.Errorf("height for c1 is wrong %f", x)
	}

	annotated := theBoard.Annotate()
	gocv.IMWrite("out/board2_annotated.png", annotated)
}

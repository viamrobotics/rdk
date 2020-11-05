package vision

import (
	"os"
	"testing"

	"gocv.io/x/gocv"
)

func TestGetMinChessCorner(t *testing.T) {
	x := getMinChessCorner("A8")
	if x.X != 0 {
		t.Errorf("x is wrong for A8")
	}
	if x.Y != 0 {
		t.Errorf("y is wrong for A8")
	}

	x = getMinChessCorner("H1")
	if x.X != 700 {
		t.Errorf("x is wrong for H1")
	}
	if x.Y != 700 {
		t.Errorf("y is wrong for H1")
	}

}

func TestWarpColorAndDepthToChess1(t *testing.T) {
	img := gocv.IMRead("data/board1.png", gocv.IMReadUnchanged)
	dm, err := ParseDepthMap("data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	corners, err := FindChessCorners(img)
	if err != nil {
		t.Fatal(err)
	}

	a, b, err := WarpColorAndDepthToChess(img, dm.ToMat(), corners)
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)
	gocv.IMWrite("out/board1_warped.png", a)

	x := GetChessPieceHeight("B1", b)
	if int(x) != 19 {
		t.Errorf("height for B1 is wrong %f", x)
	}

	x = GetChessPieceHeight("E1", b)
	if x < 70 || x > 100 {
		t.Errorf("height for E1 is wrong %f", x)
	}

}

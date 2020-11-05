package vision

import (
	"os"
	"testing"

	"gocv.io/x/gocv"
)

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

	a, _, err := WarpColorAndDepthToChess(img, dm.ToMat(), corners)
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)
	gocv.IMWrite("out/board1_warped.png", a)
}

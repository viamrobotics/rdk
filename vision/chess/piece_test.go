package chess

import (
	"fmt"
	"image"
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

func TestPiece1(t *testing.T) {
	theBoard, err := FindAndWarpBoardFromFiles("data/board2.png", "data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	theClassifier, err := buildPieceModel(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(theBoard.Piece(theClassifier, "E1"))
	fmt.Println(theBoard.Piece(theClassifier, "E3"))
	fmt.Println(theBoard.Piece(theClassifier, "E7"))
}

func _TestPieceWalk(t *testing.T) {
	theBoard, err := FindAndWarpBoardFromFiles("data/board2.png", "data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	theClassifier, err := buildPieceModel(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	img := gocv.NewMat()
	theBoard.color.CopyTo(&img)

	corner := getMinChessCorner("E1")
	middle := image.Point{corner.X + 50, corner.Y + 50}

	markers := []image.Point{}

	n := 0
	err = vision.Walk(middle.X, middle.Y, 1000, func(x, y int) error {
		if x < 0 || y < 0 || x >= 800 || y >= 800 {
			return nil
		}
		if x%3 != 0 || y%3 != 0 {
			return nil
		}
		n = n + 1
		if n%1000 == 0 {
			fmt.Printf("%d %d\n", n, len(markers))
		}
		//data := img.GetVecbAt(y, x)
		data := _avgColor(img, x, y)
		t := PieceFromColor(theClassifier, data)
		if t == "white" {
			markers = append(markers, image.Point{x, y})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range markers {
		gocv.Circle(&img, p, 1, vision.Green.C, 1)
	}

	gocv.IMWrite("foo.png", img)
}

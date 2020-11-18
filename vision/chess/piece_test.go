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

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	if game.SquareColorStatus(theBoard, "e1") != "white" {
		t.Errorf("e1 wrong")
	}
	if game.SquareColorStatus(theBoard, "e3") != "empty" {
		t.Errorf("e3 wrong")
	}
	if game.SquareColorStatus(theBoard, "e7") != "black" {
		t.Errorf("e7 wrong")
	}

	nextBoard, err := FindAndWarpBoardFromFiles("data/board3.png", "data/board3.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	if game.SquareColorStatus(nextBoard, "e1") != "white" {
		t.Errorf("e1 wrong")
	}
	if game.SquareColorStatus(nextBoard, "e4") != "white" {
		t.Errorf("e1 wrong")
	}

	if game.SquareColorStatus(nextBoard, "e2") != "empty" {
		t.Errorf("e3 wrong")
	}
	if game.SquareColorStatus(nextBoard, "e3") != "empty" {
		t.Errorf("e3 wrong")
	}

	if game.SquareColorStatus(nextBoard, "e5") != "black" {
		t.Errorf("e7 wrong")
	}

	if game.SquareColorStatus(nextBoard, "e7") != "black" {
		t.Errorf("e7 wrong")
	}

}

func _TestPieceWalk(t *testing.T) {
	theBoard, err := FindAndWarpBoardFromFiles("data/board2.png", "data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	img := gocv.NewMat()
	theBoard.color.CopyTo(&img)

	corner := getMinChessCorner("e1")
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
		data := _avgColor(img, x, y)
		t := pieceFromColor(game.pieceColorClassifier, data)
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

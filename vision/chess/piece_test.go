package chess

import (
	"testing"

	"gocv.io/x/gocv"
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

	gocv.IMWrite("out/board2-edges.png", theBoard.edges)

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

	gocv.IMWrite("out/board3-edges.png", nextBoard.edges)
}

package chess

import (
	"testing"

	"gocv.io/x/gocv"
)

func _testPieceStatusHelper(t *testing.T, game *Game, board *Board, square, correct string) {
	res, err := game.SquareColorStatus(board, square)
	if err != nil {
		t.Fatal(err)
	}
	if res != correct {
		t.Errorf("square: %s got: %s, wanted: %s", square, res, correct)
	}
}

func TestPiece1(t *testing.T) {
	theBoard, err := FindAndWarpBoardFromFiles("data/board2.png", "data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	_testPieceStatusHelper(t, game, theBoard, "e1", "white")
	_testPieceStatusHelper(t, game, theBoard, "e3", "empty")
	_testPieceStatusHelper(t, game, theBoard, "e7", "black")

	gocv.IMWrite("out/board2-edges.png", *theBoard.edges)

	nextBoard, err := FindAndWarpBoardFromFiles("data/board3.png", "data/board3.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	_testPieceStatusHelper(t, game, nextBoard, "e1", "white")
	_testPieceStatusHelper(t, game, nextBoard, "e4", "white")

	_testPieceStatusHelper(t, game, nextBoard, "e2", "empty")
	_testPieceStatusHelper(t, game, nextBoard, "e3", "empty")

	_testPieceStatusHelper(t, game, nextBoard, "e5", "black")
	_testPieceStatusHelper(t, game, nextBoard, "e7", "black")

	gocv.IMWrite("out/board3-edges.png", *nextBoard.edges)
}

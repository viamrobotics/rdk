package chess

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/artifact"
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
	logger := golog.NewTestLogger(t)
	theBoard, err := FindAndWarpBoardFromFiles(artifact.MustPath("vision/chess/board2.png"), artifact.MustPath("vision/chess/board2.dat.gz"), true, logger)
	if err != nil {
		t.Fatal(err)
	}

	err = theBoard.WriteDebugImages("out/board2")
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

	nextBoard, err := FindAndWarpBoardFromFiles(artifact.MustPath("vision/chess/board3.png"), artifact.MustPath("vision/chess/board3.dat.gz"), true, logger)
	if err != nil {
		t.Fatal(err)
	}

	_testPieceStatusHelper(t, game, nextBoard, "e1", "white")
	_testPieceStatusHelper(t, game, nextBoard, "e4", "white")

	_testPieceStatusHelper(t, game, nextBoard, "e2", "empty")
	_testPieceStatusHelper(t, game, nextBoard, "e3", "empty")

	_testPieceStatusHelper(t, game, nextBoard, "e5", "black")
	_testPieceStatusHelper(t, game, nextBoard, "e7", "black")

	err = nextBoard.WriteDebugImages("out/board3")
	if err != nil {
		t.Fatal(err)
	}

}

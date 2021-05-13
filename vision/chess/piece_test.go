package chess

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/artifact"
)

func _testPieceStatusHelper(t *testing.T, game *Game, board *Board, square, correct string) {
	res, err := game.SquareColorStatus(board, square)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, correct)
}

func TestPiece1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	theBoard, err := FindAndWarpBoardFromFiles(artifact.MustPath("vision/chess/board2.png"), artifact.MustPath("vision/chess/board2.dat.gz"), true, logger)
	test.That(t, err, test.ShouldBeNil)

	err = theBoard.WriteDebugImages(outDir + "/board2")
	test.That(t, err, test.ShouldBeNil)

	game, err := NewGame(theBoard)
	test.That(t, err, test.ShouldBeNil)

	_testPieceStatusHelper(t, game, theBoard, "e1", "white")
	_testPieceStatusHelper(t, game, theBoard, "e3", "empty")
	_testPieceStatusHelper(t, game, theBoard, "e7", "black")

	nextBoard, err := FindAndWarpBoardFromFiles(artifact.MustPath("vision/chess/board3.png"), artifact.MustPath("vision/chess/board3.dat.gz"), true, logger)
	test.That(t, err, test.ShouldBeNil)

	_testPieceStatusHelper(t, game, nextBoard, "e1", "white")
	_testPieceStatusHelper(t, game, nextBoard, "e4", "white")

	_testPieceStatusHelper(t, game, nextBoard, "e2", "empty")
	_testPieceStatusHelper(t, game, nextBoard, "e3", "empty")

	_testPieceStatusHelper(t, game, nextBoard, "e5", "black")
	_testPieceStatusHelper(t, game, nextBoard, "e7", "black")

	err = nextBoard.WriteDebugImages(outDir + "/board3")
	test.That(t, err, test.ShouldBeNil)

}

package chess

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestGetMinChessCorner(t *testing.T) {
	x := getMinChessCorner("a8")
	test.That(t, x.X, test.ShouldEqual, 0)
	test.That(t, x.Y, test.ShouldEqual, 0)

	x = getMinChessCorner("h1")
	test.That(t, x.X, test.ShouldEqual, 700)
	test.That(t, x.Y, test.ShouldEqual, 700)
}

func _testBoardHeight(t *testing.T, game *Game, board *Board, square string, minHeight, maxHeight float64) {
	t.Helper()
	height, err := game.GetPieceHeight(board, square)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, height, test.ShouldBeBetween, minHeight, maxHeight)
}

func TestWarpColorAndDepthToChess1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	outDir := t.TempDir()

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board1", true, logger)
	test.That(t, err, test.ShouldBeNil)

	err = theBoard.WriteDebugImages(outDir + "/board1")
	test.That(t, err, test.ShouldBeNil)

	game, err := NewGame(theBoard)
	test.That(t, err, test.ShouldBeNil)

	_testBoardHeight(t, game, theBoard, "b1", 40, 58)  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100) // king
	_testBoardHeight(t, game, theBoard, "c1", 50, 71)  // bishop

	annotated := theBoard.Annotate()
	rimage.WriteImageToFile(outDir+"/board1_annotated.png", annotated)
}

func TestWarpColorAndDepthToChess2(t *testing.T) {
	logger := golog.NewTestLogger(t)
	outDir := t.TempDir()

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board2", true, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, theBoard.IsBoardBlocked(), test.ShouldBeFalse)

	game, err := NewGame(theBoard)
	test.That(t, err, test.ShouldBeNil)

	_testBoardHeight(t, game, theBoard, "b1", 40, 58)  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100) // king
	_testBoardHeight(t, game, theBoard, "c1", 50, 71)  // bishop

	annotated := theBoard.Annotate()
	rimage.WriteImageToFile(outDir+"/board2_annotated.png", annotated)

	nextBoard, err := FindAndWarpBoardFromFiles(
		artifact.MustPath("vision/chess/board3.png"), artifact.MustPath("vision/chess/board3.dat.gz"), true, logger)
	test.That(t, err, test.ShouldBeNil)

	rimage.WriteImageToFile(outDir+"/board3_annotated.png", nextBoard.Annotate())

	_testBoardHeight(t, game, nextBoard, "b1", -1, 1)   // empty
	_testBoardHeight(t, game, nextBoard, "e1", 70, 100) // king
	_testBoardHeight(t, game, nextBoard, "c1", -1, 1)   // bishop
}

func TestWarpColorAndDepthToChess3(t *testing.T) {
	logger := golog.NewTestLogger(t)
	outDir := t.TempDir()

	chessPath := artifact.MustPath("samples/chess/init")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board-1605543520", true, logger)
	test.That(t, err, test.ShouldBeNil)

	rimage.WriteImageToFile(outDir+"/board-1605543520.png", theBoard.Annotate())

	game, err := NewGame(theBoard)
	test.That(t, err, test.ShouldBeNil)

	_testBoardHeight(t, game, theBoard, "b1", 40, 58)  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100) // king
	_testBoardHeight(t, game, theBoard, "c1", 45, 74)  // bishop

	nextBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board-1605543783", true, logger)
	test.That(t, err, test.ShouldBeNil)

	_testBoardHeight(t, game, nextBoard, "b1", 40, 58)  // knight
	_testBoardHeight(t, game, nextBoard, "e1", 70, 100) // king
	_testBoardHeight(t, game, nextBoard, "e2", 20, 40)  // pawn
	_testBoardHeight(t, game, nextBoard, "c1", 45, 74)  // bishop

	rimage.WriteImageToFile(outDir+"/board-1605543783.png", nextBoard.Annotate())

	// crapPlayWithKmeans(nextBoard)
}

func TestArmBlock1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	board, err := FindAndWarpBoardFromFiles(
		artifact.MustPath("vision/chess/armblock1.png"), artifact.MustPath("vision/chess/armblock1.dat.gz"), true, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, board.IsBoardBlocked(), test.ShouldBeTrue)

	annotated := board.Annotate()
	rimage.WriteImageToFile(t.TempDir()+"/armblock1_annotated.png", annotated)
}

func TestWarpColorAndDepthToChess4(t *testing.T) {
	logger := golog.NewTestLogger(t)

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board-1610063549", true, logger)
	test.That(t, err, test.ShouldBeNil)

	rimage.WriteImageToFile(t.TempDir()+"/board-20210107-a.png", theBoard.Annotate())

	e := theBoard.SquareCenterEdges("a1")
	test.That(t, e, test.ShouldBeGreaterThanOrEqualTo, EdgeThreshold)

	d := theBoard.SquareCenterHeight("a1", DepthCheckSizeRadius)
	test.That(t, d, test.ShouldBeGreaterThanOrEqualTo, 20)

	d = theBoard.SquareCenterHeight("d7", DepthCheckSizeRadius)
	test.That(t, d, test.ShouldBeGreaterThanOrEqualTo, 10)
}

func TestWarpColorAndDepthToChess5(t *testing.T) {
	logger := golog.NewTestLogger(t)

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board5", true, logger)
	test.That(t, err, test.ShouldBeNil)

	err = theBoard.WriteDebugImages(t.TempDir() + "/board5")
	test.That(t, err, test.ShouldBeNil)
	/* TODO(erh): make this work
	test.That(t, board.IsBoardBlocked(), test.ShouldBeFalse)

	_, err := NewGame(theBoard)
	test.That(t, err, test.ShouldBeNil)
	*/
}

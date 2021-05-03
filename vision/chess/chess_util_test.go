package chess

import (
	"io/ioutil"
	"testing"

	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
)

var outDir string

func init() {
	var err error
	outDir, err = ioutil.TempDir("", "vision_chess")
	if err != nil {
		panic(err)
	}
	golog.Global.Debugf("out dir: %q", outDir)
}

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

func _testBoardHeight(t *testing.T, game *Game, board *Board, square string, minHeight, maxHeight float64, extra string) {

	height, err := game.GetPieceHeight(board, square)
	if err != nil {
		t.Errorf("%s | error on square: %s: %s", extra, square, err)
		return
	}

	if height < minHeight || height > maxHeight {
		t.Errorf("%s | wrong height for square %s, got: %f, wanted between %f %f", extra, square, height, minHeight, maxHeight)
	}

}

func TestWarpColorAndDepthToChess1(t *testing.T) {
	logger := golog.NewTestLogger(t)

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board1", true, logger)
	if err != nil {
		t.Fatal(err)
	}

	err = theBoard.WriteDebugImages(outDir + "/board1")
	if err != nil {
		t.Fatal(err)
	}

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	_testBoardHeight(t, game, theBoard, "b1", 40, 58, "board1")  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100, "board1") // king
	_testBoardHeight(t, game, theBoard, "c1", 50, 71, "board1")  // bishop

	annotated := theBoard.Annotate()
	rimage.WriteImageToFile(outDir+"/board1_annotated.png", annotated)
}

func TestWarpColorAndDepthToChess2(t *testing.T) {
	logger := golog.NewTestLogger(t)

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board2", true, logger)
	if err != nil {
		t.Fatal(err)
	}

	if theBoard.IsBoardBlocked() {
		t.Errorf("board2 blocked")
	}

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	_testBoardHeight(t, game, theBoard, "b1", 40, 58, "board2")  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100, "board2") // king
	_testBoardHeight(t, game, theBoard, "c1", 50, 71, "board2")  // bishop

	annotated := theBoard.Annotate()
	rimage.WriteImageToFile(outDir+"/board2_annotated.png", annotated)

	nextBoard, err := FindAndWarpBoardFromFiles(artifact.MustPath("vision/chess/board3.png"), artifact.MustPath("vision/chess/board3.dat.gz"), true, logger)
	if err != nil {
		t.Fatal(err)
	}

	rimage.WriteImageToFile(outDir+"/board3_annotated.png", nextBoard.Annotate())

	_testBoardHeight(t, game, nextBoard, "b1", -1, 1, "board3")   // empty
	_testBoardHeight(t, game, nextBoard, "e1", 70, 100, "board3") // king
	_testBoardHeight(t, game, nextBoard, "c1", -1, 1, "board3")   // bishop
}

func TestWarpColorAndDepthToChess3(t *testing.T) {
	logger := golog.NewTestLogger(t)

	chessPath := artifact.MustPath("samples/chess/init")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board-1605543520", true, logger)
	if err != nil {
		t.Fatal(err)
	}

	rimage.WriteImageToFile(outDir+"/board-1605543520.png", theBoard.Annotate())

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	_testBoardHeight(t, game, theBoard, "b1", 40, 58, "board-1605543520")  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100, "board-1605543520") // king
	_testBoardHeight(t, game, theBoard, "c1", 45, 74, "board-1605543520")  // bishop

	nextBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board-1605543783", true, logger)
	if err != nil {
		t.Fatal(err)
	}

	_testBoardHeight(t, game, nextBoard, "b1", 40, 58, "board-1605543783")  // knight
	_testBoardHeight(t, game, nextBoard, "e1", 70, 100, "board-1605543783") // king
	_testBoardHeight(t, game, nextBoard, "e2", 20, 40, "board-1605543783")  // pawn
	_testBoardHeight(t, game, nextBoard, "c1", 45, 74, "board-1605543783")  // bishop

	rimage.WriteImageToFile(outDir+"/board-1605543783.png", nextBoard.Annotate())

	//crapPlayWithKmeans(nextBoard)
}

func TestArmBlock1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	board, err := FindAndWarpBoardFromFiles(artifact.MustPath("vision/chess/armblock1.png"), artifact.MustPath("vision/chess/armblock1.dat.gz"), true, logger)
	if err != nil {
		t.Fatal(err)
	}

	if !board.IsBoardBlocked() {
		t.Errorf("armblock1 not blocked")
	}

	annotated := board.Annotate()
	rimage.WriteImageToFile(outDir+"/armblock1_annotated.png", annotated)

}

func TestWarpColorAndDepthToChess4(t *testing.T) {
	logger := golog.NewTestLogger(t)

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board-1610063549", true, logger)
	if err != nil {
		t.Fatal(err)
	}

	rimage.WriteImageToFile(outDir+"/board-20210107-a.png", theBoard.Annotate())

	e := theBoard.SquareCenterEdges("a1")
	if e < EdgeThreshold {
		t.Errorf("not enough edges for a1: %v", e)
	}

	d := theBoard.SquareCenterHeight("a1", DepthCheckSizeRadius)
	if d < 20 {
		t.Errorf("a1 rook is too short: %v", d)
	}

	d = theBoard.SquareCenterHeight("d7", DepthCheckSizeRadius)
	if d < 10 {
		t.Errorf("d7 pawn is too short: %v", d)
	}

}

func TestWarpColorAndDepthToChess5(t *testing.T) {
	logger := golog.NewTestLogger(t)

	chessPath := artifact.MustPath("vision/chess")
	theBoard, err := FindAndWarpBoardFromFilesRoot(chessPath+"/board5", true, logger)
	if err != nil {
		t.Fatal(err)
	}

	err = theBoard.WriteDebugImages(outDir + "/board5")
	if err != nil {
		t.Fatal(err)
	}
	/* TODO(erh): make this work
	if theBoard.IsBoardBlocked() {
		t.Errorf("blocked")
	}

	_, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}
	*/
}

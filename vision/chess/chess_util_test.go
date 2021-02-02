package chess

import (
	"os"
	"testing"

	"github.com/viamrobotics/robotcore/utils"
)

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
	os.MkdirAll("out", 0775)

	theBoard, err := FindAndWarpBoardFromFilesRoot("data/board1")
	if err != nil {
		t.Fatal(err)
	}

	err = utils.WriteImageToFile("out/board1-edges.png", theBoard.edges)
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
	utils.WriteImageToFile("out/board1_annotated.png", annotated)
}

func TestWarpColorAndDepthToChess2(t *testing.T) {
	os.MkdirAll("out", 0775)

	theBoard, err := FindAndWarpBoardFromFilesRoot("data/board2")
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
	utils.WriteImageToFile("out/board2_annotated.png", annotated)

	nextBoard, err := FindAndWarpBoardFromFiles("data/board3.png", "data/board3.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	utils.WriteImageToFile("out/board3_annotated.png", nextBoard.Annotate())

	_testBoardHeight(t, game, nextBoard, "b1", -1, 1, "board3")   // empty
	_testBoardHeight(t, game, nextBoard, "e1", 70, 100, "board3") // king
	_testBoardHeight(t, game, nextBoard, "c1", -1, 1, "board3")   // bishop
}

func TestWarpColorAndDepthToChess3(t *testing.T) {

	theBoard, err := FindAndWarpBoardFromFilesRoot("../../samples/chess/data/init/board-1605543520")
	if err != nil {
		t.Fatal(err)
	}

	utils.WriteImageToFile("out/board-1605543520.png", theBoard.Annotate())

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	_testBoardHeight(t, game, theBoard, "b1", 40, 58, "board-1605543520")  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100, "board-1605543520") // king
	_testBoardHeight(t, game, theBoard, "c1", 45, 74, "board-1605543520")  // bishop

	nextBoard, err := FindAndWarpBoardFromFilesRoot("../../samples/chess/data/init/board-1605543783")
	if err != nil {
		t.Fatal(err)
	}

	_testBoardHeight(t, game, nextBoard, "b1", 40, 58, "board-1605543783")  // knight
	_testBoardHeight(t, game, nextBoard, "e1", 70, 100, "board-1605543783") // king
	_testBoardHeight(t, game, nextBoard, "e2", 20, 40, "board-1605543783")  // pawn
	_testBoardHeight(t, game, nextBoard, "c1", 45, 74, "board-1605543783")  // bishop

	utils.WriteImageToFile("out/board-1605543783.png", nextBoard.Annotate())

	//crapPlayWithKmeans(nextBoard)
}

func TestArmBlock1(t *testing.T) {
	board, err := FindAndWarpBoardFromFiles("data/armblock1.png", "data/armblock1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	if !board.IsBoardBlocked() {
		t.Errorf("armblock1 not blocked")
	}

	annotated := board.Annotate()
	utils.WriteImageToFile("out/armblock1_annotated.png", annotated)

}

func TestWarpColorAndDepthToChess4(t *testing.T) {
	os.MkdirAll("out", 0775)

	theBoard, err := FindAndWarpBoardFromFilesRoot("data/board-1610063549")
	if err != nil {
		t.Fatal(err)
	}

	utils.WriteImageToFile("out/board-20210107-a.png", theBoard.Annotate())

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

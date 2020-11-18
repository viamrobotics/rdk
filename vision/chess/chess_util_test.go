package chess

import (
	"os"
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
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
	img := gocv.IMRead("data/board1.png", gocv.IMReadUnchanged)
	dm, err := vision.ParseDepthMap("data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	debugOut := gocv.NewMat()
	defer debugOut.Close()
	corners, err := findChessCorners(img, &debugOut)
	if err != nil {
		t.Fatal(err)
	}

	gocv.IMWrite("out/board1_corners.png", debugOut)

	a, b, err := warpColorAndDepthToChess(img, dm.ToMat(), corners)
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)
	gocv.IMWrite("out/board1_warped.png", a)

	theBoard := &Board{a, b}

	game, err := NewGame(theBoard)
	if err != nil {
		t.Fatal(err)
	}

	_testBoardHeight(t, game, theBoard, "b1", 40, 58, "board1")  // knight
	_testBoardHeight(t, game, theBoard, "e1", 70, 100, "board1") // king
	_testBoardHeight(t, game, theBoard, "c1", 50, 71, "board1")  // bishop

	annotated := theBoard.Annotate()
	gocv.IMWrite("out/board1_annotated.png", annotated)
}

func TestWarpColorAndDepthToChess2(t *testing.T) {
	img := gocv.IMRead("data/board2.png", gocv.IMReadUnchanged)
	dm, err := vision.ParseDepthMap("data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	debugOut := gocv.NewMat()
	defer debugOut.Close()
	corners, err := findChessCorners(img, &debugOut)
	if err != nil {
		t.Fatal(err)
	}

	gocv.IMWrite("out/board2_corners.png", debugOut)

	a, b, err := warpColorAndDepthToChess(img, dm.ToMat(), corners)
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("out", 0775)
	gocv.IMWrite("out/board2_warped.png", a)

	theBoard := &Board{a, b}

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
	gocv.IMWrite("out/board2_annotated.png", annotated)

	nextBoard, err := FindAndWarpBoardFromFiles("data/board3.png", "data/board3.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	gocv.IMWrite("out/board3_annotated.png", nextBoard.Annotate())

	_testBoardHeight(t, game, nextBoard, "b1", -1, 1, "board3")   // empty
	_testBoardHeight(t, game, nextBoard, "e1", 70, 100, "board3") // king
	_testBoardHeight(t, game, nextBoard, "c1", -1, 1, "board3")   // bishop
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
	gocv.IMWrite("out/armblock1_annotated.png", annotated)

}

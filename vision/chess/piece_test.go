package chess

import (
	"fmt"
	"testing"
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

/*
func TestPieceWalk(t *testing.T) {
	theBoard, err := FindAndWarpBoardFromFiles("data/board2.png", "data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	img := gocv.NewMat()
	theBoard.color.CopyTo(&img)

	corner := getMinChessCorner("E1")
	middle := image.Point{corner.X + 50, corner.Y + 50}


	for radius := 0; radius < 100; radius++ {
		foundAny := false

	}
}
*/

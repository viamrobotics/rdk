package chess

import (
	"fmt"

	"go.viam.com/robotcore/ml"
	"go.viam.com/robotcore/rimage"
)

const (
	SquareEmpty = 0
	SquareWhite = 1
	SquareBlack = 2
)

func makeArray(edges int, c rimage.Color) []float64 {
	return []float64{float64(edges), float64(c.R), float64(c.G), float64(c.B)}

}

func pieceFromColor(theClassifier ml.Classifier, edges int, data rimage.Color) string {
	res, err := theClassifier.Classify(makeArray(edges, data))
	if err != nil {
		panic(err)
	}

	switch res {
	case SquareWhite:
		return "white"
	case SquareBlack:
		return "black"
	case SquareEmpty:
		return "empty"
	default:
		panic(fmt.Errorf("unknown what # %d", res))
	}
}

func buildPieceColorModel(theBoard *Board) (ml.Classifier, error) {
	data := [][]float64{}
	correct := []int{}

	for y := '1'; y <= '8'; y++ {
		for x := 'a'; x <= 'h'; x++ {
			square := string(x) + string(y)

			middle := getChessMiddle(square)

			radius := 3

			squareType := SquareEmpty
			if square[1] == '1' || square[1] == '2' {
				squareType = SquareWhite
			}
			if square[1] == '7' || square[1] == '8' {
				squareType = SquareBlack
			}

			edges := theBoard.SquareCenterEdges(square)

			for x := middle.X - radius; x < middle.X+radius; x++ {
				for y := middle.Y - radius; y < middle.Y+radius; y++ {
					clr := theBoard.img.Color.GetXY(x, y)

					data = append(data, makeArray(edges, clr))
					correct = append(correct, squareType)
				}
			}
		}
	}

	theClassifier := &ml.GoLearnClassifier{}
	err := theClassifier.Train(data, correct)
	return theClassifier, err
}

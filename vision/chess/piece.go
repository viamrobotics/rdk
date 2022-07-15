package chess

import (
	"github.com/pkg/errors"

	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/rimage"
)

// The set of square colors.
const (
	SquareEmpty = 0
	SquareWhite = 1
	SquareBlack = 2
)

func makeArray(edges int, c rimage.Color) []float64 {
	x, y, z := c.RGB255()
	return []float64{float64(edges), float64(x), float64(y), float64(z)}
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
		panic(errors.Errorf("unknown what # %d", res))
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
					clr := theBoard.color.GetXY(x, y)

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

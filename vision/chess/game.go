package chess

import (
	"fmt"
	"image"
	"sort"

	"github.com/sjwhitworth/golearn/base"
)

type Game struct {
	pieceColorClassifier base.Classifier
}

func NewGame(initialBoard *Board) (*Game, error) {
	pieceColorClassifier, err := buildPieceColorModel(initialBoard)
	if err != nil {
		return nil, err
	}

	g := &Game{pieceColorClassifier}

	pieceHeights := []float64{}
	emptyHeights := []float64{}

	for y := '1'; y <= '8'; y++ {
		for x := 'a'; x <= 'h'; x++ {
			square := string(x) + string(y)
			height := initialBoard.SquareCenterHeight(square, DepthCheckSizeRadius)
			status := g.SquareColorStatus(initialBoard, square)

			if y == '1' || y == '2' || y == '7' || y == '8' {
				if status == "empty" {
					return nil, fmt.Errorf("initial square %s wrong, got: %s", square, status)
				}
				pieceHeights = append(pieceHeights, height)
			} else {
				if status != "empty" {
					return nil, fmt.Errorf("initial square %s wrong, got: %s", square, status)
				}
				emptyHeights = append(emptyHeights, height)
			}
		}
	}

	sort.Float64s(emptyHeights)
	sort.Float64s(pieceHeights)

	biggestEmpty := emptyHeights[len(emptyHeights)-1]
	lowestPiece := pieceHeights[0]

	if biggestEmpty >= lowestPiece {
		return nil, fmt.Errorf("heighest empty square bigger than lowest piece %f %f", biggestEmpty, lowestPiece)
	}

	if biggestEmpty >= MinPieceDepth {
		return nil, fmt.Errorf("biggestEmpty too big %f", biggestEmpty)
	}

	if lowestPiece <= MinPieceDepth {
		return nil, fmt.Errorf("lowestPiece too small %f", lowestPiece)
	}

	// TODO: should i store this info and use instead of MinPieceDepth

	return g, nil
}

func (g *Game) SquareColorStatus(board *Board, square string) string {
	corner := getMinChessCorner(square)
	middle := image.Point{corner.X + 50, corner.Y + 50}

	data := _avgColor(board.color, middle.X, middle.Y)
	return pieceFromColor(g.pieceColorClassifier, data)
}

func (g *Game) GetPieceHeight(board *Board, square string) (float64, error) {
	color := g.SquareColorStatus(board, square)
	centerHeight := board.SquareCenterHeight(square, DepthCheckSizeRadius)

	if color == "empty" {
		if centerHeight < MinPieceDepth {
			return 0, nil
		}
		return -1, fmt.Errorf("got no color but a center height of %f", centerHeight)
	}

	if centerHeight < MinPieceDepth {
		return -1, fmt.Errorf("got a color (%s) but a center height that is too small of %f", color, centerHeight)
	}

	return board.SquareCenterHeight(square, 30), nil
}

// Package chess implements computer vision algorithms useful in chess.
package chess

import (
	"image"
	"sort"

	"github.com/pkg/errors"

	"go.viam.com/rdk/ml"
)

// Game TODO.
type Game struct {
	pieceColorClassifier ml.Classifier
	edgesThreshold       int
}

const emptyStatus = "empty"

// NewGame TODO.
func NewGame(initialBoard *Board) (*Game, error) {
	pieceColorClassifier, err := buildPieceColorModel(initialBoard)
	if err != nil {
		return nil, err
	}

	g := &Game{pieceColorClassifier, -1}

	pieceHeights := []float64{}
	emptyHeights := []float64{}

	pieceEdges := []int{}
	emptyEdges := []int{}

	for y := '1'; y <= '8'; y++ {
		for x := 'a'; x <= 'h'; x++ {
			square := string(x) + string(y)
			height := initialBoard.SquareCenterHeight(square, DepthCheckSizeRadius)
			status, err := g.SquareColorStatus(initialBoard, square)
			if err != nil {
				return nil, err
			}
			edges := initialBoard.SquareCenterEdges(square)

			// fmt.Printf("%s -> %v %v %v\n", square, height, status, edges)

			if y == '1' || y == '2' || y == '7' || y == '8' {
				if status == emptyStatus {
					return nil, errors.Errorf("initial square %s wrong, got: %s", square, status)
				}
				pieceHeights = append(pieceHeights, height)
				pieceEdges = append(pieceEdges, edges)
			} else {
				if status != emptyStatus {
					return nil, errors.Errorf("initial square %s wrong, got: %s", square, status)
				}
				emptyHeights = append(emptyHeights, height)
				emptyEdges = append(emptyEdges, edges)
			}
		}
	}

	// heights ---------
	sort.Float64s(emptyHeights)
	sort.Float64s(pieceHeights)

	biggestEmpty := emptyHeights[len(emptyHeights)-1]
	lowestPiece := pieceHeights[0]

	if biggestEmpty >= lowestPiece {
		return nil, errors.Errorf("highest empty square bigger than lowest piece %f %f", biggestEmpty, lowestPiece)
	}

	if biggestEmpty >= MinPieceDepth {
		return nil, errors.Errorf("biggestEmpty too big %f", biggestEmpty)
	}

	if lowestPiece <= MinPieceDepth {
		return nil, errors.Errorf("lowestPiece too small %f", lowestPiece)
	}

	// TODO(erh): should i store this info and use instead of MinPieceDepth

	// edges -------------
	sort.Ints(pieceEdges)
	sort.Ints(emptyEdges)

	biggestEmptyEdges := emptyEdges[len(emptyEdges)-1]
	lowestPieceEdges := pieceEdges[0]

	// fmt.Printf("lowestPieceEdges: %d\n", lowestPieceEdges)
	// fmt.Printf("biggestEmptyEdges: %d\n", biggestEmptyEdges)

	if lowestPieceEdges-biggestEmptyEdges < 15 {
		return nil, errors.Errorf("not enough gap in edges %d %d", biggestEmptyEdges, lowestPieceEdges)
	}

	g.edgesThreshold = 5 + ((lowestPieceEdges + biggestEmptyEdges) / 2)
	if g.edgesThreshold > EdgeThreshold {
		g.edgesThreshold = EdgeThreshold
	}
	// fmt.Printf("g.edgesThreshold: %v lowestPieceEdges: %v biggestEmptyEdges: %v\n", g.edgesThreshold, lowestPieceEdges, biggestEmptyEdges)

	return g, nil
}

// SquareColorStatus TODO.
func (g *Game) SquareColorStatus(board *Board, square string) (string, error) {
	corner := getMinChessCorner(square)
	middle := image.Point{corner.X + 50, corner.Y + 50}
	data := board.color.AverageColor(middle, 1)

	edges := board.SquareCenterEdges(square)

	res := pieceFromColor(g.pieceColorClassifier, edges, data)

	if g.edgesThreshold >= 0 {
		if res == emptyStatus && edges > g.edgesThreshold {
			return "", errors.Errorf("got empty but had %d edges for square: %s threshold: %v", edges, square, g.edgesThreshold)
		}
	}

	// fmt.Printf("%s %v -> %s\n", square, data, res)
	// fmt.Printf("<div style='background-color:rgba(%d,%d,%d,1)'>%s %v -> %s</div>\n", data.R, data.G, data.B, square, data, res)

	return res, nil
}

// GetPieceHeight TODO.
func (g *Game) GetPieceHeight(board *Board, square string) (float64, error) {
	color, err := g.SquareColorStatus(board, square)
	if err != nil {
		return -1, err
	}

	centerHeight := board.SquareCenterHeight(square, DepthCheckSizeRadius)

	if color == emptyStatus {
		if centerHeight < MinPieceDepth {
			return 0, nil
		}
		return -1, errors.Errorf("got no color but a center height of %f for %s edges: %d", centerHeight, square, board.SquareCenterEdges(square))
	}

	if centerHeight < MinPieceDepth {
		// try again with a different idea
		centerHeight = board.SquareCenterHeight2(square, DepthCheckSizeRadius+5, true)
		if centerHeight < MinPieceDepth {
			return -1, errors.Errorf("got a color (%s) for square %s but a center height that is too small of %f edges: %d",
				color, square, centerHeight, board.SquareCenterEdges(square))
		}
	}

	return board.SquareCenterHeight(square, 30), nil
}

// GetSquaresWithPieces TODO.
func (g *Game) GetSquaresWithPieces(b *Board) ([]string, error) {
	squares := []string{}
	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)
			h, err := g.GetPieceHeight(b, s)
			if err != nil {
				return nil, err
			}
			if h > 0 {
				squares = append(squares, s)
			}
		}
	}
	return squares, nil
}

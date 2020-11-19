package chess

import (
	"fmt"
	"image"
	"sort"

	"github.com/sjwhitworth/golearn/base"
)

type Game struct {
	pieceColorClassifier base.Classifier
	edgesThreshold       int
}

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

			//fmt.Printf("%s -> %v %v %v\n", square, height, status, edges)

			if y == '1' || y == '2' || y == '7' || y == '8' {
				if status == "empty" {
					return nil, fmt.Errorf("initial square %s wrong, got: %s", square, status)
				}
				pieceHeights = append(pieceHeights, height)
				pieceEdges = append(pieceEdges, edges)
			} else {
				if status != "empty" {
					return nil, fmt.Errorf("initial square %s wrong, got: %s", square, status)
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
		return nil, fmt.Errorf("heighest empty square bigger than lowest piece %f %f", biggestEmpty, lowestPiece)
	}

	if biggestEmpty >= MinPieceDepth {
		return nil, fmt.Errorf("biggestEmpty too big %f", biggestEmpty)
	}

	if lowestPiece <= MinPieceDepth {
		return nil, fmt.Errorf("lowestPiece too small %f", lowestPiece)
	}

	// TODO: should i store this info and use instead of MinPieceDepth

	// edges -------------
	sort.Ints(pieceEdges)
	sort.Ints(emptyEdges)

	biggestEmptyEdges := emptyEdges[len(emptyEdges)-1]
	lowestPieceEdges := pieceEdges[0]

	//fmt.Printf("lowestPieceEdges: %d\n", lowestPieceEdges)
	//fmt.Printf("biggestEmptyEdges: %d\n", biggestEmptyEdges)

	if lowestPieceEdges-biggestEmptyEdges < 20 {
		return nil, fmt.Errorf("not enough gap in edges %d %d", biggestEmptyEdges, lowestPieceEdges)
	}

	g.edgesThreshold = (lowestPieceEdges + biggestEmptyEdges) / 2

	return g, nil
}

func (g *Game) SquareColorStatus(board *Board, square string) (string, error) {
	corner := getMinChessCorner(square)
	middle := image.Point{corner.X + 50, corner.Y + 50}
	data := _avgColor(board.color, middle.X, middle.Y)

	edges := board.SquareCenterEdges(square)

	res := pieceFromColor(g.pieceColorClassifier, edges, data)

	if g.edgesThreshold >= 0 {
		if res == "empty" && edges > g.edgesThreshold {
			return "", fmt.Errorf("got empty but had %d edges for square: %s", edges, square)
		}
	}

	//fmt.Printf("%s %v -> %s\n", square, data, res)
	//fmt.Printf("<div style='background-color:rgba(%d,%d,%d,1)'>%s %v -> %s</div>\n", data.R, data.G, data.B, square, data, res)

	return res, nil
}

func (g *Game) GetPieceHeight(board *Board, square string) (float64, error) {
	color, err := g.SquareColorStatus(board, square)
	if err != nil {
		return -1, err
	}
	centerHeight := board.SquareCenterHeight(square, DepthCheckSizeRadius)
	//fmt.Printf("%s %s %v\n", square, color, centerHeight)
	if color == "empty" {
		if centerHeight < MinPieceDepth {
			return 0, nil
		}
		return -1, fmt.Errorf("got no color but a center height of %f for %s edges: %d", centerHeight, square, board.SquareCenterEdges(square))
	}

	if centerHeight < MinPieceDepth {
		return -1, fmt.Errorf("got a color (%s) but a center height that is too small of %f edges: %d", color, centerHeight, board.SquareCenterEdges(square))
	}

	return board.SquareCenterHeight(square, 30), nil
}

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

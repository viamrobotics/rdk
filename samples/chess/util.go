package main

import (
	"image"

	"github.com/pkg/errors"
	"github.com/tonyOreglia/glee/pkg/bitboard"
	"github.com/tonyOreglia/glee/pkg/moves"
	"github.com/tonyOreglia/glee/pkg/position"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/chess"
)

// NumBoards is the number of boards in play.
var NumBoards = 2

type boardStateGuesser struct {
	game   *chess.Game
	boards []*chess.Board
}

func (state *boardStateGuesser) newData(newBoard *chess.Board) (bool, error) {
	var err error
	if state.game == nil {
		state.game, err = chess.NewGame(newBoard)
		if err != nil {
			return true, err
		}
	}
	state.boards = append(state.boards, newBoard)

	if len(state.boards) == 1 {
		return false, nil
	}

	if len(state.boards) > NumBoards {
		state.boards = state.boards[len(state.boards)-NumBoards:]
	}

	prev, err := state.game.GetSquaresWithPieces(state.boards[len(state.boards)-2])
	if err != nil {
		logger.Error(err)
		return true, nil
	}
	now, err := state.game.GetSquaresWithPieces(state.boards[len(state.boards)-1])
	if err != nil {
		logger.Error(err)
		return true, nil
	}

	return len(prev) != len(now), nil
}

func (state *boardStateGuesser) Ready() bool {
	return len(state.boards) >= NumBoards
}

func (state *boardStateGuesser) Clear() {
	state.boards = state.boards[:0]
}

func (state *boardStateGuesser) NewestBoard() *chess.Board {
	return state.boards[len(state.boards)-1]
}

func (state *boardStateGuesser) GetSquaresWithPieces() (map[string]bool, error) {
	counts := map[string]int{}

	for _, b := range state.boards {
		temp, err := state.game.GetSquaresWithPieces(b)
		if err != nil {
			return nil, err
		}
		for _, s := range temp {
			counts[s]++
		}
	}

	threshold := int(float64(len(state.boards)) * .7)
	squares := map[string]bool{}

	for square, count := range counts {
		if count >= threshold {
			squares[square] = true
		}
	}

	return squares, nil
}

func (state *boardStateGuesser) GetBitBoard() (*bitboard.Bitboard, error) {
	bb := bitboard.NewBitboard(0)

	m, err := state.GetSquaresWithPieces()
	if err != nil {
		return bb, err
	}
	for k := range m {
		idx, err := moves.ConvertAlgebriacToIndex(k)
		if err != nil {
			return bb, err
		}
		bb.SetBit(idx)
	}

	return bb, nil
}

var errNoMove = errors.New("no move")

func (state *boardStateGuesser) GetPrevMove(prev *position.Position) (*moves.Move, error) {
	prevSqs := prev.AllOccupiedSqsBb()
	nowSqs, err := state.GetBitBoard()
	if err != nil {
		return nil, err
	}
	if prevSqs.Value() == nowSqs.Value() {
		return nil, errNoMove
	}

	temp := bitboard.NewBitboard(prevSqs.Value() ^ nowSqs.Value())
	temp.Print()
	if temp.PopulationCount() != 2 {
		prevSqs.Print()
		nowSqs.Print()
		temp.Print()
		return nil, errors.Errorf("pop count sad %d", temp.PopulationCount())
	}

	fromBoard := bitboard.NewBitboard(prevSqs.Value() & temp.Value())
	toBoard := bitboard.NewBitboard(nowSqs.Value() & temp.Value())

	if fromBoard.PopulationCount() != 1 || toBoard.PopulationCount() != 1 {
		fromBoard.Print()
		toBoard.Print()
		panic("eliot is dumb")
	}

	from := fromBoard.Lsb()
	to := toBoard.Lsb()

	m := moves.NewMove([]int{from, to})
	return m, nil
}

func findDepthPeaks(dm *rimage.DepthMap, center image.Point, radius int) (image.Point, rimage.Depth, image.Point, rimage.Depth) {
	var lowest, highest image.Point
	lowestValue := rimage.MaxDepth
	highestValue := rimage.Depth(0)

	err := utils.Walk(center.X, center.Y, radius,
		func(x, y int) error {
			p := image.Point{x, y}

			depth := dm.Get(p)
			if depth == 0 {
				return nil
			}

			if depth < lowestValue {
				lowest = p
				lowestValue = depth
			}
			if depth > highestValue {
				highest = p
				highestValue = depth
			}
			return nil
		})
	if err != nil {
		panic(err)
	}

	return lowest, lowestValue, highest, highestValue
}

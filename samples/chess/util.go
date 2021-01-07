package main

import (
	"fmt"

	"github.com/echolabsinc/robotcore/vision/chess"

	"github.com/edaniels/golog"
	"github.com/tonyOreglia/glee/pkg/bitboard"
	"github.com/tonyOreglia/glee/pkg/moves"
	"github.com/tonyOreglia/glee/pkg/position"
)

var (
	NumBoards = 2
)

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
		toRemove := state.boards[0 : len(state.boards)-NumBoards]
		for _, b := range toRemove {
			b.Close()
		}
		state.boards = state.boards[len(state.boards)-NumBoards:]
	}

	prev, err := state.game.GetSquaresWithPieces(state.boards[len(state.boards)-2])
	if err != nil {
		golog.Global.Error(err)
		return true, nil
	}
	now, err := state.game.GetSquaresWithPieces(state.boards[len(state.boards)-1])
	if err != nil {
		golog.Global.Error(err)
		return true, nil
	}

	return len(prev) != len(now), nil
}

func (state *boardStateGuesser) Ready() bool {
	return len(state.boards) >= NumBoards
}

func (state *boardStateGuesser) Clear() {
	for _, board := range state.boards {
		board.Close()
	}
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
			counts[s] = counts[s] + 1
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

func (state *boardStateGuesser) GetPrevMove(prev *position.Position) (*moves.Move, error) {
	prevSqs := prev.AllOccupiedSqsBb()
	nowSqs, err := state.GetBitBoard()
	if err != nil {
		return nil, err
	}
	if prevSqs.Value() == nowSqs.Value() {
		return nil, nil
	}

	temp := bitboard.NewBitboard(prevSqs.Value() ^ nowSqs.Value())
	temp.Print()
	if temp.PopulationCount() != 2 {
		prevSqs.Print()
		nowSqs.Print()
		temp.Print()
		return nil, fmt.Errorf("pop count sad %d", temp.PopulationCount())
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

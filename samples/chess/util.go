package main

import (
	"fmt"

	"github.com/tonyOreglia/glee/pkg/bitboard"
	"github.com/tonyOreglia/glee/pkg/moves"
	"github.com/tonyOreglia/glee/pkg/position"

	"github.com/echolabsinc/robotcore/vision/chess"
)

type boardStateGuesser struct {
	boards []*chess.Board
}

func (state *boardStateGuesser) newData(newBoard *chess.Board) bool {
	state.boards = append(state.boards, newBoard)

	if len(state.boards) == 1 {
		return false
	}

	if len(state.boards) > 6 {
		state.boards = state.boards[len(state.boards)-6:]
	}

	prev := state.boards[len(state.boards)-2].GetSquaresWithPieces()
	now := state.boards[len(state.boards)-1].GetSquaresWithPieces()
	fmt.Println(now)

	return len(prev) != len(now)
}

func (state *boardStateGuesser) Ready() bool {
	return len(state.boards) >= 6
}

func (state *boardStateGuesser) Clear() {
	state.boards = []*chess.Board{}
}

func (state *boardStateGuesser) GetSquaresWithPieces() map[string]bool {
	counts := map[string]int{}

	for _, b := range state.boards {
		temp := b.GetSquaresWithPieces()
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

	return squares
}

func (state *boardStateGuesser) GetBitBoard() *bitboard.Bitboard {
	bb := bitboard.NewBitboard(0)

	m := state.GetSquaresWithPieces()
	for k, _ := range m {
		idx, err := moves.ConvertAlgebriacToIndex(k)
		if err != nil {
			panic(err)
		}
		bb.SetBit(idx)
	}

	return bb
}

func (state *boardStateGuesser) GetPrevMove(prev *position.Position) (*moves.Move, error) {
	prevSqs := prev.AllOccupiedSqsBb()
	nowSqs := state.GetBitBoard()
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
		panic("eliot is dumb")
	}

	from := fromBoard.Lsb()
	to := toBoard.Lsb()

	m := moves.NewMove([]int{from, to})
	return m, nil
}

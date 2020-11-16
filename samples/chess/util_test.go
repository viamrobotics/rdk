package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tonyOreglia/glee/pkg/position"

	"github.com/echolabsinc/robotcore/vision/chess"
)

func Test1(t *testing.T) {
	state := boardStateGuesser{}

	fns, err := filepath.Glob("data/init/board-*.png")
	if err != nil {
		t.Fatal(err)
	}

	for _, fn := range fns {
		depthDN := strings.Replace(fn, ".png", ".dat.gz", 1)

		board, err := chess.FindAndWarpBoardFromFiles(fn, depthDN)
		if err != nil {
			t.Fatal(err)
		}

		state.newData(board)

		if state.Ready() {
			squares := state.GetSquaresWithPieces()

			if len(squares) != 32 {
				t.Errorf("wrong number of squares %d", len(squares))
			}

			for x := 'a'; x <= 'h'; x++ {
				for _, y := range []string{"1", "2", "7", "8"} {
					sq := string(x) + y
					if !squares[sq] {
						t.Errorf("missing %s", sq)
					}
				}
			}
		}
	}

	p := position.StartingPosition()
	m := state.GetPrevMove(p)
	if m != nil {
		t.Errorf("why is there a move!!!")
	}
}

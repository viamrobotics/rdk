package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gocv.io/x/gocv"

	"github.com/tonyOreglia/glee/pkg/position"

	"github.com/echolabsinc/robotcore/vision/chess"
)

func TestInit(t *testing.T) {
	state := boardStateGuesser{}

	fns, err := filepath.Glob("data/init/board-*.png")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(fns)

	for idx, fn := range fns {
		fmt.Println(fn)
		depthDN := strings.Replace(fn, ".png", ".dat.gz", 1)

		board, err := chess.FindAndWarpBoardFromFiles(fn, depthDN)
		if err != nil {
			t.Fatal(err)
		}

		state.newData(board)
		pcs, err := state.game.GetSquaresWithPieces(board)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("\t%s\n", pcs)
		if len(pcs) != 32 {
			temp := board.Annotate()
			tempfn := fmt.Sprintf("out/init-%d.png", idx)
			gocv.IMWrite(tempfn, temp)
			fmt.Printf("\t annotated -> %s\n", tempfn)
		}

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
	m, err := state.GetPrevMove(p)
	if m != nil {
		t.Errorf("why is there a move!!!")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestOneMove(t *testing.T) {
	state := boardStateGuesser{}

	fns, err := filepath.Glob("data/e2e4/board-*.png")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(fns)

	for idx, fn := range fns {
		fmt.Println(fn)
		depthDN := strings.Replace(fn, ".png", ".dat.gz", 1)

		board, err := chess.FindAndWarpBoardFromFiles(fn, depthDN)
		if err != nil {
			t.Fatal(err)
		}

		state.newData(board)

		temp := board.Annotate()
		fmt.Println(fn)
		gocv.IMWrite(fmt.Sprintf("out/onemove-%d.png", idx), temp)
	}

	val := state.GetBitBoard().Value()
	if val != 18441959067825012735 {
		t.Errorf("TestOneMove initial value wrong %d", val)
	}

	p := position.StartingPosition()
	m, err := state.GetPrevMove(p)
	if m == nil {
		t.Errorf("why is there not a move!!!")
	}
	if err != nil {
		t.Fatal(err)
	}
	if m.String() != "e2e4" {
		t.Errorf("move is wrong: %s", m.String())
	}

}

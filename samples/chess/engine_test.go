package main

import (
	"fmt"
	"testing"

	"github.com/tonyOreglia/glee/pkg/position"
)

func TestEngine1(t *testing.T) {
	p, err := position.NewPositionFen("r1bqkbnr/pppppppp/2n5/4P3/8/8/PPPP1PPP/RNBQKBNR b KQkq - 2 1")
	if err != nil {
		t.Fatal(err)
	}

	_, m := searchForNextMove(p)

	if m.String() == "c7c6" {
		t.Errorf("this is not a legal move")
	}
}

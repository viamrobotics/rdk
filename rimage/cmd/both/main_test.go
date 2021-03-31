package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBothMain(t *testing.T) {
	assert.Error(t, realMain([]string{}))
	assert.Error(t, realMain([]string{"merge"}))
	assert.Error(t, realMain([]string{"merge", "x"}))
	assert.Error(t, realMain([]string{"to-las"}))
	assert.Error(t, realMain([]string{"to-las", "x"}))
	assert.Error(t, realMain([]string{"xxx"}))

	os.MkdirAll("out", 0775)

	out := "out/board1.both.gz"
	err := realMain([]string{"merge", "../../data/board1.png", "../../data/board1.dat.gz", out})
	if err != nil {
		t.Fatal(err)
	}

	out3 := "out/shelf.both.gz"
	err = realMain([]string{"combineRGBAndZ16", "../../data/shelf_color.png", "../../data/shelf_grayscale.png", out3})
	if err != nil {
		t.Fatal(err)
	}

	out2 := "out/board1.las"
	err = realMain([]string{"to-las", out, out2})
	if err != nil {
		t.Fatal(err)
	}

	s, err := os.Stat(out2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Less(t, int64(0), s.Size())
}

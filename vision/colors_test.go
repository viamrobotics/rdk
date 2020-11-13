package vision

import (
	"testing"

	"gocv.io/x/gocv"
)

func TestWhatColor1(t *testing.T) {
	data := gocv.Vecb{20, 20, 200}
	c, _ := WhatColor(data)
	if c != "red" {
		t.Errorf("got %s instead of red", c)
	}
}

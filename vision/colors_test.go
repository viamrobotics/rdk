package vision

import (
	"image/color"
	"testing"
)

func TestWhatColor1(t *testing.T) {
	data := color.RGBA{200, 20, 20, 0}
	c, _ := WhatColor(data)
	if c != "red" {
		t.Errorf("got %s instead of red", c)
	}
}

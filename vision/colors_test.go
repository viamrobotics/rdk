package vision

import (
	"image/color"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
)

func TestWhatColor1(t *testing.T) {
	data := color.RGBA{200, 20, 20, 0}
	c := WhatColor(data)
	if c.Name != "red" {
		t.Errorf("got %s instead of red", c)
	}
}

func _checkAllDifferent(t *testing.T, colors []Color) {
	for i, c1 := range colors {
		for j, c2 := range colors {
			d := c1.AsHSV.Distance(c2.AsHSV)
			if i == j {
				if d != 0 {
					t.Errorf("dumb")
				}
			} else {
				if d < 1 {
					t.Errorf("colors too close distance: %v\n%v\n%v", d, c1, c2)
				}
			}
		}
	}
}

func _checkAllSame(t *testing.T, colors []Color) {
	for _, c1 := range colors {
		for _, c2 := range colors {
			d := c1.AsHSV.Distance(c2.AsHSV)
			if d > 1.0 {
				t.Errorf("%v and %v are too far %v", c1, c2, d)
			}
		}
	}
}

func TestHSVColorConversion(t *testing.T) {
	c, err := colorful.Hex("#ff0000")
	if err != nil {
		t.Fatal(err)
	}
	if c.Hex() != "#ff0000" {
		t.Errorf(c.Hex())
	}
	r, g, b := c.RGB255()
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("%d %d %d", r, g, b)
	}

	H, S, V := c.Hsv()
	c2 := colorful.Hsv(H, S, V)
	if c.Hex() != c2.Hex() {
		t.Errorf(c2.Hex())
	}

	c3 := Red.AsHSV.ToColorful()
	if c.Hex() != c3.Hex() {
		t.Errorf("3 %#v %#v %s", c, c3, c3.Hex())
	}

	if Red.Hex() != "#ff0000" {
		t.Errorf("red hex wrong %s", Red.Hex())
	}

	c5, ok := colorful.MakeColor(Red)
	if !ok {
		t.Fatal(err)
	}
	if c5.Hex() != Red.Hex() {
		t.Errorf(c5.Hex())
	}
}

func TestHSVDistanceSanityCheck(t *testing.T) {
	d := White.AsHSV.Distance(Gray.AsHSV)
	if d < 1 {
		t.Fatalf("Wtf %v", d)
	}

	_checkAllDifferent(t, Colors)
	if Red.AsHSV.S != 1.0 {
		t.Errorf("%v\n", Red)
	}
	if Red.AsHSV.V != 1.0 {
		t.Errorf("%v\n", Red)
	}
	if Green.AsHSV.H != 120.0 {
		t.Errorf("%v\n", Green)
	}

}

func TestHSVDistanceBlacks1(t *testing.T) {
	data := []Color{
		NewColor(17, 23, 11, ""),
		NewColor(23, 13, 11, ""),
		NewColor(11, 23, 21, ""),
		NewColor(11, 17, 23, ""),
		NewColor(11, 11, 23, ""),
		NewColor(19, 11, 23, ""),
		NewColor(23, 11, 20, ""),
		NewColor(23, 11, 16, ""),
		NewColor(23, 11, 13, ""),
	}

	_checkAllSame(t, data)
}

func TestRationOffFrom135(t *testing.T) {
	data := [][]float64{
		[]float64{1.0, 1.0, 1.0}, // a 45 degree angle is "bad" so should be 1
		[]float64{-1.0, -1.0, 1.0},
		[]float64{-1.0, 1.0, 0.0},
		[]float64{1.0, -1.0, 0.0},
	}

	for _, x := range data {
		res := _rationOffFrom135(x[0], x[1])
		if res != x[2] {
			t.Errorf("got %v for %v", res, x)
		}
	}
}

func TestHSVDistanceChess1(t *testing.T) {

	x1 := NewColor(158, 141, 112, "squareWhite")
	x2 := NewColor(176, 154, 101, "pieceWhite")

	xd := x1.AsHSV.Distance(x2.AsHSV)
	if xd < 1 {
		t.Errorf("too close %v %v %v", x1, x2, xd)
	}

	// these are different white chess piece colors
	w1 := NewColor(132, 120, 75, "w1")
	w2 := NewColor(184, 159, 110, "w2")

	wd := w1.AsHSV.Distance(w2.AsHSV)
	if wd > 1 {
		t.Errorf("too far %v %v %v", w1, w2, wd)
	}

}

func TestHSVDistanceChess2(t *testing.T) {
	data := []Color{
		NewColor(5, 51, 85, "squareBlue"),
		NewColor(158, 141, 112, "squareWhite"),
		NewColor(176, 154, 101, "pieceWhite"),
		NewColor(19, 17, 9, "pieceBlack"),
	}
	_checkAllDifferent(t, data)

}

package vision

import (
	"image/color"
	"testing"
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
			if d > 1 {
				t.Errorf("%v and %v are too far %v", c1, c2, d)
			}
		}
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
	if Red.AsHSV.V != 255.0 {
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

func TestHSVDistanceChess1(t *testing.T) {

	x1 := NewColor(158, 141, 112, "squareWhite")
	x2 := NewColor(176, 154, 101, "pieceWhite")

	xd := x1.AsHSV.Distance(x2.AsHSV)
	if xd < 1 {
		t.Errorf("too close %v %v %v", x1, x2, xd)
	}

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

func TestHSVDistanceChess3(t *testing.T) {
	// these are parts of a white chess piece
	data := []Color{
		NewColor(130, 118, 73, ""),
		NewColor(132, 116, 82, ""),
		NewColor(132, 120, 75, ""),
		NewColor(133, 118, 77, ""),
		NewColor(133, 120, 80, ""),
		NewColor(133, 120, 82, ""),
		NewColor(134, 117, 76, ""),
		NewColor(135, 117, 83, ""),
		NewColor(135, 122, 80, ""),
		NewColor(135, 122, 82, ""),
		NewColor(135, 122, 84, ""),
		NewColor(136, 121, 84, ""),
		NewColor(136, 123, 81, ""),
		NewColor(137, 117, 77, ""),
		NewColor(137, 123, 77, ""),
		NewColor(137, 124, 82, ""),
		NewColor(137, 124, 84, ""),
		NewColor(137, 124, 86, ""),
		NewColor(138, 123, 86, ""),
		NewColor(138, 124, 80, ""),
		NewColor(139, 117, 78, ""),
		NewColor(139, 126, 84, ""),
		NewColor(140, 122, 88, ""),
		NewColor(140, 123, 86, ""),
		NewColor(140, 124, 78, ""),
		NewColor(140, 124, 80, ""),
		NewColor(140, 125, 74, ""),
		NewColor(140, 125, 86, ""),
		NewColor(140, 125, 88, ""),
		NewColor(140, 126, 80, ""),
		NewColor(141, 125, 81, ""),
		NewColor(141, 126, 87, ""),
		NewColor(141, 126, 89, ""),
		NewColor(141, 127, 81, ""),
		NewColor(142, 125, 88, ""),
		NewColor(142, 127, 74, ""),
		NewColor(143, 128, 77, ""),
		NewColor(143, 130, 88, ""),
		NewColor(145, 127, 93, ""),
		NewColor(145, 128, 91, ""),
		NewColor(145, 130, 91, ""),
		NewColor(145, 130, 93, ""),
		NewColor(146, 131, 80, ""),
		NewColor(146, 132, 88, ""),
		NewColor(147, 132, 81, ""),
		NewColor(147, 132, 91, ""),
		NewColor(148, 133, 80, ""),
		NewColor(148, 133, 82, ""),
		NewColor(148, 133, 92, ""),
		NewColor(148, 133, 94, ""),
		NewColor(149, 131, 81, ""),
		NewColor(149, 132, 91, ""),
		NewColor(149, 132, 95, ""),
		NewColor(150, 132, 82, ""),
		NewColor(150, 135, 98, ""),
		NewColor(151, 137, 89, ""),
		NewColor(152, 137, 84, ""),
		NewColor(152, 137, 96, ""),
		NewColor(152, 138, 90, ""),
		NewColor(153, 138, 97, ""),
		NewColor(154, 138, 94, ""),
		NewColor(154, 142, 97, ""),
		NewColor(155, 135, 95, ""),
		NewColor(155, 137, 87, ""),
		NewColor(156, 136, 96, ""),
		NewColor(156, 144, 97, ""),
		NewColor(158, 137, 95, ""),
		NewColor(159, 144, 93, ""),
		NewColor(160, 139, 93, ""),
		NewColor(161, 133, 87, ""),
		NewColor(161, 138, 90, ""),
		NewColor(161, 142, 95, ""),
		NewColor(162, 134, 88, ""),
		NewColor(162, 137, 90, ""),
		NewColor(162, 138, 93, ""),
		NewColor(162, 141, 95, ""),
		NewColor(162, 142, 91, ""),
		NewColor(163, 144, 90, ""),
		NewColor(164, 140, 95, ""),
		NewColor(164, 141, 93, ""),
		NewColor(164, 144, 93, ""),
		NewColor(165, 142, 94, ""),
		NewColor(165, 144, 98, ""),
		NewColor(165, 147, 97, ""),
		NewColor(166, 145, 99, ""),
		NewColor(166, 146, 97, ""),
		NewColor(166, 147, 93, ""),
		NewColor(168, 145, 95, ""),
		NewColor(169, 149, 98, ""),
		NewColor(170, 150, 99, ""),
		NewColor(172, 141, 89, ""),
		NewColor(172, 143, 88, ""),
		NewColor(172, 145, 94, ""),
		NewColor(173, 148, 101, ""),
		NewColor(174, 149, 102, ""),
		NewColor(175, 144, 92, ""),
		NewColor(175, 145, 88, ""),
		NewColor(175, 150, 103, ""),
		NewColor(175, 153, 98, ""),
		NewColor(176, 150, 106, ""),
		NewColor(177, 148, 97, ""),
		NewColor(177, 150, 99, ""),
		NewColor(177, 155, 100, ""),
		NewColor(178, 151, 98, ""),
		NewColor(178, 159, 105, ""),
		NewColor(179, 157, 100, ""),
		NewColor(180, 158, 101, ""),
		NewColor(182, 163, 109, ""),
		NewColor(183, 161, 106, ""),
		NewColor(184, 159, 110, ""),
		NewColor(185, 166, 112, ""),
		NewColor(186, 164, 109, ""),
	}

	_checkAllSame(t, data)

}

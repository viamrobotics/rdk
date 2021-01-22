package vision

import (
	"io/ioutil"
	"math"
	"strings"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
)

func TestWhatColor1(t *testing.T) {
	data := NewColor(200, 20, 20, "")
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

func _assertSame(t *testing.T, a, b HSV) {
	d := a.Distance(b)
	if d < 1 {
		return
	}
	a.distanceDebug(b, true)
	t.Errorf("%v and %v should be the same, but difference is %f", a, b, d)
}

func _assertNotSame(t *testing.T, a, b HSV) {
	d := a.Distance(b)
	if d > 1 {
		return
	}
	a.distanceDebug(b, true)
	t.Errorf("%v and %v should be different, but difference is %f", a, b, d)
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

	c6hex := "#123456"
	c6, err := NewColorFromHex(c6hex, "")
	if err != nil {
		t.Fatal(err)
	}
	if c6hex != c6.Hex() {
		t.Errorf("wtf %s %s", c6hex, c6.Hex())
	}
}

func TestHSVDistanceSanityCheckDiff(t *testing.T) {
	data := [][]float64{
		{0.0, 0.5, 0.5},
		{0.2, 0.5, 0.3},
		{0.5, 0.2, 0.3},
		{0.0, 0.9, 0.1},
		{0.9, 0.1, 0.2},
	}

	for _, x := range data {
		d := _loopedDiff(x[0], x[1])
		if math.Abs(d-x[2]) > .0001 {
			t.Errorf("input: %v output: %f", x, d)
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
	if Red.AsHSV.V != 1.0 {
		t.Errorf("%v\n", Red)
	}
	if Green.AsHSV.H != 120.0 {
		t.Errorf("%v\n", Green)
	}

}

func TestHSVDistanceSanityCheck2(t *testing.T) {
	// check rotating aroudn 360
	_assertSame(t, HSV{190, 1.0, 1.0}, HSV{195, 1.0, 1.0})
	_assertSame(t, HSV{355, 1.0, 1.0}, HSV{359, 1.0, 1.0})
	_assertSame(t, HSV{359, 1.0, 1.0}, HSV{1, 1.0, 1.0})

	// in the same hue, check value diff
	_assertSame(t, HSV{180, .5, 0}, HSV{180, .5, .05})
	_assertSame(t, HSV{180, .5, 0}, HSV{180, .5, .1})
	_assertNotSame(t, HSV{180, .5, 0}, HSV{180, .5, .15})

	_assertSame(t, HSV{180, .5, .09}, HSV{180, .5, .05})
	_assertSame(t, HSV{180, .5, .09}, HSV{180, .5, .10})
	_assertSame(t, HSV{180, .5, .09}, HSV{180, .5, .15})

	// in a dark value, hue shouldn't matter
	_assertSame(t, HSV{180, .5, .09}, HSV{0, .5, .09})

	// grays
	_assertSame(t, HSV{180, 0, .5}, HSV{180, .05, .5})
	_assertSame(t, HSV{180, 0, .5}, HSV{180, .1, .5})
	_assertNotSame(t, HSV{180, 0, .5}, HSV{180, .15, .5})

	_assertSame(t, HSV{180, .09, .5}, HSV{180, .05, .5})
	_assertSame(t, HSV{180, .09, .5}, HSV{180, .1, .5})
	_assertSame(t, HSV{180, .09, .5}, HSV{180, .15, .5})

	// in the lower left quadrant, how much hue difference is ok
	_assertSame(t, HSV{180, .4, .4}, HSV{175, .4, .4})
	_assertSame(t, HSV{180, .4, .4}, HSV{170, .4, .4})
	_assertNotSame(t, HSV{180, .4, .4}, HSV{150, .4, .4})

	// in the upper right quadrant, how much hue difference is ok
	_assertSame(t, HSV{180, .8, .8}, HSV{175, .8, .8})
	_assertSame(t, HSV{180, .8, .8}, HSV{173, .8, .8})
	_assertNotSame(t, HSV{180, .8, .8}, HSV{165, .8, .8})

	// a black vs dark blue case
	_assertNotSame(t, HSV{50, .6, .08}, HSV{210, 1.0, .18})
}

func TestHSVDistanceBlacks1(t *testing.T) {
	data := []Color{
		NewColorFromHexOrPanic("#020300", ""),
		NewColorFromHexOrPanic("#010101", ""),
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

	_assertSame(t, data[0].AsHSV, data[1].AsHSV)

	_checkAllSame(t, data)
}

func TestHSVDistanceDarks(t *testing.T) {
	veryDarkBlue := NewColorFromHexOrPanic("#0a1a1f", "")
	mostlyDarkBlue := NewColorFromHexOrPanic("#09202d", "")

	d := veryDarkBlue.AsHSV.Distance(mostlyDarkBlue.AsHSV)
	if d > 1 {
		t.Errorf("veryDarkBlue is not equal to mostlyDarkBlue %f", d)
	}

	mostlyDarkBlue2 := NewColorFromHexOrPanic("#093051", "")
	blackish := NewColorFromHexOrPanic("#201b0e", "")

	d = mostlyDarkBlue2.AsHSV.Distance(blackish.AsHSV)
	if d < 1 {
		t.Errorf("mostlyDarkBlue2 and blackish too close: %f", d)
	}

	veryDarkBlue = NewColorFromHexOrPanic("#11314c", "")

	d = mostlyDarkBlue2.AsHSV.Distance(veryDarkBlue.AsHSV)
	if d > 1 {
		t.Errorf("veryDarkBlue is not equal to mostlyDarkBlue %f", d)
	}

}

func TestRatioOffFrom135Finish(t *testing.T) {
	data := [][]float64{
		{.000, 0.50},
		{.125, 0.75},
		{.250, 1.00},
		{.375, 0.75},
		{.500, 0.50},
		{.625, 0.25},
		{.750, 0.00},
		{.875, 0.25},
		{1.00, 0.50},
	}

	for _, d := range data {
		res := _ratioOffFrom135Finish(d[0])
		if res != d[1] {
			t.Errorf("_ratioOffFrom135Finish(%f) should be %f got %f", d[0], d[1], res)
		}
	}

}

func TestRatioOffFrom135(t *testing.T) {
	data := [][]float64{
		{1.0, 1.0, 1.0}, // a 45 degree angle is "bad" so should be 1
		{-1.0, -1.0, 1.0},
		{-1.0, 1.0, 0.0},
		{1.0, -1.0, 0.0},

		{0.0, 1.0, 0.5},
		{0.0, -1.0, 0.5},
		{-1.0, 0.0, 0.5},
		{1.0, 0.0, 0.5},
	}

	for _, x := range data {
		res := _ratioOffFrom135(x[0], x[1])
		if res != x[2] {
			t.Errorf("got %v for %v", res, x)
		}
	}
}

func TestHSVDistanceChess1(t *testing.T) {

	x1 := NewColor(158, 141, 112, "squareWhite-1")
	x2 := NewColor(176, 154, 101, "pieceWhite-1")

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

	x1, err := NewColorFromHex("#8d836a", "squareWhite-2")
	if err != nil {
		t.Fatal(err)
	}
	x2, err = NewColorFromHex("#8e7e51", "pieceWhite-2")
	if err != nil {
		t.Fatal(err)
	}

	xd = x1.AsHSV.Distance(x2.AsHSV)
	if xd < 1 {
		t.Errorf("too close %v %v %v", x1, x2, xd)
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
	pieceColor, err := NewColorFromHex("#8e7e51", "a white piece")
	if err != nil {
		t.Fatal(err)
	}

	harbinger := NewColorFromHexOrPanic("#a49470", "")
	distance := pieceColor.AsHSV.Distance(harbinger.AsHSV)
	if distance < 1 {
		t.Fatalf("harbinger and other are too close %f\n", distance)
	}

	harbinger = NewColorFromHexOrPanic("#857657", "")
	distance = pieceColor.AsHSV.Distance(harbinger.AsHSV)
	if distance < 1 {
		t.Fatalf("harbinger2 and other are too close %f\n", distance)
	}

	raw, err := ioutil.ReadFile("data/hsvdistancechess3.txt")
	if err != nil {
		t.Fatal(err)
	}

	for _, squareColor := range strings.Split(string(raw), "\n") {
		squareColor = strings.TrimSpace(squareColor)
		if len(squareColor) == 0 {
			continue
		}
		myColor, err := NewColorFromHex(squareColor, "")
		if err != nil {
			t.Fatal(err)
		}
		distance := pieceColor.AsHSV.Distance(myColor.AsHSV)

		if distance < 1 {
			t.Errorf("%s %f\n", squareColor, distance)
		}
	}

}

func TestHSVDistanceChess4(t *testing.T) {
	pieceColor, err := NewColorFromHex("#052e50", "a blue square")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := ioutil.ReadFile("data/hsvdistancechess4.txt")
	if err != nil {
		t.Fatal(err)
	}

	for _, squareColor := range strings.Split(string(raw), "\n") {
		squareColor = strings.TrimSpace(squareColor)
		if len(squareColor) == 0 {
			continue
		}
		myColor, err := NewColorFromHex(squareColor, "")
		if err != nil {
			t.Fatal(err)
		}
		distance := pieceColor.AsHSV.Distance(myColor.AsHSV)

		if distance > 1 {
			t.Errorf("%s %f\n", squareColor, distance)
		}
	}

}

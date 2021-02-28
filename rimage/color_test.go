package rimage

import (
	"image/color"
	"io/ioutil"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucasb-eyer/go-colorful"
)

func _checkAllDifferent(t *testing.T, colors []Color) {
	for i, c1 := range colors {
		for j, c2 := range colors {
			d := c1.Distance(c2)
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
	_checkAllClose(t, colors, 1.0)
}

func _checkAllClose(t *testing.T, colors []Color, maxDistance float64) {
	numErrors := 0
	for _, c1 := range colors {
		for _, c2 := range colors {
			if !_assertClose(t, c1, c2, maxDistance) {
				numErrors++
				if numErrors > 20 {
					t.Fatalf("stopping after %d errors", numErrors)
				}
			}
		}
	}
}

func _testColorFailure(t *testing.T, a, b Color, threshold float64, comparison string) {
	d := a.distanceDebug(b, true)
	t.Errorf("%v(%s) %v(%s) difference should be %s %f, but is %f https://www.viam.com/color.html?#1=%s&2=%s",
		a, a.Hex(), b, b.Hex(), comparison, threshold, d, a.Hex(), b.Hex())
}

func _assertCloseHex(t *testing.T, a, b string, threshold float64) bool {
	aa := NewColorFromHexOrPanic(a)
	bb := NewColorFromHexOrPanic(b)

	return _assertClose(t, aa, bb, threshold)
}

func _assertClose(t *testing.T, a, b Color, threshold float64) bool {
	d := a.Distance(b)
	if d < threshold {
		return true
	}

	_testColorFailure(t, a, b, threshold, "<")
	return false
}

func _assertNotCloseHex(t *testing.T, a, b string, threshold float64) bool {
	aa := NewColorFromHexOrPanic(a)
	bb := NewColorFromHexOrPanic(b)

	d := aa.Distance(bb)
	if d > threshold {
		return true
	}

	_testColorFailure(t, aa, bb, threshold, ">")
	return false
}

func _assertSame(t *testing.T, a, b Color) {
	d := a.Distance(b)
	if d < 1 {
		return
	}
	_testColorFailure(t, a, b, 1, "<")
}

func _assertNotSame(t *testing.T, a, b Color) {
	d := a.Distance(b)
	if d > 1 {
		return
	}
	_testColorFailure(t, a, b, 1, ">")
}

func TestColorHSVColorConversion(t *testing.T) {
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

	if c.Hex() != Red.Hex() {
		t.Errorf("3 %#v %s", c, Red)
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
	c6, err := NewColorFromHex(c6hex)
	if err != nil {
		t.Fatal(err)
	}
	if c6hex != c6.Hex() {
		t.Errorf("wtf %s %s", c6hex, c6.Hex())
	}
}

func TestColorRoundTrip(t *testing.T) {
	c := NewColor(17, 83, 133)
	c2 := NewColorFromColor(c)
	assert.Equal(t, c.Hex(), c2.Hex())
	assert.Equal(t, "#115385", c2.Hex())

	c2 = NewColorFromColor(color.RGBA{17, 83, 133, 255})
	assert.Equal(t, c.Hex(), c2.Hex())

	c2 = NewColorFromColor(color.NRGBA{17, 83, 133, 255})
	assert.Equal(t, c.Hex(), c2.Hex())
}

func TestColorHSVDistanceSanityCheckDiff(t *testing.T) {
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

func TestColorHSVDistanceSanityCheck(t *testing.T) {
	d := White.Distance(Gray)
	if d < 1 {
		t.Fatalf("Wtf %v", d)
	}

	_checkAllDifferent(t, Colors)
	if Red.S != 1.0 {
		t.Errorf("%v\n", Red)
	}
	if Red.V != 1.0 {
		t.Errorf("%v\n", Red)
	}
	if Green.H != 120.0 {
		t.Errorf("%v\n", Green)
	}

}

func TestColorHSVDistanceSanityCheck2(t *testing.T) {
	// check rotating aroudn 360
	_assertSame(t, NewColorFromHSV(190, 1.0, 1.0), NewColorFromHSV(195, 1.0, 1.0))
	_assertSame(t, NewColorFromHSV(355, 1.0, 1.0), NewColorFromHSV(359, 1.0, 1.0))
	_assertSame(t, NewColorFromHSV(359, 1.0, 1.0), NewColorFromHSV(1, 1.0, 1.0))

	// in the same hue, check value diff
	_assertSame(t, NewColorFromHSV(180, .5, 0), NewColorFromHSV(180, .5, .05))
	_assertSame(t, NewColorFromHSV(180, .5, 0), NewColorFromHSV(180, .5, .1))
	_assertNotSame(t, NewColorFromHSV(180, .5, 0), NewColorFromHSV(180, .5, .15))

	_assertSame(t, NewColorFromHSV(180, .5, .09), NewColorFromHSV(180, .5, .05))
	_assertSame(t, NewColorFromHSV(180, .5, .09), NewColorFromHSV(180, .5, .10))
	_assertSame(t, NewColorFromHSV(180, .5, .09), NewColorFromHSV(180, .5, .15))

	// in a dark value, hue shouldn't matter
	_assertSame(t, NewColorFromHSV(180, .5, .09), NewColorFromHSV(0, .5, .09))

	// grays
	_assertSame(t, NewColorFromHSV(180, 0, .5), NewColorFromHSV(180, .05, .5))
	_assertSame(t, NewColorFromHSV(180, 0, .5), NewColorFromHSV(180, .1, .5))
	_assertNotSame(t, NewColorFromHSV(180, 0, .5), NewColorFromHSV(180, .15, .5))

	_assertSame(t, NewColorFromHSV(180, .09, .5), NewColorFromHSV(180, .05, .5))
	_assertSame(t, NewColorFromHSV(180, .09, .5), NewColorFromHSV(180, .1, .5))
	_assertSame(t, NewColorFromHSV(180, .09, .5), NewColorFromHSV(180, .15, .5))

	// in the lower left quadrant, how much hue difference is ok
	_assertSame(t, NewColorFromHSV(180, .4, .4), NewColorFromHSV(175, .4, .4))
	_assertSame(t, NewColorFromHSV(180, .4, .4), NewColorFromHSV(170, .4, .4))
	_assertNotSame(t, NewColorFromHSV(180, .4, .4), NewColorFromHSV(150, .4, .4))

	// in the upper right quadrant, how much hue difference is ok
	_assertSame(t, NewColorFromHSV(180, .8, .8), NewColorFromHSV(175, .8, .8))
	_assertSame(t, NewColorFromHSV(180, .8, .8), NewColorFromHSV(173, .8, .8))
	_assertNotSame(t, NewColorFromHSV(180, .8, .8), NewColorFromHSV(165, .8, .8))

	// a black vs dark blue case
	_assertNotSame(t, NewColorFromHSV(50, .6, .08), NewColorFromHSV(210, 1.0, .18))
}

func TestColorHSVDistanceBlacks1(t *testing.T) {
	data := []Color{
		NewColorFromHexOrPanic("#020300"),
		NewColorFromHexOrPanic("#010101"),
		NewColor(17, 23, 11),
		NewColor(23, 13, 11),
		NewColor(11, 23, 21),
		NewColor(11, 17, 23),
		NewColor(11, 11, 23),
		NewColor(19, 11, 23),
		NewColor(23, 11, 20),
		NewColor(23, 11, 16),
		NewColor(23, 11, 13),
	}

	_assertSame(t, data[0], data[1])

	_checkAllSame(t, data)
}

func TestColorHSVDistanceDarks(t *testing.T) {
	veryDarkBlue := NewColorFromHexOrPanic("#0a1a1f")
	mostlyDarkBlue := NewColorFromHexOrPanic("#09202d")

	d := veryDarkBlue.Distance(mostlyDarkBlue)
	if d > 1 {
		t.Errorf("veryDarkBlue is not equal to mostlyDarkBlue %f", d)
	}

	mostlyDarkBlue2 := NewColorFromHexOrPanic("#093051")
	blackish := NewColorFromHexOrPanic("#201b0e")

	d = mostlyDarkBlue2.Distance(blackish)
	if d < 1 {
		t.Errorf("mostlyDarkBlue2 and blackish too close: %f", d)
	}

	veryDarkBlue = NewColorFromHexOrPanic("#11314c")

	d = mostlyDarkBlue2.Distance(veryDarkBlue)
	if d > 1 {
		t.Errorf("veryDarkBlue is not equal to mostlyDarkBlue %f", d)
	}

}

func TestColorRatioOffFrom135Finish(t *testing.T) {
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

func TestColorRatioOffFrom135(t *testing.T) {
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

func TestColorHSVDistanceChess1(t *testing.T) {

	x1 := NewColor(158, 141, 112)
	x2 := NewColor(176, 154, 101)

	xd := x1.Distance(x2)
	if xd < 1 {
		t.Errorf("too close %v %v %v", x1, x2, xd)
	}

	w1 := NewColor(132, 120, 75)
	w2 := NewColor(184, 159, 110)
	_assertNotSame(t, w1, w2) // note: i changed this as i was trying to force something that shouldn't be true

	x1 = NewColorFromHexOrPanic("#8d836a")
	x2 = NewColorFromHexOrPanic("#8e7e51")
	_assertNotSame(t, x1, x2)
}

func TestColorHSVDistanceChess2(t *testing.T) {
	data := []Color{
		NewColor(5, 51, 85),
		NewColor(158, 141, 112),
		NewColor(176, 154, 101),
		NewColor(19, 17, 9),
	}
	_checkAllDifferent(t, data)

}

func TestColorHSVDistanceChess3(t *testing.T) {
	pieceColor, err := NewColorFromHex("#8e7e51")
	if err != nil {
		t.Fatal(err)
	}

	harbinger := NewColorFromHexOrPanic("#a49470")
	distance := pieceColor.Distance(harbinger)
	if distance < 1 {
		t.Fatalf("harbinger and other are too close %f\n", distance)
	}

	harbinger = NewColorFromHexOrPanic("#857657")
	distance = pieceColor.Distance(harbinger)
	if distance < 1 {
		t.Fatalf("harbinger2 and other are too close %f\n", distance)
	}

	allColors, err := readColorsFromFile("data/hsvdistancechess3.txt")
	if err != nil {
		t.Fatal(err)
	}

	for _, myColor := range allColors {
		distance := pieceColor.Distance(myColor)

		if distance < 1 {
			t.Errorf("%s %f\n", myColor.Hex(), distance)
		}
	}

	_checkAllClose(t, allColors, 2)

}

func TestColorHSVDistanceChess4(t *testing.T) {
	pieceColor, err := NewColorFromHex("#052e50")
	if err != nil {
		t.Fatal(err)
	}

	allColors, err := readColorsFromFile("data/hsvdistancechess4.txt")
	if err != nil {
		t.Fatal(err)
	}

	for _, myColor := range allColors {
		distance := pieceColor.Distance(myColor)

		if distance > 1 {
			t.Errorf("%s %f\n", myColor.Hex(), distance)
		}
	}

	_checkAllClose(t, allColors, 2)

}

func TestColorHSVDistanceChess5(t *testing.T) {
	allColors, err := readColorsFromFile("data/hsvdistancechess5.txt")
	if err != nil {
		t.Fatal(err)
	}

	_checkAllClose(t, allColors, 2)
}

func readColorsFromFile(fn string) ([]Color, error) {
	raw, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	all := []Color{}
	for _, squareColor := range strings.Split(string(raw), "\n") {
		squareColor = strings.TrimSpace(squareColor)
		if len(squareColor) == 0 {
			continue
		}
		myColor, err := NewColorFromHex(squareColor)
		if err != nil {
			return nil, err
		}
		all = append(all, myColor)
	}

	return all, nil
}

func TestColorHSVDistanceChessA(t *testing.T) {
	_assertNotCloseHex(t, "#8c9173", "#7b7e6c", 1.0)
	_assertNotCloseHex(t, "#909571", "#83876f", 1.0)
	_assertNotCloseHex(t, "#0d1e2a", "#0e273f", 1.0)
	_assertNotCloseHex(t, "#041726", "#031e39", 1.0)

}

func TestColorHSVDistanceChessB(t *testing.T) {
	a := NewColorFromHexOrPanic("#828263")
	b := NewColorFromHexOrPanic("#868363")
	_assertSame(t, a, b)
}

func TestColorHSVDistanceRandom1(t *testing.T) {
	_assertCloseHex(t, "#182b2b", "#0f2725", 1.2)
	_assertCloseHex(t, "#2f433c", "#283e3d", 1.1)
	_assertCloseHex(t, "#001b3d", "#002856", 1.1)
	_assertCloseHex(t, "#393330", "#291f1f", 1.0)

	_assertCloseHex(t, "#282737", "#261f2d", 1.2)
	_assertNotCloseHex(t, "#282737", "#261f2d", 0.9)

	_assertCloseHex(t, "#1b3351", "#1d233c", 1.2)

	_assertCloseHex(t, "#303330", "#202825", 1.1)
	_assertCloseHex(t, "#000204", "#162320", 1.1)
	_assertCloseHex(t, "#1d252f", "#192326", 1.1)

	_assertCloseHex(t, "#013b74", "#0e2f53", 1.0)
	_assertCloseHex(t, "#022956", "#0f284a", 1.0)
	_assertCloseHex(t, "#001d35", "#071723", 1.1)
	_assertCloseHex(t, "#747373", "#595863", 1.07)

	_assertNotCloseHex(t, "#515445", "#524e4d", 1.1)
	_assertNotCloseHex(t, "#9fa59c", "#adc3c5", 1.1)
	_assertNotCloseHex(t, "#adc3c5", "#9ab0a7", 1.02)

	_assertNotCloseHex(t, "#adc3c5", "#aaaca0", 1.2)
	_assertNotCloseHex(t, "#adc3c5", "#abafa2", 1.2)
	_assertNotCloseHex(t, "#031c31", "#002b64", 1.2)

	_assertCloseHex(t, "#9cb7ab", "#adc3c5", 1.11)  // shiny
	_assertCloseHex(t, "#899d96", "#adc3c5", 1.125) // shiny

}

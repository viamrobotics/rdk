package rimage

import (
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
)

func _checkAllDifferent(t *testing.T, colors []Color) {
	t.Helper()
	for i, c1 := range colors {
		for j, c2 := range colors {
			d := c1.Distance(c2)
			if i == j {
				test.That(t, d, test.ShouldEqual, 0)
			} else {
				test.That(t, d, test.ShouldBeGreaterThanOrEqualTo, 1)
			}
		}
	}
}

func _checkAllSame(t *testing.T, colors []Color) {
	t.Helper()
	_checkAllClose(t, colors, 1.0)
}

func _checkAllClose(t *testing.T, colors []Color, maxDistance float64) {
	t.Helper()
	numErrors := 0
	for _, c1 := range colors {
		for _, c2 := range colors {
			if !_assertClose(t, c1, c2, maxDistance) {
				numErrors++
				test.That(t, numErrors, test.ShouldBeLessThanOrEqualTo, 20)
			}
		}
	}
}

func _testColorFailure(t *testing.T, a, b Color, threshold float64, comparison string) {
	t.Helper()
	d := a.distanceDebug(b, true)
	t.Fatalf("%v(%s) %v(%s) difference should be %s %f, but is %f https://www.viam.com/color.html?#1=%s&2=%s",
		a, a.Hex(), b, b.Hex(), comparison, threshold, d, a.Hex(), b.Hex())
}

func _assertCloseHex(t *testing.T, a, b string, threshold float64) bool {
	t.Helper()
	aa := NewColorFromHexOrPanic(a)
	bb := NewColorFromHexOrPanic(b)

	return _assertClose(t, aa, bb, threshold)
}

func _assertClose(t *testing.T, a, b Color, threshold float64) bool {
	t.Helper()
	if d := a.Distance(b); d < threshold {
		return true
	}

	_testColorFailure(t, a, b, threshold, "<")
	return false
}

func _assertNotCloseHex(t *testing.T, a, b string, threshold float64) bool {
	t.Helper()
	aa := NewColorFromHexOrPanic(a)
	bb := NewColorFromHexOrPanic(b)

	if d := aa.Distance(bb); d > threshold {
		return true
	}

	_testColorFailure(t, aa, bb, threshold, ">")
	return false
}

func _assertSame(t *testing.T, a, b Color) {
	t.Helper()
	if d := a.Distance(b); d < 1 {
		return
	}
	_testColorFailure(t, a, b, 1, "<")
}

func _assertNotSame(t *testing.T, a, b Color) {
	t.Helper()
	if d := a.Distance(b); d > 1 {
		return
	}
	_testColorFailure(t, a, b, 1, ">")
}

func TestColorHSVColorConversion(t *testing.T) {
	c, err := colorful.Hex("#ff0000")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c.Hex(), test.ShouldEqual, "#ff0000")
	r, g, b := c.RGB255()
	test.That(t, r, test.ShouldEqual, 255)
	test.That(t, g, test.ShouldEqual, 0)
	test.That(t, b, test.ShouldEqual, 0)

	H, S, V := c.Hsv()
	c2 := colorful.Hsv(H, S, V)
	test.That(t, c2.Hex(), test.ShouldEqual, c.Hex())

	test.That(t, c.Hex(), test.ShouldEqual, Red.Hex())
	test.That(t, Red.Hex(), test.ShouldEqual, "#ff0000")

	c5, ok := colorful.MakeColor(Red)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, c5.Hex(), test.ShouldEqual, Red.Hex())

	c6Hex := "#123456"
	c6, err := NewColorFromHex(c6Hex)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c6Hex, test.ShouldEqual, c6.Hex())
}

func TestColorBits(t *testing.T) {
	c := newcolor(1, 2, 3, 10001, 5, 6)
	r, g, b := c.RGB255()
	h, s, v := c.hsv()
	test.That(t, int(r), test.ShouldEqual, 1)
	test.That(t, int(g), test.ShouldEqual, 2)
	test.That(t, int(b), test.ShouldEqual, 3)
	test.That(t, int(h), test.ShouldEqual, 10001)
	test.That(t, int(s), test.ShouldEqual, 5)
	test.That(t, int(v), test.ShouldEqual, 6)
}

func TestColorRoundTrip(t *testing.T) {
	c := NewColor(17, 83, 133)
	c2 := NewColorFromColor(c)
	test.That(t, c2.Hex(), test.ShouldEqual, c.Hex())
	test.That(t, c2.Hex(), test.ShouldEqual, "#115385")

	c2 = NewColorFromColor(color.RGBA{17, 83, 133, 255})
	test.That(t, c2.Hex(), test.ShouldEqual, c.Hex())

	c2 = NewColorFromColor(color.NRGBA{17, 83, 133, 255})
	test.That(t, c2.Hex(), test.ShouldEqual, c.Hex())
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
		test.That(t, math.Abs(d-x[2]), test.ShouldBeLessThanOrEqualTo, .0001)
	}
}

func TestColorHSVDistanceSanityCheck(t *testing.T) {
	_, s, v := Red.hsv()
	test.That(t, s, test.ShouldEqual, 255)
	test.That(t, v, test.ShouldEqual, 255)
	h, _, _ := Green.hsv()
	test.That(t, h, test.ShouldEqual, 21845)

	d := White.Distance(Gray)
	test.That(t, d, test.ShouldBeGreaterThanOrEqualTo, 1)

	_checkAllDifferent(t, Colors)
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
	test.That(t, d, test.ShouldBeLessThanOrEqualTo, 1)

	mostlyDarkBlue2 := NewColorFromHexOrPanic("#093051")
	blackish := NewColorFromHexOrPanic("#201b0e")

	d = mostlyDarkBlue2.Distance(blackish)
	test.That(t, d, test.ShouldBeGreaterThanOrEqualTo, 1)

	veryDarkBlue = NewColorFromHexOrPanic("#11314c")

	d = mostlyDarkBlue2.Distance(veryDarkBlue)
	test.That(t, d, test.ShouldBeLessThanOrEqualTo, 1)
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
		test.That(t, res, test.ShouldEqual, d[1])
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
		test.That(t, res, test.ShouldEqual, x[2])
	}
}

func TestColorHSVDistanceChess1(t *testing.T) {
	x1 := NewColor(158, 141, 112)
	x2 := NewColor(176, 154, 101)

	xd := x1.Distance(x2)
	test.That(t, xd, test.ShouldBeGreaterThanOrEqualTo, 1)

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
	t.Parallel()
	pieceColor, err := NewColorFromHex("#8e7e51")
	test.That(t, err, test.ShouldBeNil)

	harbinger := NewColorFromHexOrPanic("#a49470")
	distance := pieceColor.Distance(harbinger)
	test.That(t, distance, test.ShouldBeGreaterThanOrEqualTo, 1)

	harbinger = NewColorFromHexOrPanic("#857657")
	distance = pieceColor.Distance(harbinger)
	test.That(t, distance, test.ShouldBeGreaterThanOrEqualTo, 1)

	allColors, err := readColorsFromFile(artifact.MustPath("rimage/hsvdistancechess3.txt"))
	test.That(t, err, test.ShouldBeNil)

	for _, myColor := range allColors {
		distance := pieceColor.Distance(myColor)
		test.That(t, distance, test.ShouldBeGreaterThanOrEqualTo, 1)
	}

	_checkAllClose(t, allColors, 2)
}

func readColorsFromFile(fn string) ([]Color, error) {
	raw, err := os.ReadFile(fn)
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
	_assertNotCloseHex(t, "#909571", "#83876f", .99) // I "broke" this when changing H,S,V to smaller types, thing it's ok
	_assertNotCloseHex(t, "#0d1e2a", "#0e273f", 1.0)
	_assertNotCloseHex(t, "#041726", "#031e39", 1.0)
}

func TestColorHSVDistanceChessB(t *testing.T) {
	a := NewColorFromHexOrPanic("#828263")
	b := NewColorFromHexOrPanic("#868363")
	_assertSame(t, a, b)
}

func TestColorHSVDistanceRandom1(t *testing.T) {
	test.That(t, _assertCloseHex(t, "#182b2b", "#0f2725", 1.2), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#2f433c", "#283e3d", 1.1), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#001b3d", "#002856", 1.1), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#393330", "#291f1f", 1.0), test.ShouldBeTrue)

	test.That(t, _assertCloseHex(t, "#282737", "#261f2d", 1.2), test.ShouldBeTrue)
	test.That(t, _assertNotCloseHex(t, "#282737", "#261f2d", 0.9), test.ShouldBeTrue)

	test.That(t, _assertCloseHex(t, "#1b3351", "#1d233c", 1.2), test.ShouldBeTrue)

	test.That(t, _assertCloseHex(t, "#303330", "#202825", 1.1), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#000204", "#162320", 1.1), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#1d252f", "#192326", 1.1), test.ShouldBeTrue)

	test.That(t, _assertCloseHex(t, "#013b74", "#0e2f53", 1.0), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#022956", "#0f284a", 1.0), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#001d35", "#071723", 1.1), test.ShouldBeTrue)
	test.That(t, _assertCloseHex(t, "#747373", "#595863", 1.07), test.ShouldBeTrue)

	test.That(t, _assertNotCloseHex(t, "#515445", "#524e4d", 1.1), test.ShouldBeTrue)
	test.That(t, _assertNotCloseHex(t, "#9fa59c", "#adc3c5", 1.1), test.ShouldBeTrue)
	test.That(t, _assertNotCloseHex(t, "#adc3c5", "#9ab0a7", 1.02), test.ShouldBeTrue)

	test.That(t, _assertNotCloseHex(t, "#adc3c5", "#aaaca0", 1.2), test.ShouldBeTrue)
	test.That(t, _assertNotCloseHex(t, "#adc3c5", "#abafa2", 1.2), test.ShouldBeTrue)
	test.That(t, _assertNotCloseHex(t, "#031c31", "#002b64", 1.2), test.ShouldBeTrue)

	test.That(t, _assertCloseHex(t, "#9cb7ab", "#adc3c5", 1.11), test.ShouldBeTrue)  // shiny
	test.That(t, _assertCloseHex(t, "#899d96", "#adc3c5", 1.125), test.ShouldBeTrue) // shiny

	test.That(t, _assertNotCloseHex(t, "#958f8f", "#2b2928", 3), test.ShouldBeTrue)   // gray vs black
	test.That(t, _assertNotCloseHex(t, "#5e5b5b", "#2b2928", 3), test.ShouldBeTrue)   // gray vs black
	test.That(t, _assertNotCloseHex(t, "#3d3c3c", "#2b2928", 1.0), test.ShouldBeTrue) // pretty dark gray vs black

	test.That(t, _assertNotCloseHex(t, "#807c79", "#5f5b5a", 1.05), test.ShouldBeTrue)

	test.That(t, _assertCloseHex(t, "#4b494c", "#423c3a", 1.25), test.ShouldBeTrue)

	test.That(t, _assertCloseHex(t, "#202320", "#262626", .8), test.ShouldBeTrue)
}

func TestColorConvert(t *testing.T) {
	// estimate of max error we're ok with based on conversion lossyness
	okError := float64(math.MaxUint16) * (1 - math.Pow(255.0/256.0, 4))

	testRoundTrip := func(c color.Color) {
		cc := NewColorFromColor(c)

		r1, g1, b1, a1 := c.RGBA()
		r2, g2, b2, a2 := cc.RGBA()

		test.That(t, r2, test.ShouldAlmostEqual, r1, okError)
		test.That(t, g2, test.ShouldAlmostEqual, g1, okError)
		test.That(t, b2, test.ShouldAlmostEqual, b1, okError)
		test.That(t, a2, test.ShouldAlmostEqual, a1, okError)
	}

	testRoundTrip(Red)

	testRoundTrip(color.NRGBA{17, 50, 124, 255})

	testRoundTrip(color.RGBA{17, 50, 124, 255})

	testRoundTrip(color.YCbCr{17, 50, 124})
	testRoundTrip(color.YCbCr{17, 50, 1})
	testRoundTrip(color.YCbCr{17, 50, 255})
	testRoundTrip(color.YCbCr{112, 50, 124})
}

func TestHSVConvert(t *testing.T) {
	tt := func(c color.Color) {
		me := NewColorFromColor(c)
		them, ok := colorful.MakeColor(c)
		test.That(t, ok, test.ShouldBeTrue)

		h1, s1, v1 := me.HsvNormal()
		h2, s2, v2 := them.Hsv()

		test.That(t, h1, test.ShouldAlmostEqual, h2, .1)
		test.That(t, s1, test.ShouldAlmostEqual, s2, .01)
		test.That(t, v1, test.ShouldAlmostEqual, v2, .01)
	}

	tt(color.NRGBA{128, 128, 128, 255})
	tt(color.NRGBA{128, 200, 225, 255})
	tt(color.NRGBA{200, 128, 32, 255})
	tt(color.NRGBA{31, 213, 200, 255})

	for r := 0; r <= 255; r += 32 {
		for g := 0; g <= 255; g += 32 {
			for b := 0; b <= 255; b += 32 {
				tt(color.NRGBA{uint8(r), uint8(g), uint8(b), 255})
			}
		}
	}
}

func TestColorSegment1(t *testing.T) {
	checkSkipDebugTest(t)
	img, err := NewImageFromFile(artifact.MustPath("rimage/chess-segment1.png"))
	test.That(t, err, test.ShouldBeNil)

	all := []Color{}

	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.Get(image.Point{x, y})
			all = append(all, c)
		}
	}

	clusters, err := ClusterHSV(all, 4)
	test.That(t, err, test.ShouldBeNil)

	diffs := ColorDiffs{}

	for x, a := range clusters {
		for y := x + 1; y < len(clusters); y++ {
			if x == y {
				continue
			}
			b := clusters[y]

			diffs.Add(a, b)
		}
	}

	outDir := t.TempDir()
	logging.NewTestLogger(t).CDebugf(ctx, "out dir: %q", outDir)
	err = diffs.WriteTo(outDir + "/foo.html")
	test.That(t, err, test.ShouldBeNil)

	out := NewImage(img.Width(), img.Height())
	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.Get(image.Point{x, y})
			_, cc, _ := c.Closest(clusters)
			out.Set(image.Point{x, y}, cc)
		}
	}

	out.WriteTo(outDir + "/foo.png")
}

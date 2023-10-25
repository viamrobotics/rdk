package rimage

import (
	"fmt"
	"image/color"
	"math"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/floats"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

// Color is a more featured color type than color.Color. Prefer to use
// this over color.Color where possible.
//
// Byte layout
// 0: r
// 1: g
// 2: b
// 3&4: h
// 5: s
// 6: v
// 7: unused
type Color uint64

func newcolor(r, g, b uint8, h uint16, s, v uint8) Color {
	x := uint64(r)
	x |= uint64(g) << 8
	x |= uint64(b) << 16
	x |= uint64(h) << 24
	x |= uint64(s) << 40
	x |= uint64(v) << 48
	return Color(x)
}

// NewColor returns a color based off RGB.
func NewColor(r, g, b uint8) Color {
	h, s, v := rgbToHsv(r, g, b)
	return newcolor(r, g, b, h, s, v)
}

// NewColorFromHexOrPanic returns a color from a RGB hex value. It
// panics if there is an error parsing.
func NewColorFromHexOrPanic(hex string) Color {
	c, err := NewColorFromHex(hex)
	if err != nil {
		panic(err)
	}
	return c
}

// NewColorFromHex returns a color from a RGB hex value.
func NewColorFromHex(hex string) (Color, error) {
	var r, g, b uint8
	n, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	if n != 3 || err != nil {
		return Color(0), errors.Wrapf(err, "couldn't parse hex (%s) n: %d", hex, n)
	}
	return NewColor(r, g, b), nil
}

// NewColorFromHSV returns a color based off HSV.
func NewColorFromHSV(h, s, v float64) Color {
	cc := colorful.Hsv(h, s, v)
	r, g, b := cc.RGB255()
	h2, s2, v2 := tobytehsvfloat(h, s, v)
	return newcolor(r, g, b, h2, s2, v2)
}

// NewColorFromArray returns a color based off the bytes in
// array mapping to the byte fields of a Color.
func NewColorFromArray(buf []float64) Color {
	return newcolor(
		uint8(buf[0]),
		uint8(buf[1]),
		uint8(buf[2]),
		uint16(buf[3]),
		uint8(buf[4]),
		uint8(buf[5]),
	)
}

// NewColorFromColor takes in a go Color and finds the best
// conversion to our Color.
func NewColorFromColor(c color.Color) Color {
	switch cc := c.(type) {
	case Color:
		return cc
	case color.NRGBA:
		return NewColor(cc.R, cc.G, cc.B)
	case color.RGBA:
		if cc.A == 255 {
			return NewColor(cc.R, cc.G, cc.B)
		}
	case color.YCbCr:
		r, g, b := color.YCbCrToRGB(cc.Y, cc.Cb, cc.Cr)
		return NewColor(r, g, b)
	}

	cc, ok := colorful.MakeColor(c)
	if !ok {
		// assume full black
		return NewColor(0, 0, 0)
	}
	r, g, b := cc.RGB255()
	return NewColor(r, g, b)
}

func tobytehsvfloat(h, s, v float64) (uint16, uint8, uint8) {
	return uint16(math.MaxUint16 * (h / 360.0)), uint8(s * 255), uint8(v * 255)
}

// AverageColor returns the average of the HSV color. H is angle in degrees.
// optional weights for the average.
func AverageColor(colors []Color, weights ...float64) Color {
	if len(weights) != 0 && len(colors) != len(weights) {
		panic(fmt.Sprintf("have %d colors and %d weights, must be equal", len(colors), len(weights)))
	}
	if len(weights) == 0 {
		weights = make([]float64, len(colors))
		for i := range weights {
			weights[i] = 1.
		}
	}
	avgH, avgS, avgV := 0.0, 0.0, 0.0

	if num := float64(len(colors)); num <= 0. {
		return NewColorFromHSV(avgH, avgS, avgV)
	}
	// turn hue into cartestian coordinates to average, then transform back into angle
	hueX, hueY := 0.0, 0.0
	for i, c := range colors {
		h, s, v := c.HsvNormal()
		hueX += math.Cos(utils.DegToRad(h)) * weights[i]
		hueY += math.Sin(utils.DegToRad(h)) * weights[i]
		avgS += s * weights[i]
		avgV += v * weights[i]
	}
	hueX /= floats.Sum(weights)
	hueY /= floats.Sum(weights)
	avgH = utils.RadToDeg(math.Atan2(hueY, hueX))
	avgS /= floats.Sum(weights)
	avgV /= floats.Sum(weights)
	return NewColorFromHSV(avgH, avgS, avgV)
}

// RGB255 returns the RGB representation of the color.
func (c Color) RGB255() (uint8, uint8, uint8) {
	return uint8(c & 0xFF), uint8((c >> 8) & 0xFF), uint8((c >> 16) & 0xFF)
}

func (c Color) hsv() (uint16, uint8, uint8) {
	return uint16((c >> 24) & 0xFFFF), uint8((c >> 40) & 0xFF), uint8((c >> 48) & 0xFF)
}

// RawFloatArray returns the byte fields of the color.
func (c Color) RawFloatArray() []float64 {
	return c.RawFloatArrayFill(make([]float64, 6))
}

// RawFloatArrayFill sets the bytes fields of the color on the given slice
// that must have a length of at least 6.
func (c Color) RawFloatArrayFill(buf []float64) []float64 {
	r, g, b := c.RGB255()
	h, s, v := c.hsv()

	buf[0] = float64(r)
	buf[1] = float64(g)
	buf[2] = float64(b)
	buf[3] = float64(h)
	buf[4] = float64(s)
	buf[5] = float64(v)
	return buf
}

// String returns a human readable representation of the color.
func (c Color) String() string {
	h, s, v := c.ScaleHSV()
	return fmt.Sprintf("%s (%3d,%4.2f,%4.2f)", c.Hex(), int(h*360), s, v)
}

// HsvNormal returns a normalized HSV representation of the color.
// h : 0 -> 360, s,v : 0 -> 1.0.
func (c Color) HsvNormal() (float64, float64, float64) {
	h, s, v := c.hsv()
	return 360.0 * float64(h) / float64(math.MaxUint16), float64(s) / 255.0, float64(v) / 255.0
}

// ScaleHSV returns a scaled HSV representation of the color.
func (c Color) ScaleHSV() (float64, float64, float64) {
	h, s, v := c.hsv()
	return float64(h) / float64(math.MaxUint16), float64(s) / 255.0, float64(v) / 255.0
}

// Hex returns the RGB hexadecimal representation of the color.
func (c Color) Hex() string {
	r, g, b := c.RGB255()
	return fmt.Sprintf("#%.2x%.2x%.2x", r, g, b)
}

// RGBA returns the non-alpha-premultiplied RGBA values of the color.
func (c Color) RGBA() (r, g, b, a uint32) {
	R, G, B := c.RGB255()
	a = uint32(255)
	r = uint32(R)
	r |= r << 8
	// r *= a
	// r /= 0xff
	g = uint32(G)
	g |= g << 8
	// g *= a
	// g /= 0xff
	b = uint32(B)
	b |= b << 8
	// b *= a
	// b /= 0xff
	a |= a << 8
	return
}

// Closest returns the color that is closet to this color based
// on the given slice of colors.
func (c Color) Closest(others []Color) (int, Color, float64) {
	if len(others) == 0 {
		panic("HSV::Closest passed nother")
	}

	best := others[0]
	bestDistance := c.Distance(best)
	bestIndex := 0

	for i, x := range others[1:] {
		d := c.Distance(x)
		if d < bestDistance {
			bestDistance = d
			best = x
			bestIndex = i + 1 // +1 is because of the range above
		}
	}

	return bestIndex, best, bestDistance
}

// a and b are between 0 and 1 but it's circular
// so .999 and .001 are .002 apart.
func _loopedDiff(a, b float64) float64 {
	A := math.Max(a, b)
	B := math.Min(a, b)

	d := A - B
	if d > .5 {
		d = 1 - d
	}
	return d
}

func (c Color) toColorful() colorful.Color {
	r, g, b := c.RGB255()
	return colorful.Color{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
	}
}

// DistanceLab returns a measure of visual similarity between two colors.
func (c Color) DistanceLab(b Color) float64 {
	return c.toColorful().DistanceLab(b.toColorful())
}

// Distance returns the "distance" between two colors.
func (c Color) Distance(b Color) float64 {
	debug := false
	return c.distanceDebug(b, debug)
}

func (c Color) distanceDebug(b Color, debug bool) float64 {
	h1, s1, v1 := c.ScaleHSV()
	h2, s2, v2 := b.ScaleHSV()

	wh := 40.0 // ~ 360 / 7 - about 8 degrees of hue change feels like a different color in general
	ws := 6.5
	wv := 5.0

	ac := -1.0
	dd := 1.0
	var section int

	switch {
	case v1 < .13 || v2 < .13:
		section = 1
		// we're in the dark range
		wh /= 30
		ws /= 7
		wv *= 1.5

		if v1 < .1 && v2 < .1 {
			ws /= 3
		}
	case (s1 < .25 && v1 < .25) || (s2 < .25 && v2 < .25):
		section = 2
		// we're in the bottom left quadrat
		wv *= 3.0
		wh /= 20
		ws /= 2
	case s1 < .10 || s2 < .10:
		section = 3
		// we're in the very light range
		wh *= .06 * (v1 + v2) * ((s1 + s2) * 5)
		ws *= 1.15
		wv *= 1.65

		if s1 > .1 || s2 > .2 {
			wh *= 2
		}
		dd = math.Sqrt(.95 + s1 + s2)
		wh *= dd
	case (s1 < .3 && v1 < .345) || (s2 < .3 && v2 < .35):
		section = 4
		// bottom left bigger quadrant
		ac = _ratioOffFrom135(v1-v2, s1-s2)
		wh /= 2.5
		ws *= 1.1
		if v1 < .25 && v2 < .25 {
			wh /= 2
		}
		if ac < .5 {
			wh *= 1.25
		}
		dd = math.Pow(1.5-v1-v2, 2)
		wh *= dd
	case s1 < .19 && s2 < .19:
		section = 5
		// we're in the light range
		wh *= .3
		ws *= 1.25
		wv *= 1.25

		if v1 > .6 && v2 > .6 {
			// this is shiny stuff, be a little more hue generous
			wh *= 1
			wv *= .7
		}
	case s1 > .9 && s2 > .9:
		section = 6
		// in the very right side of the chart
		wh *= 1.2
		ws *= 1.1
		wv *= .7
	case v1 < .20 || v2 < .20:
		section = 7
		wv *= 2.8
		ws /= 4
		wh *= .4
	case v1 < .25 || v2 < .25:
		section = 8
		wv *= 1.5
		ws /= 5
		wh *= .5
	default:
		section = 9
		// if dd is 0, hue is less important, if dd is 2, hue is more important
		dd = utils.Square(math.Min(s1, s2)) + utils.Square(math.Min(v1, v2)) // 0 -> 2

		ddScale := 5.0
		dds := dd / ddScale
		dds += (1 - (1 / ddScale))
		wh *= dds

		if s1 < .5 || s2 < .5 {
			wh *= 1
			ws *= 2.3
			wv *= 1.0
		} else {
			ws *= .5
		}
	}

	hd := _loopedDiff(h1, h2)
	sum := utils.Square(wh * hd)
	sum += utils.Square(ws * (s1 - s2))
	sum += utils.Square(wv * (v1 - v2))

	res := math.Sqrt(sum)

	if debug {
		logging.Global().Debugf("%v -- %v", c, b)
		logging.Global().Debugf("\twh: %5.1f ws: %5.1f wv: %5.1f", wh, ws, wv)
		logging.Global().Debugf("\t    %5.3f     %5.3f     %5.3f", math.Abs(hd), math.Abs(s1-s2), math.Abs(v1-v2))
		logging.Global().Debugf("\t    %5.3f     %5.3f     %5.3f", utils.Square(hd), utils.Square(s1-s2), utils.Square(v1-v2))
		logging.Global().Debugf("\t    %5.3f     %5.3f     %5.3f", utils.Square(wh*hd), utils.Square(ws*(s1-s2)), utils.Square(wv*(v1-v2)))
		logging.Global().Debugf("\t res: %f ac: %f dd: %f section: %d", res, ac, dd, section)
	}
	return res
}

func _ratioOffFrom135(y, x float64) float64 {
	a := math.Atan2(y, x)
	if a < 0 {
		a += math.Pi
	}
	a /= math.Pi

	return _ratioOffFrom135Finish(a)
}

func _ratioOffFrom135Finish(a float64) float64 {
	// a is now between 0 and 1
	// this is how far along the curve of 0 degrees to 180 degrees
	// things in the 0 -> .5 range are V : vworse than things in the .5 -> 1 range

	// .25 is the worst, aka 1
	// .75 is the best, aka 0

	if a <= .5 {
		a = math.Abs(a - .25)
		return 1 - (2 * a)
	}

	a = 2 * math.Abs(.75-a)

	return a
}

func rgbToHsv(r, g, b uint8) (uint16, uint8, uint8) {
	min := utils.MinUint8(utils.MinUint8(r, g), b)
	v := utils.MaxUint8(utils.MaxUint8(r, g), b)
	C := float64(v - min)

	var h uint16
	var s uint8
	if v > 0 {
		// TODO(erh): can make even faster
		s = uint8(255.0 * C / float64(v))
	}

	h = 0 // We use 0 instead of undefined as in wp.
	if min != v {
		var h2 float64
		switch {
		case v == b:
			h2 = (float64(r)-float64(g))/C + 4.0
		case v == g:
			h2 = (float64(b)-float64(r))/C + 2.0
		case v == r:
			h2 = (float64(g) - float64(b)) / C
			if h2 >= 6 || h2 <= -6 {
				panic("i thought this was impossible")
			}
		}

		h2 *= 60.0
		if h2 < 0.0 {
			h2 += 360.0
		}
		h = uint16(math.MaxUint16 * h2 / 360.0)
	}
	return h, s, v
}

// Commonly used colors.
var (
	Red     = NewColor(255, 0, 0)
	DarkRed = NewColor(64, 32, 32)

	Green = NewColor(0, 255, 0)

	Blue     = NewColor(0, 0, 255)
	DarkBlue = NewColor(32, 32, 64)

	White = NewColor(255, 255, 255)
	Gray  = NewColor(128, 128, 128)
	Black = NewColor(0, 0, 0)

	Yellow = NewColor(255, 255, 0)
	Cyan   = NewColor(0, 255, 255)
	Purple = NewColor(255, 0, 255)
	Pink   = NewColor(100, 30, 40)

	Colors = []Color{
		Red,
		DarkRed,
		Green,
		Blue,
		DarkBlue,
		White,
		Gray,
		Black,
		Yellow,
		Cyan,
		Purple,
		Pink,
	}
)

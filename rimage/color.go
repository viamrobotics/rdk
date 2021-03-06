package rimage

import (
	"fmt"
	"image/color"
	"math"

	"github.com/edaniels/golog"
	"github.com/lucasb-eyer/go-colorful"

	"go.viam.com/robotcore/utils"
)

type Color struct {
	R, G, B uint8
	H, S, V float64
}

func (c Color) String() string {
	return fmt.Sprintf("%s (%3d,%4.2f,%4.2f", c.Hex(), int(c.H), c.S, c.V)
}

func (c Color) ScaleHSV() (float64, float64, float64) {
	return c.H / 360, c.S, c.V
}

func (c Color) Hex() string {
	return fmt.Sprintf("#%.2x%.2x%.2x", c.R, c.G, c.B)
}

func (c Color) RGBA() (r, g, b, a uint32) {
	a = uint32(255)
	r = uint32(c.R)
	r |= r << 8
	r *= a
	r /= 0xff
	g = uint32(c.G)
	g |= g << 8
	g *= a
	g /= 0xff
	b = uint32(c.B)
	b |= b << 8
	b *= a
	b /= 0xff
	a |= a << 8
	return
}

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
// so .999 and .001 are .002 apart
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
	cc, ok := colorful.MakeColor(c)
	if !ok {
		panic(fmt.Errorf("bad color %v", c))
	}
	return cc
}

func (c Color) DistanceLab(b Color) float64 {
	return c.toColorful().DistanceLab(b.toColorful())
}

func (c Color) Distance(b Color) float64 {
	debug := false
	return c.distanceDebug(b, debug)
}

func (c Color) distanceDebug(b Color, debug bool) float64 {
	h1, s1, v1 := c.ScaleHSV()
	h2, s2, v2 := b.ScaleHSV()

	wh := 40.0 // ~ 360 / 7 - about 8 degrees of hue change feels like a different color ing enral
	ws := 6.5
	wv := 5.0

	ac := -1.0
	dd := 1.0
	var section int

	if v1 < .13 || v2 < .13 {
		section = 1
		// we're in the dark range
		wh /= 30
		ws /= 7
		wv *= 1.5

		if v1 < .1 && v2 < .1 {
			ws /= 3
		}
	} else if (s1 < .25 && v1 < .25) || (s2 < .25 && v2 < .25) {
		section = 2
		// we're in the bottom left quadrat
		wh /= 5
		ws /= 2
	} else if (s1 < .3 && v1 < .35) || (s2 < .3 && v2 < .35) {
		section = 3
		// bottom left bigger quadrant
		wh /= 2.5
	} else if s1 < .10 || s2 < .10 {
		section = 4
		// we're in the very light range
		wh *= .06 * (v1 + v2) * ((s1 + s2) * 5)
		ws *= 1.15
		wv *= 1.54
	} else if s1 < .19 && s2 < .19 {
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
	} else if s1 > .9 && s2 > .9 {
		section = 6
		// in the very right side of the chart
		wh *= 1.2
		ws *= 1.1
		wv *= .7
	} else if v1 < .20 || v2 < .20 {
		section = 7
		wv *= 2.8
		ws /= 4
		wh *= .4
	} else if v1 < .25 || v2 < .25 {
		section = 8
		wv *= 1.5
		ws /= 5
		wh *= .5
	} else {
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
		golog.Global.Debugf("%v -- %v", c, b)
		golog.Global.Debugf("\twh: %5.1f ws: %5.1f wv: %5.1f", wh, ws, wv)
		golog.Global.Debugf("\t    %5.3f     %5.3f     %5.3f", math.Abs(hd), math.Abs(s1-s2), math.Abs(v1-v2))
		golog.Global.Debugf("\t    %5.3f     %5.3f     %5.3f", utils.Square(hd), utils.Square(s1-s2), utils.Square(v1-v2))
		golog.Global.Debugf("\t    %5.3f     %5.3f     %5.3f", utils.Square(wh*hd), utils.Square(ws*(s1-s2)), utils.Square(wv*(v1-v2)))
		golog.Global.Debugf("\t res: %f ac: %f dd: %f section: %d", res, ac, dd, section)
	}
	return res
}

func _ratioOffFrom135(y, x float64) float64 {
	a := math.Atan2(y, x)
	if a < 0 {
		a = a + math.Pi
	}
	a = a / math.Pi

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

func NewColor(r, g, b uint8) Color {
	cc := colorful.Color{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
	}
	h, s, v := cc.Hsv()

	return Color{
		R: r,
		G: g,
		B: b,
		H: h,
		S: s,
		V: v,
	}
}

func NewColorFromHexOrPanic(hex string) Color {
	c, err := NewColorFromHex(hex)
	if err != nil {
		panic(err)
	}
	return c
}

func NewColorFromHex(hex string) (Color, error) {
	var r, g, b uint8
	n, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	if n != 3 || err != nil {
		return Color{}, fmt.Errorf("couldn't parse hex (%s) n: %d err: %w", hex, n, err)
	}
	return NewColor(r, g, b), nil
}

func NewColorFromHSV(h, s, v float64) Color {
	cc := colorful.Hsv(h, s, v)
	r, g, b := cc.RGB255()
	return Color{
		R: r,
		G: g,
		B: b,
		H: h,
		S: s,
		V: v,
	}
}

func NewColorFromColor(c color.Color) Color {
	if cc, ok := c.(Color); ok {
		return cc
	}
	cc, ok := colorful.MakeColor(c)
	if !ok {
		panic(fmt.Errorf("bad color %v", c))
	}
	r, g, b := cc.RGB255()
	h, s, v := cc.Hsv()

	return Color{
		R: r,
		G: g,
		B: b,
		H: h,
		S: s,
		V: v,
	}
}

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
	}
)

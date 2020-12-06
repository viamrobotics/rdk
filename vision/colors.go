package vision

import (
	"fmt"
	"image/color"
	"math"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/echolabsinc/robotcore/rcutil"
)

type HSV struct {
	H float64
	S float64
	V float64
}

func (c HSV) String() string {
	return fmt.Sprintf("hsv: %3d,%4.2f,%4.2f", int(c.H), c.S, c.V)
}

func (c HSV) Scale() (float64, float64, float64) {
	return c.H / 360, c.S, c.V
}

func (c HSV) ToColorful() colorful.Color {
	return colorful.Hsv(c.H, c.S, c.V)
}

func (a HSV) Distance(b HSV) float64 {
	debug := false

	h1, s1, v1 := a.Scale()
	h2, s2, v2 := b.Scale()

	if debug {
		fmt.Printf("%v -- %1.2f %1.2f %1.2f \n%v -- %1.2f %1.2f %1.2f\n", a, h1, s1, v1, b, h2, s2, v2)
	}

	wh := 40.0 // ~ 360 / 7 - about 8 degrees of hue change feels like a different color ing enral
	ws := 5.0
	wv := 5.0

	ac := -1.0
	if v1 < .1 {
		// we're in the dark range
		wh /= 1000
		ws *= 100
		wv *= 100
	} else if s1 < .1 {
		// we're in the light range
		wh /= 1000
		ws *= 100
		wv *= 100
	} else if s1 < .45 || v1 < .45 || s2 < .45 || v2 < .45 {
		// we're playing with the angle of the v1,s1 -> v2,s2 vector
		ac = _ratioOffFrom135(v2-v1, s2-s1) // this is 0(more similar) -> 1(less similar)
		ac = math.Pow(ac, .3333)
		wh *= rcutil.Square(1 - ac) // the further from normal the more we care about hue
		ws *= 4.2 * ac
		wv *= 1.1 * ac
	} else {
		// we're playing with the angle of the v1,s1 -> v2,s2 vector
		ac = _ratioOffFrom135(v2-v1, s2-s1) // this is 0(more similar) -> 1(less similar)
		wh *= rcutil.Square(1 - ac)         // the further from normal the more we care about hue
		ws *= 3.2 * ac
		wv *= 1.2 * ac
	}

	sum := rcutil.Square(wh * (h1 - h2))
	sum += rcutil.Square(ws * (s1 - s2))
	sum += rcutil.Square(wv * (v1 - v2))

	res := math.Sqrt(sum)

	if debug {
		fmt.Printf("\twh: %5.1f ws: %5.1f wv: %5.1f\n", wh, ws, wv)
		fmt.Printf("\t    %5.3f     %5.3f     %5.3f\n", math.Abs(h1-h2), math.Abs(s1-s2), math.Abs(v1-v2))
		fmt.Printf("\t    %5.3f     %5.3f     %5.3f\n", rcutil.Square(h1-h2), rcutil.Square(s1-s2), rcutil.Square(v1-v2))
		fmt.Printf("\t    %5.3f     %5.3f     %5.3f\n", rcutil.Square(wh*(h1-h2)), rcutil.Square(ws*(s1-s2)), rcutil.Square(wv*(v1-v2)))
		fmt.Printf("\t res: %f\n", res)
		fmt.Printf("\t ac: %f\n", ac)
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
	// things in the 0 -> .5 range are worse than things in the .5 -> 1 range

	// .25 is the worst, aka 1
	// .75 is the best, aka 0

	if a <= .5 {
		a = math.Abs(a - .25)
		return 1 - (2 * a)
	}

	a = 2 * math.Abs(.75-a)

	return a
}

func NewHSV(h, s, v float64) HSV {
	return HSV{h, s, v}
}

func ConvertToColorful(c color.RGBA) colorful.Color {
	return colorful.Color{
		R: float64(c.R) / 255.0,
		G: float64(c.G) / 255.0,
		B: float64(c.B) / 255.0,
	}
}

func ConvertToHSV(c color.RGBA) HSV {
	return NewHSV(ConvertToColorful(c).Hsv())
}

// ---

type Color struct {
	C     color.RGBA
	Name  string
	CC    colorful.Color
	AsHSV HSV
}

func (c Color) RGBA() (uint32, uint32, uint32, uint32) {
	return c.C.RGBA()
}

func (c Color) Distance(other color.RGBA) float64 {
	return ColorDistance(c.C, other)
}

func (c Color) Hex() string {
	return fmt.Sprintf("#%.2x%.2x%.2x", c.C.R, c.C.G, c.C.B)
}

func (c Color) String() string {
	return fmt.Sprintf("Color(%s %s %s)", c.Hex(), c.AsHSV.String(), c.Name)
}

func NewColorFromHex(hex, name string) (Color, error) {
	var r, g, b uint8
	n, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	if n != 3 || err != nil {
		return Color{}, fmt.Errorf("couldn't parse hex (%s) n: %d err: %w", hex, n, err)
	}
	return NewColor(r, g, b, name), nil

}

func NewColor(r, g, b uint8, name string) Color {
	c := Color{C: color.RGBA{r, g, b, 255}, Name: name}
	c.CC = ConvertToColorful(c.C)
	c.AsHSV = ConvertToHSV(c.C)
	return c
}

var (
	Red     = NewColor(255, 0, 0, "red")
	DarkRed = NewColor(64, 32, 32, "darkRed")

	Green = NewColor(0, 255, 0, "green")

	Blue     = NewColor(0, 0, 255, "blue")
	DarkBlue = NewColor(32, 32, 64, "darkBlue")

	White = NewColor(255, 255, 255, "white")
	Gray  = NewColor(128, 128, 128, "gray")
	Black = NewColor(0, 0, 0, "black")

	Yellow = NewColor(255, 255, 0, "yellow")
	Cyan   = NewColor(0, 255, 255, "cyan")
	Purple = NewColor(255, 0, 255, "purple")

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

func distance(a, b []int) float64 {
	if len(a) != len(b) {
		panic("not the same distance")
	}

	accum := 0.0

	for idx, x := range a {
		y := b[idx]
		accum += math.Pow(float64(y-x), 2)
	}
	return math.Sqrt(accum)
}

func colorDistanceRaw(r1, g1, b1, r2, g2, b2 float64) float64 {

	r_line := (r1 + r2) / 2

	diff := (2 + (r_line / 256)) * rcutil.Square(r2-r1)
	diff += 4 * rcutil.Square(g2-g1)
	diff += (2 + ((255 - r_line) / 256)) * rcutil.Square(b2-b1)

	return math.Sqrt(diff)
}

func ColorDistance(a, b color.RGBA) float64 {
	return colorDistanceRaw(
		float64(a.R), float64(a.G), float64(a.B),
		float64(b.R), float64(b.G), float64(b.B))
}

func WhatColor(data color.RGBA) Color {
	distance := 1000000000.0
	c := Red

	//fmt.Println("---")
	for _, clr := range Colors {
		x := clr.Distance(data)
		//fmt.Printf("\t %s %f\n", name, x)
		if x > distance {
			continue
		}
		distance = x
		c = clr
	}

	return c
}

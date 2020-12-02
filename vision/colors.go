package vision

import (
	"fmt"
	"image/color"
	"math"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/echolabsinc/robotcore/rcutil"
)

type HCL struct {
	H float64
	C float64
	L float64
}

func (hcl HCL) String() string {
	return fmt.Sprintf("hcl: %6.2f %5.2f %5.2f", hcl.H, hcl.C, hcl.L)
}

func (a HCL) Distance(b HCL) float64 {
	sum := rcutil.Square(a.H - b.H)
	sum += 10 * rcutil.Square(a.C-b.C)
	sum += rcutil.Square(a.L - b.L)
	return math.Sqrt(sum)
}

func NewHCL(h, c, l float64) HCL {
	return HCL{h, c, l}
}

// ---

type HSV struct {
	H float64
	S float64
	V float64
}

func (c HSV) String() string {
	return fmt.Sprintf("hsv: %5.1f,%5.2f,%4.1f", c.H, c.S, c.V)
}

func (c HSV) Scale() (float64, float64, float64) {
	return c.H / 240, c.S, c.V / 255
}

func (a HSV) Distance(b HSV) float64 {
	debug := false

	h1, s1, v1 := a.Scale()
	h2, s2, v2 := b.Scale()

	if debug {
		fmt.Printf("%v -- %1.2f %1.2f %1.2f \n%v -- %1.2f %1.2f %1.2f\n", a, h1, s1, v1, b, h2, s2, v2)
	}

	wh := 100.0
	ws := 1.0
	wv := 1.0

	if v1 < .1 {
		// we're in the dark range
		wh = .1
		ws = 100
		wv = 100
	} else if s1 < .1 {
		// we're in the light range
		wh = .1
		ws = 100
		wv = 100
	} else {
		wh = v1*50 + s1*50
		ws = v1 * 100
		wv = s1 * 100
	}

	sum := wh * rcutil.Square(h1-h2)
	//fmt.Printf("\t\t%v\n", sum)
	sum += ws * rcutil.Square(s1-s2)
	//fmt.Printf("\t\t%v\n", sum)
	sum += wv * rcutil.Square(v1-v2)
	//fmt.Printf("\t\t%v\n", sum)

	res := math.Sqrt(sum)

	// we're playing with the angle of the v1,s1 -> v2,s2 vector
	ac := .5 + _rationOffFrom135(v2-v1, s2-s1)/2
	res = res * ac

	if debug {
		fmt.Printf("\twh: %5.1f ws: %5.1f wv: %5.1f\n", wh, ws, wv)
		fmt.Printf("\t    %5.3f     %5.3f     %5.3f\n", math.Abs(h1-h2), math.Abs(s1-s2), math.Abs(v1-v2))
		fmt.Printf("\t    %5.3f     %5.3f     %5.3f\n", rcutil.Square(h1-h2), rcutil.Square(s1-s2), rcutil.Square(v1-v2))
		fmt.Printf("\t res: %f\n", res)
		fmt.Printf("\t ac: %f\n", ac)
	}
	return res
}

func _rationOffFrom135(y, x float64) float64 {
	a := math.Atan2(y, x)
	//print("\t%f" % a )
	if a < 0 {
		a = a + math.Pi
	}
	//print("\t%f" % a )
	a = a / math.Pi
	//print("\t%f" % a )
	a = .75 - a
	//print("\t%f" % a )
	a = 2 * math.Abs(a)
	//print("\t%f" % a )
	return a
}

func NewHSV(h, s, v float64) HSV {
	return HSV{h, s, v}
}

func ConvertToHSV(c color.RGBA) HSV {
	temp, b := colorful.MakeColor(c)
	if !b {
		panic("wtf") // this should never happen
	}

	return NewHSV(temp.Hsv())
}

// ---

type Color struct {
	C     color.RGBA
	Name  string
	AsHSV HSV
}

func (c Color) RGBA() (uint32, uint32, uint32, uint32) {
	return c.C.RGBA()
}

func (c Color) Distance(other color.RGBA) float64 {
	return ColorDistance(c.C, other)
}

func (c Color) String() string {
	return fmt.Sprintf("Color(rgb %d,%d,%d %s %s)", c.C.R, c.C.G, c.C.B, c.AsHSV.String(), c.Name)
}

func NewColor(r, g, b uint8, name string) Color {
	c := Color{C: color.RGBA{r, g, b, 1}, Name: name}
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

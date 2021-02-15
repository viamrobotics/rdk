package utils

import (
	"fmt"
	"html/template"
	"image/color"
	"io/ioutil"
	"math"
	"strings"

	"github.com/edaniels/golog"
	"github.com/lucasb-eyer/go-colorful"
)

type HSV struct {
	H float64 // degrees 0-360
	S float64 // 0-1
	V float64 // 0-1
}

func (c HSV) String() string {
	return fmt.Sprintf("hsv: %3d,%4.2f,%4.2f", int(c.H), c.S, c.V)
}

func (c HSV) Scale() (float64, float64, float64) {
	return c.H / 360, c.S, c.V
}

func (c HSV) ToColor() Color {
	return NewColorFromColorful(c.ToColorful())
}

func (c HSV) Hex() string {
	return c.ToColorful().Hex()
}

func (c HSV) RGBA() (r, g, b, a uint32) {
	return c.ToColorful().RGBA()
}

func (c HSV) ToColorful() colorful.Color {
	return colorful.Hsv(c.H, c.S, c.V)
}

func (c HSV) Closest(others []HSV) (int, HSV, float64) {
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

func (c HSV) Distance(b HSV) float64 {
	debug := false
	return c.distanceDebug(b, debug)
}

func (c HSV) distanceDebug(b HSV, debug bool) float64 {
	h1, s1, v1 := c.Scale()
	h2, s2, v2 := b.Scale()

	wh := 40.0 // ~ 360 / 7 - about 8 degrees of hue change feels like a different color ing enral
	ws := 6.5
	wv := 5.0

	ac := -1.0
	dd := 1.0

	if v1 < .13 || v2 < .13 {
		// we're in the dark range
		wh /= 30
		ws /= 5
		wv *= 1.5

		if v1 < .1 && v2 < .1 {
			ws /= 3
		}
	} else if s1 < .10 || s2 < .10 {
		// we're in the very light range
		wh /= 100
		ws *= 1.5
		wv *= 1.5
	} else if s1 < .19 && s2 < .19 {
		// we're in the light range
		wh /= 20
		ws *= 1.25
		wv *= 1.25
	} else if (s1 < .25 && v1 < .25) || (s2 < .25 && v2 < .25) {
		// we're in the bottom left quadrat
		wh /= 5
	} else if (s1 < .3 && v1 < .3) || (s2 < .3 && v2 < .3) {
		// bottom left bigger quadrant
		wh /= 2.5
	} else if s1 > .9 && s2 > .9 {
		// in the very right side of the chart
		wv *= .6
	} else if v1 < .20 || v2 < .20 {
		wv *= 3
		ws /= 3
		wh *= .5
	} else if v1 < .25 || v2 < .25 {
		wv *= 1.5
		ws /= 3
		wh *= .5
	} else {
		// if dd is 0, hue is less important, if dd is 2, hue is more important
		dd = Square(math.Min(s1, s2)) + Square(math.Min(v1, v2)) // 0 -> 2

		ddScale := 5.0
		dds := dd / ddScale
		dds += (1 - (1 / ddScale))
		wh *= dds

		if s1 < .5 || s2 < .5 {
			wh *= 1
			ws *= 2.3
			wv *= 1.0
		}

	}

	hd := _loopedDiff(h1, h2)
	sum := Square(wh * hd)
	sum += Square(ws * (s1 - s2))
	sum += Square(wv * (v1 - v2))

	res := math.Sqrt(sum)

	if debug {
		golog.Global.Debugf("%v -- %v", c, b)
		golog.Global.Debugf("\twh: %5.1f ws: %5.1f wv: %5.1f", wh, ws, wv)
		golog.Global.Debugf("\t    %5.3f     %5.3f     %5.3f", math.Abs(hd), math.Abs(s1-s2), math.Abs(v1-v2))
		golog.Global.Debugf("\t    %5.3f     %5.3f     %5.3f", Square(hd), Square(s1-s2), Square(v1-v2))
		golog.Global.Debugf("\t    %5.3f     %5.3f     %5.3f", Square(wh*hd), Square(ws*(s1-s2)), Square(wv*(v1-v2)))
		golog.Global.Debugf("\t res: %f ac: %f dd: %f", res, ac, dd)
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

func ConvertToColorful2(c color.Color) colorful.Color {
	r, g, b, _ := c.RGBA()
	return colorful.Color{
		R: float64(r) / 65535.0,
		G: float64(g) / 65535.0,
		B: float64(b) / 65535.0,
	}
}

func ConvertToHSV(c color.RGBA) HSV {
	return NewHSV(ConvertToColorful(c).Hsv())
}

func ConvertToHSV2(c color.Color) HSV {
	return NewHSV(ConvertToColorful2(c).Hsv())
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

func (c Color) Distance(other Color) float64 {
	return c.AsHSV.Distance(other.AsHSV)
}

func (c Color) Hex() string {
	return fmt.Sprintf("#%.2x%.2x%.2x", c.C.R, c.C.G, c.C.B)
}

func (c Color) String() string {
	return fmt.Sprintf("Color(%s %s %s)", c.Hex(), c.AsHSV.String(), c.Name)
}

func NewColorFromColorful(c colorful.Color) Color {
	r, g, b := c.RGB255()
	return NewColor(r, g, b, "")
}

func NewColorFromHexOrPanic(hex, name string) Color {
	c, err := NewColorFromHex(hex, name)
	if err != nil {
		panic(err)
	}
	return c
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

func colorDistanceRaw(r1, g1, b1, r2, g2, b2 float64) float64 {

	rLine := (r1 + r2) / 2

	diff := (2 + (rLine / 256)) * Square(r2-r1)
	diff += 4 * Square(g2-g1)
	diff += (2 + ((255 - rLine) / 256)) * Square(b2-b1)

	return math.Sqrt(diff)
}

func ColorDistance(a, b color.RGBA) float64 {
	return colorDistanceRaw(
		float64(a.R), float64(a.G), float64(a.B),
		float64(b.R), float64(b.G), float64(b.B))
}

func WhatColor(data Color) Color {
	distance := 1000000000.0
	c := Red

	//fmt.Println("---")
	for _, clr := range Colors {
		x := clr.Distance(data)
		//golog.Global.Debugf("\t %s %f\n", name, x)
		if x > distance {
			continue
		}
		distance = x
		c = clr
	}

	return c
}

type ColorDiff struct {
	Left  HSV
	Right HSV
	Diff  float64
}

type ColorDiffs []ColorDiff

func (x *ColorDiffs) output() string {
	t := "<html><body><table>" +
		"{{ range .}}" +
		"<tr>" +
		"<td style='background-color:{{.Left.ToColorful.Hex}}'>{{ .Left.ToColorful.Hex }}&nbsp;</td>" +
		"<td style='background-color:{{.Right.ToColorful.Hex}}'>{{ .Right.ToColorful.Hex }}&nbsp;</td>" +
		"<td>{{ .Diff }}</td>" +
		"</tr>" +
		"{{end}}" +
		"</table></body></html>"

	tt, err := template.New("temp").Parse(t)
	if err != nil {
		panic(err)
	}

	w := strings.Builder{}
	err = tt.Execute(&w, x)
	if err != nil {
		panic(err)
	}
	return w.String()
}

func (x *ColorDiffs) WriteTo(fn string) error {
	return ioutil.WriteFile(fn, []byte(x.output()), 0640)
}

func ComputeColorDiffs(all []HSV) ColorDiffs {
	diffs := ColorDiffs{}
	for i, c := range all {
		for _, c2 := range all[i+1:] {
			d := c.Distance(c2)
			diffs = append(diffs, ColorDiff{c, c2, d})
		}
	}
	return diffs
}

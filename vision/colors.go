package vision

import (
	"fmt"
	"image/color"
	"math"
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
	sum := (a.H - b.H) * (a.H - b.H)
	sum += (a.C - b.C) * (a.C - b.C)
	sum += (a.L - b.L) * (a.L - b.L)
	return math.Sqrt(sum)
}

func NewHCL(h, c, l float64) HCL {
	return HCL{h, c, l}
}

// ---

type Color struct {
	C        color.RGBA
	Name     string
	RootName string
}

func (c Color) RGBA() (uint32, uint32, uint32, uint32) {
	return c.C.RGBA()
}

func (c Color) Distance(other color.RGBA) float64 {
	return ColorDistance(c.C, other)
}

func (c Color) String() string {
	return fmt.Sprintf("rgb %d,%d,%d", c.C.R, c.C.G, c.C.B)
}

var (
	Red     = Color{color.RGBA{255, 0, 0, 1}, "red", "red"}
	DarkRed = Color{color.RGBA{64, 32, 32, 1}, "darkRed", "red"}

	Green = Color{color.RGBA{0, 255, 0, 1}, "green", "green"}

	Blue     = Color{color.RGBA{0, 0, 255, 1}, "blue", "blue"}
	DarkBlue = Color{color.RGBA{32, 32, 64, 1}, "darkBlue", "blue"}

	White = Color{color.RGBA{255, 255, 255, 1}, "white", "white"}
	Gray  = Color{color.RGBA{128, 128, 128, 1}, "gray", "gray"}
	Black = Color{color.RGBA{0, 0, 0, 1}, "black", "black"}

	Yellow = Color{color.RGBA{255, 255, 0, 1}, "yellow", "yellow"}
	Cyan   = Color{color.RGBA{0, 255, 255, 1}, "cyan", "cyan"}
	Purple = Color{color.RGBA{255, 0, 255, 1}, "purple", "purple"}

	Colors = map[string]Color{
		"red":      Red,
		"darkRed":  DarkRed,
		"green":    Green,
		"blue":     Blue,
		"darkBlue": DarkBlue,
		"white":    White,
		"gray":     Gray,
		"black":    Black,
		"yellow":   Yellow,
		"cyan":     Cyan,
		"purple":   Purple,
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

	diff := (2 + (r_line / 256)) * (r2 - r1) * (r2 - r1)
	diff += 4 * (g2 - g1) * (g2 - g1)
	diff += (2 + ((255 - r_line) / 256)) * (b2 - b1) * (b2 - b1)

	return math.Sqrt(diff)
}

func ColorDistance(a, b color.RGBA) float64 {
	return colorDistanceRaw(
		float64(a.R), float64(a.G), float64(a.B),
		float64(b.R), float64(b.G), float64(b.B))
}

func WhatColor(data color.RGBA) (string, Color) {
	distance := 1000000000.0
	n := ""
	c := Red

	//fmt.Println("---")
	for name, clr := range Colors {
		x := clr.Distance(data)
		//fmt.Printf("\t %s %f\n", name, x)
		if x > distance {
			continue
		}
		distance = x
		n = name
		c = clr
	}

	return n, c
}

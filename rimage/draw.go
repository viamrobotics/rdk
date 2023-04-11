package rimage

import (
	"image"
	"image/color"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
)

var font *truetype.Font

// init sets up the fonts we want to use.
func init() {
	var err error
	font, err = truetype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}
}

// Font returns the font we use for drawing.
func Font() *truetype.Font {
	return font
}

// DrawString writes a string to the given context at a particular point.
func DrawString(dc *gg.Context, text string, p image.Point, c color.Color, size float64) {
	dc.SetFontFace(truetype.NewFace(Font(), &truetype.Options{Size: size}))
	dc.SetColor(c)
	dc.DrawStringWrapped(text, float64(p.X), float64(p.Y), 0, 0, float64(dc.Width()), 1, 0)
}

// DrawRectangleEmpty draws the given rectangle into the context. The positions of the
// rectangle are used to place it within the context.
func DrawRectangleEmpty(dc *gg.Context, r image.Rectangle, c color.Color, width float64) {
	dc.SetColor(c)

	dc.DrawLine(float64(r.Min.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Min.Y))
	dc.SetLineWidth(width)
	dc.Stroke()

	dc.DrawLine(float64(r.Min.X), float64(r.Min.Y), float64(r.Min.X), float64(r.Max.Y))
	dc.SetLineWidth(width)
	dc.Stroke()

	dc.DrawLine(float64(r.Max.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Max.Y))
	dc.SetLineWidth(width)
	dc.Stroke()

	dc.DrawLine(float64(r.Min.X), float64(r.Max.Y), float64(r.Max.X), float64(r.Max.Y))
	dc.SetLineWidth(width)
	dc.Stroke()
}

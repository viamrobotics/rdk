package rimage

import (
	"image"
	"image/color"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
)

var font *truetype.Font

func init() {
	var err error
	font, err = truetype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}
}

func Font() *truetype.Font {
	return font
}

func DrawString(dc *gg.Context, text string, p image.Point, c color.Color, size float64) {
	dc.SetFontFace(truetype.NewFace(Font(), &truetype.Options{Size: size}))
	dc.SetColor(c)
	dc.DrawString(text, float64(p.X), float64(p.Y))
}

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

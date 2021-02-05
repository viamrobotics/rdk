package utils

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/lmittmann/ppm"
	"github.com/lucasb-eyer/go-colorful"
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

func WriteImageToFile(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	switch filepath.Ext(path) {
	case ".png":
		return png.Encode(f, img)
	case ".ppm":
		return ppm.Encode(f, img)
	default:
		return fmt.Errorf("utils.WriteImageToFile unsupported format: %s", filepath.Ext(path))
	}

}

func ReadImageFromFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
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

func SimpleEdgeDetection(img image.Image, t1 float64, blur float64) (*image.Gray, error) {
	img = imaging.Blur(img, blur)

	out := image.NewGray(img.Bounds())

	for y := 0; y < img.Bounds().Max.Y; y++ {
		for x := 0; x < img.Bounds().Max.X-1; x++ {
			c0, b := colorful.MakeColor(img.At(x, y))
			if !b {
				continue
			}
			c1, b := colorful.MakeColor(img.At(x+1, y))
			if !b {
				continue
			}

			//fmt.Printf("%d %d %v\n", x, y, c0.DistanceLab(c1))
			if c0.DistanceLab(c1) >= t1 {
				out.SetGray(x, y, color.Gray{255})
			} else {
				out.SetGray(x, y, color.Gray{0})

			}
		}
	}

	for x := 0; x < img.Bounds().Max.X; x++ {
		for y := 0; y < img.Bounds().Max.Y-1; y++ {
			c0, b := colorful.MakeColor(img.At(x, y))
			if !b {
				continue
			}
			c1, b := colorful.MakeColor(img.At(x, y+1))
			if !b {
				continue
			}

			//fmt.Printf("%d %d %v\n", x, y, c0.DistanceLab(c1))
			if c0.DistanceLab(c1) >= t1 {
				out.SetGray(x, y, color.Gray{255})
			}
		}
	}

	return out, nil
}

func CountBrightSpots(img *image.Gray, center image.Point, radius int, threshold uint8) int {
	num := 0

	for x := center.X - radius; x < center.X+radius; x++ {
		for y := center.Y - radius; y < center.Y+radius; y++ {
			d := img.GrayAt(x, y)
			if d.Y >= threshold {
				num++
			}
		}
	}

	return num
}

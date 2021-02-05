package vision

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"strings"

	"github.com/viamrobotics/robotcore/utils"

	"github.com/fogleman/gg"
)

type Image struct {
	img *image.RGBA

	width  int
	height int
}

func NewImageFromFile(fn string) (Image, error) {
	if strings.HasSuffix(fn, ".both.gz") {

		f, err := os.Open(fn)
		if err != nil {
			return Image{}, err
		}
		defer f.Close()

		in, err := gzip.NewReader(f)
		if err != nil {
			return Image{}, err
		}

		defer in.Close()

		allData, err := ioutil.ReadAll(in)

		if err != nil {
			return Image{}, err
		}

		img, _, err := readNextImageDepthPairFromBoth(allData)
		if err != nil {
			return Image{}, err
		}

		return NewImage(img), nil
	}

	img, err := utils.ReadImageFromFile(fn)
	if err != nil {
		return Image{}, err
	}
	return NewImage(img), nil
}

func NewImage(img image.Image) Image {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return Image{img: rgba, height: bounds.Max.Y, width: bounds.Max.X}
}

func (i *Image) Rows() int {
	return i.height
}

func (i *Image) Cols() int {
	return i.width
}

func (i *Image) Height() int {
	return i.height
}

func (i *Image) Width() int {
	return i.width
}

func (i *Image) ColorHSV(p image.Point) utils.HSV {
	return utils.ConvertToHSV(i.ColorRowCol(p.Y, p.X))
}

func (i *Image) At(x, y int) color.Color {
	return i.img.At(x, y)
}

func (i *Image) Bounds() image.Rectangle {
	return i.img.Bounds()
}

func (i *Image) ColorModel() color.Model {
	return i.img.ColorModel()
}

func (i *Image) Color(p image.Point) color.RGBA {
	return i.ColorRowCol(p.Y, p.X)
}

func (i *Image) ColorXY(x, y int) color.RGBA {
	return i.ColorRowCol(y, x)
}

func (i *Image) ColorRowCol(row, col int) color.RGBA {

	if row < 0 || col < 0 || row >= i.height || col >= i.width {
		panic(fmt.Errorf("bad row or col want: %d %d width: %d height: %d", row, col, i.width, i.height))
	}

	r, g, b, a := i.img.At(col, row).RGBA()
	return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}
}

func (i *Image) SetHSV(p image.Point, c utils.HSV) {
	i.SetColor(p, c.ToColor())
}

func (i *Image) SetColor(p image.Point, c utils.Color) {
	i.SetColorRowCol(p.Y, p.X, c)
}

func (i *Image) SetColorRowCol(row, col int, c utils.Color) {
	i.img.Set(col, row, c)
}

// does not return a copy
func (i *Image) Image() image.Image {
	return i.img
}

func (i *Image) ImageCopy() *image.RGBA {
	bounds := i.img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, i.img, bounds.Min, draw.Src)
	return rgba
}

// TODO(erh): move this to a better file
func PointDistance(a, b image.Point) float64 {
	x := utils.SquareInt(b.X - a.X)
	x += utils.SquareInt(b.Y - a.Y)
	return math.Sqrt(float64(x))
}

func (i *Image) WriteTo(fn string) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, i.img)
}

func (i *Image) ToBytes() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := png.Encode(buf, i.img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (i *Image) Circle(center image.Point, radius int, c utils.Color) {
	dc := gg.NewContextForRGBA(i.img) // no copy
	dc.DrawCircle(float64(center.X), float64(center.Y), 1)
	dc.SetColor(c.C)
	dc.Fill()
}

func (i *Image) AverageColor(p image.Point, radius int) utils.HSV {
	return i.AverageColorXY(p.X, p.Y, radius)
}

func (i *Image) AverageColorXY(x, y int, radius int) utils.HSV {
	h := 0.0
	s := 0.0
	v := 0.0

	num := 0.0

	for X := x - radius; X < x+radius; X++ {
		for Y := y - radius; Y < y+radius; Y++ {
			if X < 0 || Y < 0 || X >= i.width || Y >= i.height {
				continue
			}

			data := i.ColorHSV(image.Point{X, Y})
			h += data.H
			s += data.S
			v += data.V

			num++
		}
	}

	return utils.HSV{h / num, s / num, v / num}
}

func (i *Image) ClusterHSV(numClusters int) ([]utils.HSV, error) {
	allColors := make([]utils.HSV, i.width*i.height)
	for x := 0; x < i.Width(); x++ {
		for y := 0; y < i.Height(); y++ {
			allColors[(x*i.height)+y] = i.ColorHSV(image.Point{x, y})
		}
	}
	return ClusterHSV(allColors, numClusters)
}

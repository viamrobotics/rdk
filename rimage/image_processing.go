package rimage

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
)

// return avg color, avg distances to avg color
func (i *Image) AverageColorAndStats(p image.Point, radius int) (Color, float64) {
	avg := i.AverageColor(p, 1)

	total := 0.0
	num := 0.0

	for xx := p.X - radius; xx <= p.X+radius; xx++ {
		for yy := p.Y - radius; yy <= p.Y+radius; yy++ {
			if xx < 0 || yy < 0 || xx >= i.width || yy >= i.height {
				continue
			}

			num++

			myColor := i.Get(image.Point{xx, yy})
			myDistance := avg.Distance(myColor)
			total += myDistance
		}
	}

	avgDistance := total / num

	return avg, avgDistance
}

func (i *Image) AverageColor(p image.Point, radius int) Color {
	return i.AverageColorXY(p.X, p.Y, radius)
}

func (i *Image) AverageColorXY(x, y int, radius int) Color {
	h := 0.0
	s := 0.0
	v := 0.0

	num := 0.0

	for X := x - radius; X <= x+radius; X++ {
		for Y := y - radius; Y <= y+radius; Y++ {
			if X < 0 || Y < 0 || X >= i.width || Y >= i.height {
				continue
			}

			data := i.Get(image.Point{X, Y})
			h += 360 * float64(data.h) / float64(math.MaxUint16)
			s += float64(data.s) / 255.0
			v += float64(data.v) / 255.0

			num++
		}
	}

	return NewColorFromHSV(h/num, s/num, v/num)
}

// TODO(erh): this and SimpleEdgeDetection are suoer similar, we shouldn't have both probably? or if we do need better names
func (i *Image) InterestingPixels(t float64) *image.Gray {
	out := image.NewGray(i.Bounds())

	for x := 0; x < i.Width(); x += 3 {
		for y := 0; y < i.Height(); y += 3 {

			_, avgDistance := i.AverageColorAndStats(image.Point{x + 1, y + 1}, 1)

			clr := color.Gray{0}
			if avgDistance > t {
				clr = color.Gray{255}
			}

			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					xx := x + a
					yy := y + b
					out.SetGray(xx, yy, clr)
				}
			}

		}
	}

	return out
}

func SimpleEdgeDetection(img *Image, t1 float64, blur float64) (*image.Gray, error) {
	img = ConvertImage(imaging.Blur(img, blur))

	out := image.NewGray(img.Bounds())

	for y := 0; y < img.Bounds().Max.Y; y++ {
		for x := 0; x < img.Bounds().Max.X-1; x++ {
			c0 := img.GetXY(x, y)
			c1 := img.GetXY(x+1, y)

			if c0.DistanceLab(c1) >= t1 {
				out.SetGray(x, y, color.Gray{255})
			} else {
				out.SetGray(x, y, color.Gray{0})

			}
		}
	}

	for x := 0; x < img.Bounds().Max.X; x++ {
		for y := 0; y < img.Bounds().Max.Y-1; y++ {
			c0 := img.GetXY(x, y)
			c1 := img.GetXY(x, y+1)

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

func (i *Image) Rotate(amount int) *Image {
	if amount != 180 {
		// made this a panic
		panic(fmt.Errorf("rimage.Image can only rotate 180 degrees right now"))
	}

	i2 := NewImage(i.width, i.height)

	for x := 0; x < i.width; x++ {
		for y := 0; y < i.height; y++ {
			val := i.GetXY(i.width-1-x, i.height-1-y)
			i2.SetXY(x, y, val)
		}
	}

	return i2
}

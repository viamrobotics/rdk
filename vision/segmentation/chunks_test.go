package segmentation

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/viamrobotics/robotcore/vision"
)

type chunkImageDebug struct {
}

func (cid *chunkImageDebug) Process(d *vision.MultipleImageTestDebugger, fn string, img vision.Image) error {

	type MyPoint struct {
		corner      image.Point
		avgDistance float64
	}

	points := []MyPoint{}
	maxDistance := 0.0

	t1 := .2

	out1 := image.NewGray(img.Bounds())

	for x := 0; x < img.Width(); x += 3 {
		for y := 0; y < img.Height(); y += 3 {

			avg := img.AverageColorXY(x+1, y+1, 1)

			total := 0.0
			num := 0.0

			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					xx := x + a
					yy := y + b
					if xx >= img.Width() || yy >= img.Height() {
						continue
					}
					num++

					myColor := img.ColorHSV(image.Point{xx, yy})
					myDistance := avg.Distance(myColor)
					if myDistance > 1 && x > 405 && x < 495 && y > 505 && y < 595 {
						fmt.Printf("%v,%v avg: %v myColor: %v myDistance: %v\n", xx, yy, avg, myColor, myDistance)
					}
					total += myDistance
				}
			}

			avgDistance := total / num

			points = append(points, MyPoint{image.Point{x, y}, avgDistance})

			if avgDistance > maxDistance {
				maxDistance = avgDistance
			}

			clr := color.Gray{0}
			if avgDistance > t1 {
				clr = color.Gray{255}
			}

			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					xx := x + a
					yy := y + b
					out1.SetGray(xx, yy, clr)
				}
			}

		}
	}

	d.GotDebugImage(out1, "t")

	if maxDistance < t1 {
		return fmt.Errorf("maxDistance too low %v", maxDistance)
	}

	out := image.NewGray(img.Bounds())

	for _, p := range points {
		scale := (p.avgDistance - t1) / (maxDistance - t1)
		if scale < 0 {
			scale = 0
		}
		clr := color.Gray{uint8(254 * scale)}

		for a := 0; a < 3; a++ {
			for b := 0; b < 3; b++ {
				xx := p.corner.X + a
				yy := p.corner.Y + b
				out.SetGray(xx, yy, clr)
			}
		}
	}

	d.GotDebugImage(out, "interesting1")

	return nil
}

func TestChunk1(t *testing.T) {
	d := vision.NewMultipleImageTestDebugger("segmentation/test1", "*.png")
	err := d.Process(&chunkImageDebug{})
	if err != nil {
		t.Fatal(err)
	}

}

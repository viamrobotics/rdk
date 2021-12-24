package action

import (
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/fogleman/gg"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func roverWalk(pc *rimage.ImageWithDepth, debug bool, logger golog.Logger) (image.Image, int) {
	var dc *gg.Context
	if debug {
		dc = gg.NewContextForImage(image.NewRGBA(pc.Color.Bounds()))
	}

	points := 0

	middleX := pc.Width() / 2

	if middleX < 10 {
		panic("wtf")
	}

	if pc.Height() < 10 {
		panic("wtf")
	}

	radius := pc.Width() / 4

	err := utils.Walk(middleX, pc.Height()-1, radius,
		func(x, y int) error {
			if x < 0 || x >= pc.Width() || y < 0 || y >= pc.Height() {
				return nil
			}

			p := image.Point{x, y}
			other := image.Point{x, y}
			if x < middleX {
				other.X = x + 1
			} else if x > middleX {
				other.X = x - 1
			}

			if y < pc.Height()-1 {
				other.Y = y + 1
			}

			d := pc.Depth.Get(p)
			d2 := pc.Depth.Get(other)

			c := pc.Color.Get(p)
			c2 := pc.Color.Get(other)

			colorDiff := c.Distance(c2)

			if d > 0 && d2 > 0 && utils.AbsInt(int(d-d2)) > 20 && colorDiff > .3 {
				if dc != nil {
					dc.DrawCircle(float64(p.X), float64(p.Y), 1)
					dc.SetColor(rimage.Red)
					dc.Fill()
				}
				points++
			} else if colorDiff > 2 {
				if dc != nil {
					dc.DrawCircle(float64(p.X), float64(p.Y), 1)
					dc.SetColor(rimage.Green)
					dc.Fill()
				}
				points++
			}

			return nil
		})
	if err != nil {
		panic(err)
	}

	if dc != nil {
		rimage.DrawRectangleEmpty(dc,
			image.Rect(
				middleX-radius, pc.Height()-radius,
				middleX+radius, pc.Height()-1),
			rimage.Red, 2)

		dc.SetColor(rimage.Red)
		dc.Fill()

		rimage.DrawString(dc, fmt.Sprintf("%d", points), image.Point{20, 80}, rimage.Green, 64)
	}

	logger.Debugf("\t %d", points)

	if dc != nil {
		return dc.Image(), points
	}

	return nil, points
}

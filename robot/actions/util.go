package actions

import (
	"fmt"
	"image"

	"go.viam.com/robotcore/utils"
	"go.viam.com/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/fogleman/gg"
)

func roverWalk(pc *vision.PointCloud, debug bool) (image.Image, int) {

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

	err := vision.Walk(middleX, pc.Height()-1, radius,
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

			c := pc.Color.ColorHSV(p)
			c2 := pc.Color.ColorHSV(other)

			colorDiff := c.Distance(c2)

			if d > 0 && d2 > 0 && utils.AbsInt(d-d2) > 20 && colorDiff > .3 {
				if dc != nil {
					dc.DrawCircle(float64(p.X), float64(p.Y), 1)
					dc.SetColor(utils.Red.C)
					dc.Fill()
				}
				points++
			} else if colorDiff > 2 {
				if dc != nil {
					dc.DrawCircle(float64(p.X), float64(p.Y), 1)
					dc.SetColor(utils.Green.C)
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
		utils.DrawRectangleEmpty(dc,
			image.Rect(
				middleX-radius, pc.Height()-radius,
				middleX+radius, pc.Height()-1),
			utils.Red.C, 2)

		dc.SetColor(utils.Red.C)
		dc.Fill()

		utils.DrawString(dc, fmt.Sprintf("%d", points), image.Point{20, 80}, utils.Green.C, 64)
	}

	golog.Global.Debugf("\t %d", points)

	if dc != nil {
		return dc.Image(), points
	}

	return nil, points
}

// ------

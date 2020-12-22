package main

import (
	"fmt"
	"image"

	"github.com/echolabsinc/robotcore/rcutil"
	"github.com/echolabsinc/robotcore/utils/log"
	"github.com/echolabsinc/robotcore/vision"

	"gocv.io/x/gocv"
)

func roverWalk(pc *vision.PointCloud, debug *gocv.Mat) int {

	points := 0

	middleX := pc.Width() / 2

	if middleX < 10 {
		panic("wtf")
	}

	if pc.Height() < 10 {
		panic("wtf")
	}

	radius := pc.Width() / 4

	vision.Walk(middleX, pc.Height()-1, radius,
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

			if rcutil.AbsInt(d-d2) > 20 && colorDiff > .3 {
				if debug != nil {
					gocv.Circle(debug, p, 1, vision.Red.C, 1)
				}
				points++
			} else if colorDiff > 2 {
				if debug != nil {
					gocv.Circle(debug, p, 1, vision.Green.C, 1)
				}
				points++
			}

			return nil
		})

	if debug != nil {
		gocv.Rectangle(debug, image.Rect(
			middleX-radius, pc.Height()-1,
			middleX+radius, pc.Height()-radius),
			vision.Red.C, 1)

		gocv.PutText(debug, fmt.Sprintf("%d", points), image.Point{20, 80}, gocv.FontHersheyPlain, 5, vision.Green.C, 2)

	}

	log.Global.Debugf("\t %d\n", points)

	return points
}

package main

import (
	"fmt"
	"image"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/rcutil"
	"github.com/echolabsinc/robotcore/vision"
)

func roverColorize(pc *vision.PointCloud, debug gocv.Mat) {

	for x := 0; x < pc.Width(); x++ {
		for y := 0; y < pc.Height(); y++ {
			p := image.Point{x, y}
			d := pc.Depth.Get(p)

			if d == 0 {
				continue
			}

			if d < 600 {
				gocv.Circle(&debug, p, 1, vision.Red.C, 1)
			} else if d < 900 {
				gocv.Circle(&debug, p, 1, vision.Green.C, 1)
			} else if d < 1300 {
				gocv.Circle(&debug, p, 1, vision.Blue.C, 1)
			} else if d < 2000 {
				gocv.Circle(&debug, p, 1, vision.Purple.C, 1)
			} else {
				gocv.Circle(&debug, p, 1, vision.White.C, 1)
			}

		}
	}
}

func roverWalk(pc *vision.PointCloud, debug gocv.Mat) int {

	points := 0

	middleX := pc.Width() / 2

	vision.Walk(middleX, pc.Height()-1, pc.Width()/4,
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
				other.Y = y - 1
			}

			d := pc.Depth.Get(p)
			d2 := pc.Depth.Get(other)

			c := pc.Color.ColorHSV(p)
			c2 := pc.Color.ColorHSV(other)

			colorDiff := c.Distance(c2)

			if rcutil.AbsInt(d-d2) > 20 {
				gocv.Circle(&debug, p, 1, vision.Red.C, 1)
				points++
			} else if colorDiff > 2 {
				gocv.Circle(&debug, p, 1, vision.Green.C, 1)
				points++
			}

			return nil
		})

	fmt.Printf("\t %d\n", points)

	return points
}

func roverWalkOld(pc *vision.PointCloud, debug gocv.Mat) {

	prevD := 0

	for y := pc.Height() - 1; y >= 0; y-- {
		x := pc.Width() / 2

		p := image.Point{x, y}
		d := pc.Depth.Get(p)

		if d == 0 {
			continue
		}

		if prevD == 0 {
			prevD = d
			continue
		}

		diff := d - prevD
		fmt.Printf("%v %v\n", p, d-prevD)
		if rcutil.AbsInt(diff) > 10 {
			gocv.Circle(&debug, p, 1, vision.Red.C, 1)
		}

		prevD = d
	}
}

package main

import (
	"fmt"
	"image"

	"gocv.io/x/gocv"

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

	for y := pc.Height() - 1; y >= pc.Height()-400; y-- {
		x := pc.Width() / 2
		p := image.Point{x, y}
		d := pc.Depth.Get(p)
		if d == 0 {
			continue
		}

		fmt.Printf("%v %v\n", p, d)
	}
}

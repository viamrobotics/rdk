package main

import (
	"image"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

func roverColorize(pc *vision.PointCloud) {
	mat := pc.Color.MatUnsafe()

	//for x := (pc.Width()/2) - 200; x < (pc.Width() / 2) + 200; x++ {
	for x := 0; x < pc.Width(); x++ {
		for y := 0; y < pc.Height(); y++ {
			p := image.Point{x, y}
			d := pc.Depth.Get(p)

			if d == 0 {
				gocv.Circle(&mat, p, 1, vision.Black.C, 1)
				continue
			}
			if d < 1000 {
				gocv.Circle(&mat, p, 1, vision.Red.C, 1)
			} else if d < 2000 {
				gocv.Circle(&mat, p, 1, vision.Green.C, 1)
			} else if d < 3500 {
				gocv.Circle(&mat, p, 1, vision.Blue.C, 1)
			} else if d < 5000 {
				gocv.Circle(&mat, p, 1, vision.Purple.C, 1)
			} else if d < 8000 {
				gocv.Circle(&mat, p, 1, vision.White.C, 1)
			}

		}
	}

}

package vision

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

var (
	myPinks = []Color{
		Color{color.RGBA{178, 97, 117, 0}, "deepPink", "pink"},
		Color{color.RGBA{184, 102, 126, 0}, "deepPink", "pink"},
		Color{color.RGBA{202, 105, 134, 0}, "deepPink", "pink"},
		Color{color.RGBA{192, 93, 118, 0}, "deepPink", "pink"},
		Color{color.RGBA{209, 129, 150, 0}, "deepPink", "pink"},
		Color{color.RGBA{128, 72, 50, 0}, "brownPink", "pink"},
		Color{color.RGBA{136, 64, 37, 0}, "brownPink", "pink"},
		Color{color.RGBA{219, 103, 169, 0}, "brownPink", "pink"},
		Color{color.RGBA{233, 108, 130, 0}, "brownPink", "pink"},
		Color{color.RGBA{190, 104, 162, 0}, "brownPink", "pink"},
		Color{color.RGBA{167, 74, 107, 0}, "brownPink", "pink"},
	}
)

func myPinkDistance(data gocv.Vecb) float64 {
	d := 10000000000.0
	for _, c := range myPinks {
		temp := colorDistance(data, c)
		if temp < d {
			d = temp
		}
	}
	return d
}

func FindChessCornersPinkCheat_inQuadrant(out *gocv.Mat, cnts [][]image.Point, xQ, yQ int) image.Point {

	minX := xQ * (out.Cols() / 2)
	minY := yQ * (out.Rows() / 2)

	maxX := (1 + xQ) * (out.Cols() / 2)
	maxY := (1 + yQ) * (out.Rows() / 2)

	//fmt.Printf("\t %d %d  %d %d %d %d\n", xQ, yQ, minX, minY, maxX, maxY)

	longest := 0.0
	best := cnts[0]
	bestIdx := 0

	for idx, c := range cnts {
		if c[0].X < minX || c[0].X > maxX || c[0].Y < minY || c[0].Y > maxY {
			continue
		}
		arclength := gocv.ArcLength(c, true)
		if arclength > longest {
			longest = arclength
			best = c
			bestIdx = idx
		}
	}

	//fmt.Printf("\t\t %d, %d arclength: %f\n", best[0].X, best[0].Y, longest)
	myCenter := center(best)

	for i := 0; i < 5; i++ { // move at most 10 for sanity
		data := out.GetVecbAt(myCenter.Y, myCenter.X)
		blackDistance := colorDistance(data, Black)
		if blackDistance > 2 {
			break
		}
		myCenter.X = myCenter.X + ((xQ * 2) - 1)
		myCenter.Y = myCenter.Y + ((yQ * 2) - 1)
	}

	gocv.DrawContours(out, cnts, bestIdx, Green.C, 1)
	gocv.Circle(out, myCenter, 5, Red.C, 2)

	return myCenter
}

func FindChessCornersPinkCheat(img gocv.Mat, out *gocv.Mat) ([]image.Point, error) {

	if out == nil {
		return nil, fmt.Errorf("processFindCornersBad needs an out")
	}

	temp := gocv.NewMat()
	defer temp.Close()
	gocv.CvtColor(img, &temp, gocv.ColorBGRToRGBA)

	img.CopyTo(out)

	for x := 0; x <= img.Cols(); x++ {
		for y := 0; y <= img.Rows(); y++ {
			data := temp.GetVecbAt(y, x)
			p := image.Point{x, y}

			d := myPinkDistance(data)

			if d < 40 {
				gocv.Circle(out, p, 1, Green.C, 1)
			} else {
				gocv.Circle(out, p, 1, Black.C, 1)
			}

			if false {
				if y == 40 && x > 130 && x < 160 {
					fmt.Printf("  --  %d %d %v %f\n", x, y, data, d)
					gocv.Circle(out, p, 1, Red.C, 1)
				}
			}

		}
	}

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(*out, &edges, 30, 200)

	cnts := gocv.FindContours(edges, gocv.RetrievalTree, gocv.ChainApproxSimple)

	a1Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 0, 0)
	a8Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 1, 0)
	h1Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 0, 1)
	h8Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 1, 1)

	return []image.Point{a1Corner, a8Corner, h1Corner, h8Corner}, nil
}

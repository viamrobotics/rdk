package chess

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

func center(contour []image.Point) image.Point {

	r := gocv.BoundingRect(contour)

	return image.Point{(r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2}
}

func processFindCornersBad(img gocv.Mat, out *gocv.Mat) ([]image.Point, error) {
	if out == nil {
		return nil, fmt.Errorf("processFindCornersBad needs an out")
	}
	img.CopyTo(out)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(*out, &edges, 30, 200)

	cnts := gocv.FindContours(edges, gocv.RetrievalTree, gocv.ChainApproxSimple)
	fmt.Printf("num cnts: %d\n", len(cnts))

	biggestArea := 0.0
	for _, c := range cnts {
		area := gocv.ContourArea(c)
		if area > biggestArea {
			biggestArea = area
		}
	}

	for idx, c := range cnts {
		arcLength := gocv.ArcLength(c, true)
		curve := gocv.ApproxPolyDP(c, 0.015*arcLength, true)
		area := gocv.ContourArea(c)

		good := (area > 300 && len(curve) == 4) || (area > 1800 && area < 4000)
		if !good {
			continue
		}

		fmt.Printf("\t len: %d area: %f\n", len(curve), area)
		gocv.DrawContours(out, cnts, idx, vision.Green.C, 1)
		gocv.PutText(out, fmt.Sprintf("%d", int(area)), c[0], gocv.FontHersheyPlain, 1.2, vision.Green.C, 2)

		myCenter := center(c)
		gocv.Circle(out, myCenter, 5, vision.Red.C, 2)
	}

	//cnts = sorted(cnts, key = cv2.contourArea, reverse = True)[:10]
	//screenCnt = None

	return nil, nil
}

var (
	myPinks = []vision.Color{
		vision.Color{color.RGBA{178, 97, 117, 0}, "deepPink", "pink"},
		vision.Color{color.RGBA{184, 102, 126, 0}, "deepPink", "pink"},
		vision.Color{color.RGBA{202, 105, 134, 0}, "deepPink", "pink"},
		vision.Color{color.RGBA{192, 93, 118, 0}, "deepPink", "pink"},
		vision.Color{color.RGBA{209, 129, 150, 0}, "deepPink", "pink"},
		vision.Color{color.RGBA{128, 72, 50, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{136, 64, 37, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{219, 103, 169, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{233, 108, 130, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{190, 104, 162, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{167, 74, 107, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{191, 81, 95, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{201, 83, 97, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{211, 94, 106, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{167, 68, 90, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{179, 71, 90, 0}, "brownPink", "pink"},
		vision.Color{color.RGBA{202, 83, 94, 0}, "brownPink", "pink"},
	}
)

func myPinkDistance(data gocv.Vecb) float64 {
	d := 10000000000.0
	for _, c := range myPinks {
		temp := vision.ColorDistance(data, c)
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
	// walk up into the corner
	myCenter := center(best)

	for i := 0; i < 5; i++ { // move only a little for sanity
		data := out.GetVecbAt(myCenter.Y, myCenter.X)
		blackDistance := vision.ColorDistance(data, vision.Black)
		if blackDistance > 2 {
			break
		}
		myCenter.X = myCenter.X + ((xQ * 2) - 1)
		myCenter.Y = myCenter.Y + ((yQ * 2) - 1)
	}

	gocv.DrawContours(out, cnts, bestIdx, vision.Blue.C, 1)
	gocv.Circle(out, myCenter, 5, vision.Red.C, 2)

	return myCenter
}

func _avgColor(img gocv.Mat, x, y int) gocv.Vecb {
	b := 0
	g := 0
	r := 0

	num := 0

	for X := x - 1; X < x+1; X++ {
		for Y := y - 1; Y < y+1; Y++ {
			data := img.GetVecbAt(Y, X)
			b += int(data[0])
			g += int(data[1])
			r += int(data[2])
			num++
		}
	}

	done := gocv.Vecb{uint8(b / num), uint8(g / num), uint8(r / num)}
	return done
}

func FindChessCornersPinkCheat(img gocv.Mat, out *gocv.Mat) ([]image.Point, error) {

	if out == nil {
		return nil, fmt.Errorf("processFindCornersBad needs an out")
	}

	img.CopyTo(out)

	for x := 1; x < img.Cols(); x++ {
		for y := 1; y < img.Rows(); y++ {
			//data := img.GetVecbAt(y, x)
			data := _avgColor(img, x, y)
			p := image.Point{x, y}

			d := myPinkDistance(data)

			if d < 40 {
				gocv.Circle(out, p, 1, vision.Green.C, 1)
			} else {
				gocv.Circle(out, p, 1, vision.Black.C, 1)
			}

			if false {
				if y == 40 && x > 130 && x < 160 {
					fmt.Printf("  --  %d %d %v %f\n", x, y, data, d)
					gocv.Circle(out, p, 1, vision.Red.C, 1)
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

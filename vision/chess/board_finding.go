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
		vision.Color{color.RGBA{208, 73, 99, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{223, 79, 101, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{195, 78, 109, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{198, 65, 106, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{192, 57, 83, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{183, 68, 107, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{171, 61, 100, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{156, 65, 102, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{221, 68, 93, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{205, 63, 87, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{220, 108, 119, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{205, 101, 103, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{172, 90, 112, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{164, 48, 81, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{149, 47, 85, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{142, 45, 120, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{139, 37, 75, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{203, 108, 142, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{196, 97, 139, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{173, 96, 140, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{161, 112, 144, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{140, 82, 108, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{126, 71, 107, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{221, 105, 164, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{223, 117, 159, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{232, 127, 154, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{234, 109, 153, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{201, 148, 184, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{237, 158, 174, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{191, 121, 171, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{179, 145, 183, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{180, 128, 179, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{167, 125, 164, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{217, 144, 163, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{181, 124, 133, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{177, 75, 134, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{163, 69, 132, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{201, 132, 147, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{163, 69, 132, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{154, 80, 136, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{125, 81, 211, 0}, "myPink", "pink"},
		vision.Color{color.RGBA{210, 85, 127, 0}, "myPink", "pink"},
	}
)

func MyPinkDistance(data gocv.Vecb) float64 {
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

	for i := 0; i < 50; i++ { // move only a little for sanity
		data := out.GetVecbAt(myCenter.Y, myCenter.X)
		blackDistance := vision.ColorDistance(data, vision.Black)
		if blackDistance > 2 {
			break
		}
		xWalk := ((xQ * 2) - 1)
		yWalk := ((yQ * 2) - 1)

		stop := false
		for j := 0; j < 100; j++ {
			temp := myCenter
			temp.X += j * -1 * xWalk
			data := out.GetVecbAt(temp.Y, temp.X)
			blackDistance := vision.ColorDistance(data, vision.Black)
			if blackDistance > 2 {
				stop = true
				break
			}
		}
		if stop {
			break
		}

		myCenter.X += xWalk
		myCenter.Y += yWalk
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

	redLittleCircles := []image.Point{}

	for x := 1; x < img.Cols(); x++ {
		for y := 1; y < img.Rows(); y++ {
			//data := img.GetVecbAt(y, x)
			data := _avgColor(img, x, y)
			p := image.Point{x, y}

			d := MyPinkDistance(data)

			if d < 40 {
				gocv.Circle(out, p, 1, vision.Green.C, 1)
			} else {
				gocv.Circle(out, p, 1, vision.Black.C, 1)
			}

			if false {
				if y == 157 && x > 310 && x < 340 {
					fmt.Printf("  --  %d %d %v %f\n", x, y, data, d)
					redLittleCircles = append(redLittleCircles, p)
				}
			}

		}
	}

	gocv.GaussianBlur(*out, out, image.Point{3, 3}, 30, 50, 4)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(*out, &edges, 20, 500)

	cnts := gocv.FindContours(edges, gocv.RetrievalTree, gocv.ChainApproxTC89KCOS) //ChainApproxSimple)

	a1Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 0, 0)
	a8Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 1, 0)
	h1Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 0, 1)
	h8Corner := FindChessCornersPinkCheat_inQuadrant(out, cnts, 1, 1)

	for _, p := range redLittleCircles {
		gocv.Circle(out, p, 1, vision.Red.C, 1)
	}

	return []image.Point{a1Corner, a8Corner, h1Corner, h8Corner}, nil
}

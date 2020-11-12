package chess

import (
	"fmt"
	"image"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

func center(contour []image.Point) image.Point {

	r := gocv.BoundingRect(contour)

	return image.Point{(r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2}
	/*
		x := 0
		y := 0

		for _, p := range contour {
			x += p.X
			y += p.Y
		}

		return image.Point{ x / len(contour), y / len(contour) }
	*/
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

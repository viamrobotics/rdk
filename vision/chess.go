package vision

import (
	"fmt"
	//"image"
	"image/color"
	//"math"

	"gocv.io/x/gocv"
)

var (
	Blue  = color.RGBA{0, 0, 255, 0}
	Green = color.RGBA{0, 255, 0, 0}
)

func initialize_mask(adaptiveThresh, img gocv.Mat) {
	contours := gocv.FindContours(adaptiveThresh, gocv.RetrievalTree, gocv.ChainApproxSimple)
	fmt.Printf("num contours: %d\n", len(contours))
}

func process2(img gocv.Mat) {
	contours := gocv.FindContours(img, gocv.RetrievalList, gocv.ChainApproxSimple)
	fmt.Printf("num contours: %d\n", len(contours))
}

func process(img gocv.Mat, out *gocv.Mat) {
	temp := gocv.NewMat()
	defer temp.Close()

	gocv.CvtColor(img, &temp, gocv.ColorBGRToGray)
	gocv.BilateralFilter(temp, out, 11, 17.0, 17.0)

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
		if area < biggestArea {
			//continue
		}
		fmt.Printf("\t %d\n", len(curve))
		gocv.DrawContours(out, cnts, idx, Blue, 1)
	}

	//cnts = sorted(cnts, key = cv2.contourArea, reverse = True)[:10]
	//screenCnt = None

}

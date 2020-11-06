package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gocv.io/x/gocv"
)

var (
	Blue = color.RGBA{0, 0, 255, 0}
)

func initialize_mask(adaptiveThresh, img gocv.Mat) {
	contours := gocv.FindContours(adaptiveThresh, gocv.RetrievalTree, gocv.ChainApproxSimple)
	fmt.Printf("num contours: %d\n", len(contours))
}

func process2(img gocv.Mat) {
	contours := gocv.FindContours(img, gocv.RetrievalList, gocv.ChainApproxSimple)
	fmt.Printf("num contours: %d\n", len(contours))
}

func process(img gocv.Mat) {
	edges := gocv.NewMat()
	defer edges.Close()
	lines := gocv.NewMat()
	defer lines.Close()

	gocv.Canny(img, &edges, 100, 500)
	gocv.HoughLinesP(edges, &lines, 1000, math.Pi/180, 1200)
	fmt.Printf("num lines: %d\n", lines.Rows())
	for i := 0; i < lines.Rows(); i++ {
		pt1 := image.Pt(int(lines.GetVeciAt(i, 0)[0]), int(lines.GetVeciAt(i, 0)[1]))
		pt2 := image.Pt(int(lines.GetVeciAt(i, 0)[2]), int(lines.GetVeciAt(i, 0)[3]))
		gocv.Line(&img, pt1, pt2, Blue, 2)
	}

}

func fromGithub(img gocv.Mat) {
	gocv.CvtColor(img, &img, gocv.ColorRGBToGray)

	/*
		# Setting all pixels above the threshold value to white and those below to black
		# Adaptive thresholding is used to combat differences of illumination in the picture
		adaptiveThresh = cv2.adaptiveThreshold(gray, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY, 125, 1)
		if debug:
			# Show thresholded image
			cv2.imshow("Adaptive Thresholding", adaptiveThresh)
			cv2.waitKey(0)
			cv2.destroyAllWindows()

		return adaptiveThresh,img

	*/
}

func closeupProcess(img gocv.Mat) {
	//gocv.GaussianBlur(img, &img, image.Pt(23, 23), 30, 50, 4) // TODO: play with params
	//process(img)
	fromGithub(img)
	//process(img)
}

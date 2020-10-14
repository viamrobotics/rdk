package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

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

func webcamDisplaySilly() {
	// set to use a video capture device 0
	deviceID := 0

	// open webcam
	webcam, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer webcam.Close()

	// open display window
	window := gocv.NewWindow("test1")
	defer window.Close()

	// prepare image matrix
	img := gocv.NewMat()
	defer img.Close()

	fmt.Printf("start reading camera device: %v\n", deviceID)
	for {
		if ok := webcam.Read(&img); !ok {
			fmt.Printf("cannot read device %v\n", deviceID)
			continue
		}
		if img.Empty() {
			continue
		}

		process(img)

		window.IMShow(img)
		window.WaitKey(1)
		time.Sleep(1000 * time.Millisecond)
	}
}

func hardCodedEliot(img gocv.Mat) {
	rotationMatrix := gocv.GetRotationMatrix2D(image.Point{img.Rows() / 2, img.Cols() / 2}, -54, 1.0)
	gocv.WarpAffine(img, &img, rotationMatrix, image.Point{
		int(float64(img.Rows()) * 1.5),
		int(float64(img.Cols()) * 1.5)})

	src := []image.Point{
		image.Pt(0, 0),
		image.Pt(img.Rows()-200, 75),
		image.Pt(img.Rows()-65, img.Cols()-120),
		image.Pt(0, img.Cols()+50),
	}
	dst := []image.Point{
		image.Pt(0, 0),
		image.Pt(img.Rows(), 0),
		image.Pt(img.Rows(), img.Cols()),
		image.Pt(0, img.Cols()),
	}

	m := gocv.GetPerspectiveTransform(src, dst)
	defer m.Close()

	gocv.WarpPerspective(img, &img, m, image.Point{img.Rows(), img.Cols()})
	croppedMat := img.Region(image.Rect(170, 160, 580, 600))
	croppedMat.CopyTo(&img)

	
	boardOffset := 10
	boardWidth := img.Cols() - (boardOffset * 2)
	boardHeight := img.Rows() - (boardOffset * 2)

	squareWidth := boardWidth / 8
	squareHeight := boardHeight / 8
	
	for x := boardOffset + squareWidth / 2; x < boardWidth; x = x + squareWidth {
		for y := boardOffset + squareHeight / 2; y < boardHeight; y = y + squareHeight {
			gocv.Circle(&img, image.Point{x,y}, 5, Blue, 2)
		}
	}
}

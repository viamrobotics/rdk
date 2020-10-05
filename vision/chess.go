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
	blue = color.RGBA{0, 0, 255, 0}
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

	gocv.Canny(img, &edges, 50, 200)
	gocv.HoughLinesP(edges, &lines, 1, math.Pi/180, 5)

	for i := 0; i < lines.Rows(); i++ {
		pt1 := image.Pt(int(lines.GetVeciAt(i, 0)[0]), int(lines.GetVeciAt(i, 0)[1]))
		pt2 := image.Pt(int(lines.GetVeciAt(i, 0)[2]), int(lines.GetVeciAt(i, 0)[3]))
		gocv.Line(&img, pt1, pt2, blue, 2)
	}

}

func main() {
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

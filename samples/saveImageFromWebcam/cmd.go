package main

import (
	"fmt"

	"gocv.io/x/gocv"
)


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

		gocv.IMWrite("data.bmp", img)
		
		window.IMShow(img)
		window.WaitKey(1)
	}
}

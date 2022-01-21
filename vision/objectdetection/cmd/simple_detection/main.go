package main

import (
	"flag"
	"os"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/objectdetection"

	"github.com/edaniels/golog"
)

func main() {
	imgPtr := flag.String("img", "", "path to image to apply simple detection to")
	threshPtr := flag.Int("thresh", 20, "grayscale value that sets the threshold for detection")
	sizePtr := flag.Int("size", 500, "minimum size of a detection")
	flag.Parse()
	logger := golog.NewLogger("simple_detection")
	detect(*imgPtr, *threshPtr, *sizePtr, logger)
	os.Exit(0)
}

func detect(imgPath string, thresh, size int, logger golog.Logger) {
	img, err := rimage.NewImageFromFile(imgPath)
	if err != nil {
		logger.Fatal(err)
	}

	// get the bounding boxes
	d := objectdetection.NewSimpleDetector(thresh, size)
	bbs, err := d.Inference(img)
	if err != nil {
		logger.Fatal(err)
	}
	for i, bb := range bbs {
		logger.Infof("detection %d: upperLeft(%d, %d), lowerRight(%d,%d)", i, bb.BoundingBox.Min.X, bb.BoundingBox.Min.Y, bb.BoundingBox.Max.X, bb.BoundingBox.Max.Y)
	}
	// overlay them over the image
	ovImg := objectdetection.Overlay(img, bbs)
	err = rimage.WriteImageToFile("./simple_detection.png", ovImg)
	if err != nil {
		logger.Fatal(err)
	}
}

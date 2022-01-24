package main

import (
	"context"
	"flag"
	"image"
	"os"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/objectdetection"
)

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Next(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageFromFile(s.filePath)
	return img, func() {}, err
}

type payload struct {
	Original      image.Image
	Modified      image.Image
	BoundingBoxes []objectdetection.Detection
}

func main() {
	imgPtr := flag.String("img", "", "path to image to apply simple detection to")
	threshPtr := flag.Int("thresh", 20, "grayscale value that sets the threshold for detection")
	sizePtr := flag.Int("size", 500, "minimum size of a detection")
	flag.Parse()
	logger := golog.NewLogger("simple_detection")
	pipeline(*imgPtr, *threshPtr, *sizePtr, logger)
	logger.Info("Done")
	os.Exit(0)
}

func pipeline(imgPath string, thresh, size int, logger golog.Logger) {
	// create source
	src := &camera.ImageSource{ImageSource: &simpleSource{imgPath}}
	// create preprocessor
	p := objectdetection.RemoveBlue()
	// create the detector
	d := objectdetection.NewSimpleDetector(thresh)
	// create filter
	f := objectdetection.NewAreaFilter(size)

	// make a pipeline
	source := func(out chan<- *payload) {
		for x := 0; x < 20; x++ {
			img, _, err := src.Next(context.Background())
			if err != nil {
				close(out)
				logger.Fatal(err)
			}
			pl := &payload{Original: img}
			out <- pl
		}
		close(out)
	}
	preprocess := func(in <-chan *payload, out chan<- *payload) {
		for pl := range in {
			pl.Modified = p(pl.Original)
			out <- pl
		}
		close(out)
	}
	detection := func(in <-chan *payload, out chan<- *payload) {
		var err error
		for pl := range in {
			pl.BoundingBoxes, err = d(pl.Modified)
			if err != nil {
				close(out)
				logger.Fatal(err)
			}
			out <- pl
		}
		close(out)
	}
	filter := func(in <-chan *payload, out chan<- *payload) {
		for pl := range in {
			pl.BoundingBoxes = f(pl.BoundingBoxes)
			out <- pl
		}
		close(out)
	}
	printer := func(in <-chan *payload) {
		for pl := range in {
			logger.Info("Next Image:")
			for i, bb := range pl.BoundingBoxes {
				box := bb.BoundingBox()
				logger.Infof("detection %d: upperLeft(%d, %d), lowerRight(%d,%d)", i, box.Min.X, box.Min.Y, box.Max.X, box.Max.Y)
				// overlay them over the image
				//ovImg := objectdetection.Overlay(img, bbs)
				//err = rimage.WriteImageToFile("./simple_detection.png", ovImg)
				//if err != nil {
				//	logger.Fatal(err)
				//}
			}
		}
	}

	// run the pipeline
	images := make(chan *payload)
	processed := make(chan *payload)
	detected := make(chan *payload)
	filtered := make(chan *payload)
	go source(images)
	go preprocess(images, processed)
	go detection(processed, detected)
	go filter(detected, filtered)
	printer(filtered)
}

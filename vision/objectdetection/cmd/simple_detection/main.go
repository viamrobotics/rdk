package main

import (
	"context"
	"flag"
	"image"
	"net"
	"net/url"
	"os"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/imagesource"
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

func main() {
	imgPtr := flag.String("img", "", "path to image to apply simple detection to")
	urlPtr := flag.String("url", "", "url to image source to apply simple detection to")
	threshPtr := flag.Float64("thresh", 20, "grayscale value that sets the threshold for detection between 0(black) and 256(white)")
	sizePtr := flag.Int("size", 500, "minimum size of a detection")
	streamPtr := flag.String("stream", "color", "type of url stream")
	flag.Parse()
	logger := golog.NewLogger("simple_detection")
	if *imgPtr == "" && *urlPtr == "" {
		logger.Fatal("must either have a -img argument or -url argument for the image source")
	}
	if *imgPtr != "" && *urlPtr != "" {
		logger.Fatal("cannot have both a path argument and a url argument for image source, must choose one")
	}
	if *imgPtr != "" {
		src := &camera.ImageSource{ImageSource: &simpleSource{*imgPtr}}
		pipeline(src, *threshPtr, *sizePtr, logger)
	} else {
		u, err := url.Parse(*urlPtr)
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("url parse: %v", u)
		host, port, err := net.SplitHostPort(u.Host)
		if err != nil {
			logger.Fatal(err)
		}
		args := u.Path + "?" + u.RawQuery
		logger.Infof("host: %s, port: %s, args: %s", host, port, args)
		portNum, err := strconv.ParseInt(port, 0, 32)
		if err != nil {
			logger.Fatal(err)
		}
		cfg := &rimage.AttrConfig{Host: host, Port: int(portNum), Args: args, Stream: *streamPtr, Aligned: false}
		src, err := imagesource.NewServerSource(cfg, logger)
		if err != nil {
			logger.Fatal(err)
		}
		pipeline(src, *threshPtr, *sizePtr, logger)
	}
	logger.Info("Done")
	os.Exit(0)
}

func pipeline(src gostream.ImageSource, thresh float64, size int, logger golog.Logger) {
	// create preprocessor
	p, err := objectdetection.RemoveColorChannel("b")
	if err != nil {
		logger.Fatal(err)
	}
	// create detector
	d := objectdetection.NewSimpleDetector(thresh)
	// create filter
	f := objectdetection.NewAreaFilter(size)

	// make a pipeline
	pipe, err := objectdetection.NewSource(src, p, d, f)
	if err != nil {
		logger.Fatal(err)
	}
	// run forever
	for {
		result, err := pipe.NextResult(context.Background())
		if err != nil {
			logger.Fatal(err)
		} else {
			logger.Info("Next detection:")
		}
		for i, bb := range result.Detections {
			box := bb.BoundingBox()
			logger.Infof("detection %d: upperLeft(%d, %d), lowerRight(%d,%d)", i, box.Min.X, box.Min.Y, box.Max.X, box.Max.Y)
		}
		ovImg, err := objectdetection.Overlay(result.OriginalImage, result.Detections)
		if err != nil {
			logger.Fatal(err)
		}
		err = rimage.WriteImageToFile("./simple_detection.jpg", ovImg)
		if err != nil {
			logger.Fatal(err)
		}
	}
}

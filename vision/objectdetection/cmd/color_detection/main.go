package main

import (
	"context"
	"flag"
	"image"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

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
	threshPtr := flag.Float64("thresh", 10, "the tolerance around the selected color")
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

func pipeline(src gostream.ImageSource, tol float64, size int, logger golog.Logger) {
	// create preprocessor
	p, err := objectdetection.RemoveColorChannel("b")
	if err != nil {
		logger.Fatal(err)
	}
	// create detector
	col := rimage.NewColor(79, 56, 21)
	hue, _, _ := col.HsvNormal()
	d, err := objectdetection.NewColorDetector(tol, hue)
	if err != nil {
		logger.Fatal(err)
	}
	// create filter
	f := objectdetection.NewAreaFilter(size)

	// make a pipeline
	pipe, err := objectdetection.NewSource(src, p, d, f)
	if err != nil {
		logger.Fatal(err)
	}
	defer pipe.Close()
	// run forever
	for {
		start := time.Now()
		result, err := pipe.NextResult(context.Background())
		if err != nil {
			logger.Fatal(err)
		}
		duration := time.Since(start)
		for i, bb := range result.Detections {
			box := bb.BoundingBox()
			logger.Infof("detection %d: upperLeft(%d, %d), lowerRight(%d,%d)", i, box.Min.X, box.Min.Y, box.Max.X, box.Max.Y)
		}
		logger.Infof("FPS: %.2f", 1./duration.Seconds())
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

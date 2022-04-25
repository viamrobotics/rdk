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
	threshPtr := flag.Float64("thresh", .1, "the tolerance around the selected color")
	sizePtr := flag.Int("size", 500, "minimum size of a detection")
	streamPtr := flag.String("stream", "color", "type of url stream")
	colorPtr := flag.String("color", "#416C1C", "color as a hex string")
	flag.Parse()
	logger := golog.NewLogger("simple_detection")
	if *imgPtr == "" && *urlPtr == "" {
		logger.Fatal("must either have a -img argument or -url argument for the image source")
	}
	if *imgPtr != "" && *urlPtr != "" {
		logger.Fatal("cannot have both a path argument and a url argument for image source, must choose one")
	}
	if *imgPtr != "" {
		src := &simpleSource{*imgPtr}
		cam, err := camera.New(src, nil, nil)
		if err != nil {
			logger.Fatal(err)
		}
		pipeline(cam, *threshPtr, *sizePtr, *colorPtr, logger)
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
		cfg := &imagesource.ServerAttrs{
			Host:    host,
			Port:    int(portNum),
			Args:    args,
			Aligned: false,
			AttrConfig: &camera.AttrConfig{
				Stream: *streamPtr,
			},
		}
		src, err := imagesource.NewServerSource(cfg, logger)
		if err != nil {
			logger.Fatal(err)
		}
		pipeline(src, *threshPtr, *sizePtr, *colorPtr, logger)
	}
	logger.Info("Done")
	os.Exit(0)
}

func pipeline(src gostream.ImageSource, tol float64, size int, colorString string, logger golog.Logger) {
	detCfg := &objectdetection.ColorDetectorConfig{
		SegmentSize:       size,
		Tolerance:         tol,
		DetectColorString: colorString,
	}
	// create detector
	det, err := objectdetection.NewColorDetector(detCfg)
	if err != nil {
		logger.Fatal(err)
	}
	pipe, err := objectdetection.NewSource(src, det)
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

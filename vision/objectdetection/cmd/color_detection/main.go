// Package main is a color detection tool.
package main

import (
	"context"
	"flag"
	"image"
	"os"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/objectdetection"
)

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageFromFile(s.filePath)
	return img, func() {}, err
}

func main() {
	imgPtr := flag.String("img", "", "path to image to apply simple detection to")
	urlPtr := flag.String("url", "", "url to image source to apply simple detection to")
	threshPtr := flag.Float64("thresh", .1, "the hue tolerance around the selected color")
	satPtr := flag.Float64("sat", .1, "the minimum saturation cutoff")
	valPtr := flag.Float64("val", .1, "the minimum value cutoff")
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
		cam, err := camera.NewFromReader(context.Background(), src, nil, camera.UnspecifiedStream)
		if err != nil {
			logger.Fatal(err)
		}
		pipeline(cam, *threshPtr, *satPtr, *valPtr, *sizePtr, *colorPtr, logger)
	} else {
		cfg := &videosource.ServerAttrs{
			URL:    *urlPtr,
			Stream: *streamPtr,
		}
		src, err := videosource.NewServerSource(context.Background(), cfg, logger)
		if err != nil {
			logger.Fatal(err)
		}
		pipeline(src, *threshPtr, *satPtr, *valPtr, *sizePtr, *colorPtr, logger)
	}
	logger.Info("Done")
	os.Exit(0)
}

func pipeline(src gostream.VideoSource, tol, sat, val float64, size int, colorString string, logger golog.Logger) {
	detCfg := &objectdetection.ColorDetectorConfig{
		SegmentSize:       size,
		HueTolerance:      tol,
		SaturationCutoff:  sat,
		ValueCutoff:       val,
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

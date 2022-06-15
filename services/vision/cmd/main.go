package main

import (
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
)

func main() {
	logger := golog.NewLogger("test")

	modelLoc := "data/effdet0.tflite"
	picLoc := "data/dogscute.jpeg"

	attrs := config.AttributeMap{
		"model_path":  modelLoc,
		"label_path":  "",
		"num_threads": 1,
	}

	cfg := vision.DetectorConfig{Name: "testdetector", Type: "tflite", Parameters: attrs}
	detector, err := vision.NewTfliteDetector(&cfg, logger)
	if err != nil {
		logger.Error(err)
	}

	img, err := rimage.NewImageFromFile(picLoc)
	if err != nil {
		logger.Error(err)
	}

	detections, err := detector(img)
	if err != nil {
		logger.Error(err)
	}

	for i := 0; i < 5; i++ {
		fmt.Println(detections[i]) // nolint: forbidigo
	}
}

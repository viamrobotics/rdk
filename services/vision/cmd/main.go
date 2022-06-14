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

	/*
		Make a DetectorConfig with the params =
		type TfliteDetectorConfig struct {
		//this should come from the attributes part of the detector config
		ModelPath  string  `json:"model_path"`
		NumThreads *int    `json:"num_threads"`
		LabelPath  *string `json:"label_path"`
		ServiceURL *string `json:"service_url"`
		}

	*/

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
		fmt.Println("error1")
		fmt.Println(err)
	}

	img, err := rimage.NewImageFromFile(picLoc)
	if err != nil {
		fmt.Println("error2")
		fmt.Println(err)
	} // the image is totally working...

	detections, err := detector(img)
	if err != nil {
		fmt.Println("error3")
		fmt.Println(err)
	}

	//fmt.Println(detections)

	for i := 0; i < 5; i++ {
		fmt.Println(detections[i])
	}
}

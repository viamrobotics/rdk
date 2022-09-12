package main

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
)

func main() {
	modelLoc := "kjdata/eliotmodel2.tflite"
	picLoc := "kjdata/stop.jpeg"
	ctx := context.Background()
	logger := golog.NewLogger("blah")
	conf := vision.VisModelConfig{
		Name: "a_detector",
		Type: "tflite_detector",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "kjdata/coco17.txt",
			"num_threads": 2,
		},
	}

	pic, err := rimage.NewImageFromFile(picLoc)
	if err != nil {
		fmt.Println(err)
	}

	det, model, err := vision.NewTFLiteDetector(ctx, &conf, logger)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("THE MODEL INFO:")
	fmt.Printf("Model input types %v\n", model.Info.InputTensorType)
	fmt.Printf("Model output types %v\n", model.Info.OutputTensorTypes)

	detections, err := det(ctx, pic)
	if err != nil {
		fmt.Println(err)
	}
	for _, d := range detections[0:5] {
		fmt.Println(d)
	}
}

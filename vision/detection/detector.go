package detection

import (
	"image"
)

type Detector interface {
	Inference(image.Image) ([]Detection, error)
}

type Detection struct {
	image.Rectangle
	Score float64
}

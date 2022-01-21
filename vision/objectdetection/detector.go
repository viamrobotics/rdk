package objectdetection

import (
	"image"
)

type Detector interface {
	Inference(image.Image) ([]*Detection, error)
}

type Detection struct {
	BoundingBox image.Rectangle
	Score       float64
}

func (d *Detection) Area() int {
	return d.BoundingBox.Dx() * d.BoundingBox.Dy()
}

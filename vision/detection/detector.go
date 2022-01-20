package detection

import (
	"image"

	"github.com/edaniels/gostream"
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

func StreamDetections(source gostream.ImageSource, d Detector) {
}

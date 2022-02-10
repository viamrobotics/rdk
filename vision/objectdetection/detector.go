// Package objectdetection defines a functional way to create object detection pipelines by feeding in
// images from a gostream.ImageSource source.
package objectdetection

import (
	"image"

	"github.com/pkg/errors"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision"
)

// Detector returns a slice of object detections from an input image.
type Detector func(image.Image) ([]Detection, error)

// Detection returns a bounding box around the object and a confidence score of the detection.
type Detection interface {
	BoundingBox() *image.Rectangle
	Score() float64
}

// detection2D is a simple struct for storing 2D detections.
type detection2D struct {
	boundingBox image.Rectangle
	score       float64
}

// BoundingBox returns a bounding box around the detected object.
func (d *detection2D) BoundingBox() *image.Rectangle {
	return &d.boundingBox
}

// Score returns a confidence score of the detection between 0.0 and 1.0.
func (d *detection2D) Score() float64 {
	return d.score
}

// ToObjects projects the detections to 3D using the camera's Projector
func ToObjects(dets []Detection, img *rimage.ImageWithDepth, proj rimage.Projector) ([]*vision.Object, error) {
	if proj == nil {
		return nil, errors.New("objectdetection: cannot have nil Projector when projecting objects to 3D")
	}
	objects := make([]*vision.Object, len(dets))
	for i, d := range dets {
		bb := d.BoundingBox()
		pc, err := proj.ImageWithDepthToPointCloud(img, bb)
		if err != nil {
			return nil, err
		}
		objects[i] = vision.NewObject(pc)
	}
	return objects, nil
}

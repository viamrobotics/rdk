// Package objectdetection defines a functional way to create object detection pipelines by feeding in
// images from a gostream.ImageSource source.
package objectdetection

import (
	"image"
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

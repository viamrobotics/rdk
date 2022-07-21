// Package objectdetection defines a functional way to create object detection pipelines by feeding in
// images from a gostream.ImageSource source.
package objectdetection

import (
	"context"
	"fmt"
	"image"

	"github.com/pkg/errors"
)

// Detector returns a slice of object detections from an input image.
type Detector func(context.Context, image.Image) ([]Detection, error)

// Build zips up a preprocessor-detector-postprocessor stream into a detector.
func Build(prep Preprocessor, det Detector, post Postprocessor) (Detector, error) {
	if det == nil {
		return nil, errors.New("must have a Detector to build a detection pipeline")
	}
	if prep == nil {
		prep = func(img image.Image) image.Image { return img }
	}
	if post == nil {
		post = func(inp []Detection) []Detection { return inp }
	}
	return func(ctx context.Context, img image.Image) ([]Detection, error) {
		preprocessed := prep(img)
		detections, err := det(ctx, preprocessed)
		if err != nil {
			return nil, err
		}
		return post(detections), nil
	}, nil
}

// Detection returns a bounding box around the object and a confidence score of the detection.
type Detection interface {
	BoundingBox() *image.Rectangle
	Score() float64
	Label() string
}

// NewDetection creates a simple 2D detection.
func NewDetection(boundingBox image.Rectangle, score float64, label string) Detection {
	return &detection2D{boundingBox, score, label}
}

// detection2D is a simple struct for storing 2D detections.
type detection2D struct {
	boundingBox image.Rectangle
	score       float64
	label       string
}

// BoundingBox returns a bounding box around the detected object.
func (d *detection2D) BoundingBox() *image.Rectangle {
	return &d.boundingBox
}

// Score returns a confidence score of the detection between 0.0 and 1.0.
func (d *detection2D) Score() float64 {
	return d.score
}

// Label returns the class label of the object in the bounding box.
func (d *detection2D) Label() string {
	return d.label
}

// String turns the detection into a string.
func (d *detection2D) String() string {
	return fmt.Sprintf("Label: %s, Score: %.2f, Box: %v", d.label, d.score, d.boundingBox)
}

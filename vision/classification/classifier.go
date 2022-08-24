package classification

import (
	"context"
	"errors"
	"fmt"
	"image"
)

// Detection returns a bounding box around the object and a confidence score of the detection.
type Classification interface {
	Score() float64
	Label() string
}

type Classifications []Classification

func (cc Classifications) Top1() (Classification, error) {
	if len(cc) < 1 {
		return nil, errors.New("")
	}
	var maxScore float64
	var maxLabel string
	for _, c := range cc {
		if c.Score() > maxScore {
			maxScore = c.Score()
			maxLabel = c.Label()
		}
	}
	return NewClassification(maxScore, maxLabel), nil
}

type Classifier func(context.Context, image.Image) (Classifications, error)

// NewDetection creates a simple 2D detection.
func NewClassification(score float64, label string) Classification {
	return &classification2D{score, label}
}

// detection2D is a simple struct for storing 2D detections.
type classification2D struct {
	score float64
	label string
}

// Score returns a confidence score of the detection between 0.0 and 1.0.
func (c *classification2D) Score() float64 {
	return c.score
}

// Label returns the class label of the object in the bounding box.
func (c *classification2D) Label() string {
	return c.label
}

// String turns the detection into a string.
func (c *classification2D) String() string {
	return fmt.Sprintf("Label: %s, Score: %.2f", c.label, c.score)
}

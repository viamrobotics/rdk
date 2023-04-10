// Package classification implements a classifier for use as a visModel in the vision service
package classification

import (
	"context"
	"fmt"
	"image"
	"sort"

	"github.com/pkg/errors"
)

// Classification returns a confidence score of the classification and a label of the class.
type Classification interface {
	Score() float64
	Label() string
}

// Classifications is a list of the Classification object.
type Classifications []Classification

// TopN finds the N Classifications with the highest confidence scores.
func (cc Classifications) TopN(n int) (Classifications, error) {
	if len(cc) < n {
		return nil, errors.Errorf("cannot produce top %v results from list of length %v", n, len(cc))
	}
	sort.Slice(cc, func(i, j int) bool { return cc[i].Score() > cc[j].Score() })
	return cc[0:n], nil
}

// A Classifier is defined as a function from an image to a list of Classifications.
type Classifier = func(context.Context, image.Image) (Classifications, error)

// NewClassification creates a simple 2D classification.
func NewClassification(score float64, label string) Classification {
	return &classification2D{score, label}
}

// classificationD is a simple struct for storing 2D classifications.
type classification2D struct {
	score float64
	label string
}

// Score returns a confidence score of the classification between 0.0 and 1.0.
func (c *classification2D) Score() float64 {
	return c.score
}

// Label returns the class label of the object in the bounding box.
func (c *classification2D) Label() string {
	return c.label
}

// String turns the classification into a string.
func (c *classification2D) String() string {
	return fmt.Sprintf("Label: %s, Score: %.2f", c.label, c.score)
}

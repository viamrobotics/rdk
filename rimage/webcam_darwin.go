package rimage

import (
	"fmt"

	"github.com/edaniels/gostream"
)

func NewWebcamSource(attrs map[string]string) (gostream.ImageSource, error) {
	return nil, fmt.Errorf("webcam not supported on darwin")
}

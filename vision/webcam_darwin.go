package vision

import (
	"fmt"
)

func NewWebcamSource(attrs map[string]string) (ImageDepthSource, error) {
	return nil, fmt.Errorf("webcam not supported on darwin")
}

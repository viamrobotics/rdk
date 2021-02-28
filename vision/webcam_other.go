// +build !linux

package vision

import (
	"errors"
	"fmt"
)

type webcamDepthSource struct {
}

func (w *webcamDepthSource) Next() (*DepthMap, error) {
	return nil, errors.New("not implemented")
}

func (w *webcamDepthSource) Close() error {
	return nil
}

func findWebcamDepth(debug bool) (*webcamDepthSource, error) {
	return nil, fmt.Errorf("no depth camera found (not implemented)")
}

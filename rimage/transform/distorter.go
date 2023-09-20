//go:build !no_cgo

package transform

import "github.com/pkg/errors"

// DistortionType is the name of the distortion model.
type DistortionType string

const (
	// BrownConradyDistortionType is for simple lenses of narrow field easily modeled as a pinhole camera.
	BrownConradyDistortionType = DistortionType("brown_conrady")
	// KannalaBrandtDistortionType is for wide-angle and fisheye lense distortion.
	KannalaBrandtDistortionType = DistortionType("kannala_brandt")
)

// Distorter defines a Transform that takes an undistorted image and distorts it according to the model.
type Distorter interface {
	ModelType() DistortionType
	CheckValid() error
	Parameters() []float64
	Transform(x, y float64) (float64, float64)
}

// InvalidDistortionError is used when the distortion_parameters are invalid.
func InvalidDistortionError(msg string) error {
	return errors.Wrapf(errors.New("invalid distortion_parameters"), msg)
}

// NewDistorter returns a Distorter given a valid DistortionType and its parameters.
func NewDistorter(distortionType DistortionType, parameters []float64) (Distorter, error) {
	switch distortionType { //nolint:exhaustive
	case BrownConradyDistortionType:
		return NewBrownConrady(parameters)
	default:
		return nil, errors.Errorf("do not know how to parse %q distortion model", distortionType)
	}
}

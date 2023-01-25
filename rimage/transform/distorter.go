package transform

import "github.com/pkg/errors"

// DistortionType is the name of the distortion model.
type DistortionType string

const (
	// NoneDistortionType applies no distortion to an input image. Essentially an identity transform.
	NoneDistortionType = DistortionType("no_distortion")
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
	case NoneDistortionType, DistortionType(""):
		return &NoDistortion{}, nil
	default:
		return nil, errors.Errorf("do no know how to parse %q distortion model", distortionType)
	}
}

// NoDistortion applies no Distortion to the camera.
type NoDistortion struct{}

// CheckValid returns an error if invalid.
func (nd *NoDistortion) CheckValid() error { return nil }

// ModelType returns the name of the model.
func (nd *NoDistortion) ModelType() DistortionType { return NoneDistortionType }

// Parameters returns nothing, because there is no distortion.
func (nd *NoDistortion) Parameters() []float64 { return []float64{} }

// Transform is the identity transform.
func (nd *NoDistortion) Transform(x, y float64) (float64, float64) { return x, y }

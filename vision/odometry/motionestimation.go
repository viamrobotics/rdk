package odometry

import (
	"go.viam.com/rdk/rimage"
	"gonum.org/v1/gonum/mat"
)

type Motion3D struct {
	Rotation    *mat.Dense
	Translation *mat.Dense
}

// NewMotion3DFromRotationTranslation returns a new pointer to Motion3D from a rotation and a translation matrix
func NewMotion3DFromRotationTranslation(rotation, translation *mat.Dense) *Motion3D {
	return &Motion3D{
		Rotation:    rotation,
		Translation: translation,
	}
}

// EstimateMotionFrom2Frames estimates the 3D motion of the camera between frame img1 and frame img2
func EstimateMotionFrom2Frames(img1, img2 rimage.Image) (*Motion3D, error) {
	return nil, nil
}

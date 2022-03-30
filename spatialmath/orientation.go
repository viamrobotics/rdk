package spatialmath

import (
	"github.com/pkg/errors"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"gonum.org/v1/gonum/num/quat"
)

// Orientation is an interface used to express the different parameterizations of the orientation
// of a rigid object or a frame of reference in 3D Euclidean space.
type Orientation interface {
	OrientationVectorRadians() *OrientationVector
	OrientationVectorDegrees() *OrientationVectorDegrees
	AxisAngles() *R4AA
	Quaternion() quat.Number
	EulerAngles() *EulerAngles
	RotationMatrix() *RotationMatrix
}

// NewZeroOrientation returns an orientatation which signifies no rotation.
func NewZeroOrientation() Orientation {
	return &quaternion{1, 0, 0, 0}
}

// OrientationAlmostEqual will return a bool describing whether 2 poses have approximately the same orientation.
func OrientationAlmostEqual(o1, o2 Orientation) bool {
	return QuatToR3AA(OrientationBetween(o1, o2).Quaternion()).Norm2() < 1e-4
}

// OrientationBetween returns the orientation representing the difference between the two given orientations.
func OrientationBetween(o1, o2 Orientation) Orientation {
	q := quaternion(quat.Mul(o2.Quaternion(), quat.Conj(o1.Quaternion())))
	return &q
}

// OrientationInverse returns the orientation representing the inverse of the given orientation.
func OrientationInverse(o Orientation) Orientation {
	q := quaternion(quat.Inv(o.Quaternion()))
	return &q
}

func NewOrientationFromProtobuf(orientMsg *commonpb.Orientation) (Orientation, error) {
	switch rep := orientMsg.Value.(type) {
	case *commonpb.Orientation_Quaternion:
		return NewQuaternionFromProtobuf(rep.Quaternion), nil
	case *commonpb.Orientation_AxisAngle:
		return NewAxisAngleFromProtobuf(rep.AxisAngle), nil
	case *commonpb.Orientation_EulerAngles:
		return NewEulerAnglesFromProtobuf(rep.EulerAngles), nil
	case *commonpb.Orientation_OrientationVector:
		return NewOrientationVectorFromProtobuf(rep.OrientationVector), nil
	default:
		return nil, errors.Errorf("Orientation message value has unexpected type %T", rep)
	}
}

func OrientationToProtobuf(orient Orientation) *commonpb.Orientation {
	asQuat := orient.Quaternion()
	quatMsg := &commonpb.Quaternion{
		Real: asQuat.Real,
		I:    asQuat.Imag,
		J:    asQuat.Jmag,
		K:    asQuat.Kmag,
	}
	return &commonpb.Orientation{
		Value: &commonpb.Orientation_Quaternion{
			Quaternion: quatMsg,
		},
	}
}

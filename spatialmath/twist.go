package spatialmath

import (
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pTpb "go.viam.com/rdk/proto/api/component/posetracker/v1"
)

type Twist interface {
	TranslationalVelocity() r3.Vector
	AngularVelocity() Orientation
}

func NewZeroTwist() Twist {
	return newZeroDualQuaternion()
}

func NewTwist(transVelocity r3.Vector, angularVelocity Orientation) Twist {
	return newDualQuaternion(transVelocity, angularVelocity)
}

func TwistToProtobuf(tw Twist) *pTpb.RigidBodyTwist {
	angularVel := tw.AngularVelocity()
	avMsg := OrientationToProtobuf(angularVel)
	transVel := tw.TranslationalVelocity()
	tvMsg := &commonpb.Vector3{
		X: transVel.X,
		Y: transVel.Y,
		Z: transVel.Z,
	}
	return &pTpb.RigidBodyTwist{
		TranslationalVelocity: tvMsg,
		AngularVelocity:       avMsg,
	}
}

func NewTwistFromProtobuf(twMsg *pTpb.RigidBodyTwist) (Twist, error) {
	avMsg := twMsg.AngularVelocity
	angularVel, err := NewOrientationFromProtobuf(avMsg)
	if err != nil {
		return nil, err
	}
	twist := newDualQuaternionFromRotation(angularVel)
	transVel := r3.Vector{
		X: twMsg.TranslationalVelocity.X,
		Y: twMsg.TranslationalVelocity.Y,
		Z: twMsg.TranslationalVelocity.Z,
	}
	twist.SetTranslation(transVel)
	return twist, nil
}

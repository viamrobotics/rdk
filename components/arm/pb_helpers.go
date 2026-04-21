package arm

import (
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/utils"
)

// MoveOptions define parameters to be obeyed during arm movement.
type MoveOptions struct {
	MaxVelRads, MaxAccRads float64
	// MaxVelRadsJoints contains per-joint maximum velocity in radians per second.
	MaxVelRadsJoints []float64
	// MaxAccRadsJoints contains per-joint maximum acceleration in radians per second squared.
	MaxAccRadsJoints []float64
}

func moveOptionsFromProtobuf(protobuf *pb.MoveOptions) *MoveOptions {
	if protobuf == nil {
		return nil
	}

	var vel, acc float64
	// the proto indicates MaxVelDegsPerSec and MaxAccDegsPerSec2 are ignored when either/both MaxVelDegsPerSecJoints
	// MaxAccDegsPerSec2Joints are set. We deliberately don't do it here as this is a translation layer
	// it's up the implementer to return such error
	if protobuf.MaxVelDegsPerSec != nil {
		vel = *protobuf.MaxVelDegsPerSec
	}
	if protobuf.MaxAccDegsPerSec2 != nil {
		acc = *protobuf.MaxAccDegsPerSec2
	}
	opts := &MoveOptions{
		MaxVelRads: utils.DegToRad(vel),
		MaxAccRads: utils.DegToRad(acc),
	}
	if len(protobuf.MaxVelDegsPerSecJoints) > 0 {
		opts.MaxVelRadsJoints = make([]float64, len(protobuf.MaxVelDegsPerSecJoints))
		for i, v := range protobuf.MaxVelDegsPerSecJoints {
			opts.MaxVelRadsJoints[i] = utils.DegToRad(v)
		}
	}
	if len(protobuf.MaxAccDegsPerSec2Joints) > 0 {
		opts.MaxAccRadsJoints = make([]float64, len(protobuf.MaxAccDegsPerSec2Joints))
		for i, v := range protobuf.MaxAccDegsPerSec2Joints {
			opts.MaxAccRadsJoints[i] = utils.DegToRad(v)
		}
	}
	return opts
}

func (opts *MoveOptions) toProtobuf() *pb.MoveOptions {
	vel := utils.RadToDeg(opts.MaxVelRads)
	acc := utils.RadToDeg(opts.MaxAccRads)
	pbOpts := &pb.MoveOptions{
		MaxVelDegsPerSec:  &vel,
		MaxAccDegsPerSec2: &acc,
	}
	if len(opts.MaxVelRadsJoints) > 0 {
		pbOpts.MaxVelDegsPerSecJoints = make([]float64, len(opts.MaxVelRadsJoints))
		for i, v := range opts.MaxVelRadsJoints {
			pbOpts.MaxVelDegsPerSecJoints[i] = utils.RadToDeg(v)
		}
	}
	if len(opts.MaxAccRadsJoints) > 0 {
		pbOpts.MaxAccDegsPerSec2Joints = make([]float64, len(opts.MaxAccRadsJoints))
		for i, v := range opts.MaxAccRadsJoints {
			pbOpts.MaxAccDegsPerSec2Joints[i] = utils.RadToDeg(v)
		}
	}
	return pbOpts
}

package arm

import (
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/utils"
)

// MoveOptions define parameters to be obeyed during arm movement.
type MoveOptions struct {
	MaxVelRads, MaxAccRads float64
}

func moveOptionsFromProtobuf(protobuf *pb.MoveOptions) *MoveOptions {
	if protobuf == nil {
		return nil
	}

	var vel, acc float64
	if protobuf.MaxVelDegsPerSec != nil {
		vel = *protobuf.MaxVelDegsPerSec
	}
	if protobuf.MaxAccDegsPerSec2 != nil {
		acc = *protobuf.MaxAccDegsPerSec2
	}
	return &MoveOptions{
		MaxVelRads: utils.DegToRad(vel),
		MaxAccRads: utils.DegToRad(acc),
	}
}

func (opts *MoveOptions) toProtobuf() *pb.MoveOptions {
	vel := utils.RadToDeg(opts.MaxVelRads)
	acc := utils.RadToDeg(opts.MaxAccRads)
	return &pb.MoveOptions{
		MaxVelDegsPerSec:  &vel,
		MaxAccDegsPerSec2: &acc,
	}
}

package arm

import (
	pb "go.viam.com/api/component/arm/v1"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// MoveOptions define parameters to be obeyed during arm movement.
type MoveOptions struct {
	MaxVelRads, MaxAccRads float64
	// MaxVelRadsJoints contains per-joint maximum velocity in radians per second.
	MaxVelRadsJoints []float64
	// MaxAccRadsJoints contains per-joint maximum acceleration in radians per second squared.
	MaxAccRadsJoints []float64
	// MaxTCPSpeedMPerSec is the maximum allowable speed of the tool center point, in meters per second.
	// Zero means unset.
	MaxTCPSpeedMPerSec *float64
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
	opts.MaxTCPSpeedMPerSec = protobuf.MaxTcpSpeed
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
	if opts.MaxTCPSpeedMPerSec != nil {
		mps := opts.MaxTCPSpeedMPerSec
		pbOpts.MaxTcpSpeed = mps
	}
	return pbOpts
}

// trajectoryPointToProto converts an in-memory TrajectoryPoint to its proto form.
// `model` is used to convert referenceframe.Input back into the wire JointPositions
// (degrees-or-radians-by-joint encoding). When `model` is nil the conversion falls
// back to treating all joints as revolute, matching the unary MoveThroughJointPositions
// path on the client side.
func trajectoryPointToProto(model referenceframe.Model, p TrajectoryPoint) (*pb.TrajectoryPoint, error) {
	jp, err := referenceframe.JointPositionsFromInputs(model, p.Positions)
	if err != nil {
		return nil, err
	}
	out := &pb.TrajectoryPoint{
		Time:      durationpb.New(p.Time),
		Positions: jp,
	}
	if p.Constraints != nil {
		out.Constraints = kinematicConstraintsToProto(p.Constraints)
	}
	return out, nil
}

// trajectoryPointFromProto converts a wire TrajectoryPoint into an in-memory TrajectoryPoint.
// `model` is used to interpret JointPositions on the way back; when nil the conversion falls
// back to revolute-radians.
func trajectoryPointFromProto(model referenceframe.Model, p *pb.TrajectoryPoint) (TrajectoryPoint, error) {
	inputs, err := referenceframe.InputsFromJointPositions(model, p.GetPositions())
	if err != nil {
		return TrajectoryPoint{}, err
	}
	out := TrajectoryPoint{
		Time:      p.GetTime().AsDuration(),
		Positions: inputs,
	}
	if c := p.GetConstraints(); c != nil {
		out.Constraints = kinematicConstraintsFromProto(c)
	}
	return out, nil
}

func kinematicConstraintsToProto(c *KinematicConstraints) *pb.TrajectoryPoint_KinematicConstraints {
	out := &pb.TrajectoryPoint_KinematicConstraints{
		Velocities: &pb.JointVelocities{Values: c.Velocities},
	}
	if c.Accelerations != nil {
		out.Accelerations = &pb.JointAccelerations{Values: c.Accelerations}
	}
	return out
}

func kinematicConstraintsFromProto(c *pb.TrajectoryPoint_KinematicConstraints) *KinematicConstraints {
	out := &KinematicConstraints{
		Velocities: c.GetVelocities().GetValues(),
	}
	if a := c.GetAccelerations(); a != nil {
		out.Accelerations = a.GetValues()
	}
	return out
}

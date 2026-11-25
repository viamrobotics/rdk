package motion

import (
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

// ServoMode is a StreamServo session's command kind, fixed at Init.
type ServoMode int

// The servo modes.
const (
	ServoModeUnspecified ServoMode = iota
	ServoModePose
	ServoModeVelocity
)

// ServoState mirrors pb.ServoState.
type ServoState int

// The servo states.
const (
	ServoStateUnspecified ServoState = iota
	ServoStateRunning
	ServoStateConverged
	ServoStateStalled
	ServoStateStopped
	ServoStateStoppedTimeout
	ServoStateFaulted
)

// ServoLimit mirrors pb.ServoLimit.
type ServoLimit int

// The servo limits.
const (
	ServoLimitUnspecified ServoLimit = iota
	ServoLimitJointPosition
	ServoLimitJointSpeed
	ServoLimitTCPSpeed
	ServoLimitSingularity
)

// ServoConfig is StreamServo's Init. Nil pointers take the implementation's defaults.
type ServoConfig struct {
	Mode                    ServoMode
	Limits                  *arm.MoveOptions
	MaxAngularVelDegsPerSec *float64
	TimeoutMs               *uint32
	MaxTargetStepMM         *float64
	MaxTargetStepDeg        *float64
}

// ServoTwist is a Cartesian velocity in ReferenceFrame ("" is world).
type ServoTwist struct {
	LinearMMPerSec    r3.Vector
	AngularDegsPerSec r3.Vector
	ReferenceFrame    string
}

// ServoUpdate is one command. Exactly one of Target and Velocity is set; FeedForward only with Target.
type ServoUpdate struct {
	Target      *referenceframe.PoseInFrame
	FeedForward *ServoTwist
	Velocity    *ServoTwist
}

// ServoStarted is the session's first response: the effective limits.
type ServoStarted struct {
	Limits                  arm.MoveOptions
	MaxAngularVelDegsPerSec float64
	TimeoutMs               uint32
	RateHz                  float64
}

// ServoStatus acknowledges an update (Ack set) or reports a state change (Ack nil).
type ServoStatus struct {
	State     ServoState
	ErrMM     float64
	ErrDeg    float64
	Ack       *uint64
	Rejected  *string
	LimitedBy []ServoLimit
}

// ServoResponse is one message to the client: exactly one field is set.
type ServoResponse struct {
	Started *ServoStarted
	Status  *ServoStatus
}

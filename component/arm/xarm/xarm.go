// Package xarm implements some xArms.
package xarm

import (
	"context"
	// for embedding model file.
	_ "embed"
	"errors"
	"net"
	"runtime"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Host string `json:"host"`
}

type xArm struct {
	generic.Unimplemented
	dof      int
	tid      uint16
	conn     net.Conn
	speed    float32 // speed=max joint radians per second
	accel    float32 // acceleration=500*π/180rad/s^2
	moveHZ   float64 // Number of joint positions to send per second
	moveLock *sync.Mutex
	mp       motionplan.MotionPlanner
	model    referenceframe.Model
	started  bool
	opMgr    operation.SingleOperationManager
	robot    robot.Robot
}

//go:embed xarm6_kinematics.json
var xArm6modeljson []byte

//go:embed xarm7_kinematics.json
var xArm7modeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "xArm6", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewxArm(ctx, r, config, logger, 6)
		},
	})
	registry.RegisterComponent(arm.Subtype, "xArm7", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewxArm(ctx, r, config, logger, 7)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.SubtypeName, "xArm6",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})

	config.RegisterComponentAttributeMapConverter(arm.SubtypeName, "xArm7",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

// XArmModel returns the kinematics model of the xArm, also has all Frame information.
func XArmModel(dof int) (referenceframe.Model, error) {
	if dof == 6 {
		return referenceframe.UnmarshalModelJSON(xArm6modeljson, "")
	} else if dof == 7 {
		return referenceframe.UnmarshalModelJSON(xArm7modeljson, "")
	}
	return nil, errors.New("no kinematics model for xarm with specified degrees of freedom")
}

// NewxArm returns a new xArm with the specified dof.
func NewxArm(ctx context.Context, r robot.Robot, cfg config.Component, logger golog.Logger, dof int) (arm.LocalArm, error) {
	host := cfg.ConvertedAttributes.(*AttrConfig).Host
	conn, err := net.Dial("tcp", host+":502")
	if err != nil {
		return nil, err
	}
	model, err := XArmModel(dof)
	if err != nil {
		return nil, err
	}
	nCPU := runtime.NumCPU()
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, nCPU, logger)
	if err != nil {
		return nil, err
	}

	mutex := &sync.Mutex{}
	// Start with default speed/acceleration parameters
	// TODO(pl): add settable speed
	xA := xArm{
		dof:      dof,
		tid:      0,
		conn:     conn,
		speed:    0.3,
		accel:    8.7,
		moveHZ:   100.,
		moveLock: mutex,
		mp:       mp,
		model:    model,
		started:  false,
		robot:    r,
	}

	err = xA.start(ctx)
	if err != nil {
		return nil, err
	}

	return &xA, nil
}

func (x *xArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := x.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

func (x *xArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return x.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

// ModelFrame returns the dynamic frame of the model.
func (x *xArm) ModelFrame() referenceframe.Model {
	return x.model
}

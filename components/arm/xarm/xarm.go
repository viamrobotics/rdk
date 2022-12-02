// Package xarm implements some xArms.
package xarm

import (
	"context"
	// for embedding model file.
	_ "embed"
	"errors"
	"math"
	"net"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Host         string  `json:"host"`
	Speed        float32 `json:"speed_degs_per_sec"`
	Acceleration float32 `json:"acceleration_degs_per_sec_per_sec"`
}

const (
	defaultSpeed        = 20
	defaultAcceleration = 50
)

type xArm struct {
	generic.Unimplemented
	dof      int
	tid      uint16
	conn     net.Conn
	speed    float32 // speed=max joint radians per second
	accel    float32 // acceleration=rad/s^2
	moveHZ   float64 // Number of joint positions to send per second
	moveLock sync.Mutex
	model    referenceframe.Model
	started  bool
	opMgr    operation.SingleOperationManager
	robot    robot.Robot
}

//go:embed xarm6_kinematics.json
var xArm6modeljson []byte

//go:embed xarm7_kinematics.json
var xArm7modeljson []byte

// ModelName6DOF is a function used to get the string used to refer to the xarm model of 6 dof.
var ModelName6DOF = resource.NewDefaultModel("xArm6")

// ModelName7DOF is a function used to get the string used to refer to the xarm model of 7 dof.
var ModelName7DOF = resource.NewDefaultModel("xArm7")

// Model returns the kinematics model of the xarm arm, also has all Frame information.
func Model(name string, dof int) (referenceframe.Model, error) {
	switch dof {
	case 6:
		return referenceframe.UnmarshalModelJSON(xArm6modeljson, name)
	case 7:
		return referenceframe.UnmarshalModelJSON(xArm7modeljson, name)
	default:
		return nil, errors.New("no kinematics model for xarm with specified degrees of freedom")
	}
}

func init() {
	// xArm6
	registry.RegisterComponent(arm.Subtype, ModelName6DOF, registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewxArm(ctx, r, config, logger, 6)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.Subtype, ModelName6DOF,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)

	// xArm7
	registry.RegisterComponent(arm.Subtype, ModelName7DOF, registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewxArm(ctx, r, config, logger, 7)
		},
	})
	config.RegisterComponentAttributeMapConverter(arm.Subtype, ModelName7DOF,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)
}

// NewxArm returns a new xArm with the specified dof.
func NewxArm(ctx context.Context, r robot.Robot, cfg config.Component, logger golog.Logger, dof int) (arm.LocalArm, error) {
	armCfg := cfg.ConvertedAttributes.(*AttrConfig)

	if armCfg.Host == "" {
		return nil, errors.New("xArm host not set")
	}

	speed := armCfg.Speed
	if speed == 0 {
		speed = defaultSpeed
	}

	acceleration := armCfg.Acceleration
	if acceleration == 0 {
		acceleration = defaultAcceleration
	}

	conn, err := net.Dial("tcp", armCfg.Host+":502")
	if err != nil {
		return nil, err
	}

	model, err := Model(cfg.Name, dof)
	if err != nil {
		return nil, err
	}

	xA := xArm{
		dof:     dof,
		tid:     0,
		conn:    conn,
		speed:   speed * math.Pi / 180,
		accel:   acceleration * math.Pi / 180,
		moveHZ:  100.,
		model:   model,
		started: false,
		robot:   r,
	}

	err = xA.start(ctx)
	if err != nil {
		return nil, err
	}

	return &xA, nil
}

func (x *xArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := x.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return x.model.InputFromProtobuf(res), nil
}

func (x *xArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return x.MoveToJointPositions(ctx, x.model.ProtobufFromInput(goal), nil)
}

// ModelFrame returns the dynamic frame of the model.
func (x *xArm) ModelFrame() referenceframe.Model {
	return x.model
}

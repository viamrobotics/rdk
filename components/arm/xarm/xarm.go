// Package xarm implements some xArms.
package xarm

import (
	"context"
	// for embedding model file.
	_ "embed"
	"errors"
	"math"
	"net"
	"runtime"
	"strconv"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Host         string  `json:"host"`
	Speed        float32 `json:"speed"`        // deg/s
	Acceleration float32 `json:"acceleration"` // deg/s/s
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

// ModelName is a function used to get the string used to refer to the xarm model of specified dof.
func ModelName(dof int) string {
	return "xArm" + strconv.Itoa(dof)
}

func xarmModel(dof int) (referenceframe.Model, error) {
	switch dof {
	case 6:
		return referenceframe.UnmarshalModelJSON(xArm6modeljson, "")
	case 7:
		return referenceframe.UnmarshalModelJSON(xArm7modeljson, "")
	default:
		return nil, errors.New("no kinematics model for xarm with specified degrees of freedom")
	}
}

func init() {
	registerXArm := func(dof int) {
		registry.RegisterComponent(arm.Subtype, ModelName(dof), registry.Component{
			RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
				return NewxArm(ctx, r, config, logger, dof)
			},
		})

		config.RegisterComponentAttributeMapConverter(arm.SubtypeName, ModelName(dof),
			func(attributes config.AttributeMap) (interface{}, error) {
				var conf AttrConfig
				return config.TransformAttributeMapToStruct(&conf, attributes)
			},
			&AttrConfig{},
		)
	}

	registerXArm(6)
	registerXArm(7)
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

	model, err := xarmModel(dof)
	if err != nil {
		return nil, err
	}

	nCPU := runtime.NumCPU()
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, nCPU, logger)
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
		mp:      mp,
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
	res, err := x.GetJointPositions(ctx, nil)
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

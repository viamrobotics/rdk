package xarm

import (
	"context"
	_ "embed" // for embedding model file
	"errors"
	"net"
	"runtime"
	"sync"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/referenceframe"

	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

type xArm struct {
	dof      int
	tid      uint16
	conn     net.Conn
	speed    float32 //speed=20*π/180rad/s
	accel    float32 //acceleration=500*π/180rad/s^2
	moveLock *sync.Mutex
	model    *referenceframe.Model
	ik       kinematics.InverseKinematics
}

//go:embed xArm6_kinematics.json
var xArm6modeljson []byte

//go:embed xArm7_kinematics.json
var xArm7modeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "xArm6", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewxArm(ctx, config, logger, 6)
		},
	})
	registry.RegisterComponent(arm.Subtype, "xArm7", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewxArm(ctx, config, logger, 7)
		},
	})
}

// XArmModel returns the kinematics model of the xArm, also has all Frame information.
func xArmModel(dof int) (*referenceframe.Model, error) {
	if dof == 6 {
		return referenceframe.ParseJSON(xArm6modeljson, "")
	} else if dof == 7 {
		return referenceframe.ParseJSON(xArm7modeljson, "")
	}
	return nil, errors.New("no kinematics model for xarm with specified degrees of freedom")
}

// NewxArm returns a new xArm with the specified dof
func NewxArm(ctx context.Context, cfg config.Component, logger golog.Logger, dof int) (arm.Arm, error) {
	host := cfg.Host
	conn, err := net.Dial("tcp", host+":502")
	if err != nil {
		return nil, err
	}
	model, err := xArmModel(dof)
	if err != nil {
		return nil, err
	}
	nCPU := runtime.NumCPU()
	ik, err := kinematics.CreateCombinedIKSolver(model, logger, nCPU)
	if err != nil {
		return nil, err
	}

	mutex := &sync.Mutex{}
	// Start with default speed/acceleration parameters
	// TODO(pl): add settable speed
	xA := xArm{dof, 0, conn, 0.35, 8.7, mutex, model, ik}

	err = xA.start()
	if err != nil {
		return nil, err
	}

	return &xA, nil
}

func (x *xArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := x.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

func (x *xArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return x.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

// ModelFrame returns the dynamic frame of the model
func (x *xArm) ModelFrame() *referenceframe.Model {
	return x.model
}

package xarm

import (
	"context"
	_ "embed" // for embedding model file
	"errors"
	"net"
	"runtime"
	"sync"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
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
	ik       kinematics.InverseKinematics
}

//go:embed xArm6_kinematics.json
var xArm6modeljson []byte

//go:embed xArm7_kinematics.json
var xArm7modeljson []byte

func init() {
	registry.RegisterArm("xArm6", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		return NewxArm(ctx, config.Host, logger, 6)
	})
	registry.RegisterArm("xArm7", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		return NewxArm(ctx, config.Host, logger, 7)
	})
}

// NewxArm returns a new xArm with an IK solver and the specified
func NewxArm(ctx context.Context, host string, logger golog.Logger, dof int) (arm.Arm, error) {
	conn, err := net.Dial("tcp", host+":502")
	if err != nil {
		return &xArm{}, err
	}
	var model *kinematics.Model
	if dof == 6 {
		model, err = kinematics.ParseJSON(xArm6modeljson)
		if err != nil {
			return &xArm{}, err
		}
	} else if dof == 7 {
		model, err = kinematics.ParseJSON(xArm7modeljson)
		if err != nil {
			return &xArm{}, err
		}
	} else {
		return &xArm{}, errors.New("no kinematics model for xarm with specified degrees of freedom")
	}
	nCPU := runtime.NumCPU()
	ik := kinematics.CreateCombinedIKSolver(model, logger, nCPU)

	mutex := &sync.Mutex{}
	// Start with default speed/acceleration parameters
	// TODO(pl): add settable speed
	xA := xArm{dof, 0, conn, 0.35, 8.7, mutex, ik}

	err = xA.start()
	if err != nil {
		return &xArm{}, err
	}

	return &xA, nil
}

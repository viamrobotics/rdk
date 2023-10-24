// Package xarm implements some xArms.
package xarm

import (
	"context"
	// for embedding model file.
	_ "embed"
	"fmt"
	"net"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Config is used for converting config attributes.
type Config struct {
	Host         string  `json:"host"`
	Port         int     `json:"port"`
	Speed        float32 `json:"speed_degs_per_sec"`
	Acceleration float32 `json:"acceleration_degs_per_sec_per_sec"`

	parsedPort string
}

// Validate validates the config.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Port == 0 {
		cfg.parsedPort = defaultPort
	} else {
		cfg.parsedPort = fmt.Sprintf("%d", cfg.Port)
	}
	return deps, nil
}

const (
	defaultSpeed  = 20. // degrees per second
	defaultPort   = "502"
	defaultMoveHz = 100. // Don't change this
)

type xArm struct {
	resource.Named
	dof      int
	tid      uint16
	moveHZ   float64 // Number of joint positions to send per second
	moveLock sync.Mutex
	model    referenceframe.Model
	started  bool
	opMgr    *operation.SingleOperationManager
	logger   logging.Logger

	mu    sync.RWMutex
	conn  net.Conn
	speed float32 // speed=max joint radians per second
}

//go:embed xarm6_kinematics.json
var xArm6modeljson []byte

//go:embed xarm7_kinematics.json
var xArm7modeljson []byte

//go:embed xarmlite_kinematics.json
var xArmLitemodeljson []byte

const (
	ModelName6DOF = "xArm6"    // ModelName6DOF is the name of an xArm6
	ModelName7DOF = "xArm7"    // ModelName7DOF is the name of an xArm7
	ModelNameLite = "xArmLite" // ModelNameLite is the name of an xArmLite
)

// MakeModelFrame returns the kinematics model of the xarm arm, which has all Frame information.
func MakeModelFrame(name, modelName string) (referenceframe.Model, error) {
	switch modelName {
	case ModelName6DOF:
		return referenceframe.UnmarshalModelJSON(xArm6modeljson, name)
	case ModelNameLite:
		return referenceframe.UnmarshalModelJSON(xArmLitemodeljson, name)
	case ModelName7DOF:
		return referenceframe.UnmarshalModelJSON(xArm7modeljson, name)
	default:
		return nil, fmt.Errorf("no kinematics information for xarm of model %s", modelName)
	}
}

func init() {
	for _, armModelName := range []string{ModelName6DOF, ModelName7DOF, ModelNameLite} {
		localArmModelName := armModelName
		armModel := resource.DefaultModelFamily.WithModel(armModelName)
		resource.RegisterComponent(arm.API, armModel, resource.Registration[arm.Arm, *Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (arm.Arm, error) {
				return NewxArm(ctx, conf, logger, localArmModelName)
			},
		})
	}
}

// NewxArm returns a new xArm of the specified modelName.
func NewxArm(ctx context.Context, conf resource.Config, logger logging.Logger, modelName string) (arm.Arm, error) {
	model, err := MakeModelFrame(conf.Name, modelName)
	if err != nil {
		return nil, err
	}

	xA := xArm{
		Named:   conf.ResourceName().AsNamed(),
		dof:     len(model.DoF()),
		tid:     0,
		moveHZ:  defaultMoveHz,
		model:   model,
		started: false,
		opMgr:   operation.NewSingleOperationManager(),
		logger:  logger,
	}

	if err := xA.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
	}

	return &xA, nil
}

// Reconfigure atomically reconfigures this arm in place based on the new config.
func (x *xArm) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if newConf.Host == "" {
		return errors.New("xArm host not set")
	}

	speed := newConf.Speed
	if speed == 0 {
		speed = defaultSpeed
	}
	if speed < 0 {
		return fmt.Errorf("given speed %f cannot be negative", speed)
	}

	x.mu.Lock()
	defer x.mu.Unlock()

	newAddr := net.JoinHostPort(newConf.Host, newConf.parsedPort)
	if x.conn == nil || x.conn.RemoteAddr().String() != newAddr {
		// Need a new or replacement connection
		var d net.Dialer
		newConn, err := d.DialContext(ctx, "tcp", newAddr)
		if err != nil {
			return err
		}
		if x.conn != nil {
			if err := x.conn.Close(); err != nil {
				x.logger.Warnw("error closing old connection but will continue with reconfiguration", "error", err)
			}
		}
		x.conn = newConn

		if err := x.start(ctx); err != nil {
			return errors.Wrap(err, "failed to start on reconfigure")
		}
	}

	x.speed = float32(utils.DegToRad(float64(speed)))
	return nil
}

func (x *xArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := x.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return x.model.InputFromProtobuf(res), nil
}

func (x *xArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	// check that joint positions are not out of bounds
	positionDegs := x.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, x, positionDegs); err != nil {
		return err
	}
	return x.MoveToJointPositions(ctx, positionDegs, nil)
}

func (x *xArm) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	inputs, err := x.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gif, err := x.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}

// ModelFrame returns all the information necessary for including the arm in a FrameSystem.
func (x *xArm) ModelFrame() referenceframe.Model {
	return x.model
}

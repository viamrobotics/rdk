// Package robotimpl defines implementations of robot.Robot and robot.MutableRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"sync"

	"go.viam.com/utils/pexec"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/metadata/service"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/servo"
	"go.viam.com/core/status"

	// registration
	_ "github.com/viamrobotics/SensorExporter/go/iphone"

	// registration
	_ "go.viam.com/core/lidar/client"
	_ "go.viam.com/core/robots/fake"
	_ "go.viam.com/core/sensor/compass/client"
	_ "go.viam.com/core/sensor/compass/gy511"
	_ "go.viam.com/core/sensor/compass/lidar"
	_ "go.viam.com/core/sensor/gps/nmea"

	// these are the core image things we always want
	_ "go.viam.com/core/rimage" // this is for the core camera types
	_ "go.viam.com/core/vision" // this is for interesting camera types, depth, etc...

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
)

var _ = robot.MutableRobot(&mutableRobot{})

// mutableRobot satisfies robot.MutableRobot and defers most
// logic to its parts.
type mutableRobot struct {
	mu     sync.Mutex
	parts  *robotParts
	config *config.Config
	logger golog.Logger
}

// RemoteByName returns a remote robot by name. If it does not exist
// nil is returned.
func (r *mutableRobot) RemoteByName(name string) (robot.Robot, bool) {
	return r.parts.RemoteByName(name)
}

// BoardByName returns a board by name. If it does not exist
// nil is returned.
func (r *mutableRobot) BoardByName(name string) (board.Board, bool) {
	return r.parts.BoardByName(name)
}

// ArmByName returns an arm by name. If it does not exist
// nil is returned.
func (r *mutableRobot) ArmByName(name string) (arm.Arm, bool) {
	return r.parts.ArmByName(name)
}

// BaseByName returns a base by name. If it does not exist
// nil is returned.
func (r *mutableRobot) BaseByName(name string) (base.Base, bool) {
	return r.parts.BaseByName(name)
}

// GripperByName returns a gripper by name. If it does not exist
// nil is returned.
func (r *mutableRobot) GripperByName(name string) (gripper.Gripper, bool) {
	return r.parts.GripperByName(name)
}

// CameraByName returns a camera by name. If it does not exist
// nil is returned.
func (r *mutableRobot) CameraByName(name string) (camera.Camera, bool) {
	return r.parts.CameraByName(name)
}

// LidarByName returns a lidar by name. If it does not exist
// nil is returned.
func (r *mutableRobot) LidarByName(name string) (lidar.Lidar, bool) {
	return r.parts.LidarByName(name)
}

// SensorByName returns a sensor by name. If it does not exist
// nil is returned.
func (r *mutableRobot) SensorByName(name string) (sensor.Sensor, bool) {
	return r.parts.SensorByName(name)
}

// ServoByName returns a servo by name. If it does not exist
// nil is returned.
func (r *mutableRobot) ServoByName(name string) (servo.Servo, bool) {
	return r.parts.ServoByName(name)
}

// MotorByName returns a motor by name. If it does not exist
// nil is returned.
func (r *mutableRobot) MotorByName(name string) (motor.Motor, bool) {
	return r.parts.MotorByName(name)
}

func (r *mutableRobot) ServiceByName(name string) (interface{}, bool) {
	return r.parts.ServiceByName(name)
}

// AddCamera adds a camera to the robot.
func (r *mutableRobot) AddCamera(c camera.Camera, cc config.Component) {
	r.parts.AddCamera(c, cc)
}

// AddBase adds a base to the robot.
func (r *mutableRobot) AddBase(b base.Base, c config.Component) {
	r.parts.AddBase(b, c)
}

// AddSensor adds a base to the robot.
func (r *mutableRobot) AddSensor(s sensor.Sensor, c config.Component) {
	r.parts.AddSensor(s, c)
}

// RemoteNames returns the name of all known remote robots.
func (r *mutableRobot) RemoteNames() []string {
	return r.parts.RemoteNames()
}

// ArmNames returns the name of all known arms.
func (r *mutableRobot) ArmNames() []string {
	return r.parts.ArmNames()
}

// GripperNames returns the name of all known grippers.
func (r *mutableRobot) GripperNames() []string {
	return r.parts.GripperNames()
}

// CameraNames returns the name of all known cameras.
func (r *mutableRobot) CameraNames() []string {
	return r.parts.CameraNames()
}

// LidarNames returns the name of all known lidars.
func (r *mutableRobot) LidarNames() []string {
	return r.parts.LidarNames()
}

// BaseNames returns the name of all known bases.
func (r *mutableRobot) BaseNames() []string {
	return r.parts.BaseNames()
}

// BoardNames returns the name of all known boards.
func (r *mutableRobot) BoardNames() []string {
	return r.parts.BoardNames()
}

// SensorNames returns the name of all known sensors.
func (r *mutableRobot) SensorNames() []string {
	return r.parts.SensorNames()
}

// ServoNames returns the name of all known servos.
func (r *mutableRobot) ServoNames() []string {
	return r.parts.ServoNames()
}

// MotorNames returns the name of all known motors.
func (r *mutableRobot) MotorNames() []string {
	return r.parts.MotorNames()
}

// FunctionNames returns the name of all known functions.
func (r *mutableRobot) FunctionNames() []string {
	return r.parts.FunctionNames()
}

// ServiceNames returns the name of all known services.
func (r *mutableRobot) ServiceNames() []string {
	return r.parts.ServiceNames()
}

// ProcessManager returns the process manager for the robot.
func (r *mutableRobot) ProcessManager() pexec.ProcessManager {
	return r.parts.processManager
}

// Close attempts to cleanly close down all constituent parts of the robot.
func (r *mutableRobot) Close() error {
	return r.parts.Close()
}

// Config returns the config used to construct the robot.
// This is allowed to be partial or empty.
func (r *mutableRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgCpy := *r.config
	cfgCpy.Components = append([]config.Component{}, cfgCpy.Components...)

	for remoteName, r := range r.parts.remotes {
		rc, err := r.Config(ctx)
		if err != nil {
			return nil, err
		}

		for _, c := range rc.Components {
			if c.Frame.Parent == "" {
				for _, rc := range cfgCpy.Remotes {
					if rc.Name == remoteName {
						c.Frame.Parent = rc.Parent
						break
					}
				}
			}
			cfgCpy.Components = append(cfgCpy.Components, c)
		}

	}
	return &cfgCpy, nil
}

// Status returns the current status of the robot. Usually you
// should use the CreateStatus helper instead of directly calling
// this.
func (r *mutableRobot) Status(ctx context.Context) (*pb.Status, error) {
	return status.Create(ctx, r)
}

func (r *mutableRobot) FrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	return CreateReferenceFrameSystem(ctx, r)
}

// Logger returns the logger the robot is using.
func (r *mutableRobot) Logger() golog.Logger {
	return r.logger
}

// New returns a new robot with parts sourced from the given config.
func New(ctx context.Context, config *config.Config, logger golog.Logger) (robot.MutableRobot, error) {
	r := &mutableRobot{
		parts:  newRobotParts(logger),
		logger: logger,
	}

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(); err != nil {
				logger.Errorw("failed to close robot down after startup failure", "error", err)
			}
		}
	}()
	r.config = config

	if err := r.parts.processConfig(ctx, config, r, logger); err != nil {
		return nil, err
	}

	// if metadata exists, update it
	if svc := service.ContextService(ctx); svc != nil {
		if err := r.UpdateMetadata(svc); err != nil {
			return nil, err
		}
	}

	successful = true
	return r, nil
}

func (r *mutableRobot) newBase(ctx context.Context, config config.Component) (base.Base, error) {
	f := registry.BaseLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown base model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newArm(ctx context.Context, config config.Component) (arm.Arm, error) {
	f := registry.ArmLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown arm model: %s", config.Model)
	}

	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newGripper(ctx context.Context, config config.Component) (gripper.Gripper, error) {
	f := registry.GripperLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown gripper model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newCamera(ctx context.Context, config config.Component) (camera.Camera, error) {
	f := registry.CameraLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown camera model: %s", config.Model)
	}
	is, err := f.Constructor(ctx, r, config, r.logger)
	if err != nil {
		return nil, err
	}
	return &camera.ImageSource{is}, nil
}

func (r *mutableRobot) newLidar(ctx context.Context, config config.Component) (lidar.Lidar, error) {
	f := registry.LidarLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown lidar model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newSensor(ctx context.Context, config config.Component, sensorType sensor.Type) (sensor.Sensor, error) {
	f := registry.SensorLookup(sensorType, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown sensor model (type=%s): %s", sensorType, config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newServo(ctx context.Context, config config.Component) (servo.Servo, error) {
	f := registry.ServoLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown servo model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newMotor(ctx context.Context, config config.Component) (motor.Motor, error) {
	f := registry.MotorLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown motor model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newBoard(ctx context.Context, config config.Component) (board.Board, error) {
	f := registry.BoardLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown board model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *mutableRobot) newService(ctx context.Context, config config.Service) (interface{}, error) {
	f := registry.ServiceLookup(config.Type)
	if f == nil {
		return nil, errors.Errorf("unknown service type: %s", config.Type)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

// Refresh does nothing for now
func (r *mutableRobot) Refresh(ctx context.Context) error {
	return nil
}

// UpdateMetadata updates metadata service using the currently registered parts of the robot
func (r *mutableRobot) UpdateMetadata(svc *service.Service) error {
	// TODO: Currently just a placeholder implementation, this should be rewritten once robot/parts have more metadata about themselves
	var resources []resource.Name

	metadata, err := resource.New(resource.ResourceNamespaceCore, resource.ResourceTypeService, resource.ResourceSubtypeMetadata, "")
	if err != nil {
		return err
	}
	resources = append(resources, metadata)

	for _, name := range r.ArmNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore, // can be non-core as well
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeArm,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.BaseNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeBase,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.BoardNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeBoard,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.CameraNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeCamera,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.FunctionNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeService,
			resource.ResourceSubtypeFunction,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.GripperNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeGripper,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.LidarNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeLidar,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.RemoteNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeRemote,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.SensorNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeSensor,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.ServoNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeServo,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	for _, name := range r.MotorNames() {
		res, err := resource.New(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeMotor,
			name,
		)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}
	return svc.Replace(resources)
}

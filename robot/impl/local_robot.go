// Package robotimpl defines implementations of robot.Robot and robot.LocalRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/utils/pexec"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/input"
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
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/status"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"

	// Engines
	_ "go.viam.com/core/function/vm/engines/javascript"

	// These are the robot pieces we want by default
	_ "go.viam.com/SensorExporter/go"

	// These are the robot pieces we want by default
	_ "go.viam.com/core/base/impl"
	_ "go.viam.com/core/board/arduino"
	_ "go.viam.com/core/board/detector"
	_ "go.viam.com/core/board/jetson"
	_ "go.viam.com/core/board/numato"
	_ "go.viam.com/core/camera/velodyne" // velodyne lidary
	_ "go.viam.com/core/component/gantry/simple"
	_ "go.viam.com/core/input/gamepad" // xbox controller and similar
	_ "go.viam.com/core/motor/gpio"
	_ "go.viam.com/core/motor/gpiostepper"
	_ "go.viam.com/core/motor/tmcstepper"
	_ "go.viam.com/core/rimage" // this is for the core camera types
	_ "go.viam.com/core/rimage/imagesource"
	_ "go.viam.com/core/robots/eva" // for eva
	_ "go.viam.com/core/robots/fake"
	_ "go.viam.com/core/robots/gopro"                   // for a camera
	_ "go.viam.com/core/robots/robotiq"                 // for a gripper
	_ "go.viam.com/core/robots/softrobotics"            // for a gripper
	_ "go.viam.com/core/robots/universalrobots"         // for an arm
	_ "go.viam.com/core/robots/varm"                    // for an arm
	_ "go.viam.com/core/robots/vforcematrixtraditional" // for a traditional force matrix
	_ "go.viam.com/core/robots/vforcematrixwithmux"     // for a force matrix built using a mux
	_ "go.viam.com/core/robots/vgripper"                // for a gripper
	_ "go.viam.com/core/robots/vx300s"                  // for arm and gripper
	_ "go.viam.com/core/robots/wx250s"                  // for arm and gripper
	_ "go.viam.com/core/robots/xarm"                    // for an arm
	_ "go.viam.com/core/sensor/compass/gy511"
	_ "go.viam.com/core/sensor/compass/lidar"
	_ "go.viam.com/core/sensor/forcematrix"
	_ "go.viam.com/core/sensor/gps/merge"
	_ "go.viam.com/core/sensor/gps/nmea"
	_ "go.viam.com/core/vision" // this is for interesting camera types, depth, etc...

	// These are the services we want by default
	_ "go.viam.com/core/services/navigation"
)

var _ = robot.LocalRobot(&localRobot{})

// WebSvcName defines the name of the web service
const WebSvcName = "web1"

// localRobot satisfies robot.LocalRobot and defers most
// logic to its parts.
type localRobot struct {
	mu     sync.Mutex
	parts  *robotParts
	config *config.Config
	logger golog.Logger
}

// RemoteByName returns a remote robot by name. If it does not exist
// nil is returned.
func (r *localRobot) RemoteByName(name string) (robot.Robot, bool) {
	return r.parts.RemoteByName(name)
}

// BoardByName returns a board by name. If it does not exist
// nil is returned.
func (r *localRobot) BoardByName(name string) (board.Board, bool) {
	return r.parts.BoardByName(name)
}

// ArmByName returns an arm by name. If it does not exist
// nil is returned.
func (r *localRobot) ArmByName(name string) (arm.Arm, bool) {
	return r.parts.ArmByName(name)
}

// BaseByName returns a base by name. If it does not exist
// nil is returned.
func (r *localRobot) BaseByName(name string) (base.Base, bool) {
	return r.parts.BaseByName(name)
}

// GripperByName returns a gripper by name. If it does not exist
// nil is returned.
func (r *localRobot) GripperByName(name string) (gripper.Gripper, bool) {
	return r.parts.GripperByName(name)
}

// CameraByName returns a camera by name. If it does not exist
// nil is returned.
func (r *localRobot) CameraByName(name string) (camera.Camera, bool) {
	return r.parts.CameraByName(name)
}

// LidarByName returns a lidar by name. If it does not exist
// nil is returned.
func (r *localRobot) LidarByName(name string) (lidar.Lidar, bool) {
	return r.parts.LidarByName(name)
}

// SensorByName returns a sensor by name. If it does not exist
// nil is returned.
func (r *localRobot) SensorByName(name string) (sensor.Sensor, bool) {
	return r.parts.SensorByName(name)
}

// ServoByName returns a servo by name. If it does not exist
// nil is returned.
func (r *localRobot) ServoByName(name string) (servo.Servo, bool) {
	return r.parts.ServoByName(name)
}

// MotorByName returns a motor by name. If it does not exist
// nil is returned.
func (r *localRobot) MotorByName(name string) (motor.Motor, bool) {
	return r.parts.MotorByName(name)
}

// InputControllerByName returns an input.Controller by name. If it does not exist
// nil is returned.
func (r *localRobot) InputControllerByName(name string) (input.Controller, bool) {
	return r.parts.InputControllerByName(name)
}

func (r *localRobot) ServiceByName(name string) (interface{}, bool) {
	return r.parts.ServiceByName(name)
}

// ResourceByName returns a resource by name. If it does not exist
// nil is returned.
func (r *localRobot) ResourceByName(name resource.Name) (interface{}, bool) {
	return r.parts.ResourceByName(name)
}

// RemoteNames returns the name of all known remote robots.
func (r *localRobot) RemoteNames() []string {
	return r.parts.RemoteNames()
}

// ArmNames returns the name of all known arms.
func (r *localRobot) ArmNames() []string {
	return r.parts.ArmNames()
}

// GripperNames returns the name of all known grippers.
func (r *localRobot) GripperNames() []string {
	return r.parts.GripperNames()
}

// CameraNames returns the name of all known cameras.
func (r *localRobot) CameraNames() []string {
	return r.parts.CameraNames()
}

// LidarNames returns the name of all known lidars.
func (r *localRobot) LidarNames() []string {
	return r.parts.LidarNames()
}

// BaseNames returns the name of all known bases.
func (r *localRobot) BaseNames() []string {
	return r.parts.BaseNames()
}

// BoardNames returns the name of all known boards.
func (r *localRobot) BoardNames() []string {
	return r.parts.BoardNames()
}

// SensorNames returns the name of all known sensors.
func (r *localRobot) SensorNames() []string {
	return r.parts.SensorNames()
}

// ServoNames returns the name of all known servos.
func (r *localRobot) ServoNames() []string {
	return r.parts.ServoNames()
}

// MotorNames returns the name of all known motors.
func (r *localRobot) MotorNames() []string {
	return r.parts.MotorNames()
}

// InputControllerNames returns the name of all known input Controllers.
func (r *localRobot) InputControllerNames() []string {
	return r.parts.InputControllerNames()
}

// FunctionNames returns the name of all known functions.
func (r *localRobot) FunctionNames() []string {
	return r.parts.FunctionNames()
}

// ServiceNames returns the name of all known services.
func (r *localRobot) ServiceNames() []string {
	return r.parts.ServiceNames()
}

// ResourceNames returns the name of all known resources.
func (r *localRobot) ResourceNames() []resource.Name {
	return r.parts.ResourceNames()
}

// ProcessManager returns the process manager for the robot.
func (r *localRobot) ProcessManager() pexec.ProcessManager {
	return r.parts.processManager
}

// Close attempts to cleanly close down all constituent parts of the robot.
func (r *localRobot) Close() error {
	return r.parts.Close()
}

// Config returns the config used to construct the robot.
// This is allowed to be partial or empty.
func (r *localRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgCpy := *r.config
	cfgCpy.Components = append([]config.Component{}, cfgCpy.Components...)

	for remoteName, remote := range r.parts.remotes {
		rc, err := remote.Config(ctx)
		rcCopy := *rc
		if err != nil {
			return nil, err
		}
		rConf, err := r.getRemoteConfig(ctx, remoteName)
		if err != nil {
			return nil, err
		}
		worldName := referenceframe.World
		if rConf.Prefix {
			worldName = rConf.Name + "." + referenceframe.World
		}
		for _, c := range rcCopy.Components {
			if c.Frame != nil && c.Frame.Parent == worldName {
				if rConf.Frame == nil { // world frames of the local and remote robot perfectly overlap
					c.Frame.Parent = referenceframe.World
				} else { // attach the frames connected to world node of the remote to the Frame defined in the Remote's config
					newTranslation, newOrientation := composeFrameOffsets(rConf.Frame, c.Frame)
					c.Frame.Parent = rConf.Frame.Parent
					c.Frame.Translation = newTranslation
					c.Frame.Orientation = newOrientation
				}
			}
			cfgCpy.Components = append(cfgCpy.Components, c)
		}

	}
	return &cfgCpy, nil
}

// getRemoteConfig gets the parameters for the Remote
func (r *localRobot) getRemoteConfig(ctx context.Context, remoteName string) (*config.Remote, error) {
	for _, rConf := range r.config.Remotes {
		if rConf.Name == remoteName {
			return &rConf, nil
		}
	}
	return nil, fmt.Errorf("cannot find Remote config with name %q", remoteName)
}

// composeFrameOffsets takes two config Frames and returns the composition of their 6dof poses
func composeFrameOffsets(a, b *config.Frame) (config.Translation, spatialmath.Orientation) {
	aTrans := r3.Vector{a.Translation.X, a.Translation.Y, a.Translation.Z}
	bTrans := r3.Vector{b.Translation.X, b.Translation.Y, b.Translation.Z}
	aPose := spatialmath.NewPoseFromOrientation(aTrans, a.Orientation)
	bPose := spatialmath.NewPoseFromOrientation(bTrans, b.Orientation)
	cPose := spatialmath.Compose(aPose, bPose)
	translation := config.Translation{cPose.Point().X, cPose.Point().Y, cPose.Point().Z}
	orientation := cPose.Orientation()
	return translation, orientation
}

// Status returns the current status of the robot. Usually you
// should use the CreateStatus helper instead of directly calling
// this.
func (r *localRobot) Status(ctx context.Context) (*pb.Status, error) {
	return status.Create(ctx, r)
}

func (r *localRobot) FrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	return CreateReferenceFrameSystem(ctx, r)
}

// Logger returns the logger the robot is using.
func (r *localRobot) Logger() golog.Logger {
	return r.logger
}

// New returns a new robot with parts sourced from the given config.
func New(ctx context.Context, cfg *config.Config, logger golog.Logger) (robot.LocalRobot, error) {
	r := &localRobot{
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
	r.config = cfg

	if err := r.parts.processConfig(ctx, cfg, r, logger); err != nil {
		return nil, err
	}

	// if metadata exists, update it
	if svc := service.ContextService(ctx); svc != nil {
		if err := r.UpdateMetadata(svc); err != nil {
			return nil, err
		}
	}

	// default services

	// create web service here
	// somewhat hacky, but the web service start up needs to come last
	// TODO: use web.Type as part of #253.
	webConfig := config.Service{Name: WebSvcName, Type: config.ServiceType("web")}
	web, err := r.newService(ctx, webConfig)
	if err != nil {
		return nil, err
	}
	r.parts.AddService(web, webConfig)
	successful = true
	return r, nil
}

func (r *localRobot) newBase(ctx context.Context, config config.Component) (base.Base, error) {
	f := registry.BaseLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown base model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newGripper(ctx context.Context, config config.Component) (gripper.Gripper, error) {
	f := registry.GripperLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown gripper model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newCamera(ctx context.Context, config config.Component) (camera.Camera, error) {
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

func (r *localRobot) newLidar(ctx context.Context, config config.Component) (lidar.Lidar, error) {
	f := registry.LidarLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown lidar model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newSensor(ctx context.Context, config config.Component, sensorType sensor.Type) (sensor.Sensor, error) {
	f := registry.SensorLookup(sensorType, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown sensor model (type=%s): %s", sensorType, config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newServo(ctx context.Context, config config.Component) (servo.Servo, error) {
	f := registry.ServoLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown servo model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newMotor(ctx context.Context, config config.Component) (motor.Motor, error) {
	f := registry.MotorLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown motor model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newInputController(ctx context.Context, config config.Component) (input.Controller, error) {
	f := registry.InputControllerLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown input controller model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newBoard(ctx context.Context, config config.Component) (board.Board, error) {
	f := registry.BoardLookup(config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown board model: %s", config.Model)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newService(ctx context.Context, config config.Service) (interface{}, error) {
	f := registry.ServiceLookup(config.Type)
	if f == nil {
		return nil, errors.Errorf("unknown service type: %s", config.Type)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newResource(ctx context.Context, config config.Component) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ComponentLookup(rName.Subtype, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown component subtype: %s and/or model: %s", rName.Subtype, config.Model)
	}
	newResource, err := f.Constructor(ctx, r, config, r.logger)
	if err != nil {
		return nil, err
	}
	c := registry.ComponentSubtypeLookup(rName.Subtype)
	if c == nil {
		return newResource, nil
	}
	return c.Reconfigurable(newResource)
}

// Refresh does nothing for now
func (r *localRobot) Refresh(ctx context.Context) error {
	return nil
}

// UpdateMetadata updates metadata service using the currently registered parts of the robot
func (r *localRobot) UpdateMetadata(svc service.Metadata) error {
	// TODO: Currently just a placeholder implementation, this should be rewritten once robot/parts have more metadata about themselves
	var resources []resource.Name

	metadata := resource.NewFromSubtype(service.Subtype, "")
	resources = append(resources, metadata)

	for _, name := range r.BaseNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeBase,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.BoardNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeBoard,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.CameraNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeCamera,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.FunctionNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeService,
			resource.ResourceSubtypeFunction,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.GripperNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeGripper,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.LidarNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeLidar,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.RemoteNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeRemote,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.SensorNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeSensor,
			name,
		)

		resources = append(resources, res)
	}
	for _, name := range r.ServoNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeServo,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.MotorNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeMotor,
			name,
		)
		resources = append(resources, res)
	}

	for _, name := range r.InputControllerNames() {
		res := resource.NewName(
			resource.ResourceNamespaceCore,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeInputController,
			name,
		)
		resources = append(resources, res)
	}

	resources = append(resources, r.ResourceNames()...)
	return svc.Replace(resources)
}

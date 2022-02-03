// Package robotimpl defines implementations of robot.Robot and robot.LocalRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/pexec"

	// register arm.
	_ "go.viam.com/rdk/component/arm/register"
	"go.viam.com/rdk/component/base"

	// register base.
	_ "go.viam.com/rdk/component/base/register"
	"go.viam.com/rdk/component/board"

	// register board.
	_ "go.viam.com/rdk/component/board/register"
	"go.viam.com/rdk/component/camera"

	// register camera.
	_ "go.viam.com/rdk/component/camera/register"

	// register force matrix.
	_ "go.viam.com/rdk/component/forcematrix/register"

	// register gantry.
	_ "go.viam.com/rdk/component/gantry/register"

	// register gps.
	_ "go.viam.com/rdk/component/gps/register"

	// register gripper.
	_ "go.viam.com/rdk/component/gripper/register"

	// register imu.
	_ "go.viam.com/rdk/component/imu/register"
	"go.viam.com/rdk/component/input"

	// register input.
	_ "go.viam.com/rdk/component/input/register"
	"go.viam.com/rdk/component/motor"

	// register motor.
	_ "go.viam.com/rdk/component/motor/register"

	// register sensor.
	_ "go.viam.com/rdk/component/sensor/register"

	// register servo.
	_ "go.viam.com/rdk/component/servo/register"
	"go.viam.com/rdk/config"

	// register vm engines.
	_ "go.viam.com/rdk/function/vm/engines/javascript"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/metadata/service"

	// detect pi.
	_ "go.viam.com/rdk/platformdetector/pi"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"

	// register base remote control.
	_ "go.viam.com/rdk/services/baseremotecontrol"
	"go.viam.com/rdk/services/framesystem"

	// register navigation.
	_ "go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/web"
	"go.viam.com/rdk/status"
)

var _ = robot.LocalRobot(&localRobot{})

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

// BaseByName returns a base by name. If it does not exist
// nil is returned.
func (r *localRobot) BaseByName(name string) (base.Base, bool) {
	return r.parts.BaseByName(name)
}

// CameraByName returns a camera by name. If it does not exist
// nil is returned.
func (r *localRobot) CameraByName(name string) (camera.Camera, bool) {
	return r.parts.CameraByName(name)
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

// ResourceByName returns a resource by name. If it does not exist
// nil is returned.
func (r *localRobot) ResourceByName(name resource.Name) (interface{}, bool) {
	return r.parts.ResourceByName(name)
}

// RemoteNames returns the name of all known remote robots.
func (r *localRobot) RemoteNames() []string {
	return r.parts.RemoteNames()
}

// CameraNames returns the name of all known cameras.
func (r *localRobot) CameraNames() []string {
	return r.parts.CameraNames()
}

// BaseNames returns the name of all known bases.
func (r *localRobot) BaseNames() []string {
	return r.parts.BaseNames()
}

// BoardNames returns the name of all known boards.
func (r *localRobot) BoardNames() []string {
	return r.parts.BoardNames()
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

// ResourceNames returns the name of all known resources.
func (r *localRobot) ResourceNames() []resource.Name {
	return r.parts.ResourceNames()
}

// ProcessManager returns the process manager for the robot.
func (r *localRobot) ProcessManager() pexec.ProcessManager {
	return r.parts.processManager
}

// Close attempts to cleanly close down all constituent parts of the robot.
func (r *localRobot) Close(ctx context.Context) error {
	return r.parts.Close(ctx)
}

// Config returns the config used to construct the robot.
// This is allowed to be partial or empty.
func (r *localRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgCpy := *r.config
	cfgCpy.Components = append([]config.Component{}, cfgCpy.Components...)

	for remoteName, remote := range r.parts.remotes {
		rc, err := remote.Config(ctx)
		if err != nil {
			return nil, err
		}
		remoteWorldName := remoteName + "." + referenceframe.World
		for _, c := range rc.Components {
			if c.Frame != nil && c.Frame.Parent == referenceframe.World {
				c.Frame.Parent = remoteWorldName
			}
			cfgCpy.Components = append(cfgCpy.Components, c)
		}
	}
	return &cfgCpy, nil
}

// getRemoteConfig gets the parameters for the Remote.
func (r *localRobot) getRemoteConfig(remoteName string) (*config.Remote, error) {
	for _, rConf := range r.config.Remotes {
		if rConf.Name == remoteName {
			return &rConf, nil
		}
	}
	return nil, fmt.Errorf("cannot find Remote config with name %q", remoteName)
}

// Status returns the current status of the robot. Usually you
// should use the CreateStatus helper instead of directly calling
// this.
func (r *localRobot) Status(ctx context.Context) (*pb.Status, error) {
	return status.Create(ctx, r)
}

// FrameSystem returns the FrameSystem of the robot.
func (r *localRobot) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	logger := r.Logger()
	// create the base reference frame system
	service, ok := r.ResourceByName(framesystem.Name)
	if !ok {
		return nil, errors.New("service frame_system not found")
	}
	fsService, ok := service.(framesystem.Service)
	if !ok {
		return nil, errors.New("service is not a frame_system service")
	}
	baseFrameSys, err := fsService.LocalFrameSystem(ctx, name)
	if err != nil {
		return nil, err
	}
	logger.Debugf("base frame system %q has frames %v", baseFrameSys.Name(), baseFrameSys.FrameNames())
	// get frame system for each of its remote parts and merge to base
	for remoteName, remote := range r.parts.remotes {
		remoteFrameSys, err := remote.FrameSystem(ctx, remoteName, prefix)
		if err != nil {
			return nil, err
		}
		rConf, err := r.getRemoteConfig(remoteName)
		if err != nil {
			return nil, err
		}
		logger.Debugf("merging remote frame system  %q with frames %v", remoteFrameSys.Name(), remoteFrameSys.FrameNames())
		err = config.MergeFrameSystems(baseFrameSys, remoteFrameSys, rConf.Frame)
		if err != nil {
			return nil, err
		}
	}
	logger.Debugf("final frame system  %q has frames %v", baseFrameSys.Name(), baseFrameSys.FrameNames())
	return baseFrameSys, nil
}

// Logger returns the logger the robot is using.
func (r *localRobot) Logger() golog.Logger {
	return r.logger
}

// New returns a new robot with parts sourced from the given config.
func New(ctx context.Context, cfg *config.Config, logger golog.Logger, opts ...client.RobotClientOption) (robot.LocalRobot, error) {
	r := &localRobot{
		parts:  newRobotParts(logger, opts...),
		logger: logger,
	}

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(context.Background()); err != nil {
				logger.Errorw("failed to close robot down after startup failure", "error", err)
			}
		}
	}()
	r.config = cfg

	if err := r.parts.processConfig(ctx, cfg, r, logger); err != nil {
		return nil, err
	}

	// default services

	// create web service here
	// somewhat hacky, but the web service start up needs to come last
	webConfig := config.Service{Type: config.ServiceType(web.SubtypeName)}
	webSvc, err := r.newService(ctx, webConfig)
	if err != nil {
		return nil, err
	}
	r.parts.addResource(web.Name, webSvc)

	// if metadata exists, update it
	if svc := service.ContextService(ctx); svc != nil {
		if err := r.UpdateMetadata(svc); err != nil {
			return nil, err
		}
	}
	successful = true
	return r, nil
}

func (r *localRobot) newService(ctx context.Context, config config.Service) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ServiceLookup(rName.Subtype)
	if f == nil {
		return nil, errors.Errorf("unknown service type: %s", rName.Subtype)
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
	c := registry.ResourceSubtypeLookup(rName.Subtype)
	if c == nil || c.Reconfigurable == nil {
		return newResource, nil
	}
	return c.Reconfigurable(newResource)
}

// Refresh does nothing for now.
func (r *localRobot) Refresh(ctx context.Context) error {
	return nil
}

// UpdateMetadata updates metadata service using the currently registered parts of the robot.
func (r *localRobot) UpdateMetadata(svc service.Metadata) error {
	// TODO: Currently just a placeholder implementation, this should be rewritten once robot/parts have more metadata about themselves
	var resources []resource.Name

	metadata := resource.NameFromSubtype(service.Subtype, "")
	resources = append(resources, metadata)

	for _, name := range r.FunctionNames() {
		res := resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeFunction,
			resource.ResourceSubtypeFunction,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.RemoteNames() {
		res := resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeRemote,
			name,
		)
		resources = append(resources, res)
	}

	resources = append(resources, r.ResourceNames()...)
	return svc.Replace(resources)
}

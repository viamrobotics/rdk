package robotimpl

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/component/camera"
	"go.viam.com/core/component/gripper"
	"go.viam.com/core/component/input"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/component/servo"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"

	"github.com/edaniels/golog"
)

var errUnimplemented = errors.New("unimplemented")

// A remoteRobot implements wraps an robot.Robot. It
// assists in the un/prefixing of part names for RemoteRobots that
// are not aware they are integrated elsewhere.
// We intentionally do not promote the underlying robot.Robot
// so that any future changes are forced to consider un/prefixing
// of names.
type remoteRobot struct {
	mu    sync.Mutex
	robot robot.Robot
	conf  config.Remote
	parts *robotParts
}

// newRemoteRobot returns a new remote robot wrapping a given robot.Robot
// and its configuration.
func newRemoteRobot(robot robot.Robot, config config.Remote) *remoteRobot {
	// We pull the parts out here such that we correctly return nil for
	// when parts are accessed. This is because a networked robot client
	// may just return a non-nil wrapper for a part they may not exist.
	remoteParts := partsForRemoteRobot(robot)
	return &remoteRobot{
		robot: robot,
		conf:  config,
		parts: remoteParts,
	}
}

func (rr *remoteRobot) Refresh(ctx context.Context) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	refresher, ok := rr.robot.(robot.Refresher)
	if !ok {
		return nil
	}
	if err := refresher.Refresh(ctx); err != nil {
		return err
	}
	rr.parts = partsForRemoteRobot(rr.robot)
	return nil
}

// replace replaces this robot with the given robot. We can do a full
// replacement here since we will always get a full view of the parts,
// not one partially created from a diff.
func (rr *remoteRobot) replace(newRobot robot.Robot) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	actual, ok := newRobot.(*remoteRobot)
	if !ok {
		panic(fmt.Errorf("expected new remote to be %T but got %T", actual, newRobot))
	}

	rr.parts.replaceForRemote(actual.parts)
}

func (rr *remoteRobot) prefixName(name string) string {
	if rr.conf.Prefix {
		return fmt.Sprintf("%s.%s", rr.conf.Name, name)
	}
	return name
}

func (rr *remoteRobot) unprefixName(name string) string {
	if rr.conf.Prefix {
		return strings.TrimPrefix(name, rr.conf.Name+".")
	}
	return name
}

func (rr *remoteRobot) prefixNames(names []string) []string {
	if !rr.conf.Prefix {
		return names
	}
	newNames := make([]string, 0, len(names))
	for _, name := range names {
		newNames = append(newNames, rr.prefixName(name))
	}
	return newNames
}

func (rr *remoteRobot) prefixResourceName(name resource.Name) resource.Name {
	if !rr.conf.Prefix {
		return name
	}
	newName := rr.prefixName(name.Name)
	return resource.NewName(
		name.Namespace, name.ResourceType, name.ResourceSubtype, newName,
	)
}

func (rr *remoteRobot) unprefixResourceName(name resource.Name) resource.Name {
	if !rr.conf.Prefix {
		return name
	}
	newName := rr.unprefixName(name.Name)
	return resource.NewName(
		name.Namespace, name.ResourceType, name.ResourceSubtype, newName,
	)
}

func (rr *remoteRobot) RemoteNames() []string {
	return nil
}

func (rr *remoteRobot) ArmNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.ArmNames())
}

func (rr *remoteRobot) GripperNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.GripperNames())
}

func (rr *remoteRobot) CameraNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.CameraNames())
}

func (rr *remoteRobot) BaseNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.BaseNames())
}

func (rr *remoteRobot) BoardNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.BoardNames())
}

func (rr *remoteRobot) SensorNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.SensorNames())
}

func (rr *remoteRobot) ServoNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.ServoNames())
}

func (rr *remoteRobot) MotorNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.MotorNames())
}

func (rr *remoteRobot) InputControllerNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.InputControllerNames())
}

func (rr *remoteRobot) FunctionNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.FunctionNames())
}

func (rr *remoteRobot) ServiceNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.ServiceNames())
}

func (rr *remoteRobot) ResourceNames() []resource.Name {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	newNames := make([]resource.Name, 0, len(rr.parts.ResourceNames()))
	for _, name := range rr.parts.ResourceNames() {
		name := rr.prefixResourceName(name)
		newNames = append(newNames, name)
	}
	return newNames
}

func (rr *remoteRobot) RemoteByName(name string) (robot.Robot, bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rr *remoteRobot) ArmByName(name string) (arm.Arm, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.ArmByName(rr.unprefixName(name))
}

func (rr *remoteRobot) BaseByName(name string) (base.Base, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.BaseByName(rr.unprefixName(name))
}

func (rr *remoteRobot) GripperByName(name string) (gripper.Gripper, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.GripperByName(rr.unprefixName(name))
}

func (rr *remoteRobot) CameraByName(name string) (camera.Camera, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.CameraByName(rr.unprefixName(name))
}

func (rr *remoteRobot) BoardByName(name string) (board.Board, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.BoardByName(rr.unprefixName(name))
}

func (rr *remoteRobot) SensorByName(name string) (sensor.Sensor, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.SensorByName(rr.unprefixName(name))
}

func (rr *remoteRobot) ServoByName(name string) (servo.Servo, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.ServoByName(rr.unprefixName(name))
}

func (rr *remoteRobot) MotorByName(name string) (motor.Motor, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.MotorByName(rr.unprefixName(name))
}

func (rr *remoteRobot) InputControllerByName(name string) (input.Controller, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.InputControllerByName(rr.unprefixName(name))
}

func (rr *remoteRobot) ServiceByName(name string) (interface{}, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.ServiceByName(rr.unprefixName(name))
}

func (rr *remoteRobot) ResourceByName(name resource.Name) (interface{}, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	newName := rr.unprefixResourceName(name)
	return rr.parts.ResourceByName(newName)
}

func (rr *remoteRobot) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

func (rr *remoteRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgReal, err := rr.robot.Config(ctx)
	if err != nil {
		return nil, err
	}

	cfg := config.Config{
		Components: make([]config.Component, len(cfgReal.Components)),
	}

	for idx, c := range cfgReal.Components {
		c.Name = rr.prefixName(c.Name)
		if c.Frame != nil {
			c.Frame.Parent = rr.prefixName(c.Frame.Parent)
		}
		cfg.Components[idx] = c
	}

	return &cfg, nil
}

// FrameSystem will return the frame system from the remote robot's server
// remoteRobot may add on its own prefix if specified by the config file.
func (rr *remoteRobot) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	if rr.conf.Prefix {
		prefix = rr.prefixName(prefix)
	}
	fs, err := rr.robot.FrameSystem(ctx, name, prefix)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func (rr *remoteRobot) Status(ctx context.Context) (*pb.Status, error) {
	status, err := rr.robot.Status(ctx)
	if err != nil {
		return nil, err
	}
	var rewrittenStatus pb.Status

	if len(status.Arms) != 0 {
		rewrittenStatus.Arms = make(map[string]*pb.ArmStatus, len(status.Arms))
		for k, v := range status.Arms {
			rewrittenStatus.Arms[rr.prefixName(k)] = v
		}
	}
	if len(status.Bases) != 0 {
		rewrittenStatus.Bases = make(map[string]bool, len(status.Bases))
		for k, v := range status.Bases {
			rewrittenStatus.Bases[rr.prefixName(k)] = v
		}
	}
	if len(status.Grippers) != 0 {
		rewrittenStatus.Grippers = make(map[string]bool, len(status.Grippers))
		for k, v := range status.Grippers {
			rewrittenStatus.Grippers[rr.prefixName(k)] = v
		}
	}
	if len(status.Boards) != 0 {
		rewrittenStatus.Boards = make(map[string]*pb.BoardStatus, len(status.Boards))
		for k, v := range status.Boards {
			rewrittenStatus.Boards[rr.prefixName(k)] = v
		}
	}
	if len(status.Cameras) != 0 {
		rewrittenStatus.Cameras = make(map[string]bool, len(status.Cameras))
		for k, v := range status.Cameras {
			rewrittenStatus.Cameras[rr.prefixName(k)] = v
		}
	}
	if len(status.Sensors) != 0 {
		rewrittenStatus.Sensors = make(map[string]*pb.SensorStatus, len(status.Sensors))
		for k, v := range status.Sensors {
			rewrittenStatus.Sensors[rr.prefixName(k)] = v
		}
	}
	if len(status.Servos) != 0 {
		rewrittenStatus.Servos = make(map[string]*pb.ServoStatus, len(status.Servos))
		for k, v := range status.Servos {
			rewrittenStatus.Servos[rr.prefixName(k)] = v
		}
	}
	if len(status.Motors) != 0 {
		rewrittenStatus.Motors = make(map[string]*pb.MotorStatus, len(status.Motors))
		for k, v := range status.Motors {
			rewrittenStatus.Motors[rr.prefixName(k)] = v
		}
	}
	if len(status.InputControllers) != 0 {
		rewrittenStatus.InputControllers = make(map[string]*pb.InputControllerStatus, len(status.InputControllers))
		for k, v := range status.InputControllers {
			rewrittenStatus.InputControllers[rr.prefixName(k)] = v
		}
	}
	if len(status.Services) != 0 {
		rewrittenStatus.Services = make(map[string]bool, len(status.Services))
		for k, v := range status.Services {
			rewrittenStatus.Services[rr.prefixName(k)] = v
		}
	}
	if len(status.Functions) != 0 {
		rewrittenStatus.Functions = make(map[string]bool, len(status.Functions))
		for k, v := range status.Functions {
			rewrittenStatus.Functions[rr.prefixName(k)] = v
		}
	}

	return &rewrittenStatus, nil
}

func (rr *remoteRobot) Logger() golog.Logger {
	return rr.robot.Logger()
}

func (rr *remoteRobot) Close() error {
	return utils.TryClose(rr.robot)
}

// partsForRemoteRobot integrates all parts from a given robot
// except for its remotes. This is for a remote robot to integrate
// which should be unaware of remotes.
// Be sure to update this function if robotParts grows.
func partsForRemoteRobot(robot robot.Robot) *robotParts {
	parts := newRobotParts(robot.Logger().Named("parts"))
	for _, name := range robot.BaseNames() {
		part, ok := robot.BaseByName(name)
		if !ok {
			continue
		}
		parts.AddBase(part, config.Component{Name: name})
	}
	for _, name := range robot.BoardNames() {
		part, ok := robot.BoardByName(name)
		if !ok {
			continue
		}
		parts.AddBoard(part, config.Component{Name: name})
	}
	for _, name := range robot.SensorNames() {
		part, ok := robot.SensorByName(name)
		if !ok {
			continue
		}
		parts.AddSensor(part, config.Component{Name: name})
	}
	for _, name := range robot.FunctionNames() {
		parts.addFunction(name)
	}
	for _, name := range robot.ServiceNames() {
		part, ok := robot.ServiceByName(name)
		if !ok {
			continue
		}
		parts.AddService(part, config.Service{Name: name})
	}

	for _, name := range robot.ResourceNames() {
		part, ok := robot.ResourceByName(name)
		if !ok {
			continue
		}
		parts.addResource(name, part)
	}
	return parts
}

// replaceForRemote replaces these parts with the given parts coming from a remote.
func (parts *robotParts) replaceForRemote(newParts *robotParts) {
	var oldBoardNames map[string]struct{}
	var oldBaseNames map[string]struct{}
	var oldSensorNames map[string]struct{}
	var oldFunctionNames map[string]struct{}
	var oldServiceNames map[string]struct{}
	var oldResources map[resource.Name]struct{}

	if len(parts.boards) != 0 {
		oldBoardNames = make(map[string]struct{}, len(parts.boards))
		for name := range parts.boards {
			oldBoardNames[name] = struct{}{}
		}
	}
	if len(parts.bases) != 0 {
		oldBaseNames = make(map[string]struct{}, len(parts.bases))
		for name := range parts.bases {
			oldBaseNames[name] = struct{}{}
		}
	}
	if len(parts.sensors) != 0 {
		oldSensorNames = make(map[string]struct{}, len(parts.sensors))
		for name := range parts.sensors {
			oldSensorNames[name] = struct{}{}
		}
	}
	if len(parts.functions) != 0 {
		oldFunctionNames = make(map[string]struct{}, len(parts.functions))
		for name := range parts.functions {
			oldFunctionNames[name] = struct{}{}
		}
	}
	if len(parts.services) != 0 {
		oldServiceNames = make(map[string]struct{}, len(parts.services))
		for name := range parts.services {
			oldServiceNames[name] = struct{}{}
		}
	}

	if len(parts.resources) != 0 {
		oldResources = make(map[resource.Name]struct{}, len(parts.resources))
		for name := range parts.resources {
			oldResources[name] = struct{}{}
		}
	}

	for name, newPart := range newParts.boards {
		oldPart, ok := parts.boards[name]
		delete(oldBoardNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		parts.boards[name] = newPart
	}
	for name, newPart := range newParts.bases {
		oldPart, ok := parts.bases[name]
		delete(oldBaseNames, name)
		if ok {
			oldPart.replace(newPart)
			continue
		}
		parts.bases[name] = newPart
	}
	for name, newPart := range newParts.sensors {
		oldPart, ok := parts.sensors[name]
		delete(oldSensorNames, name)
		if ok {
			oldPart.(interface{ replace(newSensor sensor.Sensor) }).replace(newPart)
			continue
		}
		parts.sensors[name] = newPart
	}
	for name, newPart := range newParts.functions {
		_, ok := parts.functions[name]
		delete(oldFunctionNames, name)
		if ok {
			continue
		}
		parts.functions[name] = newPart
	}
	for name, newPart := range newParts.services {
		oldPart, ok := parts.services[name]
		delete(oldServiceNames, name)
		if ok {
			_ = oldPart
			// TODO(erd): how to handle service replacement?
			// oldPart.replace(newPart)
			continue
		}
		parts.services[name] = newPart
	}
	for name, newPart := range newParts.resources {
		oldPart, ok := parts.resources[name]
		delete(oldResources, name)
		if ok {
			oldPart, ok := oldPart.(resource.Reconfigurable)
			if !ok {
				panic(fmt.Errorf("expected type %T to be reconfigurable but it was not", oldPart))
			}
			newPart, ok := newPart.(resource.Reconfigurable)
			if !ok {
				panic(fmt.Errorf("expected type %T to be reconfigurable but it was not", newPart))
			}
			if err := oldPart.Reconfigure(newPart); err != nil {
				panic(err)
			}
			continue
		}
		parts.resources[name] = newPart
	}

	for name := range oldBoardNames {
		delete(parts.boards, name)
	}
	for name := range oldBaseNames {
		delete(parts.bases, name)
	}
	for name := range oldSensorNames {
		delete(parts.sensors, name)
	}
	for name := range oldFunctionNames {
		delete(parts.functions, name)
	}
	for name := range oldServiceNames {
		delete(parts.services, name)
	}
	for name := range oldResources {
		delete(parts.resources, name)
	}
}

package robot

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/multierr"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/client"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

// robotParts are the actual parts that make up a robot.
type robotParts struct {
	remotes        map[string]api.RemoteRobot
	boards         map[string]board.Board
	arms           map[string]api.Arm
	grippers       map[string]api.Gripper
	cameras        map[string]gostream.ImageSource
	lidarDevices   map[string]lidar.Device
	bases          map[string]api.Base
	sensors        map[string]sensor.Device
	providers      map[string]api.Provider
	processManager rexec.ProcessManager
}

// newRobotParts returns a properly initialized set of parts.
func newRobotParts(logger golog.Logger) *robotParts {
	return &robotParts{
		remotes:        map[string]api.RemoteRobot{},
		boards:         map[string]board.Board{},
		arms:           map[string]api.Arm{},
		grippers:       map[string]api.Gripper{},
		cameras:        map[string]gostream.ImageSource{},
		lidarDevices:   map[string]lidar.Device{},
		bases:          map[string]api.Base{},
		sensors:        map[string]sensor.Device{},
		providers:      map[string]api.Provider{},
		processManager: rexec.NewProcessManager(logger),
	}
}

// fixType ensures that the component has correct type information.
func fixType(c api.ComponentConfig, whichType api.ComponentType, pos int) api.ComponentConfig {
	if c.Type == "" {
		c.Type = whichType
	} else if c.Type != whichType {
		panic(fmt.Sprintf("different types (%s) != (%s)", whichType, c.Type))
	}
	return c
}

// AddRemote adds a remote to the parts.
func (parts *robotParts) AddRemote(remote api.RemoteRobot, c api.RemoteConfig) {
	parts.remotes[c.Name] = remote
}

// AddBoard adds a board to the parts.
func (parts *robotParts) AddBoard(b board.Board, c board.Config) {
	parts.boards[c.Name] = b
}

// AddArm adds an arm to the parts.
func (parts *robotParts) AddArm(a api.Arm, c api.ComponentConfig) {
	c = fixType(c, api.ComponentTypeArm, len(parts.arms))
	parts.arms[c.Name] = a
}

// AddGripper adds a gripper to the parts.
func (parts *robotParts) AddGripper(g api.Gripper, c api.ComponentConfig) {
	c = fixType(c, api.ComponentTypeGripper, len(parts.grippers))
	parts.grippers[c.Name] = g
}

// AddCamera adds a camera to the parts.
func (parts *robotParts) AddCamera(camera gostream.ImageSource, c api.ComponentConfig) {
	c = fixType(c, api.ComponentTypeCamera, len(parts.cameras))
	parts.cameras[c.Name] = camera
}

// AddLidar adds a lidar device to the parts.
func (parts *robotParts) AddLidar(device lidar.Device, c api.ComponentConfig) {
	c = fixType(c, api.ComponentTypeLidar, len(parts.lidarDevices))
	parts.lidarDevices[c.Name] = device
}

// AddBase adds a base to the parts.
func (parts *robotParts) AddBase(b api.Base, c api.ComponentConfig) {
	c = fixType(c, api.ComponentTypeBase, len(parts.bases))
	parts.bases[c.Name] = b
}

// AddSensor adds a sensor to the parts.
func (parts *robotParts) AddSensor(s sensor.Device, c api.ComponentConfig) {
	c = fixType(c, api.ComponentTypeSensor, len(parts.sensors))
	parts.sensors[c.Name] = s
}

// AddProvider adds a provider to the parts.
func (parts *robotParts) AddProvider(p api.Provider, c api.ComponentConfig) {
	parts.providers[c.Name] = p
}

// RemoteNames returns the names of all remotes in the parts.
func (parts *robotParts) RemoteNames() []string {
	names := []string{}
	for k := range parts.remotes {
		names = append(names, k)
	}
	return names
}

// mergeNamesWithRemotes merges names from the parts itself as well as its
// remotes.
func (parts *robotParts) mergeNamesWithRemotes(names []string, namesFunc func(remote api.Robot) []string) []string {
	// use this to filter out seen names and preserve order
	seen := utils.NewStringSet()
	for _, name := range names {
		seen[name] = struct{}{}
	}
	for _, r := range parts.remotes {
		remoteNames := namesFunc(r)
		for _, name := range remoteNames {
			if _, ok := seen[name]; ok {
				continue
			}
			names = append(names, name)
		}
	}
	return names
}

// ArmNames returns the names of all arms in the parts.
func (parts *robotParts) ArmNames() []string {
	names := []string{}
	for k := range parts.arms {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, api.Robot.ArmNames)
}

// GripperNames returns the names of all grippers in the parts.
func (parts *robotParts) GripperNames() []string {
	names := []string{}
	for k := range parts.grippers {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, api.Robot.GripperNames)
}

// CameraNames returns the names of all cameras in the parts.
func (parts *robotParts) CameraNames() []string {
	names := []string{}
	for k := range parts.cameras {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, api.Robot.CameraNames)
}

// LidarDeviceNames returns the names of all lidar devices in the parts.
func (parts *robotParts) LidarDeviceNames() []string {
	names := []string{}
	for k := range parts.lidarDevices {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, api.Robot.LidarDeviceNames)
}

// BaseNames returns the names of all bases in the parts.
func (parts *robotParts) BaseNames() []string {
	names := []string{}
	for k := range parts.bases {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, api.Robot.BaseNames)
}

// BoardNames returns the names of all boards in the parts.
func (parts *robotParts) BoardNames() []string {
	names := []string{}
	for k := range parts.boards {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, api.Robot.BoardNames)
}

// SensorNames returns the names of all sensors in the parts.
func (parts *robotParts) SensorNames() []string {
	names := []string{}
	for k := range parts.sensors {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, api.Robot.SensorNames)
}

// Close attempts to close/stop all parts.
func (parts *robotParts) Close() error {
	var allErrs error
	if err := parts.processManager.Stop(); err != nil {
		allErrs = multierr.Combine(allErrs, fmt.Errorf("error stopping process manager: %w", err))
	}

	for _, x := range parts.arms {
		if err := utils.TryClose(x); err != nil {
			allErrs = multierr.Combine(allErrs, fmt.Errorf("error closing arm: %w", err))
		}
	}

	for _, x := range parts.grippers {
		if err := utils.TryClose(x); err != nil {
			allErrs = multierr.Combine(allErrs, fmt.Errorf("error closing gripper: %w", err))
		}
	}

	for _, x := range parts.cameras {
		if err := utils.TryClose(x); err != nil {
			allErrs = multierr.Combine(allErrs, fmt.Errorf("error closing camera: %w", err))
		}
	}

	for _, x := range parts.lidarDevices {
		if err := utils.TryClose(x); err != nil {
			allErrs = multierr.Combine(allErrs, fmt.Errorf("error closing lidar: %w", err))
		}
	}

	for _, x := range parts.bases {
		if err := utils.TryClose(x); err != nil {
			allErrs = multierr.Combine(allErrs, fmt.Errorf("error closing base: %w", err))
		}
	}

	for _, x := range parts.boards {
		if err := utils.TryClose(x); err != nil {
			allErrs = multierr.Combine(allErrs, fmt.Errorf("error closing board: %w", err))
		}
	}

	for _, x := range parts.sensors {
		if err := utils.TryClose(x); err != nil {
			allErrs = multierr.Combine(allErrs, fmt.Errorf("error closing sensor: %w", err))
		}
	}

	return allErrs
}

// processConfig ingests a given config and constructs all constituent parts.
func (parts *robotParts) processConfig(
	ctx context.Context,
	config *api.Config,
	robot *Robot,
	logger golog.Logger,
) error {
	if err := parts.newProcesses(ctx, config.Processes); err != nil {
		return err
	}

	if err := parts.newRemotes(ctx, config.Remotes, logger); err != nil {
		return err
	}

	if err := parts.newBoards(ctx, config.Boards, logger); err != nil {
		return err
	}

	if err := parts.newComponents(ctx, config.Components, robot); err != nil {
		return err
	}

	return nil
}

// newProcesses constructs all processes defined.
func (parts *robotParts) newProcesses(ctx context.Context, processes []rexec.ProcessConfig) error {
	for _, procConf := range processes {
		if _, err := parts.processManager.AddProcessFromConfig(ctx, procConf); err != nil {
			return err
		}
	}
	return parts.processManager.Start(ctx)
}

// newRemotes constructs all remotes defined and integrates their parts in.
func (parts *robotParts) newRemotes(ctx context.Context, remotes []api.RemoteConfig, logger golog.Logger) error {
	for _, config := range remotes {
		robotClient, err := client.NewRobotClient(ctx, config.Address, logger)
		if err != nil {
			return fmt.Errorf("couldn't connect to robot remote (%s): %w", config.Address, err)
		}

		configCopy := config
		parts.AddRemote(newRemoteRobot(robotClient, configCopy), configCopy)
	}
	return nil
}

// newBoards constructs all boards defined.
func (parts *robotParts) newBoards(ctx context.Context, boards []board.Config, logger golog.Logger) error {
	for _, c := range boards {
		b, err := board.NewBoard(ctx, c, logger)
		if err != nil {
			return err
		}
		parts.AddBoard(b, c)
	}
	return nil
}

// newComponents constructs all components defined.
func (parts *robotParts) newComponents(ctx context.Context, components []api.ComponentConfig, r *Robot) error {
	for _, c := range components {
		switch c.Type {
		case api.ComponentTypeProvider:
			p, err := r.newProvider(ctx, c)
			if err != nil {
				return err
			}
			parts.AddProvider(p, c)
		}
	}

	for _, c := range components {
		switch c.Type {
		case api.ComponentTypeProvider:
			// hanlded above
		case api.ComponentTypeBase:
			b, err := r.newBase(ctx, c)
			if err != nil {
				return err
			}
			parts.AddBase(b, c)
		case api.ComponentTypeArm:
			a, err := r.newArm(ctx, c)
			if err != nil {
				return err
			}
			parts.AddArm(a, c)
		case api.ComponentTypeGripper:
			g, err := r.newGripper(ctx, c)
			if err != nil {
				return err
			}
			parts.AddGripper(g, c)
		case api.ComponentTypeCamera:
			camera, err := r.newCamera(ctx, c)
			if err != nil {
				return err
			}
			parts.AddCamera(camera, c)
		case api.ComponentTypeLidar:
			lidarDevice, err := r.newLidarDevice(ctx, c)
			if err != nil {
				return err
			}
			parts.AddLidar(lidarDevice, c)
		case api.ComponentTypeSensor:
			if c.SubType == "" {
				return errors.New("sensor component requires subtype")
			}
			sensorDevice, err := r.newSensor(ctx, c, sensor.DeviceType(c.SubType))
			if err != nil {
				return err
			}
			parts.AddSensor(sensorDevice, c)
		default:
			return fmt.Errorf("unknown component type: %v", c.Type)
		}
	}

	return nil
}

// RemoteByName returns the given remote robot by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) RemoteByName(name string) api.RemoteRobot {
	part, ok := parts.remotes[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.RemoteByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// BoardByName returns the given board by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) BoardByName(name string) board.Board {
	part, ok := parts.boards[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.BoardByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// ArmByName returns the given arm by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) ArmByName(name string) api.Arm {
	part, ok := parts.arms[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.ArmByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// BaseByName returns the given base by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) BaseByName(name string) api.Base {
	part, ok := parts.bases[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.BaseByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// GripperByName returns the given gripper by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) GripperByName(name string) api.Gripper {
	part, ok := parts.grippers[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.GripperByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// CameraByName returns the given camera by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) CameraByName(name string) gostream.ImageSource {
	part, ok := parts.cameras[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.CameraByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// LidarDeviceByName returns the given lidar device by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) LidarDeviceByName(name string) lidar.Device {
	part, ok := parts.lidarDevices[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.LidarDeviceByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// SensorByName returns the given sensor by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) SensorByName(name string) sensor.Device {
	part, ok := parts.sensors[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.SensorByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}

// ProviderByName returns the given provider by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) ProviderByName(name string) api.Provider {
	part, ok := parts.providers[name]
	if ok {
		return part
	}
	for _, remote := range parts.remotes {
		part := remote.ProviderByName(name)
		if part != nil {
			return part
		}
	}
	return nil
}


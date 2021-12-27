package robotimpl

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/base"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/sensor/compass"
	"go.viam.com/rdk/sensor/forcematrix"
	"go.viam.com/rdk/sensor/gps"
)

// robotParts are the actual parts that make up a robot.
type robotParts struct {
	remotes         map[string]*remoteRobot
	bases           map[string]*proxyBase
	sensors         map[string]sensor.Sensor
	services        map[string]interface{}
	functions       map[string]struct{}
	resources       map[resource.Name]interface{}
	processManager  pexec.ProcessManager
	robotClientOpts []client.RobotClientOption
}

// newRobotParts returns a properly initialized set of parts.
func newRobotParts(logger golog.Logger, opts ...client.RobotClientOption) *robotParts {
	return &robotParts{
		remotes:         map[string]*remoteRobot{},
		bases:           map[string]*proxyBase{},
		sensors:         map[string]sensor.Sensor{},
		services:        map[string]interface{}{},
		functions:       map[string]struct{}{},
		resources:       map[resource.Name]interface{}{},
		processManager:  pexec.NewProcessManager(logger),
		robotClientOpts: opts,
	}
}

// fixType ensures that the component has correct type information.
func fixType(c config.Component, whichType config.ComponentType) config.Component {
	if c.Type == "" {
		c.Type = whichType
	} else if c.Type != whichType {
		panic(fmt.Sprintf("different types (%s) != (%s)", whichType, c.Type))
	}
	return c
}

// addRemote adds a remote to the parts.
func (parts *robotParts) addRemote(r *remoteRobot, c config.Remote) {
	parts.remotes[c.Name] = r
}

// AddBase adds a base to the parts.
func (parts *robotParts) AddBase(b base.Base, c config.Component) {
	c = fixType(c, config.ComponentTypeBase)
	if proxy, ok := b.(*proxyBase); ok {
		b = proxy.actual
	}
	parts.bases[c.Name] = &proxyBase{actual: b}
}

// AddSensor adds a sensor to the parts.
func (parts *robotParts) AddSensor(s sensor.Sensor, c config.Component) {
	c = fixType(c, config.ComponentTypeSensor)
	switch pType := s.(type) {
	case *proxySensor:
		parts.sensors[c.Name] = &proxySensor{actual: pType.actual}
	case *proxyCompass:
		parts.sensors[c.Name] = newProxyCompass(pType.actual)
	case *proxyForceMatrix:
		parts.sensors[c.Name] = newProxyForceMatrix(pType.actual)
	case *proxyRelativeCompass:
		parts.sensors[c.Name] = newProxyRelativeCompass(pType.actual)
	case *proxyGPS:
		parts.sensors[c.Name] = newProxyGPS(pType.actual)
	default:
		switch s.Desc().Type {
		case compass.Type:
			parts.sensors[c.Name] = newProxyCompass(s.(compass.Compass))
		case compass.RelativeType:
			parts.sensors[c.Name] = newProxyRelativeCompass(s.(compass.RelativeCompass))
		case gps.Type:
			parts.sensors[c.Name] = newProxyGPS(s.(gps.GPS))
		case forcematrix.Type:
			parts.sensors[c.Name] = newProxyForceMatrix(s.(forcematrix.ForceMatrix))
		default:
			parts.sensors[c.Name] = &proxySensor{actual: s}
		}
	}
}

// AddService adds a service to the parts.
func (parts *robotParts) AddService(svc interface{}, c config.Service) {
	parts.services[c.Name] = svc
}

// addFunction adds a function to the parts.
func (parts *robotParts) addFunction(name string) {
	parts.functions[name] = struct{}{}
}

// addResource adds a resource to the parts.
func (parts *robotParts) addResource(name resource.Name, r interface{}) {
	parts.resources[name] = r
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
func (parts *robotParts) mergeNamesWithRemotes(names []string, namesFunc func(remote robot.Robot) []string) []string {
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

// mergeResourceNamesWithRemotes merges names from the parts itself as well as its
// remotes.
func (parts *robotParts) mergeResourceNamesWithRemotes(names []resource.Name) []resource.Name {
	// use this to filter out seen names and preserve order
	seen := make(map[resource.Name]struct{}, len(parts.resources))
	for _, name := range names {
		seen[name] = struct{}{}
	}
	for _, r := range parts.remotes {
		remoteNames := r.ResourceNames()
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
	for _, n := range parts.ResourceNames() {
		if n.Subtype == arm.Subtype {
			names = append(names, n.Name)
		}
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.ArmNames)
}

// GripperNames returns the names of all grippers in the parts.
func (parts *robotParts) GripperNames() []string {
	names := []string{}
	for _, n := range parts.ResourceNames() {
		if n.Subtype == gripper.Subtype {
			names = append(names, n.Name)
		}
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.GripperNames)
}

// CameraNames returns the names of all cameras in the parts.
func (parts *robotParts) CameraNames() []string {
	names := []string{}
	for _, n := range parts.ResourceNames() {
		if n.Subtype == camera.Subtype {
			names = append(names, n.Name)
		}
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.CameraNames)
}

// BaseNames returns the names of all bases in the parts.
func (parts *robotParts) BaseNames() []string {
	names := []string{}
	for k := range parts.bases {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.BaseNames)
}

// BoardNames returns the names of all boards in the parts.
func (parts *robotParts) BoardNames() []string {
	names := []string{}
	for _, n := range parts.ResourceNames() {
		if n.Subtype == board.Subtype {
			names = append(names, n.Name)
		}
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.BoardNames)
}

// SensorNames returns the names of all sensors in the parts.
func (parts *robotParts) SensorNames() []string {
	names := []string{}
	for k := range parts.sensors {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.SensorNames)
}

// ServoNames returns the names of all servos in the parts.
func (parts *robotParts) ServoNames() []string {
	names := []string{}
	for _, n := range parts.ResourceNames() {
		if n.Subtype == servo.Subtype {
			names = append(names, n.Name)
		}
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.ServoNames)
}

// MotorNames returns the names of all motors in the parts.
func (parts *robotParts) MotorNames() []string {
	names := []string{}
	for _, n := range parts.ResourceNames() {
		if n.Subtype == motor.Subtype {
			names = append(names, n.Name)
		}
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.MotorNames)
}

// InputControllerNames returns the names of all controllers in the parts.
func (parts *robotParts) InputControllerNames() []string {
	names := []string{}
	for _, n := range parts.ResourceNames() {
		if n.Subtype == input.Subtype {
			names = append(names, n.Name)
		}
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.InputControllerNames)
}

// FunctionNames returns the names of all functions in the parts.
func (parts *robotParts) FunctionNames() []string {
	names := []string{}
	for k := range parts.functions {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.FunctionNames)
}

// ServiceNames returns the names of all service in the parts.
func (parts *robotParts) ServiceNames() []string {
	names := []string{}
	for k := range parts.services {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.ServiceNames)
}

// ResourceNames returns the names of all resources in the parts.
func (parts *robotParts) ResourceNames() []resource.Name {
	names := []resource.Name{}
	for k := range parts.resources {
		names = append(names, k)
	}
	return parts.mergeResourceNamesWithRemotes(names)
}

// Clone provides a shallow copy of each part.
func (parts *robotParts) Clone() *robotParts {
	var clonedParts robotParts
	if len(parts.remotes) != 0 {
		clonedParts.remotes = make(map[string]*remoteRobot, len(parts.remotes))
		for k, v := range parts.remotes {
			clonedParts.remotes[k] = v
		}
	}
	if len(parts.bases) != 0 {
		clonedParts.bases = make(map[string]*proxyBase, len(parts.bases))
		for k, v := range parts.bases {
			clonedParts.bases[k] = v
		}
	}
	if len(parts.sensors) != 0 {
		clonedParts.sensors = make(map[string]sensor.Sensor, len(parts.sensors))
		for k, v := range parts.sensors {
			clonedParts.sensors[k] = v
		}
	}
	if len(parts.functions) != 0 {
		clonedParts.functions = make(map[string]struct{}, len(parts.functions))
		for k, v := range parts.functions {
			clonedParts.functions[k] = v
		}
	}
	if len(parts.services) != 0 {
		clonedParts.services = make(map[string]interface{}, len(parts.services))
		for k, v := range parts.services {
			clonedParts.services[k] = v
		}
	}
	if len(parts.resources) != 0 {
		clonedParts.resources = make(map[resource.Name]interface{}, len(parts.resources))
		for k, v := range parts.resources {
			clonedParts.resources[k] = v
		}
	}
	if parts.processManager != nil {
		clonedParts.processManager = parts.processManager.Clone()
	}
	return &clonedParts
}

// Close attempts to close/stop all parts.
func (parts *robotParts) Close(ctx context.Context) error {
	var allErrs error
	if err := parts.processManager.Stop(); err != nil {
		allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error stopping process manager"))
	}

	for _, x := range parts.services {
		if err := utils.TryClose(ctx, x); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing service"))
		}
	}

	for _, x := range parts.remotes {
		if err := utils.TryClose(ctx, x); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing remote"))
		}
	}

	for _, x := range parts.bases {
		if err := utils.TryClose(ctx, x); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing base"))
		}
	}

	for _, x := range parts.sensors {
		if err := utils.TryClose(ctx, x); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing sensor"))
		}
	}

	for _, x := range parts.resources {
		if err := utils.TryClose(ctx, x); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing resource"))
		}
	}

	return allErrs
}

// processConfig ingests a given config and constructs all constituent parts.
func (parts *robotParts) processConfig(
	ctx context.Context,
	config *config.Config,
	robot *localRobot,
	logger golog.Logger,
) error {
	if err := parts.newProcesses(ctx, config.Processes); err != nil {
		return err
	}

	if err := parts.newRemotes(ctx, config.Remotes, logger); err != nil {
		return err
	}

	if err := parts.newComponents(ctx, config.Components, robot); err != nil {
		return err
	}

	if err := parts.newServices(ctx, config.Services, robot); err != nil {
		return err
	}

	for _, f := range config.Functions {
		parts.addFunction(f.Name)
	}

	return nil
}

// processModifiedConfig ingests a given config and constructs all constituent parts.
func (parts *robotParts) processModifiedConfig(
	ctx context.Context,
	config *config.ModifiedConfigDiff,
	robot *localRobot,
	logger golog.Logger,
) error {
	if err := parts.newProcesses(ctx, config.Processes); err != nil {
		return err
	}

	if err := parts.newRemotes(ctx, config.Remotes, logger); err != nil {
		return err
	}

	if err := parts.newComponents(ctx, config.Components, robot); err != nil {
		return err
	}

	for _, f := range config.Functions {
		parts.addFunction(f.Name)
	}

	return nil
}

// newProcesses constructs all processes defined.
func (parts *robotParts) newProcesses(ctx context.Context, processes []pexec.ProcessConfig) error {
	for _, procConf := range processes {
		if _, err := parts.processManager.AddProcessFromConfig(ctx, procConf); err != nil {
			return err
		}
	}
	return parts.processManager.Start(ctx)
}

// newRemotes constructs all remotes defined and integrates their parts in.
func (parts *robotParts) newRemotes(ctx context.Context, remotes []config.Remote, logger golog.Logger) error {
	for _, config := range remotes {
		robotClient, err := client.New(ctx, config.Address, logger, parts.robotClientOpts...)
		if err != nil {
			return errors.Wrapf(err, "couldn't connect to robot remote (%s)", config.Address)
		}

		configCopy := config
		parts.addRemote(newRemoteRobot(robotClient, configCopy), configCopy)
	}
	return nil
}

// newComponents constructs all components defined.
func (parts *robotParts) newComponents(ctx context.Context, components []config.Component, r *localRobot) error {
	for _, c := range components {
		switch c.Type {
		case config.ComponentTypeBase:
			b, err := r.newBase(ctx, c)
			if err != nil {
				return err
			}
			parts.AddBase(b, c)
		case config.ComponentTypeSensor:
			if c.SubType == "" {
				return errors.New("sensor component requires subtype")
			}
			sensorDevice, err := r.newSensor(ctx, c, sensor.Type(c.SubType))
			if err != nil {
				return err
			}
			parts.AddSensor(sensorDevice, c)
		case config.ComponentTypeArm, config.ComponentTypeBoard, config.ComponentTypeCamera,
			config.ComponentTypeGantry, config.ComponentTypeGripper, config.ComponentTypeInputController,
			config.ComponentTypeMotor, config.ComponentTypeServo:
			fallthrough
		default:
			r, err := r.newResource(ctx, c)
			if err != nil {
				return err
			}
			rName := c.ResourceName()
			parts.addResource(rName, r)
		}
	}

	return nil
}

// newServices constructs all services defined.
func (parts *robotParts) newServices(ctx context.Context, services []config.Service, r *localRobot) error {
	for _, c := range services {
		svc, err := r.newService(ctx, c)
		if err != nil {
			return err
		}
		parts.AddService(svc, c)
	}

	return nil
}

// RemoteByName returns the given remote robot by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) RemoteByName(name string) (robot.Robot, bool) {
	part, ok := parts.remotes[name]
	if ok {
		return part, true
	}
	for _, remote := range parts.remotes {
		part, ok := remote.RemoteByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// BoardByName returns the given board by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) BoardByName(name string) (board.Board, bool) {
	rName := board.Named(name)
	r, ok := parts.resources[rName]
	if ok {
		part, ok := r.(board.Board)
		if ok {
			return part, true
		}
	}
	for _, remote := range parts.remotes {
		part, ok := remote.BoardByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// ArmByName returns the given arm by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) ArmByName(name string) (arm.Arm, bool) {
	rName := arm.Named(name)
	r, ok := parts.resources[rName]
	if ok {
		part, ok := r.(arm.Arm)
		if ok {
			return part, true
		}
	}
	for _, remote := range parts.remotes {
		part, ok := remote.ArmByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// BaseByName returns the given base by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) BaseByName(name string) (base.Base, bool) {
	part, ok := parts.bases[name]
	if ok {
		return part, true
	}
	for _, remote := range parts.remotes {
		part, ok := remote.BaseByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// GripperByName returns the given gripper by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) GripperByName(name string) (gripper.Gripper, bool) {
	rName := gripper.Named(name)
	r, ok := parts.resources[rName]
	if ok {
		part, ok := r.(gripper.Gripper)
		if ok {
			return part, true
		}
	}
	for _, remote := range parts.remotes {
		part, ok := remote.GripperByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// CameraByName returns the given camera by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) CameraByName(name string) (camera.Camera, bool) {
	rName := camera.Named(name)
	r, ok := parts.resources[rName]
	if ok {
		part, ok := r.(camera.Camera)
		if ok {
			return part, true
		}
	}
	for _, remote := range parts.remotes {
		part, ok := remote.CameraByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// SensorByName returns the given sensor by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) SensorByName(name string) (sensor.Sensor, bool) {
	part, ok := parts.sensors[name]
	if ok {
		return part, true
	}
	for _, remote := range parts.remotes {
		part, ok := remote.SensorByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// ServoByName returns the given servo by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) ServoByName(name string) (servo.Servo, bool) {
	servoResourceName := servo.Named(name)
	resource, ok := parts.resources[servoResourceName]
	if ok {
		part, ok := resource.(servo.Servo)
		if ok {
			return part, true
		}
	}
	for _, remote := range parts.remotes {
		part, ok := remote.ServoByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// MotorByName returns the given motor by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) MotorByName(name string) (motor.Motor, bool) {
	motorResourceName := motor.Named(name)
	resource, ok := parts.resources[motorResourceName]
	if ok {
		part, ok := resource.(motor.Motor)
		if ok {
			return part, true
		}
	}
	for _, remote := range parts.remotes {
		part, ok := remote.MotorByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// InputControllerByName returns the given input.Controller by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) InputControllerByName(name string) (input.Controller, bool) {
	rName := input.Named(name)
	resource, ok := parts.resources[rName]
	if ok {
		part, ok := resource.(input.Controller)
		if ok {
			return part, true
		}
	}
	for _, remote := range parts.remotes {
		part, ok := remote.InputControllerByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

func (parts *robotParts) ServiceByName(name string) (interface{}, bool) {
	part, ok := parts.services[name]
	if ok {
		return part, true
	}
	for _, remote := range parts.remotes {
		part, ok := remote.ServiceByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// ResourceByName returns the given resource by fully qualified name, if it exists;
// returns nil otherwise.
func (parts *robotParts) ResourceByName(name resource.Name) (interface{}, bool) {
	part, ok := parts.resources[name]
	if ok {
		return part, true
	}
	for _, remote := range parts.remotes {
		part, ok := remote.ResourceByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// PartsMergeResult is the result of merging in parts together.
type PartsMergeResult struct {
	ReplacedProcesses []pexec.ManagedProcess
}

// Process integrates the results into the given parts.
func (result *PartsMergeResult) Process(ctx context.Context, parts *robotParts) error {
	for _, proc := range result.ReplacedProcesses {
		if replaced, err := parts.processManager.AddProcess(ctx, proc, false); err != nil {
			return err
		} else if replaced != nil {
			return errors.Errorf("unexpected process replacement %v", replaced)
		}
	}
	return nil
}

// MergeAdd merges in the given added parts and returns results for
// later processing.
func (parts *robotParts) MergeAdd(toAdd *robotParts) (*PartsMergeResult, error) {
	if len(toAdd.remotes) != 0 {
		if parts.remotes == nil {
			parts.remotes = make(map[string]*remoteRobot, len(toAdd.remotes))
		}
		for k, v := range toAdd.remotes {
			parts.remotes[k] = v
		}
	}

	if len(toAdd.bases) != 0 {
		if parts.bases == nil {
			parts.bases = make(map[string]*proxyBase, len(toAdd.bases))
		}
		for k, v := range toAdd.bases {
			parts.bases[k] = v
		}
	}

	if len(toAdd.sensors) != 0 {
		if parts.sensors == nil {
			parts.sensors = make(map[string]sensor.Sensor, len(toAdd.sensors))
		}
		for k, v := range toAdd.sensors {
			parts.sensors[k] = v
		}
	}

	if len(toAdd.functions) != 0 {
		if parts.functions == nil {
			parts.functions = make(map[string]struct{}, len(toAdd.functions))
		}
		for k, v := range toAdd.functions {
			parts.functions[k] = v
		}
	}

	if len(toAdd.services) != 0 {
		if parts.services == nil {
			parts.services = make(map[string]interface{}, len(toAdd.services))
		}
		for k, v := range toAdd.services {
			parts.services[k] = v
		}
	}

	if len(toAdd.resources) != 0 {
		if parts.resources == nil {
			parts.resources = make(map[resource.Name]interface{}, len(toAdd.resources))
		}
		for k, v := range toAdd.resources {
			parts.resources[k] = v
		}
	}

	var result PartsMergeResult
	if toAdd.processManager != nil {
		// assume parts.processManager is non-nil
		replaced, err := pexec.MergeAddProcessManagers(parts.processManager, toAdd.processManager)
		if err != nil {
			return nil, err
		}
		result.ReplacedProcesses = replaced
	}

	return &result, nil
}

// MergeModify merges in the given modified parts and returns results for
// later processing.
func (parts *robotParts) MergeModify(ctx context.Context, toModify *robotParts, diff *config.Diff) (*PartsMergeResult, error) {
	var result PartsMergeResult
	if toModify.processManager != nil {
		// assume parts.processManager is non-nil
		// adding also replaces here
		replaced, err := pexec.MergeAddProcessManagers(parts.processManager, toModify.processManager)
		if err != nil {
			return nil, err
		}
		result.ReplacedProcesses = replaced
	}

	// this is the point of no return during reconfiguration

	if len(toModify.remotes) != 0 {
		for k, v := range toModify.remotes {
			old, ok := parts.remotes[k]
			if !ok {
				// should not happen
				continue
			}
			old.replace(ctx, v)
		}
	}

	if len(toModify.bases) != 0 {
		for k, v := range toModify.bases {
			old, ok := parts.bases[k]
			if !ok {
				// should not happen
				continue
			}
			old.replace(ctx, v)
		}
	}

	if len(toModify.sensors) != 0 {
		for k, v := range toModify.sensors {
			old, ok := parts.sensors[k]
			if !ok {
				// should not happen
				continue
			}
			old.(interface{ replace(newSensor sensor.Sensor) }).replace(v)
		}
	}

	// TODO(erd): how to handle service replacement?

	if len(toModify.resources) != 0 {
		for k, v := range toModify.resources {
			old, ok := parts.resources[k]
			if !ok {
				// should not happen
				continue
			}
			oldPart, ok := old.(resource.Reconfigurable)
			if !ok {
				return nil, errors.Errorf("old type %T is not reconfigurable", old)
			}
			newPart, ok := v.(resource.Reconfigurable)
			if !ok {
				return nil, errors.Errorf("new type %T is not reconfigurable", v)
			}
			if err := oldPart.Reconfigure(ctx, newPart); err != nil {
				return nil, err
			}
		}
	}

	return &result, nil
}

// MergeRemove merges in the given removed parts but does no work
// to stop the individual parts.
func (parts *robotParts) MergeRemove(toRemove *robotParts) {
	if len(toRemove.remotes) != 0 {
		for k := range toRemove.remotes {
			delete(parts.remotes, k)
		}
	}

	if len(toRemove.bases) != 0 {
		for k := range toRemove.bases {
			delete(parts.bases, k)
		}
	}

	if len(toRemove.sensors) != 0 {
		for k := range toRemove.sensors {
			delete(parts.sensors, k)
		}
	}

	if len(toRemove.functions) != 0 {
		for k := range toRemove.functions {
			delete(parts.functions, k)
		}
	}

	if len(toRemove.services) != 0 {
		for k := range toRemove.services {
			delete(parts.services, k)
		}
	}

	if len(toRemove.resources) != 0 {
		for k := range toRemove.resources {
			delete(parts.resources, k)
		}
	}

	if toRemove.processManager != nil {
		// assume parts.processManager is non-nil
		// ignoring result as we will filter out the processes to remove and stop elsewhere
		pexec.MergeRemoveProcessManagers(parts.processManager, toRemove.processManager)
	}
}

// FilterFromConfig returns a shallow copy of the parts reflecting
// a given config.
func (parts *robotParts) FilterFromConfig(ctx context.Context, conf *config.Config, logger golog.Logger) (*robotParts, error) {
	filtered := newRobotParts(logger, parts.robotClientOpts...)

	for _, conf := range conf.Processes {
		proc, ok := parts.processManager.ProcessByID(conf.ID)
		if !ok {
			continue
		}
		if _, err := filtered.processManager.AddProcess(ctx, proc, false); err != nil {
			return nil, err
		}
	}

	for _, conf := range conf.Remotes {
		part, ok := parts.remotes[conf.Name]
		if !ok {
			continue
		}
		filtered.addRemote(part, conf)
	}

	for _, compConf := range conf.Components {
		switch compConf.Type {
		case config.ComponentTypeBase:
			part, ok := parts.BaseByName(compConf.Name)
			if !ok {
				continue
			}
			filtered.AddBase(part, compConf)
		case config.ComponentTypeSensor:
			part, ok := parts.SensorByName(compConf.Name)
			if !ok {
				continue
			}
			filtered.AddSensor(part, compConf)
		case config.ComponentTypeArm, config.ComponentTypeBoard, config.ComponentTypeCamera,
			config.ComponentTypeGantry, config.ComponentTypeGripper, config.ComponentTypeInputController,
			config.ComponentTypeMotor, config.ComponentTypeServo:
			fallthrough
		default:
			rName := compConf.ResourceName()
			resource, ok := parts.ResourceByName(rName)
			if !ok {
				continue
			}
			filtered.addResource(rName, resource)
		}
	}

	for _, conf := range conf.Functions {
		_, ok := parts.functions[conf.Name]
		if !ok {
			continue
		}
		filtered.addFunction(conf.Name)
	}

	for _, conf := range conf.Services {
		part, ok := parts.services[conf.Name]
		if !ok {
			continue
		}
		filtered.AddService(part, conf)
	}

	return filtered, nil
}

// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"math"
	"runtime/debug"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	metadataclient "go.viam.com/rdk/grpc/metadata/client"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/sensor/compass"
	"go.viam.com/rdk/spatialmath"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	address        string
	conn           rpc.ClientConn
	client         pb.RobotServiceClient
	metadataClient *metadataclient.MetadataServiceClient

	namesMu       *sync.RWMutex
	baseNames     []string
	sensorNames   []string
	functionNames []string
	serviceNames  []string
	resourceNames []resource.Name

	sensorTypes map[string]sensor.Type

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()
	logger                  golog.Logger

	cachingStatus  bool
	cachedStatus   *pb.Status
	cachedStatusMu *sync.Mutex

	closeContext context.Context
}

// New constructs a new RobotClient that is served at the given address. The given
// context can be used to cancel the operation.
func New(ctx context.Context, address string, logger golog.Logger, opts ...RobotClientOption) (*RobotClient, error) {
	var rOpts robotClientOpts
	for _, opt := range opts {
		opt.apply(&rOpts)
	}

	conn, err := grpc.Dial(ctx, address, logger, rOpts.dialOptions...)
	if err != nil {
		return nil, err
	}

	metadataClient, err := metadataclient.New(ctx, address, logger, rOpts.dialOptions...)
	if err != nil {
		return nil, err
	}

	client := pb.NewRobotServiceClient(conn)
	closeCtx, cancel := context.WithCancel(context.Background())
	rc := &RobotClient{
		address:                 address,
		conn:                    conn,
		client:                  client,
		metadataClient:          metadataClient,
		sensorTypes:             map[string]sensor.Type{},
		cancelBackgroundWorkers: cancel,
		namesMu:                 &sync.RWMutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		logger:                  logger,
		cachedStatusMu:          &sync.Mutex{},
		closeContext:            closeCtx,
	}
	// refresh once to hydrate the robot.
	if err := rc.Refresh(ctx); err != nil {
		return nil, multierr.Combine(err, metadataClient.Close(), conn.Close())
	}
	if rOpts.refreshEvery != 0 {
		rc.cachingStatus = true
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(closeCtx, rOpts.refreshEvery)
		}, rc.activeBackgroundWorkers.Done)
	}
	return rc, nil
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close(ctx context.Context) error {
	rc.cancelBackgroundWorkers()
	rc.activeBackgroundWorkers.Wait()

	return multierr.Combine(rc.conn.Close(), rc.metadataClient.Close())
}

// RefreshEvery refreshes the robot on the interval given by every until the
// given context is done.
func (rc *RobotClient) RefreshEvery(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		if !utils.SelectContextOrWaitChan(ctx, ticker.C) {
			return
		}

		if err := rc.Refresh(ctx); err != nil {
			// we want to keep refreshing and hopefully the ticker is not
			// too fast so that we do not thrash.
			rc.Logger().Errorw("failed to refresh status", "error", err)
		}
	}
}

// storeStatus atomically stores the status response from a robot server if and only
// if we are automatically refreshing.
func (rc *RobotClient) storeStatus(status *pb.Status) {
	if !rc.cachingStatus {
		return
	}
	rc.cachedStatusMu.Lock()
	rc.cachedStatus = status
	rc.cachedStatusMu.Unlock()
}

// storeStatus atomically gets the status response from a robot server if and only
// if we are automatically refreshing.
func (rc *RobotClient) getCachedStatus() *pb.Status {
	if !rc.cachingStatus {
		return nil
	}
	rc.cachedStatusMu.Lock()
	defer rc.cachedStatusMu.Unlock()
	return rc.cachedStatus
}

// status actually gets the latest status from the server.
func (rc *RobotClient) status(ctx context.Context) (*pb.Status, error) {
	resp, err := rc.client.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

// Status either gets a cached or latest version of the status of the remote
// robot.
func (rc *RobotClient) Status(ctx context.Context) (*pb.Status, error) {
	if status := rc.getCachedStatus(); status != nil {
		return status, nil
	}
	return rc.status(ctx)
}

// Config gets the config from the remote robot
// It is only partial a config, including the pieces relevant to remote robots,
// And not the pieces relevant to local configuration (pins, security keys, etc...)
func (rc *RobotClient) Config(ctx context.Context) (*config.Config, error) {
	remoteConfig, err := rc.client.Config(ctx, &pb.ConfigRequest{})
	if err != nil {
		return nil, err
	}

	var cfg config.Config
	for _, c := range remoteConfig.Components {
		cc := config.Component{
			Name: c.Name,
			Type: config.ComponentType(c.Type),
		}
		// check if component has frame attribute, leave as nil if it doesn't
		if c.Parent != "" {
			cc.Frame = &config.Frame{Parent: c.Parent}
		}
		if cc.Frame != nil && c.Pose != nil {
			cc.Frame.Translation = spatialmath.Translation{
				X: c.Pose.X,
				Y: c.Pose.Y,
				Z: c.Pose.Z,
			}
			cc.Frame.Orientation = &spatialmath.OrientationVectorDegrees{
				OX:    c.Pose.OX,
				OY:    c.Pose.OY,
				OZ:    c.Pose.OZ,
				Theta: c.Pose.Theta,
			}
		}
		cfg.Components = append(cfg.Components, cc)
	}
	return &cfg, nil
}

// RemoteByName returns a remote robot by name. It is assumed to exist on the
// other end. Right now this method is unimplemented.
func (rc *RobotClient) RemoteByName(name string) (robot.Robot, bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

// ArmByName returns an arm by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) ArmByName(name string) (arm.Arm, bool) {
	resource, ok := rc.ResourceByName(arm.Named(name))
	if !ok {
		return nil, false
	}
	actualArm, ok := resource.(arm.Arm)
	if !ok {
		return nil, false
	}
	return actualArm, true
}

// BaseByName returns a base by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) BaseByName(name string) (base.Base, bool) {
	return &baseClient{rc, name}, true
}

// GripperByName returns a gripper by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) GripperByName(name string) (gripper.Gripper, bool) {
	resource, ok := rc.ResourceByName(gripper.Named(name))
	if !ok {
		return nil, false
	}
	actual, ok := resource.(gripper.Gripper)
	if !ok {
		return nil, false
	}
	return actual, true
}

// CameraByName returns a camera by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) CameraByName(name string) (camera.Camera, bool) {
	resource, ok := rc.ResourceByName(camera.Named(name))
	if !ok {
		return nil, false
	}
	actual, ok := resource.(camera.Camera)
	if !ok {
		return nil, false
	}
	return actual, true
}

// BoardByName returns a board by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) BoardByName(name string) (board.Board, bool) {
	resource, ok := rc.ResourceByName(board.Named(name))
	if !ok {
		return nil, false
	}
	actualBoard, ok := resource.(board.Board)
	if !ok {
		return nil, false
	}
	return actualBoard, true
}

// SensorByName returns a sensor by name. It is assumed to exist on the
// other end. Based on the known sensor names and types, a type specific
// sensor is attempted to be returned; otherwise it's a general purpose
// sensor.
func (rc *RobotClient) SensorByName(name string) (sensor.Sensor, bool) {
	sensorType := rc.sensorTypes[name]
	sc := &sensorClient{rc, name, sensorType}
	switch sensorType {
	case compass.Type:
		return &compassClient{sc}, true
	case compass.RelativeType:
		return &relativeCompassClient{&compassClient{sc}}, true
	default:
		return sc, true
	}
}

// ServoByName returns a servo by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) ServoByName(name string) (servo.Servo, bool) {
	nameObj := servo.Named(name)
	resource, ok := rc.ResourceByName(nameObj)
	if !ok {
		return nil, false
	}
	actualServo, ok := resource.(servo.Servo)
	if !ok {
		return nil, false
	}
	return actualServo, true
}

// MotorByName returns a motor by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) MotorByName(name string) (motor.Motor, bool) {
	nameObj := motor.Named(name)
	resource, ok := rc.ResourceByName(nameObj)
	if !ok {
		return nil, false
	}
	actualMotor, ok := resource.(motor.Motor)
	if !ok {
		return nil, false
	}
	return actualMotor, true
}

// InputControllerByName returns an input.Controller by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) InputControllerByName(name string) (input.Controller, bool) {
	resource, ok := rc.ResourceByName(input.Named(name))
	if !ok {
		return nil, false
	}
	actual, ok := resource.(input.Controller)
	if !ok {
		return nil, false
	}
	return actual, true
}

// ResourceByName returns resource by name.
func (rc *RobotClient) ResourceByName(name resource.Name) (interface{}, bool) {
	// TODO(https://github.com/viamrobotics/rdk/issues/375): remove this switch statement after the V2 migration is done
	switch name.Subtype {
	case base.Subtype:
		return &baseClient{rc, name.Name}, true
	default:
		c := registry.ResourceSubtypeLookup(name.Subtype)
		if c == nil || c.RPCClient == nil {
			// registration doesn't exist
			return nil, false
		}
		// pass in conn
		resourceClient := c.RPCClient(rc.closeContext, rc.conn, name.Name, rc.Logger())
		return resourceClient, true
	}
}

// Refresh manually updates the underlying parts of the robot based
// on a status retrieved from the server.
// TODO(https://github.com/viamrobotics/rdk/issues/57) - do not use status
// as we plan on making it a more expensive request with more details than
// needed for the purposes of this method.
func (rc *RobotClient) Refresh(ctx context.Context) (err error) {
	status, err := rc.status(ctx)
	if err != nil {
		return errors.Wrap(err, "status call failed")
	}

	rc.storeStatus(status)
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()

	// TODO: placeholder implementation
	// call metadata service.
	names, err := rc.metadataClient.Resources(ctx)
	// only return if it is not unimplemented - means a bigger error came up
	if err != nil && grpcstatus.Code(err) != codes.Unimplemented {
		return err
	}
	if err == nil {
		rc.resourceNames = make([]resource.Name, 0, len(names))
		for _, name := range names {
			newName := resource.NewName(
				resource.Namespace(name.Namespace),
				resource.TypeName(name.Type),
				resource.SubtypeName(name.Subtype),
				name.Name,
			)
			rc.resourceNames = append(rc.resourceNames, newName)
		}
	}

	rc.baseNames = nil
	if len(status.Bases) != 0 {
		rc.baseNames = make([]string, 0, len(status.Bases))
		for name := range status.Bases {
			rc.baseNames = append(rc.baseNames, name)
		}
	}

	rc.sensorNames = nil
	if len(status.Sensors) != 0 {
		rc.sensorNames = make([]string, 0, len(status.Sensors))
		for name, sensorStatus := range status.Sensors {
			rc.sensorNames = append(rc.sensorNames, name)
			rc.sensorTypes[name] = sensor.Type(sensorStatus.Type)
		}
	}
	rc.functionNames = nil
	if len(status.Functions) != 0 {
		rc.functionNames = make([]string, 0, len(status.Functions))
		for name := range status.Functions {
			rc.functionNames = append(rc.functionNames, name)
		}
	}
	rc.serviceNames = nil
	if len(status.Services) != 0 {
		rc.serviceNames = make([]string, 0, len(status.Services))
		for name := range status.Services {
			rc.serviceNames = append(rc.serviceNames, name)
		}
	}
	return nil
}

// copyStringSlice is a helper to simply copy a string slice
// so that no one mutates it.
func copyStringSlice(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// RemoteNames returns the names of all known remotes.
func (rc *RobotClient) RemoteNames() []string {
	return nil
}

// ArmNames returns the names of all known arms.
func (rc *RobotClient) ArmNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, v := range rc.ResourceNames() {
		if v.Subtype == arm.Subtype {
			names = append(names, v.Name)
		}
	}
	return copyStringSlice(names)
}

// GripperNames returns the names of all known grippers.
func (rc *RobotClient) GripperNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, v := range rc.ResourceNames() {
		if v.Subtype == gripper.Subtype {
			names = append(names, v.Name)
		}
	}
	return copyStringSlice(names)
}

// CameraNames returns the names of all known cameras.
func (rc *RobotClient) CameraNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, v := range rc.ResourceNames() {
		if v.Subtype == camera.Subtype {
			names = append(names, v.Name)
		}
	}
	return copyStringSlice(names)
}

// BaseNames returns the names of all known bases.
func (rc *RobotClient) BaseNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.baseNames)
}

// BoardNames returns the names of all known boards.
func (rc *RobotClient) BoardNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, v := range rc.ResourceNames() {
		if v.Subtype == board.Subtype {
			names = append(names, v.Name)
		}
	}
	return copyStringSlice(names)
}

// SensorNames returns the names of all known sensors.
func (rc *RobotClient) SensorNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.sensorNames)
}

// ServoNames returns the names of all known servos.
func (rc *RobotClient) ServoNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, res := range rc.ResourceNames() {
		if res.Subtype == servo.Subtype {
			names = append(names, res.Name)
		}
	}
	return copyStringSlice(names)
}

// MotorNames returns the names of all known motors.
func (rc *RobotClient) MotorNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []string{}
	for _, res := range rc.ResourceNames() {
		if res.Subtype == motor.Subtype {
			names = append(names, res.Name)
		}
	}
	return copyStringSlice(names)
}

// InputControllerNames returns the names of all known input controllers.
func (rc *RobotClient) InputControllerNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	names := []string{}
	for _, res := range rc.ResourceNames() {
		if res.Subtype == input.Subtype {
			names = append(names, res.Name)
		}
	}
	return copyStringSlice(names)
}

// FunctionNames returns the names of all known functions.
func (rc *RobotClient) FunctionNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.functionNames)
}

// ProcessManager returns a useless process manager for the sake of
// satisfying the robot.Robot interface. Maybe it should not be part
// of the interface!
func (rc *RobotClient) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

// ResourceNames returns all resource names.
func (rc *RobotClient) ResourceNames() []resource.Name {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	names := []resource.Name{}
	for _, v := range rc.resourceNames {
		names = append(
			names,
			resource.NewName(
				v.Namespace, v.ResourceType, v.ResourceSubtype, v.Name,
			),
		)
	}
	return names
}

// Logger returns the logger being used for this robot.
func (rc *RobotClient) Logger() golog.Logger {
	return rc.logger
}

// FrameSystem retrieves an ordered slice of the frame configs and then builds a FrameSystem from the configs.
func (rc *RobotClient) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	// request the full config from the remote robot's frame system service.FrameSystemConfig()
	resp, err := rc.client.FrameServiceConfig(ctx, &pb.FrameServiceConfigRequest{})
	if err != nil {
		return nil, err
	}
	configs := resp.FrameSystemConfigs
	// using the configs, build a FrameSystem using model frames and static offset frames, the configs slice should already be sorted.
	for _, conf := range configs {
		part, err := config.ProtobufToFrameSystemPart(conf)
		if err != nil {
			return nil, err
		}
		// rename everything with prefixes
		part.Name = prefix + part.Name
		if part.FrameConfig.Parent != referenceframe.World {
			part.FrameConfig.Parent = prefix + part.FrameConfig.Parent
		}
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := config.CreateFramesFromPart(part, rc.Logger())
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		err = fs.AddFrame(staticOffsetFrame, fs.GetFrame(part.FrameConfig.Parent))
		if err != nil {
			return nil, err
		}
		err = fs.AddFrame(modelFrame, staticOffsetFrame)
		if err != nil {
			return nil, err
		}
	}
	return fs, nil
}

// baseClient satisfies a gRPC based base.Base. Refer to the interface
// for descriptions of its methods.
type baseClient struct {
	rc   *RobotClient
	name string
}

func (bc *baseClient) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	resp, err := bc.rc.client.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
		Name:           bc.name,
		MillisPerSec:   millisPerSec,
		DistanceMillis: int64(distanceMillis),
	})
	if err != nil {
		return err
	}
	if resp.Success {
		return nil
	}
	return errors.New(resp.Error)
}

func (bc *baseClient) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, degsPerSec float64, block bool) error {
	resp, err := bc.rc.client.BaseMoveArc(ctx, &pb.BaseMoveArcRequest{
		Name:           bc.name,
		MillisPerSec:   millisPerSec,
		AngleDeg:       degsPerSec,
		DistanceMillis: int64(distanceMillis),
	})
	if err != nil {
		return err
	}
	if resp.Success {
		return nil
	}
	return errors.New(resp.Error)
}

func (bc *baseClient) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	resp, err := bc.rc.client.BaseSpin(ctx, &pb.BaseSpinRequest{
		Name:       bc.name,
		AngleDeg:   angleDeg,
		DegsPerSec: degsPerSec,
	})
	if err != nil {
		return err
	}
	if resp.Success {
		return nil
	}
	return errors.New(resp.Error)
}

func (bc *baseClient) Stop(ctx context.Context) error {
	_, err := bc.rc.client.BaseStop(ctx, &pb.BaseStopRequest{
		Name: bc.name,
	})
	return err
}

func (bc *baseClient) WidthGet(ctx context.Context) (int, error) {
	resp, err := bc.rc.client.BaseWidthMillis(ctx, &pb.BaseWidthMillisRequest{
		Name: bc.name,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.WidthMillis), nil
}

// sensorClient satisfies a gRPC based sensor.Sensor. Refer to the interface
// for descriptions of its methods.
type sensorClient struct {
	rc         *RobotClient
	name       string
	sensorType sensor.Type
}

func (sc *sensorClient) Readings(ctx context.Context) ([]interface{}, error) {
	resp, err := sc.rc.client.SensorReadings(ctx, &pb.SensorReadingsRequest{
		Name: sc.name,
	})
	if err != nil {
		return nil, err
	}
	readings := make([]interface{}, 0, len(resp.Readings))
	for _, r := range resp.Readings {
		readings = append(readings, r.AsInterface())
	}
	return readings, nil
}

func (sc *sensorClient) Desc() sensor.Description {
	return sensor.Description{sc.sensorType, ""}
}

// compassClient satisfies a gRPC based compass.Compass. Refer to the interface
// for descriptions of its methods.
type compassClient struct {
	*sensorClient
}

func (cc *compassClient) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := cc.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (cc *compassClient) Heading(ctx context.Context) (float64, error) {
	resp, err := cc.rc.client.CompassHeading(ctx, &pb.CompassHeadingRequest{
		Name: cc.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.Heading, nil
}

func (cc *compassClient) StartCalibration(ctx context.Context) error {
	_, err := cc.rc.client.CompassStartCalibration(ctx, &pb.CompassStartCalibrationRequest{
		Name: cc.name,
	})
	return err
}

func (cc *compassClient) StopCalibration(ctx context.Context) error {
	_, err := cc.rc.client.CompassStopCalibration(ctx, &pb.CompassStopCalibrationRequest{
		Name: cc.name,
	})
	return err
}

func (cc *compassClient) Desc() sensor.Description {
	return sensor.Description{compass.Type, ""}
}

// relativeCompassClient satisfies a gRPC based compass.RelativeCompass. Refer to the interface
// for descriptions of its methods.
type relativeCompassClient struct {
	*compassClient
}

func (rcc *relativeCompassClient) Mark(ctx context.Context) error {
	_, err := rcc.rc.client.CompassMark(ctx, &pb.CompassMarkRequest{
		Name: rcc.name,
	})
	return err
}

func (rcc *relativeCompassClient) Desc() sensor.Description {
	return sensor.Description{compass.RelativeType, ""}
}

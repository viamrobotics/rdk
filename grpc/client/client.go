// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"math"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-errors/errors"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/multierr"

	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc/dialer"

	rpcclient "go.viam.com/utils/rpc/client"

	"go.viam.com/core/base"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/component/board"
	"go.viam.com/core/component/camera"
	"go.viam.com/core/component/gripper"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/component/servo"
	"go.viam.com/core/config"
	"go.viam.com/core/grpc"
	metadataclient "go.viam.com/core/grpc/metadata/client"
	"go.viam.com/core/input"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/sensor/forcematrix"
	"go.viam.com/core/sensor/gps"
	"go.viam.com/core/spatialmath"

	"github.com/edaniels/golog"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	address        string
	conn           dialer.ClientConn
	client         pb.RobotServiceClient
	metadataClient *metadataclient.MetadataServiceClient

	namesMu              *sync.RWMutex
	baseNames            []string
	boardNames           []boardInfo
	sensorNames          []string
	motorNames           []string
	inputControllerNames []string
	functionNames        []string
	serviceNames         []string
	resourceNames        []resource.Name

	sensorTypes map[string]sensor.Type

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()
	logger                  golog.Logger

	cachingStatus  bool
	cachedStatus   *pb.Status
	cachedStatusMu *sync.Mutex

	closeContext context.Context
}

// RobotClientOptions are extra construction time options.
type RobotClientOptions struct {
	// RefreshEvery is how often to refresh the status/parts of the
	// robot. If unset, it will not be refreshed automatically.
	RefreshEvery time.Duration

	// DialOptions are options using for clients dialing gRPC servers.
	DialOptions rpcclient.DialOptions
}

// NewClientWithOptions constructs a new RobotClient that is served at the given address. The given
// context can be used to cancel the operation. Additionally, construction time options can be given.
func NewClientWithOptions(ctx context.Context, address string, opts RobotClientOptions, logger golog.Logger) (*RobotClient, error) {
	conn, err := grpc.Dial(ctx, address, opts.DialOptions, logger)
	if err != nil {
		return nil, err
	}

	metadataClient, err := metadataclient.NewClient(
		ctx,
		address,
		// TODO(https://github.com/viamrobotics/core/issues/237): configurable
		rpcclient.DialOptions{Insecure: true},
		logger,
	)
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
		return nil, err
	}
	if opts.RefreshEvery != 0 {
		rc.cachingStatus = true
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(closeCtx, opts.RefreshEvery)
		}, rc.activeBackgroundWorkers.Done)
	}
	return rc, nil
}

type boardInfo struct {
	name                  string
	spiNames              []string
	i2cNames              []string
	analogReaderNames     []string
	digitalInterruptNames []string
}

// NewClient constructs a new RobotClient that is served at the given address. The given
// context can be used to cancel the operation.
func NewClient(ctx context.Context, address string, logger golog.Logger) (*RobotClient, error) {
	return NewClientWithOptions(ctx, address, RobotClientOptions{
		DialOptions: rpcclient.DialOptions{
			// TODO(https://github.com/viamrobotics/core/issues/237): configurable
			Insecure: true,
		},
	}, logger)
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close() error {
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
	for _, info := range rc.boardNames {
		if info.name == name {
			return &boardClient{rc, info}, true
		}
	}
	return nil, false
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
	case gps.Type:
		return &gpsClient{sc}, true
	case forcematrix.Type:
		return &forcematrixClient{sc}, true
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
	return &motorClient{
		rc:   rc,
		name: name,
	}, true
}

// InputControllerByName returns an input.Controller by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) InputControllerByName(name string) (input.Controller, bool) {
	return &inputControllerClient{
		rc:   rc,
		name: name,
	}, true
}

// ServiceByName returns a service by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) ServiceByName(name string) (interface{}, bool) {
	// TODO(erd): implement
	return nil, false
}

// ResourceByName returns resource by name.
func (rc *RobotClient) ResourceByName(name resource.Name) (interface{}, bool) {
	// TODO(maximpertsov): remove this switch statement after the V2 migration is done
	switch name.Subtype {
	case board.Subtype:
		return rc.BoardByName(name.Name)
	case motor.Subtype:
		return &motorClient{rc: rc, name: name.Name}, true
	default:
		c := registry.ResourceSubtypeLookup(name.Subtype)
		if c == nil || c.RPCClient == nil {
			// registration doesn't exist
			return nil, false
		}
		// pass in conn
		resourceClient := c.RPCClient(rc.conn, name.Name, rc.Logger())
		return resourceClient, true
	}
}

// Refresh manually updates the underlying parts of the robot based
// on a status retrieved from the server.
// TODO(https://github.com/viamrobotics/core/issues/57) - do not use status
// as we plan on making it a more expensive request with more details than
// needed for the purposes of this method.
func (rc *RobotClient) Refresh(ctx context.Context) (err error) {
	status, err := rc.status(ctx)
	if err != nil {
		return errors.Errorf("status call failed: %w", err)
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

	rc.boardNames = nil
	if len(status.Boards) != 0 {
		rc.boardNames = make([]boardInfo, 0, len(status.Boards))
		for name, boardStatus := range status.Boards {
			info := boardInfo{name: name}
			if len(boardStatus.Analogs) != 0 {
				info.analogReaderNames = make([]string, 0, len(boardStatus.Analogs))
				for name := range boardStatus.Analogs {
					info.analogReaderNames = append(info.analogReaderNames, name)
				}
			}
			if len(boardStatus.DigitalInterrupts) != 0 {
				info.digitalInterruptNames = make([]string, 0, len(boardStatus.DigitalInterrupts))
				for name := range boardStatus.DigitalInterrupts {
					info.digitalInterruptNames = append(info.digitalInterruptNames, name)
				}
			}
			rc.boardNames = append(rc.boardNames, info)
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
	rc.motorNames = nil
	if len(status.Motors) != 0 {
		rc.motorNames = make([]string, 0, len(status.Motors))
		for name := range status.Motors {
			rc.motorNames = append(rc.motorNames, name)
		}
	}
	rc.inputControllerNames = nil
	if len(status.InputControllers) != 0 {
		rc.inputControllerNames = make([]string, 0, len(status.InputControllers))
		for name := range status.InputControllers {
			rc.inputControllerNames = append(rc.inputControllerNames, name)
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
	out := make([]string, 0, len(rc.boardNames))
	for _, info := range rc.boardNames {
		out = append(out, info.name)
	}
	return out
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
	return copyStringSlice(rc.motorNames)
}

// InputControllerNames returns the names of all known input controllers.
func (rc *RobotClient) InputControllerNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.inputControllerNames)
}

// FunctionNames returns the names of all known functions.
func (rc *RobotClient) FunctionNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.functionNames)
}

// ServiceNames returns the names of all known services.
func (rc *RobotClient) ServiceNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.serviceNames)
}

// ProcessManager returns a useless process manager for the sake of
// satisfying the robot.Robot interface. Maybe it should not be part
// of the interface!
func (rc *RobotClient) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

// ResourceNames returns all resource names
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

// FrameSystem retrieves an ordered slice of the frame configs and then builds a FrameSystem from the configs
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

func (bc *baseClient) WidthMillis(ctx context.Context) (int, error) {
	resp, err := bc.rc.client.BaseWidthMillis(ctx, &pb.BaseWidthMillisRequest{
		Name: bc.name,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.WidthMillis), nil
}

// boardClient satisfies a gRPC based board.Board. Refer to the interface
// for descriptions of its methods.
type boardClient struct {
	rc   *RobotClient
	info boardInfo
}

// SPIByName may need to be implemented
func (bc *boardClient) SPIByName(name string) (board.SPI, bool) {
	return nil, false
}

// I2CByName may need to be implemented
func (bc *boardClient) I2CByName(name string) (board.I2C, bool) {
	return nil, false
}

func (bc *boardClient) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	return &analogReaderClient{
		rc:               bc.rc,
		boardName:        bc.info.name,
		analogReaderName: name,
	}, true
}

func (bc *boardClient) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	return &digitalInterruptClient{
		rc:                   bc.rc,
		boardName:            bc.info.name,
		digitalInterruptName: name,
	}, true
}

func (bc *boardClient) GPIOSet(ctx context.Context, pin string, high bool) error {
	_, err := bc.rc.client.BoardGPIOSet(ctx, &pb.BoardGPIOSetRequest{
		Name: bc.info.name,
		Pin:  pin,
		High: high,
	})
	return err
}

func (bc *boardClient) GPIOGet(ctx context.Context, pin string) (bool, error) {
	resp, err := bc.rc.client.BoardGPIOGet(ctx, &pb.BoardGPIOGetRequest{
		Name: bc.info.name,
		Pin:  pin,
	})
	if err != nil {
		return false, err
	}
	return resp.High, nil
}

func (bc *boardClient) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	_, err := bc.rc.client.BoardPWMSet(ctx, &pb.BoardPWMSetRequest{
		Name:      bc.info.name,
		Pin:       pin,
		DutyCycle: uint32(dutyCycle),
	})
	return err
}

func (bc *boardClient) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	_, err := bc.rc.client.BoardPWMSetFrequency(ctx, &pb.BoardPWMSetFrequencyRequest{
		Name:      bc.info.name,
		Pin:       pin,
		Frequency: uint64(freq),
	})
	return err
}

func (bc *boardClient) SPINames() []string {
	return copyStringSlice(bc.info.spiNames)
}

func (bc *boardClient) I2CNames() []string {
	return copyStringSlice(bc.info.i2cNames)
}

func (bc *boardClient) AnalogReaderNames() []string {
	return copyStringSlice(bc.info.analogReaderNames)
}

func (bc *boardClient) DigitalInterruptNames() []string {
	return copyStringSlice(bc.info.digitalInterruptNames)
}

// Status uses the parent robot client's cached status or a newly fetched
// board status to return the state of the board.
func (bc *boardClient) Status(ctx context.Context) (*pb.BoardStatus, error) {
	if status := bc.rc.getCachedStatus(); status != nil {
		boardStatus, ok := status.Boards[bc.info.name]
		if !ok {
			return nil, errors.Errorf("no board with name (%s)", bc.info.name)
		}
		return boardStatus, nil
	}
	resp, err := bc.rc.client.BoardStatus(ctx, &pb.BoardStatusRequest{
		Name: bc.info.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (bc *boardClient) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{Remote: true}
}

// Close shuts the board down, no methods should be called on the board after this
func (bc *boardClient) Close() error {
	return nil
}

// analogReaderClient satisfies a gRPC based motor.Motor. Refer to the interface
// for descriptions of its methods.
type analogReaderClient struct {
	rc               *RobotClient
	boardName        string
	analogReaderName string
}

func (arc *analogReaderClient) Read(ctx context.Context) (int, error) {
	resp, err := arc.rc.client.BoardAnalogReaderRead(ctx, &pb.BoardAnalogReaderReadRequest{
		BoardName:        arc.boardName,
		AnalogReaderName: arc.analogReaderName,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Value), nil
}

// digitalInterruptClient satisfies a gRPC based motor.Motor. Refer to the interface
// for descriptions of its methods.
type digitalInterruptClient struct {
	rc                   *RobotClient
	boardName            string
	digitalInterruptName string
}

func (dic *digitalInterruptClient) Config(ctx context.Context) (board.DigitalInterruptConfig, error) {
	resp, err := dic.rc.client.BoardDigitalInterruptConfig(ctx, &pb.BoardDigitalInterruptConfigRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
	})
	if err != nil {
		return board.DigitalInterruptConfig{}, err
	}
	return DigitalInterruptConfigFromProto(resp.Config), nil
}

// DigitalInterruptConfigFromProto converts a proto based digital interrupt config to the
// codebase specific version.
func DigitalInterruptConfigFromProto(config *pb.DigitalInterruptConfig) board.DigitalInterruptConfig {
	return board.DigitalInterruptConfig{
		Name:    config.Name,
		Pin:     config.Pin,
		Type:    config.Type,
		Formula: config.Formula,
	}
}

func (dic *digitalInterruptClient) Value(ctx context.Context) (int64, error) {
	resp, err := dic.rc.client.BoardDigitalInterruptValue(ctx, &pb.BoardDigitalInterruptValueRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
	})
	if err != nil {
		return 0, err
	}
	return resp.Value, nil
}

func (dic *digitalInterruptClient) Tick(ctx context.Context, high bool, nanos uint64) error {
	_, err := dic.rc.client.BoardDigitalInterruptTick(ctx, &pb.BoardDigitalInterruptTickRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
		High:                 high,
		Nanos:                nanos,
	})
	return err
}

func (dic *digitalInterruptClient) AddCallback(c chan bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (dic *digitalInterruptClient) AddPostProcessor(pp board.PostProcessor) {
	debug.PrintStack()
	panic(errUnimplemented)
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

// gpsClient satisfies a gRPC based gps.GPS. Refer to the interface
// for descriptions of its methods.
type gpsClient struct {
	*sensorClient
}

func (gc *gpsClient) Readings(ctx context.Context) ([]interface{}, error) {
	loc, err := gc.Location(ctx)
	if err != nil {
		return nil, err
	}
	alt, err := gc.Altitude(ctx)
	if err != nil {
		return nil, err
	}
	speed, err := gc.Speed(ctx)
	if err != nil {
		return nil, err
	}
	horzAcc, vertAcc, err := gc.Accuracy(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{loc.Lat(), loc.Lng(), alt, speed, horzAcc, vertAcc}, nil
}

func (gc *gpsClient) Location(ctx context.Context) (*geo.Point, error) {
	resp, err := gc.rc.client.GPSLocation(ctx, &pb.GPSLocationRequest{
		Name: gc.name,
	})
	if err != nil {
		return nil, err
	}
	return geo.NewPoint(resp.Coordinate.Latitude, resp.Coordinate.Longitude), nil
}

func (gc *gpsClient) Altitude(ctx context.Context) (float64, error) {
	resp, err := gc.rc.client.GPSAltitude(ctx, &pb.GPSAltitudeRequest{
		Name: gc.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.Altitude, nil
}

func (gc *gpsClient) Speed(ctx context.Context) (float64, error) {
	resp, err := gc.rc.client.GPSSpeed(ctx, &pb.GPSSpeedRequest{
		Name: gc.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.SpeedKph, nil
}

func (gc *gpsClient) Satellites(ctx context.Context) (int, int, error) {
	return 0, 0, nil
}

func (gc *gpsClient) Accuracy(ctx context.Context) (float64, float64, error) {
	resp, err := gc.rc.client.GPSAccuracy(ctx, &pb.GPSAccuracyRequest{
		Name: gc.name,
	})
	if err != nil {
		return math.NaN(), math.NaN(), err
	}
	return resp.HorizontalAccuracy, resp.VerticalAccuracy, nil
}

func (gc *gpsClient) Valid(ctx context.Context) (bool, error) {
	return true, nil
}

// motorClient satisfies a gRPC based motor.Motor. Refer to the interface
// for descriptions of its methods.
type motorClient struct {
	rc   *RobotClient
	name string
}

func (mc *motorClient) PID() motor.PID {
	return nil
}
func (mc *motorClient) SetPower(ctx context.Context, powerPct float64) error {
	_, err := mc.rc.client.MotorPower(ctx, &pb.MotorPowerRequest{
		Name:     mc.name,
		PowerPct: powerPct,
	})
	return err
}

func (mc *motorClient) Go(ctx context.Context, powerPct float64) error {
	_, err := mc.rc.client.MotorGo(ctx, &pb.MotorGoRequest{
		Name:     mc.name,
		PowerPct: powerPct,
	})
	return err
}

func (mc *motorClient) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	_, err := mc.rc.client.MotorGoFor(ctx, &pb.MotorGoForRequest{
		Name:        mc.name,
		Rpm:         rpm,
		Revolutions: revolutions,
	})
	return err
}

func (mc *motorClient) Position(ctx context.Context) (float64, error) {
	resp, err := mc.rc.client.MotorPosition(ctx, &pb.MotorPositionRequest{
		Name: mc.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.Position, nil
}

func (mc *motorClient) PositionSupported(ctx context.Context) (bool, error) {
	resp, err := mc.rc.client.MotorPositionSupported(ctx, &pb.MotorPositionSupportedRequest{
		Name: mc.name,
	})
	if err != nil {
		return false, err
	}
	return resp.Supported, nil
}

func (mc *motorClient) Off(ctx context.Context) error {
	_, err := mc.rc.client.MotorOff(ctx, &pb.MotorOffRequest{
		Name: mc.name,
	})
	return err
}

func (mc *motorClient) IsOn(ctx context.Context) (bool, error) {
	resp, err := mc.rc.client.MotorIsOn(ctx, &pb.MotorIsOnRequest{
		Name: mc.name,
	})
	if err != nil {
		return false, err
	}
	return resp.IsOn, nil
}

func (mc *motorClient) GoTo(ctx context.Context, rpm float64, position float64) error {
	_, err := mc.rc.client.MotorGoTo(ctx, &pb.MotorGoToRequest{
		Name:     mc.name,
		Rpm:      rpm,
		Position: position,
	})
	return err
}

func (mc *motorClient) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if stopFunc != nil {
		return errors.New("stopFunc must be nil when using gRPC")
	}
	_, err := mc.rc.client.MotorGoTillStop(ctx, &pb.MotorGoTillStopRequest{
		Name: mc.name,
		Rpm:  rpm,
	})
	return err
}

func (mc *motorClient) SetToZeroPosition(ctx context.Context, offset float64) error {
	_, err := mc.rc.client.MotorZero(ctx, &pb.MotorZeroRequest{
		Name:   mc.name,
		Offset: offset,
	})
	return err
}

// inputControllerClient satisfies a gRPC based input.Controller. Refer to the interface
// for descriptions of its methods.
type inputControllerClient struct {
	rc            *RobotClient
	name          string
	streamCancel  context.CancelFunc
	streamHUP     bool
	streamRunning bool
	streamReady   bool
	streamMu      sync.Mutex
	mu            sync.RWMutex
	callbackWait  sync.WaitGroup
	callbacks     map[input.Control]map[input.EventType]input.ControlFunction
}

func (cc *inputControllerClient) Controls(ctx context.Context) ([]input.Control, error) {
	resp, err := cc.rc.client.InputControllerControls(ctx, &pb.InputControllerControlsRequest{
		Controller: cc.name,
	})
	if err != nil {
		return nil, err
	}
	var controls []input.Control
	for _, control := range resp.Controls {
		controls = append(controls, input.Control(control))
	}
	return controls, nil
}

func (cc *inputControllerClient) LastEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	resp, err := cc.rc.client.InputControllerLastEvents(ctx, &pb.InputControllerLastEventsRequest{
		Controller: cc.name,
	})
	if err != nil {
		return nil, err
	}

	eventsOut := make(map[input.Control]input.Event)
	for _, eventIn := range resp.Events {
		eventsOut[input.Control(eventIn.Control)] = input.Event{
			Time:    eventIn.Time.AsTime(),
			Event:   input.EventType(eventIn.Event),
			Control: input.Control(eventIn.Control),
			Value:   eventIn.Value,
		}
	}
	return eventsOut, nil
}

// InjectEvent allows directly sending an Event (such as a button press) from external code
func (cc *inputControllerClient) InjectEvent(ctx context.Context, event input.Event) error {
	eventMsg := &pb.InputControllerEvent{
		Time:    timestamppb.New(event.Time),
		Event:   string(event.Event),
		Control: string(event.Control),
		Value:   event.Value,
	}

	_, err := cc.rc.client.InputControllerInjectEvent(ctx, &pb.InputControllerInjectEventRequest{
		Controller: cc.name,
		Event:      eventMsg,
	})

	return err
}

func (cc *inputControllerClient) RegisterControlCallback(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
	cc.mu.Lock()
	if cc.callbacks == nil {
		cc.callbacks = make(map[input.Control]map[input.EventType]input.ControlFunction)
	}

	_, ok := cc.callbacks[control]
	if !ok {
		cc.callbacks[control] = make(map[input.EventType]input.ControlFunction)
	}

	for _, trigger := range triggers {
		if trigger == input.ButtonChange {
			cc.callbacks[control][input.ButtonRelease] = ctrlFunc
			cc.callbacks[control][input.ButtonPress] = ctrlFunc
		} else {
			cc.callbacks[control][trigger] = ctrlFunc
		}
	}
	cc.mu.Unlock()

	// We want to start one and only one connectStream()
	cc.streamMu.Lock()
	defer cc.streamMu.Unlock()
	if cc.streamRunning {
		for !cc.streamReady {
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return ctx.Err()
			}
		}
		cc.streamHUP = true
		cc.streamReady = false
		cc.streamCancel()
	} else {
		cc.streamRunning = true
		cc.rc.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer cc.rc.activeBackgroundWorkers.Done()
			cc.connectStream(cc.rc.closeContext)
		})
		cc.mu.RLock()
		ready := cc.streamReady
		cc.mu.RUnlock()
		for !ready {
			cc.mu.RLock()
			ready = cc.streamReady
			cc.mu.RUnlock()
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return ctx.Err()
			}
		}
	}

	return nil
}

func (cc *inputControllerClient) connectStream(ctx context.Context) {
	defer func() {
		cc.streamMu.Lock()
		defer cc.streamMu.Unlock()
		cc.mu.Lock()
		defer cc.mu.Unlock()
		cc.streamCancel = nil
		cc.streamRunning = false
		cc.streamHUP = false
		cc.streamReady = false
		cc.callbackWait.Wait()
	}()

	// Will retry on connection errors and disconnects
	for {
		cc.mu.Lock()
		cc.streamReady = false
		cc.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		default:
		}

		var haveCallbacks bool
		cc.mu.RLock()
		req := &pb.InputControllerEventStreamRequest{
			Controller: cc.name,
		}

		for control, v := range cc.callbacks {
			outEvent := &pb.InputControllerEventStreamRequest_Events{
				Control: string(control),
			}

			for event, ctrlFunc := range v {
				if ctrlFunc != nil {
					haveCallbacks = true
					outEvent.Events = append(outEvent.Events, string(event))
				} else {
					outEvent.CancelledEvents = append(outEvent.CancelledEvents, string(event))
				}
			}
			req.Events = append(req.Events, outEvent)
		}
		cc.mu.RUnlock()

		if !haveCallbacks {
			return
		}

		streamCtx, cancel := context.WithCancel(ctx)
		cc.streamCancel = cancel

		stream, err := cc.rc.client.InputControllerEventStream(streamCtx, req)
		if err != nil {
			cc.rc.Logger().Error(err)
			if utils.SelectContextOrWait(ctx, 3*time.Second) {
				continue
			} else {
				return
			}
		}

		cc.mu.RLock()
		hup := cc.streamHUP
		cc.mu.RUnlock()
		if !hup {
			cc.sendConnectionStatus(ctx, true)
		}

		// Handle the rest of the stream
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			cc.mu.Lock()
			cc.streamHUP = false
			cc.streamReady = true
			cc.mu.Unlock()
			eventIn, err := stream.Recv()
			if err != nil && eventIn == nil {
				cc.mu.RLock()
				hup := cc.streamHUP
				cc.mu.RUnlock()
				if hup {
					break
				}
				cc.sendConnectionStatus(ctx, false)
				if utils.SelectContextOrWait(ctx, 3*time.Second) {
					cc.rc.Logger().Error(err)
					break
				} else {
					return
				}
			}
			if err != nil {
				cc.rc.Logger().Error(err)
			}

			eventOut := input.Event{
				Time:    eventIn.Time.AsTime(),
				Event:   input.EventType(eventIn.Event),
				Control: input.Control(eventIn.Control),
				Value:   eventIn.Value,
			}
			cc.execCallback(ctx, eventOut)
		}
	}
}

func (cc *inputControllerClient) sendConnectionStatus(ctx context.Context, connected bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	evType := input.Disconnect
	now := time.Now()
	if connected {
		evType = input.Connect
	}

	for control := range cc.callbacks {
		eventOut := input.Event{
			Time:    now,
			Event:   evType,
			Control: control,
			Value:   0,
		}
		cc.execCallback(ctx, eventOut)
	}
}

func (cc *inputControllerClient) execCallback(ctx context.Context, event input.Event) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	callbackMap, ok := cc.callbacks[event.Control]
	if !ok {
		return
	}

	callback, ok := callbackMap[event.Event]
	if ok && callback != nil {
		cc.callbackWait.Add(1)
		utils.PanicCapturingGo(func() {
			defer cc.callbackWait.Done()
			callback(ctx, event)
		})
	}
	callbackAll, ok := callbackMap[input.AllEvents]
	if ok && callbackAll != nil {
		cc.callbackWait.Add(1)
		utils.PanicCapturingGo(func() {
			defer cc.callbackWait.Done()
			callbackAll(ctx, event)
		})
	}
}

// forcematrixClient satisfies a gRPC based
// forcematrix.ForceMatrix.
// Refer to the ForceMatrix interface for descriptions of its methods.
type forcematrixClient struct {
	*sensorClient
}

func (fmc *forcematrixClient) Readings(ctx context.Context) ([]interface{}, error) {
	matrix, err := fmc.Matrix(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{matrix}, nil
}

func (fmc *forcematrixClient) Matrix(ctx context.Context) ([][]int, error) {
	resp, err := fmc.rc.client.ForceMatrixMatrix(ctx,
		&pb.ForceMatrixMatrixRequest{
			Name: fmc.name,
		})
	if err != nil {
		return nil, err
	}
	return protoToMatrix(resp), nil
}

func (fmc *forcematrixClient) IsSlipping(ctx context.Context) (bool, error) {
	resp, err := fmc.rc.client.ForceMatrixSlipDetection(ctx,
		&pb.ForceMatrixSlipDetectionRequest{
			Name: fmc.name,
		})
	if err != nil {
		return false, err
	}

	return resp.GetIsSlipping(), nil
}

func (fmc *forcematrixClient) Desc() sensor.Description {
	return sensor.Description{forcematrix.Type, ""}
}

// Ensure implements ForceMatrix
var _ = forcematrix.ForceMatrix(&forcematrixClient{})

// protoToMatrix is a helper function to convert protobuf matrix values into a 2-dimensional int slice.
func protoToMatrix(matrixResponse *pb.ForceMatrixMatrixResponse) [][]int {
	numRows := matrixResponse.Matrix.Rows
	numCols := matrixResponse.Matrix.Cols

	matrix := make([][]int, numRows)
	for row := range matrix {
		matrix[row] = make([]int, numCols)
		for col := range matrix[row] {
			matrix[row][col] = int(matrixResponse.Matrix.Data[row*int(numCols)+col])
		}
	}
	return matrix
}

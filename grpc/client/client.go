// Package client contains a gRPC based robot.Robot client.
package client

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"math"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	rpcclient "go.viam.com/utils/rpc/client"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/grpc"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	"go.viam.com/core/pointcloud"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/resource"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/sensor/forcematrix"
	"go.viam.com/core/sensor/imu"
	"go.viam.com/core/servo"
	"go.viam.com/core/spatialmath"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	address string
	conn    dialer.ClientConn
	client  pb.RobotServiceClient

	namesMu       *sync.RWMutex
	baseNames     []string
	gripperNames  []string
	boardNames    []boardInfo
	cameraNames   []string
	lidarNames    []string
	sensorNames   []string
	servoNames    []string
	motorNames    []string
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
}

// RobotClientOptions are extra construction time options.
type RobotClientOptions struct {
	// RefreshEvery is how often to refresh the status/parts of the
	// robot. If unset, it will not be refreshed automatically.
	RefreshEvery time.Duration

	// Insecure determines if the gRPC connection is TLS based.
	Insecure bool
}

// NewClientWithOptions constructs a new RobotClient that is served at the given address. The given
// context can be used to cancel the operation. Additionally, construction time options can be given.
func NewClientWithOptions(ctx context.Context, address string, opts RobotClientOptions, logger golog.Logger) (*RobotClient, error) {
	ctx, timeoutCancel := context.WithTimeout(ctx, 20*time.Second)
	defer timeoutCancel()

	conn, err := rpcclient.Dial(ctx, address, rpcclient.DialOptions{Insecure: opts.Insecure}, logger)
	if err != nil {
		return nil, err
	}

	client := pb.NewRobotServiceClient(conn)
	closeCtx, cancel := context.WithCancel(context.Background())
	rc := &RobotClient{
		address:                 address,
		conn:                    conn,
		client:                  client,
		sensorTypes:             map[string]sensor.Type{},
		cancelBackgroundWorkers: cancel,
		logger:                  logger,
		namesMu:                 &sync.RWMutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		cachedStatusMu:          &sync.Mutex{},
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
	return NewClientWithOptions(ctx, address, RobotClientOptions{Insecure: true}, logger)
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close() error {
	rc.cancelBackgroundWorkers()
	rc.activeBackgroundWorkers.Wait()
	return rc.conn.Close()
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
			rc.logger.Errorw("failed to refresh status", "error", err)
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
			cc.Frame.Translation = config.Translation{
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
	return &armClient{rc: rc, name: name}, true
}

// BaseByName returns a base by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) BaseByName(name string) (base.Base, bool) {
	return &baseClient{rc, name}, true
}

// GripperByName returns a gripper by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) GripperByName(name string) (gripper.Gripper, bool) {
	return &gripperClient{rc, name}, true
}

// CameraByName returns a camera by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) CameraByName(name string) (camera.Camera, bool) {
	return &cameraClient{rc, name}, true
}

// LidarByName returns a lidar by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) LidarByName(name string) (lidar.Lidar, bool) {
	return &lidarClient{rc, name}, true
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
	case imu.Type:
		return &imuClient{sc}, true
	case forcematrix.Type:
		return &forcematrixClient{sc}, true
	default:
		return sc, true
	}
}

// ServoByName returns a servo by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) ServoByName(name string) (servo.Servo, bool) {
	return &servoClient{
		rc:   rc,
		name: name,
	}, true
}

// MotorByName returns a motor by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) MotorByName(name string) (motor.Motor, bool) {
	return &motorClient{
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
	switch name.Subtype {
	case arm.Subtype:
		return &armClient{rc: rc, name: name.Name}, true
	default:
		return nil, false
	}
}

// Refresh manually updates the underlying parts of the robot based
// on a status retrieved from the server.
// TODO(https://github.com/viamrobotics/core/issues/57) - do not use status
// as we plan on making it a more expensive request with more details than
// needed for the purposes of this method.
func (rc *RobotClient) Refresh(ctx context.Context) error {
	status, err := rc.status(ctx)
	if err != nil {
		return errors.Errorf("status call failed: %w", err)
	}

	rc.storeStatus(status)
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	// TODO: placeholder implementation
	rc.resourceNames = []resource.Name{}
	if len(status.Arms) != 0 {
		for name := range status.Arms {
			rc.resourceNames = append(rc.resourceNames, arm.Named(name))
		}
	}
	rc.baseNames = nil
	if len(status.Bases) != 0 {
		rc.baseNames = make([]string, 0, len(status.Bases))
		for name := range status.Bases {
			rc.baseNames = append(rc.baseNames, name)
		}
	}
	rc.gripperNames = nil
	if len(status.Grippers) != 0 {
		rc.gripperNames = make([]string, 0, len(status.Grippers))
		for name := range status.Grippers {
			rc.gripperNames = append(rc.gripperNames, name)
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
	rc.cameraNames = nil
	if len(status.Cameras) != 0 {
		rc.cameraNames = make([]string, 0, len(status.Cameras))
		for name := range status.Cameras {
			rc.cameraNames = append(rc.cameraNames, name)
		}
	}
	rc.lidarNames = nil
	if len(status.Lidars) != 0 {
		rc.lidarNames = make([]string, 0, len(status.Lidars))
		for name := range status.Lidars {
			rc.lidarNames = append(rc.lidarNames, name)
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
	rc.servoNames = nil
	if len(status.Servos) != 0 {
		rc.servoNames = make([]string, 0, len(status.Servos))
		for name := range status.Servos {
			rc.servoNames = append(rc.servoNames, name)
		}
	}
	rc.motorNames = nil
	if len(status.Motors) != 0 {
		rc.motorNames = make([]string, 0, len(status.Motors))
		for name := range status.Motors {
			rc.motorNames = append(rc.motorNames, name)
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
	return copyStringSlice(rc.gripperNames)
}

// CameraNames returns the names of all known cameras.
func (rc *RobotClient) CameraNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.cameraNames)
}

// LidarNames returns the names of all known lidars.
func (rc *RobotClient) LidarNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.lidarNames)
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
	return copyStringSlice(rc.servoNames)
}

// MotorNames returns the names of all known motors.
func (rc *RobotClient) MotorNames() []string {
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
	return copyStringSlice(rc.motorNames)
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

// FrameSystem not implemented for remote robots
func (rc *RobotClient) FrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	debug.PrintStack()
	return nil, errUnimplemented
}

// Logger returns the logger being used for this robot.
func (rc *RobotClient) Logger() golog.Logger {
	return rc.logger
}

// baseClient satisfies a gRPC based base.Base. Refer to the interface
// for descriptions of its methods.
type baseClient struct {
	rc   *RobotClient
	name string
}

func (bc *baseClient) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	resp, err := bc.rc.client.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
		Name:           bc.name,
		MillisPerSec:   millisPerSec,
		DistanceMillis: int64(distanceMillis),
	})
	if err != nil {
		return 0, err
	}
	moved := int(resp.DistanceMillis)
	if resp.Success {
		return moved, nil
	}
	return moved, errors.New(resp.Error)
}

func (bc *baseClient) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	resp, err := bc.rc.client.BaseSpin(ctx, &pb.BaseSpinRequest{
		Name:       bc.name,
		AngleDeg:   angleDeg,
		DegsPerSec: degsPerSec,
	})
	if err != nil {
		return math.NaN(), err
	}
	spun := resp.AngleDeg
	if resp.Success {
		return spun, nil
	}
	return spun, errors.New(resp.Error)
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

// armClient satisfies a gRPC based arm.Arm. Refer to the interface
// for descriptions of its methods.
type armClient struct {
	rc   *RobotClient
	name string
}

func (ac *armClient) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	resp, err := ac.rc.client.ArmCurrentPosition(ctx, &pb.ArmCurrentPositionRequest{
		Name: ac.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Position, nil
}

func (ac *armClient) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	_, err := ac.rc.client.ArmMoveToPosition(ctx, &pb.ArmMoveToPositionRequest{
		Name: ac.name,
		To:   c,
	})
	return err
}

func (ac *armClient) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	_, err := ac.rc.client.ArmMoveToJointPositions(ctx, &pb.ArmMoveToJointPositionsRequest{
		Name: ac.name,
		To:   pos,
	})
	return err
}

func (ac *armClient) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	resp, err := ac.rc.client.ArmCurrentJointPositions(ctx, &pb.ArmCurrentJointPositionsRequest{
		Name: ac.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

func (ac *armClient) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	_, err := ac.rc.client.ArmJointMoveDelta(ctx, &pb.ArmJointMoveDeltaRequest{
		Name:       ac.name,
		Joint:      int32(joint),
		AmountDegs: amountDegs,
	})
	return err
}

// gripperClient satisfies a gRPC based gripper.Gripper. Refer to the interface
// for descriptions of its methods.
type gripperClient struct {
	rc   *RobotClient
	name string
}

func (gc *gripperClient) Open(ctx context.Context) error {
	_, err := gc.rc.client.GripperOpen(ctx, &pb.GripperOpenRequest{
		Name: gc.name,
	})
	return err
}

func (gc *gripperClient) Grab(ctx context.Context) (bool, error) {
	resp, err := gc.rc.client.GripperGrab(ctx, &pb.GripperGrabRequest{
		Name: gc.name,
	})
	if err != nil {
		return false, err
	}
	return resp.Grabbed, nil
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

// cameraClient satisfies a gRPC based camera.Camera. Refer to the interface
// for descriptions of its methods.
type cameraClient struct {
	rc   *RobotClient
	name string
}

func (cc *cameraClient) Next(ctx context.Context) (image.Image, func(), error) {
	resp, err := cc.rc.client.CameraFrame(ctx, &pb.CameraFrameRequest{
		Name:     cc.name,
		MimeType: grpc.MimeTypeViamBest,
	})
	if err != nil {
		return nil, nil, err
	}
	switch resp.MimeType {
	case grpc.MimeTypeRawRGBA:
		img := image.NewNRGBA(image.Rect(0, 0, int(resp.DimX), int(resp.DimY)))
		img.Pix = resp.Frame
		return img, func() {}, nil
	case grpc.MimeTypeRawIWD:
		img, err := rimage.ImageWithDepthFromRawBytes(int(resp.DimX), int(resp.DimY), resp.Frame)
		return img, func() {}, err
	default:
		return nil, nil, errors.Errorf("do not how to decode MimeType %s", resp.MimeType)
	}

}

func (cc *cameraClient) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	resp, err := cc.rc.client.PointCloud(ctx, &pb.PointCloudRequest{
		Name:     cc.name,
		MimeType: grpc.MimeTypePCD,
	})
	if err != nil {
		return nil, err
	}

	if resp.MimeType != grpc.MimeTypePCD {
		return nil, fmt.Errorf("unknown pc mime type %s", resp.MimeType)
	}

	return pointcloud.ReadPCD(bytes.NewReader(resp.Frame))
}

func (cc *cameraClient) Close() error {
	return nil
}

// lidarClient satisfies a gRPC based lidar.Lidar. Refer to the interface
// for descriptions of its methods.
type lidarClient struct {
	rc   *RobotClient
	name string
}

func (ldc *lidarClient) Info(ctx context.Context) (map[string]interface{}, error) {
	resp, err := ldc.rc.client.LidarInfo(ctx, &pb.LidarInfoRequest{
		Name: ldc.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Info.AsMap(), nil
}

func (ldc *lidarClient) Start(ctx context.Context) error {
	_, err := ldc.rc.client.LidarStart(ctx, &pb.LidarStartRequest{
		Name: ldc.name,
	})
	return err
}

func (ldc *lidarClient) Stop(ctx context.Context) error {
	_, err := ldc.rc.client.LidarStop(ctx, &pb.LidarStopRequest{
		Name: ldc.name,
	})
	return err
}

func (ldc *lidarClient) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	resp, err := ldc.rc.client.LidarScan(ctx, &pb.LidarScanRequest{
		Name:     ldc.name,
		Count:    int32(options.Count),
		NoFilter: options.NoFilter,
	})
	if err != nil {
		return nil, err
	}
	return MeasurementsFromProto(resp.Measurements), nil
}

func (ldc *lidarClient) Range(ctx context.Context) (float64, error) {
	resp, err := ldc.rc.client.LidarRange(ctx, &pb.LidarRangeRequest{
		Name: ldc.name,
	})
	if err != nil {
		return 0, err
	}
	return float64(resp.Range), nil
}

func (ldc *lidarClient) Bounds(ctx context.Context) (r2.Point, error) {
	resp, err := ldc.rc.client.LidarBounds(ctx, &pb.LidarBoundsRequest{
		Name: ldc.name,
	})
	if err != nil {
		return r2.Point{}, err
	}
	return r2.Point{float64(resp.X), float64(resp.Y)}, nil
}

func (ldc *lidarClient) AngularResolution(ctx context.Context) (float64, error) {
	resp, err := ldc.rc.client.LidarAngularResolution(ctx, &pb.LidarAngularResolutionRequest{
		Name: ldc.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.AngularResolution, nil
}

func measurementFromProto(pm *pb.LidarMeasurement) *lidar.Measurement {
	return lidar.NewMeasurement(pm.AngleDeg, pm.Distance)
}

// MeasurementsFromProto converts proto based LiDAR measurements to the
// interface.
func MeasurementsFromProto(pms []*pb.LidarMeasurement) lidar.Measurements {
	ms := make(lidar.Measurements, 0, len(pms))
	for _, pm := range pms {
		ms = append(ms, measurementFromProto(pm))
	}
	return ms
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

// imuClient satisfies a gRPC based imu.IMU. Refer to the interface
// for descriptions of its methods.
type imuClient struct {
	*sensorClient
}

func (ic *imuClient) Readings(ctx context.Context) ([]interface{}, error) {
	vel, err := ic.AngularVelocity(ctx)
	if err != nil {
		return nil, err
	}
	orientation, err := ic.Orientation(ctx)
	if err != nil {
		return nil, err
	}
	ea := orientation.EulerAngles()
	return []interface{}{vel.X, vel.Y, vel.Z, ea.Roll, ea.Pitch, ea.Yaw}, nil
}

func (ic *imuClient) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	resp, err := ic.rc.client.IMUAngularVelocity(ctx, &pb.IMUAngularVelocityRequest{
		Name: ic.name,
	})
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	return spatialmath.AngularVelocity{
		X: resp.AngularVelocity.X,
		Y: resp.AngularVelocity.Y,
		Z: resp.AngularVelocity.Z,
	}, nil
}

func (ic *imuClient) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	resp, err := ic.rc.client.IMUOrientation(ctx, &pb.IMUOrientationRequest{
		Name: ic.name,
	})
	if err != nil {
		return nil, err
	}
	return &spatialmath.EulerAngles{
		Roll:  resp.Orientation.Roll,
		Pitch: resp.Orientation.Pitch,
		Yaw:   resp.Orientation.Yaw,
	}, nil
}

func (ic *imuClient) Desc() sensor.Description {
	return sensor.Description{imu.Type, ""}
}

// servoClient satisfies a gRPC based servo.Servo. Refer to the interface
// for descriptions of its methods.
type servoClient struct {
	rc   *RobotClient
	name string
}

func (sc *servoClient) Move(ctx context.Context, angleDeg uint8) error {
	_, err := sc.rc.client.ServoMove(ctx, &pb.ServoMoveRequest{
		Name:     sc.name,
		AngleDeg: uint32(angleDeg),
	})
	return err
}

func (sc *servoClient) Current(ctx context.Context) (uint8, error) {
	resp, err := sc.rc.client.ServoCurrent(ctx, &pb.ServoCurrentRequest{
		Name: sc.name,
	})
	if err != nil {
		return 0, err
	}
	return uint8(resp.AngleDeg), nil
}

// motorClient satisfies a gRPC based motor.Motor. Refer to the interface
// for descriptions of its methods.
type motorClient struct {
	rc   *RobotClient
	name string
}

func (mc *motorClient) Power(ctx context.Context, powerPct float32) error {
	_, err := mc.rc.client.MotorPower(ctx, &pb.MotorPowerRequest{
		Name:     mc.name,
		PowerPct: powerPct,
	})
	return err
}

func (mc *motorClient) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	_, err := mc.rc.client.MotorGo(ctx, &pb.MotorGoRequest{
		Name:      mc.name,
		Direction: d,
		PowerPct:  powerPct,
	})
	return err
}

func (mc *motorClient) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	_, err := mc.rc.client.MotorGoFor(ctx, &pb.MotorGoForRequest{
		Name:        mc.name,
		Direction:   d,
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

func (mc *motorClient) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if stopFunc != nil {
		return errors.New("stopFunc must be nil when using gRPC")
	}
	_, err := mc.rc.client.MotorGoTillStop(ctx, &pb.MotorGoTillStopRequest{
		Name:      mc.name,
		Direction: d,
		Rpm:       rpm,
	})
	return err
}

func (mc *motorClient) Zero(ctx context.Context, offset float64) error {
	_, err := mc.rc.client.MotorZero(ctx, &pb.MotorZeroRequest{
		Name:   mc.name,
		Offset: offset,
	})
	return err
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

func (fmc *forcematrixClient) Desc() sensor.Description {
	return sensor.Description{forcematrix.Type, ""}
}

// Ensure implements ForceMatrix
var _ = forcematrix.ForceMatrix(&forcematrixClient{})

// protoToMatrix is a helper function to convert protobuf matrix values into a 2-dimensional int slice.
func protoToMatrix(matrixResponse *pb.ForceMatrixMatrixResponse) [][]int {
	rows := matrixResponse.Matrix.Rows
	cols := matrixResponse.Matrix.Cols
	matrix := make([][]int, rows)
	for r := range matrix {
		matrix[r] = make([]int, cols)
		for c := range matrix[r] {
			matrix[r][c] = int(matrixResponse.Matrix.Data[r*int(cols)+c])
		}
	}
	return matrix
}

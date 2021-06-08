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

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/grpc"
	"go.viam.com/core/lidar"
	"go.viam.com/core/pointcloud"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rexec"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
	rpcclient "go.viam.com/core/rpc/client"
	"go.viam.com/core/rpc/dialer"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/utils"

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

	namesMu      *sync.Mutex
	armNames     []string
	baseNames    []string
	gripperNames []string
	boardNames   []boardInfo
	cameraNames  []string
	lidarNames   []string
	sensorNames  []string

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
		namesMu:                 &sync.Mutex{},
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
	motorNames            []string
	servoNames            []string
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

// RemoteByName returns a remote robot by name. It is assumed to exist on the
// other end. Right now this method is unimplemented.
func (rc *RobotClient) RemoteByName(name string) robot.Robot {
	debug.PrintStack()
	panic(errUnimplemented)
}

// ArmByName returns a arm by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) ArmByName(name string) arm.Arm {
	return &armClient{rc, name}
}

// BaseByName returns a base by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) BaseByName(name string) base.Base {
	return &baseClient{rc, name}
}

// GripperByName returns a gripper by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) GripperByName(name string) gripper.Gripper {
	return &gripperClient{rc, name}
}

// CameraByName returns a camera by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) CameraByName(name string) camera.Camera {
	return &cameraClient{rc, name}
}

// LidarByName returns a lidar by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) LidarByName(name string) lidar.Lidar {
	return &lidarClient{rc, name}
}

// BoardByName returns a board by name. It is assumed to exist on the
// other end.
func (rc *RobotClient) BoardByName(name string) board.Board {
	for _, info := range rc.boardNames {
		if info.name == name {
			return &boardClient{rc, info}
		}
	}
	return nil
}

// SensorByName returns a sensor by name. It is assumed to exist on the
// other end. Based on the known sensor names and types, a type specific
// sensor is attempted to be returned; otherwise it's a general purpose
// sensor.
func (rc *RobotClient) SensorByName(name string) sensor.Sensor {
	sensorType := rc.sensorTypes[name]
	sc := &sensorClient{rc, name, sensorType}
	switch sensorType {
	case compass.Type:
		return &compassClient{sc}
	case compass.RelativeType:
		return &relativeCompassClient{&compassClient{sc}}
	default:
		return sc
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
	rc.armNames = nil
	if len(status.Arms) != 0 {
		rc.armNames = make([]string, 0, len(status.Arms))
		for name := range status.Arms {
			rc.armNames = append(rc.armNames, name)
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
			if len(boardStatus.Motors) != 0 {
				info.motorNames = make([]string, 0, len(boardStatus.Motors))
				for name := range boardStatus.Motors {
					info.motorNames = append(info.motorNames, name)
				}
			}
			if len(boardStatus.Servos) != 0 {
				info.servoNames = make([]string, 0, len(boardStatus.Servos))
				for name := range boardStatus.Servos {
					info.servoNames = append(info.servoNames, name)
				}
			}
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
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.armNames)
}

// GripperNames returns the names of all known grippers.
func (rc *RobotClient) GripperNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.gripperNames)
}

// CameraNames returns the names of all known cameras.
func (rc *RobotClient) CameraNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.cameraNames)
}

// LidarNames returns the names of all known lidars.
func (rc *RobotClient) LidarNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.lidarNames)
}

// BaseNames returns the names of all known bases.
func (rc *RobotClient) BaseNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.baseNames)
}

// BoardNames returns the names of all known boards.
func (rc *RobotClient) BoardNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	out := make([]string, 0, len(rc.boardNames))
	for _, info := range rc.boardNames {
		out = append(out, info.name)
	}
	return out
}

// SensorNames returns the names of all known sensors.
func (rc *RobotClient) SensorNames() []string {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()
	return copyStringSlice(rc.sensorNames)
}

// ProcessManager returns a useless process manager for the sake of
// satisfying the robot.Robot interface. Maybe it should not be part
// of the interface!
func (rc *RobotClient) ProcessManager() rexec.ProcessManager {
	return rexec.NoopProcessManager
}

// Config is not yet implemented and probably will not be due to it not
// making much sense in a remote context.
func (rc *RobotClient) Config(ctx context.Context) (*config.Config, error) {
	debug.PrintStack()
	return nil, errUnimplemented
}

// ProviderByName is not yet implemented and probably will not be due to it not
// making much sense in a remote context.
func (rc *RobotClient) ProviderByName(name string) robot.Provider {
	return nil
}

// AddProvider is not yet implemented and probably will not be due to it not
// making much sense in a remote context.
func (rc *RobotClient) AddProvider(p robot.Provider, c config.Component) {

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

// WidthMillis needs to be implemented.
func (bc *baseClient) WidthMillis(ctx context.Context) (int, error) {
	debug.PrintStack()
	return 0, errUnimplemented
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

// JointMoveDelta needs to be implemented.
func (ac *armClient) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	debug.PrintStack()
	return errUnimplemented
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

func (bc *boardClient) Motor(name string) board.Motor {
	return &motorClient{
		rc:        bc.rc,
		boardName: bc.info.name,
		motorName: name,
	}
}

func (bc *boardClient) Servo(name string) board.Servo {
	return &servoClient{
		rc:        bc.rc,
		boardName: bc.info.name,
		servoName: name,
	}
}

// AnalogReader needs to be implemented.
func (bc *boardClient) AnalogReader(name string) board.AnalogReader {
	return &analogReaderClient{
		rc:               bc.rc,
		boardName:        bc.info.name,
		analogReaderName: name,
	}
}

// DigitalInterrupt needs to be implemented.
func (bc *boardClient) DigitalInterrupt(name string) board.DigitalInterrupt {
	return &digitalInterruptClient{
		rc:                   bc.rc,
		boardName:            bc.info.name,
		digitalInterruptName: name,
	}
}

func (bc *boardClient) MotorNames() []string {
	return copyStringSlice(bc.info.motorNames)
}

func (bc *boardClient) ServoNames() []string {
	return copyStringSlice(bc.info.servoNames)
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

// motorClient satisfies a gRPC based board.Motor. Refer to the interface
// for descriptions of its methods.
type motorClient struct {
	rc        *RobotClient
	boardName string
	motorName string
}

// Power needs to be implemented.
func (mc *motorClient) Power(ctx context.Context, powerPct float32) error {
	debug.PrintStack()
	return errUnimplemented
}

func (mc *motorClient) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	_, err := mc.rc.client.BoardMotorGo(ctx, &pb.BoardMotorGoRequest{
		BoardName: mc.boardName,
		MotorName: mc.motorName,
		Direction: d,
		PowerPct:  powerPct,
	})
	return err
}

func (mc *motorClient) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	_, err := mc.rc.client.BoardMotorGoFor(ctx, &pb.BoardMotorGoForRequest{
		BoardName:   mc.boardName,
		MotorName:   mc.motorName,
		Direction:   d,
		Rpm:         rpm,
		Revolutions: revolutions,
	})
	return err
}

// Position needs to be implemented.
func (mc *motorClient) Position(ctx context.Context) (float64, error) {
	debug.PrintStack()
	return 0, errUnimplemented
}

// PositionSupported needs to be implemented.
func (mc *motorClient) PositionSupported(ctx context.Context) (bool, error) {
	debug.PrintStack()
	return false, errUnimplemented
}

// Off needs to be implemented.
func (mc *motorClient) Off(ctx context.Context) error {
	debug.PrintStack()
	return errUnimplemented
}

// IsOn needs to be implemented.
func (mc *motorClient) IsOn(ctx context.Context) (bool, error) {
	debug.PrintStack()
	return false, errUnimplemented
}

// servoClient satisfies a gRPC based board.Servo. Refer to the interface
// for descriptions of its methods.
type servoClient struct {
	rc        *RobotClient
	boardName string
	servoName string
}

func (sc *servoClient) Move(ctx context.Context, angleDeg uint8) error {
	_, err := sc.rc.client.BoardServoMove(ctx, &pb.BoardServoMoveRequest{
		BoardName: sc.boardName,
		ServoName: sc.servoName,
		AngleDeg:  uint32(angleDeg),
	})
	return err
}

// Current needs to be implemented.
func (sc *servoClient) Current(ctx context.Context) (uint8, error) {
	debug.PrintStack()
	return 0, errUnimplemented
}

// analogReaderClient satisfies a gRPC based board.Motor. Refer to the interface
// for descriptions of its methods.
type analogReaderClient struct {
	rc               *RobotClient
	boardName        string
	analogReaderName string
}

func (arc *analogReaderClient) Read(ctx context.Context) (int, error) {
	debug.PrintStack()
	panic(errUnimplemented)
}

// digitalInterruptClient satisfies a gRPC based board.Motor. Refer to the interface
// for descriptions of its methods.
type digitalInterruptClient struct {
	rc                   *RobotClient
	boardName            string
	digitalInterruptName string
}

func (dic *digitalInterruptClient) Config() board.DigitalInterruptConfig {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (dic *digitalInterruptClient) Value() int64 {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (dic *digitalInterruptClient) Tick(high bool, nanos uint64) {
	debug.PrintStack()
	panic(errUnimplemented)
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

// compassClient satisfies a gRPC based compass.Device. Refer to the interface
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

// relativeCompassClient satisfies a gRPC based compass.RelativeDevice. Refer to the interface
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

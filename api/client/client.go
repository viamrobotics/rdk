package client

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"runtime/debug"
	"sync"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/rpc"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"google.golang.org/grpc"
)

var errUnimplemented = errors.New("unimplemented")

type RobotClient struct {
	conn   rpc.ClientConn
	client pb.RobotServiceClient

	armNames         []string
	baseNames        []string
	gripperNames     []string
	boardNames       []string
	cameraNames      []string
	lidarDeviceNames []string
	sensorNames      []string

	sensorTypes map[string]sensor.DeviceType

	activeBackgroundWorkers sync.WaitGroup
	cancelBackgroundWorkers func()
	logger                  golog.Logger

	cachingStatus  bool
	cachedStatus   *pb.Status
	cachedStatusMu sync.Mutex
}

type RobotClientOptions struct {
	RefreshStatusEvery time.Duration
	Secure             bool
}

func NewRobotClientWithOptions(ctx context.Context, address string, opts RobotClientOptions, logger golog.Logger) (*RobotClient, error) {
	// TODO(erd): address insecure
	ctx, timeoutCancel := context.WithTimeout(ctx, 5*time.Second)
	defer timeoutCancel()
	var conn rpc.ClientConn
	var err error
	dialOpts := []grpc.DialOption{grpc.WithBlock()}
	// if this is secure, there's no way via RobotClientOptions to set credentials yet
	if !opts.Secure {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	if dialer := rpc.ContextDialer(ctx); dialer != nil {
		conn, err = dialer.Dial(ctx, address, dialOpts...)
	} else {
		conn, err = grpc.DialContext(ctx, address, dialOpts...)
	}
	if err != nil {
		return nil, err
	}
	client := pb.NewRobotServiceClient(conn)
	closeCtx, cancel := context.WithCancel(context.Background())
	rc := &RobotClient{
		conn:                    conn,
		client:                  client,
		sensorTypes:             map[string]sensor.DeviceType{},
		cancelBackgroundWorkers: cancel,
		logger:                  logger,
	}
	if err := rc.populateNames(ctx); err != nil {
		return nil, err
	}
	if opts.RefreshStatusEvery != 0 {
		rc.cachingStatus = true
		rc.activeBackgroundWorkers.Add(1)
		go func() {
			defer rc.activeBackgroundWorkers.Done()
			rc.refreshStatusEvery(closeCtx, opts.RefreshStatusEvery)
		}()
	}
	return rc, nil
}

func NewRobotClient(ctx context.Context, address string, logger golog.Logger) (*RobotClient, error) {
	return NewRobotClientWithOptions(ctx, address, RobotClientOptions{}, logger)
}

func (rc *RobotClient) Close() error {
	rc.cancelBackgroundWorkers()
	rc.activeBackgroundWorkers.Wait()
	return rc.conn.Close()
}

func (rc *RobotClient) refreshStatusEvery(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		status, err := rc.status(ctx)
		if err != nil {
			rc.logger.Errorw("failed to refresh status", "error", err)
			continue
		}
		rc.storeStatus(status)
	}
}

func (rc *RobotClient) storeStatus(status *pb.Status) {
	if !rc.cachingStatus {
		return
	}
	rc.cachedStatusMu.Lock()
	rc.cachedStatus = status
	rc.cachedStatusMu.Unlock()
}

func (rc *RobotClient) getCachedStatus() *pb.Status {
	if !rc.cachingStatus {
		return nil
	}
	rc.cachedStatusMu.Lock()
	defer rc.cachedStatusMu.Unlock()
	return rc.cachedStatus
}

func (rc *RobotClient) RemoteByName(name string) api.Robot {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rc *RobotClient) ArmByName(name string) api.Arm {
	return &armClient{rc, name}
}

func (rc *RobotClient) BaseByName(name string) api.Base {
	return &baseClient{rc, name}
}

func (rc *RobotClient) GripperByName(name string) api.Gripper {
	return &gripperClient{rc, name}
}

func (rc *RobotClient) CameraByName(name string) gostream.ImageSource {
	return &cameraClient{rc, name}
}

func (rc *RobotClient) LidarDeviceByName(name string) lidar.Device {
	return &lidarDeviceClient{rc, name}
}

func (rc *RobotClient) BoardByName(name string) board.Board {
	return &boardClient{rc, name}
}

func (rc *RobotClient) SensorByName(name string) sensor.Device {
	sensorType := rc.sensorTypes[name]
	sc := &sensorClient{rc, name}
	switch sensorType {
	case compass.DeviceType:
		return &compassClient{sc}
	case compass.RelativeDeviceType:
		return &relativeCompassClient{&compassClient{sc}}
	default:
		return sc
	}
}

func (rc *RobotClient) populateNames(ctx context.Context) error {
	status, err := rc.Status(ctx)
	if err != nil {
		return err
	}
	rc.storeStatus(status)
	for name := range status.Arms {
		rc.armNames = append(rc.armNames, name)
	}
	for name := range status.Bases {
		rc.baseNames = append(rc.baseNames, name)
	}
	for name := range status.Grippers {
		rc.gripperNames = append(rc.gripperNames, name)
	}
	for name := range status.Boards {
		rc.boardNames = append(rc.boardNames, name)
	}
	for name := range status.Cameras {
		rc.cameraNames = append(rc.cameraNames, name)
	}
	for name := range status.LidarDevices {
		rc.lidarDeviceNames = append(rc.lidarDeviceNames, name)
	}
	for name, sensorStatus := range status.Sensors {
		rc.sensorNames = append(rc.sensorNames, name)
		rc.sensorTypes[name] = sensor.DeviceType(sensorStatus.Type)
	}
	return nil
}

func (rc *RobotClient) RemoteNames() []string {
	debug.PrintStack()
	panic(errUnimplemented)
}

func copyStringSlice(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func (rc *RobotClient) ArmNames() []string {
	return copyStringSlice(rc.armNames)
}

func (rc *RobotClient) GripperNames() []string {
	return copyStringSlice(rc.gripperNames)
}

func (rc *RobotClient) CameraNames() []string {
	return copyStringSlice(rc.cameraNames)
}

func (rc *RobotClient) LidarDeviceNames() []string {
	return copyStringSlice(rc.lidarDeviceNames)
}

func (rc *RobotClient) BaseNames() []string {
	return copyStringSlice(rc.baseNames)
}

func (rc *RobotClient) BoardNames() []string {
	return copyStringSlice(rc.boardNames)
}

func (rc *RobotClient) SensorNames() []string {
	return copyStringSlice(rc.sensorNames)
}

func (rc *RobotClient) GetConfig(ctx context.Context) (api.Config, error) {
	debug.PrintStack()
	return api.Config{}, errUnimplemented
}

func (rc *RobotClient) status(ctx context.Context) (*pb.Status, error) {
	resp, err := rc.client.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (rc *RobotClient) Status(ctx context.Context) (*pb.Status, error) {
	if status := rc.getCachedStatus(); status != nil {
		return status, nil
	}
	return rc.status(ctx)
}

func (rc *RobotClient) ProviderByModel(model string) api.Provider {
	return nil
}

func (rc *RobotClient) AddProvider(p api.Provider, c api.Component) {}

func (rc *RobotClient) Logger() golog.Logger {
	return nil
}

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
	debug.PrintStack()
	return 0, errUnimplemented
}

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

func (ac *armClient) JointMoveDelta(ctx context.Context, joint int, amount float64) error {
	debug.PrintStack()
	return errUnimplemented
}

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

type boardClient struct {
	rc   *RobotClient
	name string
}

func (bc *boardClient) Motor(name string) board.Motor {
	return &motorClient{
		rc:        bc.rc,
		boardName: bc.name,
		motorName: name,
	}
}

func (bc *boardClient) Servo(name string) board.Servo {
	return &servoClient{
		rc:        bc.rc,
		boardName: bc.name,
		servoName: name,
	}
}

func (bc *boardClient) AnalogReader(name string) board.AnalogReader {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (bc *boardClient) DigitalInterrupt(name string) board.DigitalInterrupt {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (bc *boardClient) GetConfig(ctx context.Context) (board.Config, error) {
	debug.PrintStack()
	return board.Config{}, errUnimplemented
}

func (bc *boardClient) Status(ctx context.Context) (*pb.BoardStatus, error) {
	if status := bc.rc.getCachedStatus(); status != nil {
		boardStatus, ok := status.Boards[bc.name]
		if !ok {
			return nil, fmt.Errorf("no board with name (%s)", bc.name)
		}
		return boardStatus, nil
	}
	resp, err := bc.rc.client.BoardStatus(ctx, &pb.BoardStatusRequest{
		Name: bc.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

type motorClient struct {
	rc        *RobotClient
	boardName string
	motorName string
}

func (mc *motorClient) Power(ctx context.Context, power byte) error {
	debug.PrintStack()
	return errUnimplemented
}

func (mc *motorClient) Go(ctx context.Context, d pb.DirectionRelative, power byte) error {
	_, err := mc.rc.client.BoardMotorGo(ctx, &pb.BoardMotorGoRequest{
		BoardName: mc.boardName,
		MotorName: mc.motorName,
		Direction: d,
		Power:     utils.ScaleByteToUInt32(power),
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

func (mc *motorClient) Position(ctx context.Context) (float64, error) {
	debug.PrintStack()
	return 0, errUnimplemented
}

func (mc *motorClient) PositionSupported(ctx context.Context) (bool, error) {
	debug.PrintStack()
	return false, errUnimplemented
}

func (mc *motorClient) Off(ctx context.Context) error {
	debug.PrintStack()
	return errUnimplemented
}

func (mc *motorClient) IsOn(ctx context.Context) (bool, error) {
	debug.PrintStack()
	return false, errUnimplemented
}

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

func (sc *servoClient) Current(ctx context.Context) (uint8, error) {
	debug.PrintStack()
	return 0, errUnimplemented
}

type cameraClient struct {
	rc   *RobotClient
	name string
}

func (cc *cameraClient) Next(ctx context.Context) (image.Image, func(), error) {
	resp, err := cc.rc.client.CameraFrame(ctx, &pb.CameraFrameRequest{
		Name:     cc.name,
		MimeType: "image/raw-rgba",
	})
	if err != nil {
		return nil, nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, int(resp.DimX), int(resp.DimY)))
	img.Pix = resp.Frame
	return img, func() {}, err
}

func (cc *cameraClient) Close() error {
	return nil
}

type lidarDeviceClient struct {
	rc   *RobotClient
	name string
}

func (ldc *lidarDeviceClient) Info(ctx context.Context) (map[string]interface{}, error) {
	resp, err := ldc.rc.client.LidarInfo(ctx, &pb.LidarInfoRequest{
		Name: ldc.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Info.AsMap(), nil
}

func (ldc *lidarDeviceClient) Start(ctx context.Context) error {
	_, err := ldc.rc.client.LidarStart(ctx, &pb.LidarStartRequest{
		Name: ldc.name,
	})
	return err
}

func (ldc *lidarDeviceClient) Stop(ctx context.Context) error {
	_, err := ldc.rc.client.LidarStop(ctx, &pb.LidarStopRequest{
		Name: ldc.name,
	})
	return err
}

func (ldc *lidarDeviceClient) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
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

func (ldc *lidarDeviceClient) Range(ctx context.Context) (int, error) {
	resp, err := ldc.rc.client.LidarRange(ctx, &pb.LidarRangeRequest{
		Name: ldc.name,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Range), nil
}

func (ldc *lidarDeviceClient) Bounds(ctx context.Context) (image.Point, error) {
	resp, err := ldc.rc.client.LidarBounds(ctx, &pb.LidarBoundsRequest{
		Name: ldc.name,
	})
	if err != nil {
		return image.Point{}, err
	}
	return image.Point{int(resp.X), int(resp.Y)}, nil
}

func (ldc *lidarDeviceClient) AngularResolution(ctx context.Context) (float64, error) {
	resp, err := ldc.rc.client.LidarAngularResolution(ctx, &pb.LidarAngularResolutionRequest{
		Name: ldc.name,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.AngularResolution, nil
}

func MeasurementFromProto(pm *pb.LidarMeasurement) *lidar.Measurement {
	return lidar.NewMeasurement(pm.AngleDeg, pm.Distance)
}

func MeasurementsFromProto(pms []*pb.LidarMeasurement) lidar.Measurements {
	ms := make(lidar.Measurements, 0, len(pms))
	for _, pm := range pms {
		ms = append(ms, MeasurementFromProto(pm))
	}
	return ms
}

type sensorClient struct {
	rc   *RobotClient
	name string
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

type relativeCompassClient struct {
	*compassClient
}

func (rcc *relativeCompassClient) Mark(ctx context.Context) error {
	_, err := rcc.rc.client.CompassMark(ctx, &pb.CompassMarkRequest{
		Name: rcc.name,
	})
	return err
}

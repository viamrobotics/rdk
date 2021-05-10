// Package server contains a gRPC based api.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"sync"
	"time"

	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot/action"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"
)

// Server implements the contract from robot.proto that ultimately satisfies
// an api.Robot as a gRPC server.
type Server struct {
	pb.UnimplementedRobotServiceServer
	r                       api.Robot
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancel                  func()
}

// New constructs a gRPC service server for a Robot.
func New(r api.Robot) pb.RobotServiceServer {
	cancelCtx, cancel := context.WithCancel(context.Background())
	return &Server{
		r:         r,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}
}

// Close cleanly shuts down the server.
func (s *Server) Close() error {
	s.cancel()
	s.activeBackgroundWorkers.Wait()
	return nil
}

// Status returns the robot's underlying status.
func (s *Server) Status(ctx context.Context, _ *pb.StatusRequest) (*pb.StatusResponse, error) {
	status, err := s.r.Status(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.StatusResponse{Status: status}, nil
}

const defaultStreamInterval = 1 * time.Second

// StatusStream periodically sends the robot's status.
func (s *Server) StatusStream(req *pb.StatusStreamRequest, server pb.RobotService_StatusStreamServer) error {
	every := defaultStreamInterval
	if reqEvery := req.Every.AsDuration(); reqEvery != time.Duration(0) {
		every = reqEvery
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		default:
		}
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		case <-ticker.C:
		}
		status, err := s.r.Status(server.Context())
		if err != nil {
			return err
		}
		if err := server.Send(&pb.StatusStreamResponse{Status: status}); err != nil {
			return err
		}
	}
}

// DoAction runs an action on the underlying robot.
func (s *Server) DoAction(ctx context.Context, req *pb.DoActionRequest) (*pb.DoActionResponse, error) {
	act := action.LookupAction(req.Name)
	if act == nil {
		return nil, fmt.Errorf("unknown action name [%s]", req.Name)
	}
	s.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer s.activeBackgroundWorkers.Done()
		act(s.cancelCtx, s.r)
	})
	return &pb.DoActionResponse{}, nil
}

// ArmCurrentPosition gets the current position of an arm of the underlying robot.
func (s *Server) ArmCurrentPosition(ctx context.Context, req *pb.ArmCurrentPositionRequest) (*pb.ArmCurrentPositionResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}
	pos, err := arm.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.ArmCurrentPositionResponse{Position: pos}, nil
}

// ArmCurrentJointPositions gets the current joint position of an arm of the underlying robot.
func (s *Server) ArmCurrentJointPositions(ctx context.Context, req *pb.ArmCurrentJointPositionsRequest) (*pb.ArmCurrentJointPositionsResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}
	pos, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.ArmCurrentJointPositionsResponse{Positions: pos}, nil
}

// ArmMoveToPosition moves an arm of the underlying robot to the requested position.
func (s *Server) ArmMoveToPosition(ctx context.Context, req *pb.ArmMoveToPositionRequest) (*pb.ArmMoveToPositionResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.ArmMoveToPositionResponse{}, arm.MoveToPosition(ctx, req.To)
}

// ArmMoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *Server) ArmMoveToJointPositions(ctx context.Context, req *pb.ArmMoveToJointPositionsRequest) (*pb.ArmMoveToJointPositionsResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.ArmMoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, req.To)
}

// BaseMoveStraight moves a base of the underlying robot straight.
func (s *Server) BaseMoveStraight(ctx context.Context, req *pb.BaseMoveStraightRequest) (*pb.BaseMoveStraightResponse, error) {
	base := s.r.BaseByName(req.Name)
	if base == nil {
		return nil, fmt.Errorf("no base with name (%s)", req.Name)
	}
	millisPerSec := 500.0 // TODO(erh): this is proably the wrong default
	if req.MillisPerSec != 0 {
		millisPerSec = req.MillisPerSec
	}
	moved, err := base.MoveStraight(ctx, int(req.DistanceMillis), millisPerSec, false)
	if err != nil {
		if moved == 0 {
			return nil, err
		}
		return &pb.BaseMoveStraightResponse{Success: false, Error: err.Error(), DistanceMillis: int64(moved)}, nil
	}
	return &pb.BaseMoveStraightResponse{Success: true, DistanceMillis: int64(moved)}, nil
}

// BaseSpin spins a base of the underlying robot.
func (s *Server) BaseSpin(ctx context.Context, req *pb.BaseSpinRequest) (*pb.BaseSpinResponse, error) {
	base := s.r.BaseByName(req.Name)
	if base == nil {
		return nil, fmt.Errorf("no base with name (%s)", req.Name)
	}
	degsPerSec := 64.0
	if req.DegsPerSec != 0 {
		degsPerSec = req.DegsPerSec
	}
	spun, err := base.Spin(ctx, req.AngleDeg, degsPerSec, false)
	if err != nil {
		if math.IsNaN(spun) || spun == 0 {
			return nil, err
		}
		return &pb.BaseSpinResponse{Success: false, Error: err.Error(), AngleDeg: spun}, nil
	}
	return &pb.BaseSpinResponse{Success: true, AngleDeg: spun}, nil

}

// BaseSpin stops a base of the underlying robot.
func (s *Server) BaseStop(ctx context.Context, req *pb.BaseStopRequest) (*pb.BaseStopResponse, error) {
	base := s.r.BaseByName(req.Name)
	if base == nil {
		return nil, fmt.Errorf("no base with name (%s)", req.Name)
	}
	return &pb.BaseStopResponse{}, base.Stop(ctx)
}

// GripperOpen opens a gripper of the underlying robot.
func (s *Server) GripperOpen(ctx context.Context, req *pb.GripperOpenRequest) (*pb.GripperOpenResponse, error) {
	gripper := s.r.GripperByName(req.Name)
	if gripper == nil {
		return nil, fmt.Errorf("no gripper with that name %s", req.Name)
	}
	return &pb.GripperOpenResponse{}, gripper.Open(ctx)
}

// GripperGrab requests a gripper of the underlying robot to grab.
func (s *Server) GripperGrab(ctx context.Context, req *pb.GripperGrabRequest) (*pb.GripperGrabResponse, error) {
	gripper := s.r.GripperByName(req.Name)
	if gripper == nil {
		return nil, fmt.Errorf("no gripper with that name %s", req.Name)
	}
	grabbed, err := gripper.Grab(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GripperGrabResponse{Grabbed: grabbed}, nil
}

const (
	mimeTypeViamBest = "image/viambest"
	mimeTypeRawIWD   = "image/raw-iwd"
	mimeTypeRawRGBA  = "image/raw-rgba"
	mimeTypeBoth     = "image/both"
	mimeTypeJPEG     = "image/jpeg"
	mimeTypePNG      = "image/png"
)

// CameraFrame returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *Server) CameraFrame(ctx context.Context, req *pb.CameraFrameRequest) (*pb.CameraFrameResponse, error) {
	camera := s.r.CameraByName(req.Name)
	if camera == nil {
		return nil, fmt.Errorf("no camera with name (%s)", req.Name)
	}

	img, release, err := camera.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	// choose the best/fastest representation
	if req.MimeType == mimeTypeViamBest {
		iwd, ok := img.(*rimage.ImageWithDepth)
		if ok && iwd.Depth != nil && iwd.Color != nil {
			req.MimeType = mimeTypeRawIWD
		} else {
			req.MimeType = mimeTypeRawRGBA
		}
	}

	bounds := img.Bounds()
	resp := pb.CameraFrameResponse{
		MimeType: req.MimeType,
		DimX:     int64(bounds.Dx()),
		DimY:     int64(bounds.Dy()),
	}

	var buf bytes.Buffer
	switch req.MimeType {
	case mimeTypeRawRGBA:
		resp.MimeType = mimeTypeRawRGBA
		imgCopy := image.NewRGBA(bounds)
		draw.Draw(imgCopy, bounds, img, bounds.Min, draw.Src)
		buf.Write(imgCopy.Pix)
	case mimeTypeRawIWD:
		resp.MimeType = mimeTypeRawIWD
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return nil, fmt.Errorf("want %s but don't have %T", mimeTypeRawIWD, iwd)
		}
		err := iwd.RawBytesWrite(&buf)
		if err != nil {
			return nil, fmt.Errorf("error writing %s: %w", mimeTypeRawIWD, err)
		}

	case mimeTypeBoth:
		resp.MimeType = mimeTypeBoth
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return nil, fmt.Errorf("want %s but don't have %T", mimeTypeBoth, iwd)
		}
		if iwd.Color == nil || iwd.Depth == nil {
			return nil, fmt.Errorf("for %s need depth and color info", mimeTypeBoth)
		}
		if err := rimage.BothEncode(iwd, &buf); err != nil {
			return nil, err
		}
	case mimeTypeJPEG:
		resp.MimeType = mimeTypeJPEG
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	case "", mimeTypePNG:
		resp.MimeType = mimeTypePNG
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("do not know how to encode %q", req.MimeType)
	}
	resp.Frame = buf.Bytes()
	return &resp, nil
}

// CameraFrame renderse a frame from a camera of the underlying robot to an HTTP response. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *Server) CameraRenderFrame(ctx context.Context, req *pb.CameraRenderFrameRequest) (*httpbody.HttpBody, error) {
	resp, err := s.CameraFrame(ctx, (*pb.CameraFrameRequest)(req))
	if err != nil {
		return nil, err
	}

	return &httpbody.HttpBody{
		ContentType: resp.MimeType,
		Data:        resp.Frame,
	}, nil
}

// LidarInfo returns the info of a lidar device of the underlying robot.
func (s *Server) LidarInfo(ctx context.Context, req *pb.LidarInfoRequest) (*pb.LidarInfoResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	info, err := lidarDevice.Info(ctx)
	if err != nil {
		return nil, err
	}
	str, err := structpb.NewStruct(info)
	if err != nil {
		return nil, err
	}
	return &pb.LidarInfoResponse{Info: str}, nil
}

// LidarStart starts a lidar device of the underlying robot.
func (s *Server) LidarStart(ctx context.Context, req *pb.LidarStartRequest) (*pb.LidarStartResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	err := lidarDevice.Start(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarStartResponse{}, nil
}

// LidarStop stops a lidar device of the underlying robot.
func (s *Server) LidarStop(ctx context.Context, req *pb.LidarStopRequest) (*pb.LidarStopResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	err := lidarDevice.Stop(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarStopResponse{}, nil
}

// LidarScan returns a scan from a lidar device of the underlying robot.
func (s *Server) LidarScan(ctx context.Context, req *pb.LidarScanRequest) (*pb.LidarScanResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	opts := scanOptionsFromProto(req)
	ms, err := lidarDevice.Scan(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &pb.LidarScanResponse{Measurements: measurementsToProto(ms)}, nil
}

// LidarRange returns the range of a lidar device of the underlying robot.
func (s *Server) LidarRange(ctx context.Context, req *pb.LidarRangeRequest) (*pb.LidarRangeResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	r, err := lidarDevice.Range(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarRangeResponse{Range: int64(r)}, nil
}

// LidarBounds returns the scan bounds of a lidar device of the underlying robot.
func (s *Server) LidarBounds(ctx context.Context, req *pb.LidarBoundsRequest) (*pb.LidarBoundsResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	bounds, err := lidarDevice.Bounds(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarBoundsResponse{X: int64(bounds.X), Y: int64(bounds.Y)}, nil
}

// LidarAngularResolution returns the scan angular resolution of a lidar device of the underlying robot.
func (s *Server) LidarAngularResolution(ctx context.Context, req *pb.LidarAngularResolutionRequest) (*pb.LidarAngularResolutionResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	angRes, err := lidarDevice.AngularResolution(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarAngularResolutionResponse{AngularResolution: angRes}, nil
}

func scanOptionsFromProto(req *pb.LidarScanRequest) lidar.ScanOptions {
	return lidar.ScanOptions{
		Count:    int(req.Count),
		NoFilter: req.NoFilter,
	}
}

func measurementToProto(m *lidar.Measurement) *pb.LidarMeasurement {
	x, y := m.Coords()
	return &pb.LidarMeasurement{
		Angle:    m.AngleRad(),
		AngleDeg: m.AngleDeg(),
		Distance: m.Distance(),
		X:        x,
		Y:        y,
	}
}

func measurementsToProto(ms lidar.Measurements) []*pb.LidarMeasurement {
	pms := make([]*pb.LidarMeasurement, 0, len(ms))
	for _, m := range ms {
		pms = append(pms, measurementToProto(m))
	}
	return pms
}

// BoardStatus returns the status of a board of the underlying robot.
func (s *Server) BoardStatus(ctx context.Context, req *pb.BoardStatusRequest) (*pb.BoardStatusResponse, error) {
	b := s.r.BoardByName(req.Name)
	if b == nil {
		return nil, fmt.Errorf("no board with name (%s)", req.Name)
	}

	status, err := b.Status(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.BoardStatusResponse{Status: status}, nil
}

// BoardMotorGo requests the motor of a board of the underlying robot to go.
func (s *Server) BoardMotorGo(ctx context.Context, req *pb.BoardMotorGoRequest) (*pb.BoardMotorGoResponse, error) {
	b := s.r.BoardByName(req.BoardName)
	if b == nil {
		return nil, fmt.Errorf("no board with name (%s)", req.BoardName)
	}

	theMotor := b.Motor(req.MotorName)
	if theMotor == nil {
		return nil, fmt.Errorf("unknown motor: %s", req.MotorName)
	}

	return &pb.BoardMotorGoResponse{}, theMotor.Go(ctx, req.Direction, req.PowerPct)
}

// BoardMotorGoFor requests the motor of a board of the underlying robot to go for a certain amount based off
// the request.
func (s *Server) BoardMotorGoFor(ctx context.Context, req *pb.BoardMotorGoForRequest) (*pb.BoardMotorGoForResponse, error) {
	b := s.r.BoardByName(req.BoardName)
	if b == nil {
		return nil, fmt.Errorf("no board with name (%s)", req.BoardName)
	}

	theMotor := b.Motor(req.MotorName)
	if theMotor == nil {
		return nil, fmt.Errorf("unknown motor: %s", req.MotorName)
	}

	// erh: this isn't right semantically.
	// GoFor with 0 rotations means something important.
	rVal := 0.0
	if req.Revolutions != 0 {
		rVal = req.Revolutions
	}

	return &pb.BoardMotorGoForResponse{}, theMotor.GoFor(ctx, req.Direction, req.Rpm, rVal)
}

// BoardServoMove requests the servo of a board of the underlying robot to move.
func (s *Server) BoardServoMove(ctx context.Context, req *pb.BoardServoMoveRequest) (*pb.BoardServoMoveResponse, error) {
	b := s.r.BoardByName(req.BoardName)
	if b == nil {
		return nil, fmt.Errorf("no board with name (%s)", req.BoardName)
	}

	theServo := b.Servo(req.ServoName)
	if theServo == nil {
		return nil, fmt.Errorf("unknown servo: %s", req.ServoName)
	}

	return &pb.BoardServoMoveResponse{}, theServo.Move(ctx, uint8(req.AngleDeg))
}

// SensorReadings returns the readings of a sensor of the underlying robot.
func (s *Server) SensorReadings(ctx context.Context, req *pb.SensorReadingsRequest) (*pb.SensorReadingsResponse, error) {
	sensorDevice := s.r.SensorByName(req.Name)
	if sensorDevice == nil {
		return nil, fmt.Errorf("no sensor with name (%s)", req.Name)
	}
	readings, err := sensorDevice.Readings(ctx)
	if err != nil {
		return nil, err
	}
	readingsP := make([]*structpb.Value, 0, len(readings))
	for _, r := range readings {
		v, err := structpb.NewValue(r)
		if err != nil {
			return nil, err
		}
		readingsP = append(readingsP, v)
	}
	return &pb.SensorReadingsResponse{Readings: readingsP}, nil
}

func (s *Server) compassByName(name string) (compass.Device, error) {
	sensorDevice := s.r.SensorByName(name)
	if sensorDevice == nil {
		return nil, fmt.Errorf("no sensor with name (%s)", name)
	}
	return sensorDevice.(compass.Device), nil
}

// CompassHeading returns the heading of a compass of the underlying robot.
func (s *Server) CompassHeading(ctx context.Context, req *pb.CompassHeadingRequest) (*pb.CompassHeadingResponse, error) {
	compassDevice, err := s.compassByName(req.Name)
	if err != nil {
		return nil, err
	}
	heading, err := compassDevice.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.CompassHeadingResponse{Heading: heading}, nil
}

// CompassStartCalibration requests the compass of the underlying robot to start calibration.
func (s *Server) CompassStartCalibration(ctx context.Context, req *pb.CompassStartCalibrationRequest) (*pb.CompassStartCalibrationResponse, error) {
	compassDevice, err := s.compassByName(req.Name)
	if err != nil {
		return nil, err
	}
	if err := compassDevice.StartCalibration(ctx); err != nil {
		return nil, err
	}
	return &pb.CompassStartCalibrationResponse{}, nil
}

// CompassStopCalibration requests the compass of the underlying robot to stop calibration.
func (s *Server) CompassStopCalibration(ctx context.Context, req *pb.CompassStopCalibrationRequest) (*pb.CompassStopCalibrationResponse, error) {
	compassDevice, err := s.compassByName(req.Name)
	if err != nil {
		return nil, err
	}
	if err := compassDevice.StopCalibration(ctx); err != nil {
		return nil, err
	}
	return &pb.CompassStopCalibrationResponse{}, nil
}

// CompassMark requests the relative compass of the underlying robot to mark its position.
func (s *Server) CompassMark(ctx context.Context, req *pb.CompassMarkRequest) (*pb.CompassMarkResponse, error) {
	compassDevice, err := s.compassByName(req.Name)
	if err != nil {
		return nil, err
	}
	rel, ok := compassDevice.(compass.RelativeDevice)
	if !ok {
		return nil, errors.New("compass is not relative")
	}
	if err := rel.Mark(ctx); err != nil {
		return nil, err
	}
	return &pb.CompassMarkResponse{}, nil
}

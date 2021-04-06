package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot/actions"
	"go.viam.com/robotcore/sensor/compass"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/types/known/structpb"
)

type Server struct {
	pb.UnimplementedRobotServiceServer
	r api.Robot
}

func New(r api.Robot) pb.RobotServiceServer {
	return &Server{r: r}
}

func (s *Server) Status(ctx context.Context, _ *pb.StatusRequest) (*pb.StatusResponse, error) {
	status, err := s.r.Status(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.StatusResponse{Status: status}, nil
}

const defaultStreamInterval = 1 * time.Second

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

func (s *Server) DoAction(ctx context.Context, req *pb.DoActionRequest) (*pb.DoActionResponse, error) {
	action := actions.LookupAction(req.Name)
	if action == nil {
		return nil, fmt.Errorf("unknown action name [%s]", req.Name)
	}
	go action(s.r)
	return &pb.DoActionResponse{}, nil
}

func (s *Server) ControlBase(ctx context.Context, req *pb.ControlBaseRequest) (*pb.ControlBaseResponse, error) {
	base := s.r.BaseByName(req.Name)
	if base == nil {
		return nil, fmt.Errorf("no base with name (%s)", req.Name)
	}

	switch v := req.Action.(type) {
	case *pb.ControlBaseRequest_Stop:
		if v.Stop {
			return &pb.ControlBaseResponse{}, base.Stop(ctx)
		}
		return &pb.ControlBaseResponse{}, nil
	case *pb.ControlBaseRequest_Move:
		if v.Move == nil {
			return &pb.ControlBaseResponse{}, errors.New("move unspecified")
		}
		millisPerSec := 500.0 // TODO(erh): this is proably the wrong default
		if v.Move.Speed != 0 {
			millisPerSec = v.Move.Speed
		}
		switch o := v.Move.Option.(type) {
		case *pb.MoveBase_StraightDistanceMillis:
			return &pb.ControlBaseResponse{}, base.MoveStraight(ctx, int(o.StraightDistanceMillis), millisPerSec, false)
		case *pb.MoveBase_SpinAngleDeg:
			return &pb.ControlBaseResponse{}, base.Spin(ctx, o.SpinAngleDeg, 64, false)
		default:
			return nil, fmt.Errorf("unknown move %T", o)
		}
	default:
		return nil, fmt.Errorf("unknown action %T", v)
	}
}

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

func (s *Server) MoveArmToPosition(ctx context.Context, req *pb.MoveArmToPositionRequest) (*pb.MoveArmToPositionResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.MoveArmToPositionResponse{}, arm.MoveToPosition(ctx, req.To)
}

func (s *Server) MoveArmToJointPositions(ctx context.Context, req *pb.MoveArmToJointPositionsRequest) (*pb.MoveArmToJointPositionsResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.MoveArmToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, req.To)
}

func (s *Server) ControlGripper(ctx context.Context, req *pb.ControlGripperRequest) (*pb.ControlGripperResponse, error) {
	gripper := s.r.GripperByName(req.Name)
	if gripper == nil {
		return nil, fmt.Errorf("no gripper with that name %s", req.Name)
	}

	var grabbed bool
	switch req.Action {
	case pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_OPEN:
		if err := gripper.Open(ctx); err != nil {
			return nil, err
		}
	case pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_GRAB:
		var err error
		grabbed, err = gripper.Grab(ctx)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown action: (%s)", req.Action)
	}

	return &pb.ControlGripperResponse{Grabbed: grabbed}, nil
}

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

func (s *Server) ControlBoardMotor(ctx context.Context, req *pb.ControlBoardMotorRequest) (*pb.ControlBoardMotorResponse, error) {
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
	if req.Rotations != 0 {
		rVal = req.Rotations
	}

	if rVal == 0 {
		return &pb.ControlBoardMotorResponse{}, theMotor.Go(ctx, req.Direction, byte(req.Speed))
	}

	return &pb.ControlBoardMotorResponse{}, theMotor.GoFor(ctx, req.Direction, req.Speed, rVal)
}

func (s *Server) ControlBoardServo(ctx context.Context, req *pb.ControlBoardServoRequest) (*pb.ControlBoardServoResponse, error) {
	b := s.r.BoardByName(req.BoardName)
	if b == nil {
		return nil, fmt.Errorf("no board with name (%s)", req.BoardName)
	}

	theServo := b.Servo(req.ServoName)
	if theServo == nil {
		return nil, fmt.Errorf("unknown servo: %s", req.ServoName)
	}

	return &pb.ControlBoardServoResponse{}, theServo.Move(ctx, uint8(req.AngleDeg))
}

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

	bounds := img.Bounds()
	resp := pb.CameraFrameResponse{
		MimeType: req.MimeType,
		DimX:     int64(bounds.Dx()),
		DimY:     int64(bounds.Dy()),
	}
	var buf bytes.Buffer
	switch req.MimeType {
	case "image/raw-rgba":
		imgCopy := image.NewRGBA(bounds)
		draw.Draw(imgCopy, bounds, img, bounds.Min, draw.Src)
		buf.Write(imgCopy.Pix)
	case "", "image/jpeg":
		resp.MimeType = "image/jpeg"
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("do not know how to encode %q", req.MimeType)
	}
	resp.Frame = buf.Bytes()
	return &resp, nil
}

func (s *Server) RenderCameraFrame(ctx context.Context, req *pb.CameraFrameRequest) (*httpbody.HttpBody, error) {
	resp, err := s.CameraFrame(ctx, req)
	if err != nil {
		return nil, err
	}

	return &httpbody.HttpBody{
		ContentType: resp.MimeType,
		Data:        resp.Frame,
	}, nil
}

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

func (s *Server) LidarScan(ctx context.Context, req *pb.LidarScanRequest) (*pb.LidarScanResponse, error) {
	lidarDevice := s.r.LidarDeviceByName(req.Name)
	if lidarDevice == nil {
		return nil, fmt.Errorf("no lidar device with name (%s)", req.Name)
	}
	opts := ScanOptionsFromProto(req)
	ms, err := lidarDevice.Scan(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &pb.LidarScanResponse{Measurements: MeasurementsToProto(ms)}, nil
}

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

func ScanOptionsFromProto(req *pb.LidarScanRequest) lidar.ScanOptions {
	return lidar.ScanOptions{
		Count:    int(req.Count),
		NoFilter: req.NoFilter,
	}
}

func MeasurementToProto(m *lidar.Measurement) *pb.LidarMeasurement {
	x, y := m.Coords()
	return &pb.LidarMeasurement{
		Angle:    m.AngleRad(),
		AngleDeg: m.AngleDeg(),
		Distance: m.Distance(),
		X:        x,
		Y:        y,
	}
}

func MeasurementsToProto(ms lidar.Measurements) []*pb.LidarMeasurement {
	pms := make([]*pb.LidarMeasurement, 0, len(ms))
	for _, m := range ms {
		pms = append(pms, MeasurementToProto(m))
	}
	return pms
}

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
	sensorType := api.GetSensorDeviceType(sensorDevice)
	if sensorType != compass.DeviceType && sensorType != compass.RelativeDeviceType {
		return nil, fmt.Errorf("unexpected sensor type %q", sensorType)
	}
	return sensorDevice.(compass.Device), nil
}

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

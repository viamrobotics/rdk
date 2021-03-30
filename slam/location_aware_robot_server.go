package slam

import (
	"context"
	"errors"
	"fmt"
	"math"

	pb "go.viam.com/robotcore/proto/slam/v1"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"go.uber.org/multierr"
)

const defaultClientMoveAmountMillis = 200

type LocationAwareRobotServer struct {
	pb.UnimplementedSlamServiceServer
	lar *LocationAwareRobot
}

func NewLocationAwareRobotServer(lar *LocationAwareRobot) *LocationAwareRobotServer {
	return &LocationAwareRobotServer{lar: lar}
}

func (s *LocationAwareRobotServer) Save(ctx context.Context, req *pb.SaveRequest) (*pb.SaveResponse, error) {
	s.lar.serverMu.Lock()
	defer s.lar.serverMu.Unlock()
	if req.File == "" {
		return nil, errors.New("file to save to required")
	}
	return &pb.SaveResponse{}, s.lar.rootArea.WriteToFile(req.File)
}

func (s *LocationAwareRobotServer) Stats(ctx context.Context, _ *pb.StatsRequest) (*pb.StatsResponse, error) {
	return &pb.StatsResponse{BasePosition: &pb.BasePosition{
		X: int64(s.lar.basePosX),
		Y: int64(s.lar.basePosY),
	}}, nil
}

func (s *LocationAwareRobotServer) Calibrate(ctx context.Context, _ *pb.CalibrateRequest) (resp *pb.CalibrateResponse, err error) {
	s.lar.serverMu.Lock()
	defer s.lar.serverMu.Unlock()
	if s.lar.compassSensor == nil {
		return &pb.CalibrateResponse{}, nil
	}
	if err := s.lar.compassSensor.StartCalibration(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if stopErr := s.lar.compassSensor.StopCalibration(ctx); stopErr != nil {
			err = multierr.Combine(err, stopErr)
		}
	}()
	step := 10.0
	for i := 0.0; i < 360; i += step {
		if err := compass.ReduceBase(s.lar.baseDevice).Spin(ctx, step, 0, true); err != nil {
			return nil, err
		}
	}
	return &pb.CalibrateResponse{}, nil
}

func (s *LocationAwareRobotServer) MoveRobot(ctx context.Context, req *pb.MoveRobotRequest) (*pb.MoveRobotResponse, error) {
	if req.Direction == pb.Direction_DIRECTION_UNSPECIFIED {
		return nil, errors.New("move direction required")
	}
	amount := defaultClientMoveAmountMillis
	if err := s.lar.Move(ctx, &amount, &req.Direction); err != nil {
		return nil, err
	}
	return &pb.MoveRobotResponse{
		NewPosition: &pb.BasePosition{
			X: int64(s.lar.basePosX),
			Y: int64(s.lar.basePosY),
		},
	}, nil
}

func (s *LocationAwareRobotServer) MoveRobotForward(ctx context.Context, _ *pb.MoveRobotForwardRequest) (*pb.MoveRobotForwardResponse, error) {
	amount := defaultClientMoveAmountMillis
	if err := s.lar.Move(ctx, &amount, nil); err != nil {
		return nil, err
	}
	return &pb.MoveRobotForwardResponse{
		NewPosition: &pb.BasePosition{
			X: int64(s.lar.basePosX),
			Y: int64(s.lar.basePosY),
		},
	}, nil
}

func (s *LocationAwareRobotServer) MoveRobotBackward(ctx context.Context, _ *pb.MoveRobotBackwardRequest) (*pb.MoveRobotBackwardResponse, error) {
	amount := -defaultClientMoveAmountMillis
	if err := s.lar.Move(ctx, &amount, nil); err != nil {
		return nil, err
	}
	return &pb.MoveRobotBackwardResponse{
		NewPosition: &pb.BasePosition{
			X: int64(s.lar.basePosX),
			Y: int64(s.lar.basePosY),
		},
	}, nil
}

func (s *LocationAwareRobotServer) TurnRobotTo(ctx context.Context, req *pb.TurnRobotToRequest) (*pb.TurnRobotToResponse, error) {
	if req.Direction == pb.Direction_DIRECTION_UNSPECIFIED {
		return nil, errors.New("rotation direction required")
	}
	if err := s.lar.rotateTo(ctx, req.Direction); err != nil {
		return nil, err
	}
	return &pb.TurnRobotToResponse{}, nil
}

func (s *LocationAwareRobotServer) UpdateRobotDeviceOffset(ctx context.Context, req *pb.UpdateRobotDeviceOffsetRequest) (*pb.UpdateRobotDeviceOffsetResponse, error) {
	if req.OffsetIndex < 0 || int(req.OffsetIndex) >= len(s.lar.deviceOffsets) {
		return nil, errors.New("bad offset index")
	}
	if req.Offset == nil {
		return nil, errors.New("device offset required")
	}
	s.lar.deviceOffsets[req.OffsetIndex] = DeviceOffset{req.Offset.Angle, req.Offset.DistanceX, req.Offset.DistanceY}
	return nil, nil
}

func (s *LocationAwareRobotServer) StartLidar(ctx context.Context, req *pb.StartLidarRequest) (*pb.StartLidarResponse, error) {
	if err := s.lar.validateLidarDeviceNumber(req.DeviceNumber); err != nil {
		return nil, err
	}
	if err := s.lar.devices[req.DeviceNumber].Start(ctx); err != nil {
		return nil, err
	}
	return &pb.StartLidarResponse{}, nil
}

func (s *LocationAwareRobotServer) StopLidar(ctx context.Context, req *pb.StopLidarRequest) (*pb.StopLidarResponse, error) {
	if err := s.lar.validateLidarDeviceNumber(req.DeviceNumber); err != nil {
		return nil, err
	}
	if err := s.lar.devices[req.DeviceNumber].Stop(ctx); err != nil {
		return nil, err
	}
	return &pb.StopLidarResponse{}, nil
}

func (s *LocationAwareRobotServer) GetLidarSeed(ctx context.Context, req *pb.GetLidarSeedRequest) (*pb.GetLidarSeedResponse, error) {
	seeds := make([]string, len(s.lar.devices))
	for i, dev := range s.lar.devices {
		if fake, ok := dev.(*fake.Lidar); ok {
			seeds[i] = fmt.Sprintf("%d", fake.Seed())
		} else {
			seeds[i] = "real-device"
		}
	}
	return &pb.GetLidarSeedResponse{
		Seeds: seeds,
	}, nil
}

func (s *LocationAwareRobotServer) SetLidarSeed(ctx context.Context, req *pb.SetLidarSeedRequest) (*pb.SetLidarSeedResponse, error) {
	if err := s.lar.validateLidarDeviceNumber(req.DeviceNumber); err != nil {
		return nil, err
	}

	if fake, ok := s.lar.devices[req.DeviceNumber].(*fake.Lidar); ok {
		fake.SetSeed(req.Seed)
		return &pb.SetLidarSeedResponse{}, nil
	}
	return nil, errors.New("cannot set seed on real device")
}

func (s *LocationAwareRobotServer) SetClientZoom(ctx context.Context, req *pb.SetClientZoomRequest) (*pb.SetClientZoomResponse, error) {
	if req.Zoom < 1 {
		return nil, errors.New("zoom must be >= 1")
	}
	s.lar.clientZoom = req.Zoom
	return &pb.SetClientZoomResponse{}, nil
}

func (s *LocationAwareRobotServer) SetClientLidarViewMode(ctx context.Context, req *pb.SetClientLidarViewModeRequest) (*pb.SetClientLidarViewModeResponse, error) {
	if req.Mode == pb.LidarViewMode_LIDAR_VIEW_MODE_UNSPECIFIED {
		return nil, errors.New("mode required")
	}
	s.lar.clientLidarViewMode = req.Mode
	return &pb.SetClientLidarViewModeResponse{}, nil
}

func (s *LocationAwareRobotServer) SetClientClickMode(ctx context.Context, req *pb.SetClientClickModeRequest) (*pb.SetClientClickModeResponse, error) {
	if req.Mode == pb.ClickMode_CLICK_MODE_UNSPECIFIED {
		return nil, errors.New("mode required")
	}
	s.lar.clientClickMode = req.Mode
	return &pb.SetClientClickModeResponse{}, nil
}

func (lar *LocationAwareRobot) validateLidarDeviceNumber(num int32) error {
	if num < 0 || num >= int32(len(lar.devices)) {
		return errors.New("invalid device number")
	}
	return nil
}

func (lar *LocationAwareRobot) HandleClick(ctx context.Context, x, y, viewWidth, viewHeight int) (string, error) {
	switch lar.clientClickMode {
	case pb.ClickMode_CLICK_MODE_MOVE:
		dir := DirectionFromXY(x, y, viewWidth, viewHeight)
		amount := defaultClientMoveAmountMillis
		if err := lar.Move(ctx, &amount, &dir); err != nil {
			return "", err
		}
		return fmt.Sprintf("moved %q\n%s", dir, lar), nil
	case pb.ClickMode_CLICK_MODE_INFO:
		_, bounds, areas := lar.areasToView()

		bounds.X = int(math.Ceil(float64(bounds.X) * float64(lar.unitsPerMeter) / lar.clientZoom))
		bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(lar.unitsPerMeter) / lar.clientZoom))

		basePosX, basePosY := lar.basePos()
		minX := basePosX - bounds.X/2
		minY := basePosY - bounds.Y/2

		areaX := minX + int(float64(bounds.X)*(float64(x)/float64(viewWidth)))
		areaY := minY + int(float64(bounds.Y)*(float64(y)/float64(viewHeight)))

		distanceCenterF := math.Sqrt(float64(((areaX - basePosX) * (areaX - basePosX)) + ((areaY - basePosY) * (areaY - basePosY))))
		distanceCenter := int(distanceCenterF)
		frontY := basePosY - lar.baseDeviceWidthUnits/2
		distanceFront := int(math.Sqrt(float64(((areaX - basePosX) * (areaX - basePosX)) + ((areaY - frontY) * (areaY - frontY)))))

		xForAngle := (areaX - basePosX)
		yForAngle := (areaY - basePosY)
		yForAngle *= -1
		angelCenterRad := math.Atan2(float64(xForAngle), float64(yForAngle))
		angleCenter := utils.RadToDeg(angelCenterRad)
		if angleCenter < 0 {
			angleCenter = 360 + angleCenter
		}

		var present bool
		for _, area := range areas {
			area.Mutate(func(area MutableArea) {
				present = area.At(areaX, areaY) != 0
			})
			if present {
				break
			}
		}

		return fmt.Sprintf("(%d,%d): object=%t, angleCenter=%f,%f, distanceCenter=%dcm distanceFront=%dcm", areaX, areaY, present, angleCenter, angelCenterRad, distanceCenter, distanceFront), nil
	default:
		return "", fmt.Errorf("do not know how to handle click in mode %q", lar.clientClickMode)
	}
}

package slam

import (
	"context"
	"fmt"
	"math"

	"github.com/go-errors/errors"

	"go.viam.com/core/base"
	pb "go.viam.com/core/proto/slam/v1"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/utils"

	"go.uber.org/multierr"
)

const defaultClientMoveAmountMillis = 200

// LocationAwareRobotServer is a gRPC implementation of proto/slam/v1/slam.proto.
type LocationAwareRobotServer struct {
	pb.UnimplementedSlamServiceServer
	lar *LocationAwareRobot
}

// NewLocationAwareRobotServer returns a new server for a given robot.
func NewLocationAwareRobotServer(lar *LocationAwareRobot) *LocationAwareRobotServer {
	return &LocationAwareRobotServer{lar: lar}
}

// Save writes the state of the robot to a file.
func (s *LocationAwareRobotServer) Save(ctx context.Context, req *pb.SaveRequest) (*pb.SaveResponse, error) {
	s.lar.serverMu.Lock()
	defer s.lar.serverMu.Unlock()
	if req.File == "" {
		return nil, errors.New("file to save to required")
	}
	return &pb.SaveResponse{}, s.lar.rootArea.WriteToFile(req.File)
}

// Stats returns statistics about the robot.
func (s *LocationAwareRobotServer) Stats(ctx context.Context, _ *pb.StatsRequest) (*pb.StatsResponse, error) {
	return &pb.StatsResponse{BasePosition: &pb.BasePosition{
		X: int64(s.lar.basePosX),
		Y: int64(s.lar.basePosY),
	}}, nil
}

// Calibrate asks the robot to calibrate itself. It won't operate normally during this time.
func (s *LocationAwareRobotServer) Calibrate(ctx context.Context, _ *pb.CalibrateRequest) (resp *pb.CalibrateResponse, err error) {
	s.lar.serverMu.Lock()
	defer s.lar.serverMu.Unlock()
	if s.lar.compassensor == nil {
		return &pb.CalibrateResponse{}, nil
	}
	if err := s.lar.compassensor.StartCalibration(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if stopErr := s.lar.compassensor.StopCalibration(ctx); stopErr != nil {
			err = multierr.Combine(err, stopErr)
		}
	}()
	step := 10.0
	for i := 0.0; i < 360; i += step {
		if _, err := base.Reduce(s.lar.baseDevice).Spin(ctx, step, 0, true); err != nil {
			return nil, err
		}
	}
	return &pb.CalibrateResponse{}, nil
}

// MoveRobot instructs the robot to move based on the request.
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

// MoveRobotForward moves the robot forwards by some predetermined amount.
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

// MoveRobotBackward moves the robot backwards by some predetermined amount.
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

// TurnRobotTo has the robot move to a specified direction relative to the map.
func (s *LocationAwareRobotServer) TurnRobotTo(ctx context.Context, req *pb.TurnRobotToRequest) (*pb.TurnRobotToResponse, error) {
	if req.Direction == pb.Direction_DIRECTION_UNSPECIFIED {
		return nil, errors.New("rotation direction required")
	}
	if err := s.lar.rotateTo(ctx, req.Direction); err != nil {
		return nil, err
	}
	return &pb.TurnRobotToResponse{}, nil
}

// UpdateRobotDeviceOffset updates a lidar device offset.
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

// StartLidar starts up a specific lidar.
func (s *LocationAwareRobotServer) StartLidar(ctx context.Context, req *pb.StartLidarRequest) (*pb.StartLidarResponse, error) {
	if err := s.lar.validateLidarNumber(req.DeviceNumber); err != nil {
		return nil, err
	}
	if err := s.lar.devices[req.DeviceNumber].Start(ctx); err != nil {
		return nil, err
	}
	return &pb.StartLidarResponse{}, nil
}

// StopLidar stops a specific lidar.
func (s *LocationAwareRobotServer) StopLidar(ctx context.Context, req *pb.StopLidarRequest) (*pb.StopLidarResponse, error) {
	if err := s.lar.validateLidarNumber(req.DeviceNumber); err != nil {
		return nil, err
	}
	if err := s.lar.devices[req.DeviceNumber].Stop(ctx); err != nil {
		return nil, err
	}
	return &pb.StopLidarResponse{}, nil
}

// GetLidarSeed gets the seed used for RNG of a specific lidar if it is fake.
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

// SetLidarSeed sets the seed used for RNG of a specific lidar if it is fake.
func (s *LocationAwareRobotServer) SetLidarSeed(ctx context.Context, req *pb.SetLidarSeedRequest) (*pb.SetLidarSeedResponse, error) {
	if err := s.lar.validateLidarNumber(req.DeviceNumber); err != nil {
		return nil, err
	}

	if fake, ok := s.lar.devices[req.DeviceNumber].(*fake.Lidar); ok {
		fake.SetSeed(req.Seed)
		return &pb.SetLidarSeedResponse{}, nil
	}
	return nil, errors.New("cannot set seed on real device")
}

// SetClientZoom sets how much to the UI of the map be zoomed in.
func (s *LocationAwareRobotServer) SetClientZoom(ctx context.Context, req *pb.SetClientZoomRequest) (*pb.SetClientZoomResponse, error) {
	if req.Zoom < 1 {
		return nil, errors.New("zoom must be >= 1")
	}
	s.lar.clientZoom = req.Zoom
	return &pb.SetClientZoomResponse{}, nil
}

// SetClientLidarViewMode sets what the UI should show for lidar.
func (s *LocationAwareRobotServer) SetClientLidarViewMode(ctx context.Context, req *pb.SetClientLidarViewModeRequest) (*pb.SetClientLidarViewModeResponse, error) {
	if req.Mode == pb.LidarViewMode_LIDAR_VIEW_MODE_UNSPECIFIED {
		return nil, errors.New("mode required")
	}
	s.lar.clientLidarViewMode = req.Mode
	return &pb.SetClientLidarViewModeResponse{}, nil
}

// SetClientClickMode sets what should happen when the view is clicked.
func (s *LocationAwareRobotServer) SetClientClickMode(ctx context.Context, req *pb.SetClientClickModeRequest) (*pb.SetClientClickModeResponse, error) {
	if req.Mode == pb.ClickMode_CLICK_MODE_UNSPECIFIED {
		return nil, errors.New("mode required")
	}
	s.lar.clientClickMode = req.Mode
	return &pb.SetClientClickModeResponse{}, nil
}

func (lar *LocationAwareRobot) validateLidarNumber(num int32) error {
	if num < 0 || num >= int32(len(lar.devices)) {
		return errors.New("invalid device number")
	}
	return nil
}

// HandleClick takes a client click in and performs an action based on the click mode set.
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

		bounds.X = math.Ceil(bounds.X * lar.unitsPerMeter / lar.clientZoom)
		bounds.Y = math.Ceil(bounds.Y * lar.unitsPerMeter / lar.clientZoom)

		basePosX, basePosY := lar.basePos()
		minX := basePosX - bounds.X/2
		minY := basePosY - bounds.Y/2

		areaX := minX + bounds.X*(float64(x)/float64(viewWidth))
		areaY := minY + bounds.Y*(float64(y)/float64(viewHeight))

		distanceCenter := math.Sqrt(((areaX - basePosX) * (areaX - basePosX)) + ((areaY - basePosY) * (areaY - basePosY)))
		frontY := basePosY - lar.baseDeviceWidthUnits/2
		distanceFront := math.Sqrt(((areaX - basePosX) * (areaX - basePosX)) + ((areaY - frontY) * (areaY - frontY)))

		xForAngle := (areaX - basePosX)
		yForAngle := (areaY - basePosY)
		yForAngle *= -1
		angelCenterRad := math.Atan2(xForAngle, yForAngle)
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

		return fmt.Sprintf("(%d,%d): object=%t, angleCenter=%f,%f, distanceCenter=%dcm distanceFront=%dcm", int(areaX), int(areaY), present, angleCenter, angelCenterRad, int(distanceCenter), int(distanceFront)), nil
	default:
		return "", errors.Errorf("do not know how to handle click in mode %q", lar.clientClickMode)
	}
}

// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"context"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/gps"
	functionrobot "go.viam.com/rdk/function/robot"
	functionvm "go.viam.com/rdk/function/vm"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor/compass"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/objectmanipulation"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// Server implements the contract from robot.proto that ultimately satisfies
// an robot.Robot as a gRPC server.
type Server struct {
	pb.UnimplementedRobotServiceServer
	r                       robot.Robot
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancel                  func()
}

// New constructs a gRPC service server for a Robot.
func New(r robot.Robot) pb.RobotServiceServer {
	cancelCtx, cancel := context.WithCancel(context.Background())
	return &Server{
		r:         r,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}
}

// Close cleanly shuts down the server.
func (s *Server) Close() {
	s.cancel()
	s.activeBackgroundWorkers.Wait()
}

// Status returns the robot's underlying status.
func (s *Server) Status(ctx context.Context, _ *pb.StatusRequest) (*pb.StatusResponse, error) {
	status, err := s.r.Status(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.StatusResponse{Status: status}, nil
}

// Config returns the robot's underlying config.
func (s *Server) Config(ctx context.Context, _ *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	cfg, err := s.r.Config(ctx)
	if err != nil {
		return nil, err
	}

	resp := &pb.ConfigResponse{}
	for _, c := range cfg.Components {
		cc := &pb.ComponentConfig{
			Name: c.Name,
			Type: string(c.Type),
		}
		if c.Frame != nil {
			orientation := c.Frame.Orientation
			if orientation == nil {
				orientation = spatialmath.NewZeroOrientation()
			}
			cc.Parent = c.Frame.Parent
			cc.Pose = &pb.Pose{
				X:     c.Frame.Translation.X,
				Y:     c.Frame.Translation.Y,
				Z:     c.Frame.Translation.Z,
				OX:    orientation.OrientationVectorDegrees().OX,
				OY:    orientation.OrientationVectorDegrees().OY,
				OZ:    orientation.OrientationVectorDegrees().OZ,
				Theta: orientation.OrientationVectorDegrees().Theta,
			}
		}
		resp.Components = append(resp.Components, cc)
	}

	return resp, nil
}

const defaultStreamInterval = 1 * time.Second

// StatusStream periodically sends the robot's status.
func (s *Server) StatusStream(
	req *pb.StatusStreamRequest,
	server pb.RobotService_StatusStreamServer,
) error {
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
func (s *Server) DoAction(
	ctx context.Context,
	req *pb.DoActionRequest,
) (*pb.DoActionResponse, error) {
	act := action.LookupAction(req.Name)
	if act == nil {
		return nil, errors.Errorf("unknown action name [%s]", req.Name)
	}
	s.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer s.activeBackgroundWorkers.Done()
		act(s.cancelCtx, s.r)
	})
	return &pb.DoActionResponse{}, nil
}

// BaseMoveStraight moves a base of the underlying robot straight.
func (s *Server) BaseMoveStraight(
	ctx context.Context,
	req *pb.BaseMoveStraightRequest,
) (*pb.BaseMoveStraightResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	millisPerSec := 500.0 // TODO(erh): this is probably the wrong default
	if req.MillisPerSec != 0 {
		millisPerSec = req.MillisPerSec
	}
	err := base.MoveStraight(ctx, int(req.DistanceMillis), millisPerSec, false)
	if err != nil {
		return nil, err
	}
	return &pb.BaseMoveStraightResponse{Success: true}, nil
}

// BaseMoveArc moves a base of the underlying robotin an arc.
func (s *Server) BaseMoveArc(
	ctx context.Context,
	req *pb.BaseMoveArcRequest,
) (*pb.BaseMoveArcResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	millisPerSec := 500.0 // TODO(erh): this is probably the wrong default
	if req.MillisPerSec != 0 {
		millisPerSec = req.MillisPerSec
	}
	err := base.MoveArc(ctx, int(req.DistanceMillis), millisPerSec, req.AngleDeg, false)
	if err != nil {
		return nil, err
	}
	return &pb.BaseMoveArcResponse{Success: true}, nil
}

// BaseSpin spins a base of the underlying robot.
func (s *Server) BaseSpin(
	ctx context.Context,
	req *pb.BaseSpinRequest,
) (*pb.BaseSpinResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	degsPerSec := 64.0
	if req.DegsPerSec != 0 {
		degsPerSec = req.DegsPerSec
	}
	err := base.Spin(ctx, req.AngleDeg, degsPerSec, false)
	if err != nil {
		return nil, err
	}
	return &pb.BaseSpinResponse{Success: true}, nil
}

// BaseStop stops a base of the underlying robot.
func (s *Server) BaseStop(
	ctx context.Context,
	req *pb.BaseStopRequest,
) (*pb.BaseStopResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	return &pb.BaseStopResponse{}, base.Stop(ctx)
}

// BaseWidthMillis returns the width of a base of the underlying robot.
func (s *Server) BaseWidthMillis(
	ctx context.Context,
	req *pb.BaseWidthMillisRequest,
) (*pb.BaseWidthMillisResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	width, err := base.WidthGet(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BaseWidthMillisResponse{WidthMillis: int64(width)}, nil
}

// SensorReadings returns the readings of a sensor of the underlying robot.
func (s *Server) SensorReadings(
	ctx context.Context,
	req *pb.SensorReadingsRequest,
) (*pb.SensorReadingsResponse, error) {
	sensorDevice, ok := s.r.SensorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no sensor with name (%s)", req.Name)
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

func (s *Server) compassByName(name string) (compass.Compass, error) {
	sensorDevice, ok := s.r.SensorByName(name)
	if !ok {
		return nil, errors.Errorf("no sensor with name (%s)", name)
	}
	return sensorDevice.(compass.Compass), nil
}

// CompassHeading returns the heading of a compass of the underlying robot.
func (s *Server) CompassHeading(
	ctx context.Context,
	req *pb.CompassHeadingRequest,
) (*pb.CompassHeadingResponse, error) {
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
func (s *Server) CompassStartCalibration(
	ctx context.Context,
	req *pb.CompassStartCalibrationRequest,
) (*pb.CompassStartCalibrationResponse, error) {
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
func (s *Server) CompassStopCalibration(
	ctx context.Context,
	req *pb.CompassStopCalibrationRequest,
) (*pb.CompassStopCalibrationResponse, error) {
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
func (s *Server) CompassMark(
	ctx context.Context,
	req *pb.CompassMarkRequest,
) (*pb.CompassMarkResponse, error) {
	compassDevice, err := s.compassByName(req.Name)
	if err != nil {
		return nil, err
	}
	rel, ok := compassDevice.(compass.RelativeCompass)
	if !ok {
		return nil, errors.New("compass is not relative")
	}
	if err := rel.Mark(ctx); err != nil {
		return nil, err
	}
	return &pb.CompassMarkResponse{}, nil
}

// ExecuteFunction executes the given function with access to the underlying robot.
func (s *Server) ExecuteFunction(
	ctx context.Context,
	req *pb.ExecuteFunctionRequest,
) (*pb.ExecuteFunctionResponse, error) {
	conf, err := s.r.Config(ctx)
	if err != nil {
		return nil, err
	}
	var funcConfig functionvm.FunctionConfig
	var found bool
	for _, conf := range conf.Functions {
		if conf.Name == req.Name {
			found = true
			funcConfig = conf
		}
	}
	if !found {
		return nil, errors.Errorf("no function with name (%s)", req.Name)
	}
	result, err := executeFunctionWithRobotForRPC(ctx, funcConfig, s.r)
	if err != nil {
		return nil, err
	}

	return &pb.ExecuteFunctionResponse{
		Results: result.Results,
		StdOut:  result.StdOut,
		StdErr:  result.StdErr,
	}, nil
}

// ExecuteSource executes the given source with access to the underlying robot.
func (s *Server) ExecuteSource(
	ctx context.Context,
	req *pb.ExecuteSourceRequest,
) (*pb.ExecuteSourceResponse, error) {
	result, err := executeFunctionWithRobotForRPC(
		ctx,
		functionvm.FunctionConfig{
			Name: "_",
			AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
				Engine: functionvm.EngineName(req.Engine),
				Source: req.Source,
			},
		},
		s.r,
	)
	if err != nil {
		return nil, err
	}

	return &pb.ExecuteSourceResponse{
		Results: result.Results,
		StdOut:  result.StdOut,
		StdErr:  result.StdErr,
	}, nil
}

type runCommander interface {
	RunCommand(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error)
}

// ResourceRunCommand runs an arbitrary command on a resource if it supports it.
func (s *Server) ResourceRunCommand(
	ctx context.Context,
	req *pb.ResourceRunCommandRequest,
) (*pb.ResourceRunCommandResponse, error) {
	// TODO(https://github.com/viamrobotics/rdk/issues/409): support all resources
	// we know only gps has this right now, so just look at sensors!
	resource, ok := s.r.ResourceByName(gps.Named(req.ResourceName))
	if !ok {
		return nil, errors.Errorf("no resource with name (%s)", req.ResourceName)
	}
	commander, ok := rdkutils.UnwrapProxy(resource).(runCommander)
	if !ok {
		return nil, errors.New("cannot run commands on this resource")
	}
	result, err := commander.RunCommand(ctx, req.CommandName, req.Args.AsMap())
	if err != nil {
		return nil, err
	}
	resultPb, err := structpb.NewStruct(result)
	if err != nil {
		return nil, err
	}

	return &pb.ResourceRunCommandResponse{Result: resultPb}, nil
}

// FrameServiceConfig returns all the information needed to recreate the frame system for a robot.
// That is: the directed acyclic graph of the frame system parent structure, the static offset poses between frames,
// and the kinematic/model frames for any robot parts that move or have intrinsic frame properties.
func (s *Server) FrameServiceConfig(
	ctx context.Context,
	req *pb.FrameServiceConfigRequest,
) (*pb.FrameServiceConfigResponse, error) {
	svc, ok := s.r.ResourceByName(framesystem.Name)
	if !ok {
		return nil, errors.New("no framesystem service")
	}
	fsSvc, ok := svc.(framesystem.Service)
	if !ok {
		return nil, errors.New("service is not a framesystem.Service")
	}
	sortedParts, err := fsSvc.FrameSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	configs := make([]*pb.FrameSystemConfig, len(sortedParts))
	for i, part := range sortedParts {
		c, err := part.ToProtobuf()
		if err != nil {
			if errors.Is(err, referenceframe.ErrNoModelInformation) {
				configs[i] = nil
				continue
			}
			return nil, err
		}
		configs[i] = c
	}
	return &pb.FrameServiceConfigResponse{FrameSystemConfigs: configs}, nil
}

// NavigationServiceMode returns the mode of the service.
func (s *Server) NavigationServiceMode(
	ctx context.Context,
	req *pb.NavigationServiceModeRequest,
) (*pb.NavigationServiceModeResponse, error) {
	svc, ok := s.r.ResourceByName(navigation.Name)
	if !ok {
		return nil, errors.New("no navigation service")
	}
	navSvc, ok := svc.(navigation.Service)
	if !ok {
		return nil, errors.New("service is not a navigation service")
	}
	m, err := navSvc.Mode(ctx)
	if err != nil {
		return nil, err
	}
	pbM := pb.NavigationServiceMode_NAVIGATION_SERVICE_MODE_UNSPECIFIED
	switch m {
	case navigation.ModeManual:
		pbM = pb.NavigationServiceMode_NAVIGATION_SERVICE_MODE_MANUAL
	case navigation.ModeWaypoint:
		pbM = pb.NavigationServiceMode_NAVIGATION_SERVICE_MODE_WAYPOINT
	}
	return &pb.NavigationServiceModeResponse{
		Mode: pbM,
	}, nil
}

// NavigationServiceSetMode sets the mode of the service.
func (s *Server) NavigationServiceSetMode(
	ctx context.Context,
	req *pb.NavigationServiceSetModeRequest,
) (*pb.NavigationServiceSetModeResponse, error) {
	svc, ok := s.r.ResourceByName(navigation.Name)
	if !ok {
		return nil, errors.New("no navigation service")
	}
	navSvc, ok := svc.(navigation.Service)
	if !ok {
		return nil, errors.New("service is not a navigation service")
	}
	switch req.Mode {
	case pb.NavigationServiceMode_NAVIGATION_SERVICE_MODE_MANUAL:
		if err := navSvc.SetMode(ctx, navigation.ModeManual); err != nil {
			return nil, err
		}
	case pb.NavigationServiceMode_NAVIGATION_SERVICE_MODE_WAYPOINT:
		if err := navSvc.SetMode(ctx, navigation.ModeWaypoint); err != nil {
			return nil, err
		}
	case pb.NavigationServiceMode_NAVIGATION_SERVICE_MODE_UNSPECIFIED:
		fallthrough
	default:
		return nil, errors.Errorf("unknown mode %q", req.Mode.String())
	}
	return &pb.NavigationServiceSetModeResponse{}, nil
}

// NavigationServiceLocation returns the location of the robot.
func (s *Server) NavigationServiceLocation(
	ctx context.Context,
	req *pb.NavigationServiceLocationRequest,
) (*pb.NavigationServiceLocationResponse, error) {
	svc, ok := s.r.ResourceByName(navigation.Name)
	if !ok {
		return nil, errors.New("no navigation service")
	}
	navSvc, ok := svc.(navigation.Service)
	if !ok {
		return nil, errors.New("service is not a navigation service")
	}
	loc, err := navSvc.Location(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.NavigationServiceLocationResponse{
		Location: &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
	}, nil
}

// NavigationServiceWaypoints returns the navigation waypoints of the robot.
func (s *Server) NavigationServiceWaypoints(
	ctx context.Context,
	req *pb.NavigationServiceWaypointsRequest,
) (*pb.NavigationServiceWaypointsResponse, error) {
	svc, ok := s.r.ResourceByName(navigation.Name)
	if !ok {
		return nil, errors.New("no navigation service")
	}
	navSvc, ok := svc.(navigation.Service)
	if !ok {
		return nil, errors.New("service is not a navigation service")
	}
	wps, err := navSvc.Waypoints(ctx)
	if err != nil {
		return nil, err
	}
	pbWps := make([]*pb.NavigationServiceWaypoint, 0, len(wps))
	for _, wp := range wps {
		pbWps = append(pbWps, &pb.NavigationServiceWaypoint{
			Id:       wp.ID.Hex(),
			Location: &commonpb.GeoPoint{Latitude: wp.Lat, Longitude: wp.Long},
		})
	}
	return &pb.NavigationServiceWaypointsResponse{
		Waypoints: pbWps,
	}, nil
}

// NavigationServiceAddWaypoint adds a new navigation waypoint.
func (s *Server) NavigationServiceAddWaypoint(
	ctx context.Context,
	req *pb.NavigationServiceAddWaypointRequest,
) (*pb.NavigationServiceAddWaypointResponse, error) {
	svc, ok := s.r.ResourceByName(navigation.Name)
	if !ok {
		return nil, errors.New("no navigation service")
	}
	navSvc, ok := svc.(navigation.Service)
	if !ok {
		return nil, errors.New("service is not a navigation service")
	}
	err := navSvc.AddWaypoint(ctx, geo.NewPoint(req.Location.Latitude, req.Location.Longitude))
	return &pb.NavigationServiceAddWaypointResponse{}, err
}

// NavigationServiceRemoveWaypoint removes a navigation waypoint.
func (s *Server) NavigationServiceRemoveWaypoint(
	ctx context.Context,
	req *pb.NavigationServiceRemoveWaypointRequest,
) (*pb.NavigationServiceRemoveWaypointResponse, error) {
	svc, ok := s.r.ResourceByName(navigation.Name)
	if !ok {
		return nil, errors.New("no navigation service")
	}
	navSvc, ok := svc.(navigation.Service)
	if !ok {
		return nil, errors.New("service is not a navigation service")
	}
	id, err := primitive.ObjectIDFromHex(req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.NavigationServiceRemoveWaypointResponse{}, navSvc.RemoveWaypoint(ctx, id)
}

// ObjectManipulationServiceDoGrab commands a gripper to move and grab
// an object at the passed camera point.
func (s *Server) ObjectManipulationServiceDoGrab(
	ctx context.Context,
	req *pb.ObjectManipulationServiceDoGrabRequest,
) (*pb.ObjectManipulationServiceDoGrabResponse, error) {
	svc, ok := s.r.ResourceByName(objectmanipulation.Name)
	if !ok {
		return nil, errors.New("no objectmanipulation service")
	}
	omSvc, ok := svc.(objectmanipulation.Service)
	if !ok {
		return nil, errors.New("service is not a objectmanipulation service")
	}
	cameraPointProto := req.GetCameraPoint()
	cameraPoint := &r3.Vector{
		X: cameraPointProto.X,
		Y: cameraPointProto.Y,
		Z: cameraPointProto.Z,
	}
	hasGrabbed, err := omSvc.DoGrab(ctx, req.GetGripperName(), req.GetArmName(), req.GetCameraName(), cameraPoint)
	if err != nil {
		return nil, err
	}
	return &pb.ObjectManipulationServiceDoGrabResponse{HasGrabbed: hasGrabbed}, nil
}

type executionResultRPC struct {
	Results []*structpb.Value
	StdOut  string
	StdErr  string
}

func executeFunctionWithRobotForRPC(ctx context.Context, f functionvm.FunctionConfig, r robot.Robot) (*executionResultRPC, error) {
	execResult, err := functionrobot.Execute(ctx, f, r)
	if err != nil {
		return nil, err
	}
	pbResults := make([]*structpb.Value, 0, len(execResult.Results))
	for _, result := range execResult.Results {
		val := result.Interface()
		if (val == functionvm.Undefined{}) {
			val = "<undefined>" // TODO(erd): holdover for now to make my life easier :)
		}
		pbVal, err := structpb.NewValue(val)
		if err != nil {
			return nil, err
		}
		pbResults = append(pbResults, pbVal)
	}

	return &executionResultRPC{
		Results: pbResults,
		StdOut:  execResult.StdOut,
		StdErr:  execResult.StdErr,
	}, nil
}

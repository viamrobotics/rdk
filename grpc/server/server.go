// Package server contains a gRPC based robot.Robot server implementation.
//
// It should be used by an rpc.Server.
package server

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"sync"
	"time"

	"github.com/go-errors/errors"
	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/board"
	functionrobot "go.viam.com/core/function/robot"
	functionvm "go.viam.com/core/function/vm"
	"go.viam.com/core/grpc"
	"go.viam.com/core/input"
	"go.viam.com/core/lidar"
	"go.viam.com/core/pointcloud"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/sensor/forcematrix"
	"go.viam.com/core/sensor/gps"
	"go.viam.com/core/sensor/imu"
	"go.viam.com/core/services/navigation"
	"go.viam.com/core/spatialmath"
	coreutils "go.viam.com/core/utils"
	"go.viam.com/core/vision/segmentation"

	// Engines
	_ "go.viam.com/core/function/vm/engines/javascript"
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
			cc.Pose = &pb.ArmPosition{
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
		return nil, errors.Errorf("unknown action name [%s]", req.Name)
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
	arm, ok := s.r.ArmByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no arm with name (%s)", req.Name)
	}
	pos, err := arm.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.ArmCurrentPositionResponse{Position: pos}, nil
}

// ArmCurrentJointPositions gets the current joint position of an arm of the underlying robot.
func (s *Server) ArmCurrentJointPositions(ctx context.Context, req *pb.ArmCurrentJointPositionsRequest) (*pb.ArmCurrentJointPositionsResponse, error) {
	arm, ok := s.r.ArmByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no arm with name (%s)", req.Name)
	}
	pos, err := arm.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.ArmCurrentJointPositionsResponse{Positions: pos}, nil
}

// ArmMoveToPosition moves an arm of the underlying robot to the requested position.
func (s *Server) ArmMoveToPosition(ctx context.Context, req *pb.ArmMoveToPositionRequest) (*pb.ArmMoveToPositionResponse, error) {
	arm, ok := s.r.ArmByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.ArmMoveToPositionResponse{}, arm.MoveToPosition(ctx, req.To)
}

// ArmMoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *Server) ArmMoveToJointPositions(ctx context.Context, req *pb.ArmMoveToJointPositionsRequest) (*pb.ArmMoveToJointPositionsResponse, error) {
	arm, ok := s.r.ArmByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.ArmMoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, req.To)
}

// ArmJointMoveDelta moves a specific joint of an arm of the underlying robot by the given amount.
func (s *Server) ArmJointMoveDelta(ctx context.Context, req *pb.ArmJointMoveDeltaRequest) (*pb.ArmJointMoveDeltaResponse, error) {
	arm, ok := s.r.ArmByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.ArmJointMoveDeltaResponse{}, arm.JointMoveDelta(ctx, int(req.Joint), req.AmountDegs)
}

// BaseMoveStraight moves a base of the underlying robot straight.
func (s *Server) BaseMoveStraight(ctx context.Context, req *pb.BaseMoveStraightRequest) (*pb.BaseMoveStraightResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	millisPerSec := 500.0 // TODO(erh): this is probably the wrong default
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
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
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

// BaseStop stops a base of the underlying robot.
func (s *Server) BaseStop(ctx context.Context, req *pb.BaseStopRequest) (*pb.BaseStopResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	return &pb.BaseStopResponse{}, base.Stop(ctx)
}

// BaseWidthMillis returns the width of a base of the underlying robot.
func (s *Server) BaseWidthMillis(ctx context.Context, req *pb.BaseWidthMillisRequest) (*pb.BaseWidthMillisResponse, error) {
	base, ok := s.r.BaseByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no base with name (%s)", req.Name)
	}
	width, err := base.WidthMillis(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BaseWidthMillisResponse{WidthMillis: int64(width)}, nil
}

// GripperOpen opens a gripper of the underlying robot.
func (s *Server) GripperOpen(ctx context.Context, req *pb.GripperOpenRequest) (*pb.GripperOpenResponse, error) {
	gripper, ok := s.r.GripperByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no gripper with that name %s", req.Name)
	}
	return &pb.GripperOpenResponse{}, gripper.Open(ctx)
}

// GripperGrab requests a gripper of the underlying robot to grab.
func (s *Server) GripperGrab(ctx context.Context, req *pb.GripperGrabRequest) (*pb.GripperGrabResponse, error) {
	gripper, ok := s.r.GripperByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no gripper with that name %s", req.Name)
	}
	grabbed, err := gripper.Grab(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GripperGrabResponse{Grabbed: grabbed}, nil
}

// PointCloud returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *Server) PointCloud(ctx context.Context, req *pb.PointCloudRequest) (*pb.PointCloudResponse, error) {
	camera, ok := s.r.CameraByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no camera with name (%s)", req.Name)
	}

	pc, err := camera.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = pc.ToPCD(&buf)
	if err != nil {
		return nil, err
	}

	return &pb.PointCloudResponse{
		MimeType: grpc.MimeTypePCD,
		Frame:    buf.Bytes(),
	}, nil
}

// ObjectPointClouds returns an array of objects from the frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned. Also returns a 3Vector array of the center points of each object.
func (s *Server) ObjectPointClouds(ctx context.Context, req *pb.ObjectPointCloudsRequest) (*pb.ObjectPointCloudsResponse, error) {
	camera, ok := s.r.CameraByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no camera with name (%s)", req.Name)
	}

	pc, err := camera.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	config := segmentation.ObjectConfig{int(req.MinPointsInPlane), int(req.MinPointsInSegment), req.ClusteringRadius}
	segments, err := segmentation.NewObjectSegmentation(ctx, pc, config)
	if err != nil {
		return nil, err
	}

	frames := make([][]byte, segments.N())
	centers := make([]pointcloud.Vec3, segments.N())
	boundingBoxes := make([]pointcloud.BoxGeometry, segments.N())
	for i, seg := range segments.Objects {
		var buf bytes.Buffer
		err := seg.ToPCD(&buf)
		if err != nil {
			return nil, err
		}
		frames[i] = buf.Bytes()
		centers[i] = seg.Center
		boundingBoxes[i] = seg.BoundingBox
	}

	return &pb.ObjectPointCloudsResponse{
		MimeType:      grpc.MimeTypePCD,
		Frames:        frames,
		Centers:       pointsToProto(centers),
		BoundingBoxes: boxesToProto(boundingBoxes),
	}, nil
}

// CameraFrame returns a frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned.
func (s *Server) CameraFrame(ctx context.Context, req *pb.CameraFrameRequest) (*pb.CameraFrameResponse, error) {
	camera, ok := s.r.CameraByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no camera with name (%s)", req.Name)
	}

	img, release, err := camera.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if release != nil {
			release()
		}
	}()

	// choose the best/fastest representation
	if req.MimeType == grpc.MimeTypeViamBest {
		iwd, ok := img.(*rimage.ImageWithDepth)
		if ok && iwd.Depth != nil && iwd.Color != nil {
			req.MimeType = grpc.MimeTypeRawIWD
		} else {
			req.MimeType = grpc.MimeTypeRawRGBA
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
	case grpc.MimeTypeRawRGBA:
		resp.MimeType = grpc.MimeTypeRawRGBA
		imgCopy := image.NewRGBA(bounds)
		draw.Draw(imgCopy, bounds, img, bounds.Min, draw.Src)
		buf.Write(imgCopy.Pix)
	case grpc.MimeTypeRawIWD:
		resp.MimeType = grpc.MimeTypeRawIWD
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return nil, errors.Errorf("want %s but don't have %T", grpc.MimeTypeRawIWD, iwd)
		}
		err := iwd.RawBytesWrite(&buf)
		if err != nil {
			return nil, errors.Errorf("error writing %s: %w", grpc.MimeTypeRawIWD, err)
		}

	case grpc.MimeTypeBoth:
		resp.MimeType = grpc.MimeTypeBoth
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return nil, errors.Errorf("want %s but don't have %T", grpc.MimeTypeBoth, iwd)
		}
		if iwd.Color == nil || iwd.Depth == nil {
			return nil, errors.Errorf("for %s need depth and color info", grpc.MimeTypeBoth)
		}
		if err := rimage.EncodeBoth(iwd, &buf); err != nil {
			return nil, err
		}
	case grpc.MimeTypeJPEG:
		resp.MimeType = grpc.MimeTypeJPEG
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	case "", grpc.MimeTypePNG:
		resp.MimeType = grpc.MimeTypePNG
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("do not know how to encode %q", req.MimeType)
	}
	resp.Frame = buf.Bytes()
	return &resp, nil
}

// CameraRenderFrame renders a frame from a camera of the underlying robot to an HTTP response. A specific MIME type
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

// LidarInfo returns the info of a lidar of the underlying robot.
func (s *Server) LidarInfo(ctx context.Context, req *pb.LidarInfoRequest) (*pb.LidarInfoResponse, error) {
	lidar, ok := s.r.LidarByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no lidar with name (%s)", req.Name)
	}
	info, err := lidar.Info(ctx)
	if err != nil {
		return nil, err
	}
	str, err := structpb.NewStruct(info)
	if err != nil {
		return nil, err
	}
	return &pb.LidarInfoResponse{Info: str}, nil
}

// LidarStart starts a lidar of the underlying robot.
func (s *Server) LidarStart(ctx context.Context, req *pb.LidarStartRequest) (*pb.LidarStartResponse, error) {
	lidar, ok := s.r.LidarByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no lidar with name (%s)", req.Name)
	}
	err := lidar.Start(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarStartResponse{}, nil
}

// LidarStop stops a lidar of the underlying robot.
func (s *Server) LidarStop(ctx context.Context, req *pb.LidarStopRequest) (*pb.LidarStopResponse, error) {
	lidar, ok := s.r.LidarByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no lidar with name (%s)", req.Name)
	}
	err := lidar.Stop(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarStopResponse{}, nil
}

// LidarScan returns a scan from a lidar of the underlying robot.
func (s *Server) LidarScan(ctx context.Context, req *pb.LidarScanRequest) (*pb.LidarScanResponse, error) {
	lidar, ok := s.r.LidarByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no lidar with name (%s)", req.Name)
	}
	opts := scanOptionsFromProto(req)
	ms, err := lidar.Scan(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &pb.LidarScanResponse{Measurements: measurementsToProto(ms)}, nil
}

// LidarRange returns the range of a lidar of the underlying robot.
func (s *Server) LidarRange(ctx context.Context, req *pb.LidarRangeRequest) (*pb.LidarRangeResponse, error) {
	lidar, ok := s.r.LidarByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no lidar with name (%s)", req.Name)
	}
	r, err := lidar.Range(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarRangeResponse{Range: int64(r)}, nil
}

// LidarBounds returns the scan bounds of a lidar of the underlying robot.
func (s *Server) LidarBounds(ctx context.Context, req *pb.LidarBoundsRequest) (*pb.LidarBoundsResponse, error) {
	lidar, ok := s.r.LidarByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no lidar with name (%s)", req.Name)
	}
	bounds, err := lidar.Bounds(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.LidarBoundsResponse{X: int64(bounds.X), Y: int64(bounds.Y)}, nil
}

// LidarAngularResolution returns the scan angular resolution of a lidar of the underlying robot.
func (s *Server) LidarAngularResolution(ctx context.Context, req *pb.LidarAngularResolutionRequest) (*pb.LidarAngularResolutionResponse, error) {
	lidar, ok := s.r.LidarByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no lidar with name (%s)", req.Name)
	}
	angRes, err := lidar.AngularResolution(ctx)
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

func pointToProto(p pointcloud.Vec3) *pb.Vector3 {
	return &pb.Vector3{
		X: p.X,
		Y: p.Y,
		Z: p.Z,
	}
}

func pointsToProto(vs []pointcloud.Vec3) []*pb.Vector3 {
	pvs := make([]*pb.Vector3, 0, len(vs))
	for _, v := range vs {
		pvs = append(pvs, pointToProto(v))
	}
	return pvs
}

func boxToProto(b pointcloud.BoxGeometry) *pb.BoxGeometry {
	return &pb.BoxGeometry{
		Width:  b.Width,
		Length: b.Length,
		Depth:  b.Depth,
	}
}

func boxesToProto(bs []pointcloud.BoxGeometry) []*pb.BoxGeometry {
	pbs := make([]*pb.BoxGeometry, 0, len(bs))
	for _, v := range bs {
		pbs = append(pbs, boxToProto(v))
	}
	return pbs
}

// BoardStatus returns the status of a board of the underlying robot.
func (s *Server) BoardStatus(ctx context.Context, req *pb.BoardStatusRequest) (*pb.BoardStatusResponse, error) {
	b, ok := s.r.BoardByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.Name)
	}

	status, err := b.Status(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.BoardStatusResponse{Status: status}, nil
}

// BoardGPIOSet sets a given pin of a board of the underlying robot to either low or high.
func (s *Server) BoardGPIOSet(ctx context.Context, req *pb.BoardGPIOSetRequest) (*pb.BoardGPIOSetResponse, error) {
	b, ok := s.r.BoardByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.Name)
	}

	return &pb.BoardGPIOSetResponse{}, b.GPIOSet(ctx, req.Pin, req.High)
}

// BoardGPIOGet gets the high/low state of a given pin of a board of the underlying robot.
func (s *Server) BoardGPIOGet(ctx context.Context, req *pb.BoardGPIOGetRequest) (*pb.BoardGPIOGetResponse, error) {
	b, ok := s.r.BoardByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.Name)
	}

	high, err := b.GPIOGet(ctx, req.Pin)
	if err != nil {
		return nil, err
	}
	return &pb.BoardGPIOGetResponse{High: high}, nil
}

// BoardPWMSet sets a given pin of the underlying robot to the given duty cycle.
func (s *Server) BoardPWMSet(ctx context.Context, req *pb.BoardPWMSetRequest) (*pb.BoardPWMSetResponse, error) {
	b, ok := s.r.BoardByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.Name)
	}

	return &pb.BoardPWMSetResponse{}, b.PWMSet(ctx, req.Pin, byte(req.DutyCycle))
}

// BoardPWMSetFrequency sets a given pin of a board of the underlying robot to the given PWM frequency. 0 will use the board's default PWM frequency.
func (s *Server) BoardPWMSetFrequency(ctx context.Context, req *pb.BoardPWMSetFrequencyRequest) (*pb.BoardPWMSetFrequencyResponse, error) {
	b, ok := s.r.BoardByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.Name)
	}

	return &pb.BoardPWMSetFrequencyResponse{}, b.PWMSetFreq(ctx, req.Pin, uint(req.Frequency))
}

// BoardAnalogReaderRead reads off the current value of an analog reader of a board of the underlying robot.
func (s *Server) BoardAnalogReaderRead(ctx context.Context, req *pb.BoardAnalogReaderReadRequest) (*pb.BoardAnalogReaderReadResponse, error) {
	b, ok := s.r.BoardByName(req.BoardName)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.BoardName)
	}

	theReader, ok := b.AnalogReaderByName(req.AnalogReaderName)
	if !ok {
		return nil, errors.Errorf("unknown analog reader: %s", req.AnalogReaderName)
	}

	val, err := theReader.Read(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardAnalogReaderReadResponse{Value: int32(val)}, nil
}

// BoardDigitalInterruptConfig returns the config the interrupt was created with.
func (s *Server) BoardDigitalInterruptConfig(ctx context.Context, req *pb.BoardDigitalInterruptConfigRequest) (*pb.BoardDigitalInterruptConfigResponse, error) {
	b, ok := s.r.BoardByName(req.BoardName)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.BoardName)
	}

	interrupt, ok := b.DigitalInterruptByName(req.DigitalInterruptName)
	if !ok {
		return nil, errors.Errorf("unknown digital interrupt: %s", req.DigitalInterruptName)
	}

	config, err := interrupt.Config(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardDigitalInterruptConfigResponse{Config: digitalInterruptConfigToProto(&config)}, nil
}

func digitalInterruptConfigToProto(config *board.DigitalInterruptConfig) *pb.DigitalInterruptConfig {
	return &pb.DigitalInterruptConfig{
		Name:    config.Name,
		Pin:     config.Pin,
		Type:    config.Type,
		Formula: config.Formula,
	}
}

// BoardDigitalInterruptValue returns the current value of the interrupt which is based on the type of interrupt.
func (s *Server) BoardDigitalInterruptValue(ctx context.Context, req *pb.BoardDigitalInterruptValueRequest) (*pb.BoardDigitalInterruptValueResponse, error) {
	b, ok := s.r.BoardByName(req.BoardName)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.BoardName)
	}

	interrupt, ok := b.DigitalInterruptByName(req.DigitalInterruptName)
	if !ok {
		return nil, errors.Errorf("unknown digital interrupt: %s", req.DigitalInterruptName)
	}

	val, err := interrupt.Value(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardDigitalInterruptValueResponse{Value: val}, nil
}

// BoardDigitalInterruptTick is to be called either manually if the interrupt is a proxy to some real hardware interrupt or for tests.
func (s *Server) BoardDigitalInterruptTick(ctx context.Context, req *pb.BoardDigitalInterruptTickRequest) (*pb.BoardDigitalInterruptTickResponse, error) {
	b, ok := s.r.BoardByName(req.BoardName)
	if !ok {
		return nil, errors.Errorf("no board with name (%s)", req.BoardName)
	}

	interrupt, ok := b.DigitalInterruptByName(req.DigitalInterruptName)
	if !ok {
		return nil, errors.Errorf("unknown digital interrupt: %s", req.DigitalInterruptName)
	}

	return &pb.BoardDigitalInterruptTickResponse{}, interrupt.Tick(ctx, req.High, req.Nanos)
}

// SensorReadings returns the readings of a sensor of the underlying robot.
func (s *Server) SensorReadings(ctx context.Context, req *pb.SensorReadingsRequest) (*pb.SensorReadingsResponse, error) {
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
func (s *Server) ExecuteFunction(ctx context.Context, req *pb.ExecuteFunctionRequest) (*pb.ExecuteFunctionResponse, error) {
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
func (s *Server) ExecuteSource(ctx context.Context, req *pb.ExecuteSourceRequest) (*pb.ExecuteSourceResponse, error) {
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

// ServoCurrent returns the current set angle (degrees) of the servo a board of the underlying robot.
func (s *Server) ServoCurrent(ctx context.Context, req *pb.ServoCurrentRequest) (*pb.ServoCurrentResponse, error) {
	theServo, ok := s.r.ServoByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no servo with name (%s)", req.Name)
	}

	angleDeg, err := theServo.Current(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ServoCurrentResponse{AngleDeg: uint32(angleDeg)}, nil
}

// ServoMove requests the servo of a board of the underlying robot to move.
func (s *Server) ServoMove(ctx context.Context, req *pb.ServoMoveRequest) (*pb.ServoMoveResponse, error) {
	theServo, ok := s.r.ServoByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no servo with name (%s)", req.Name)
	}

	return &pb.ServoMoveResponse{}, theServo.Move(ctx, uint8(req.AngleDeg))
}

// MotorPower sets the percentage of power the motor of the underlying robot should employ between 0-1.
func (s *Server) MotorPower(ctx context.Context, req *pb.MotorPowerRequest) (*pb.MotorPowerResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	return &pb.MotorPowerResponse{}, theMotor.Power(ctx, req.PowerPct)
}

// MotorGo requests the motor of the underlying robot to go.
func (s *Server) MotorGo(ctx context.Context, req *pb.MotorGoRequest) (*pb.MotorGoResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	return &pb.MotorGoResponse{}, theMotor.Go(ctx, req.Direction, req.PowerPct)
}

// MotorGoFor requests the motor of the underlying robot to go for a certain amount based off
// the request.
func (s *Server) MotorGoFor(ctx context.Context, req *pb.MotorGoForRequest) (*pb.MotorGoForResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	// erh: this isn't right semantically.
	// GoFor with 0 rotations means something important.
	rVal := 0.0
	if req.Revolutions != 0 {
		rVal = req.Revolutions
	}

	return &pb.MotorGoForResponse{}, theMotor.GoFor(ctx, req.Direction, req.Rpm, rVal)
}

// MotorPosition reports the position of the motor of the underlying robot based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of MotorGoFor.
func (s *Server) MotorPosition(ctx context.Context, req *pb.MotorPositionRequest) (*pb.MotorPositionResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	pos, err := theMotor.Position(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.MotorPositionResponse{Position: pos}, nil
}

// MotorPositionSupported returns whether or not the motor of the underlying robot supports reporting of its position which
// is reliant on having an encoder.
func (s *Server) MotorPositionSupported(ctx context.Context, req *pb.MotorPositionSupportedRequest) (*pb.MotorPositionSupportedResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	supported, err := theMotor.PositionSupported(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.MotorPositionSupportedResponse{Supported: supported}, nil
}

// MotorOff turns the motor of the underlying robot off.
func (s *Server) MotorOff(ctx context.Context, req *pb.MotorOffRequest) (*pb.MotorOffResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	return &pb.MotorOffResponse{}, theMotor.Off(ctx)
}

// MotorIsOn returns whether or not the motor of the underlying robot is currently on.
func (s *Server) MotorIsOn(ctx context.Context, req *pb.MotorIsOnRequest) (*pb.MotorIsOnResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	isOn, err := theMotor.IsOn(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.MotorIsOnResponse{IsOn: isOn}, nil
}

// MotorGoTo requests the motor of the underlying robot to go a specific position.
func (s *Server) MotorGoTo(ctx context.Context, req *pb.MotorGoToRequest) (*pb.MotorGoToResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	return &pb.MotorGoToResponse{}, theMotor.GoTo(ctx, req.Rpm, req.Position)
}

// MotorGoTillStop requests the motor of the underlying robot to go until stopped either physically or by a limit switch.
func (s *Server) MotorGoTillStop(ctx context.Context, req *pb.MotorGoTillStopRequest) (*pb.MotorGoTillStopResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	return &pb.MotorGoTillStopResponse{}, theMotor.GoTillStop(ctx, req.Direction, req.Rpm, nil)
}

// MotorZero requests the motor of the underlying robot to reset it's zero/home position.
func (s *Server) MotorZero(ctx context.Context, req *pb.MotorZeroRequest) (*pb.MotorZeroResponse, error) {
	theMotor, ok := s.r.MotorByName(req.Name)
	if !ok {
		return nil, errors.Errorf("no motor with name (%s)", req.Name)
	}

	return &pb.MotorZeroResponse{}, theMotor.Zero(ctx, req.Offset)
}

type runCommander interface {
	RunCommand(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error)
}

// ResourceRunCommand runs an arbitrary command on a resource if it supports it.
func (s *Server) ResourceRunCommand(ctx context.Context, req *pb.ResourceRunCommandRequest) (*pb.ResourceRunCommandResponse, error) {
	// TODO(erd): support all resources
	// we know only gps has this right now, so just look at sensors!
	resource, ok := s.r.SensorByName(req.ResourceName)
	if !ok {
		return nil, errors.Errorf("no resource with name (%s)", req.ResourceName)
	}
	commander, ok := coreutils.UnwrapProxy(resource).(runCommander)
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

// NavigationServiceMode returns the mode of the service.
func (s *Server) NavigationServiceMode(ctx context.Context, req *pb.NavigationServiceModeRequest) (*pb.NavigationServiceModeResponse, error) {
	svc, ok := s.r.ServiceByName("navigation")
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
func (s *Server) NavigationServiceSetMode(ctx context.Context, req *pb.NavigationServiceSetModeRequest) (*pb.NavigationServiceSetModeResponse, error) {
	svc, ok := s.r.ServiceByName("navigation")
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
	default:
		return nil, errors.Errorf("unknown mode %q", req.Mode.String())
	}
	return &pb.NavigationServiceSetModeResponse{}, nil
}

// NavigationServiceLocation returns the location of the robot.
func (s *Server) NavigationServiceLocation(ctx context.Context, req *pb.NavigationServiceLocationRequest) (*pb.NavigationServiceLocationResponse, error) {
	svc, ok := s.r.ServiceByName("navigation")
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
		Location: &pb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
	}, nil
}

// NavigationServiceWaypoints returns the navigation waypoints of the robot.
func (s *Server) NavigationServiceWaypoints(ctx context.Context, req *pb.NavigationServiceWaypointsRequest) (*pb.NavigationServiceWaypointsResponse, error) {
	svc, ok := s.r.ServiceByName("navigation")
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
			Location: &pb.GeoPoint{Latitude: wp.Lat, Longitude: wp.Long},
		})
	}
	return &pb.NavigationServiceWaypointsResponse{
		Waypoints: pbWps,
	}, nil
}

// NavigationServiceAddWaypoint adds a new navigation waypoint.
func (s *Server) NavigationServiceAddWaypoint(ctx context.Context, req *pb.NavigationServiceAddWaypointRequest) (*pb.NavigationServiceAddWaypointResponse, error) {
	svc, ok := s.r.ServiceByName("navigation")
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
func (s *Server) NavigationServiceRemoveWaypoint(ctx context.Context, req *pb.NavigationServiceRemoveWaypointRequest) (*pb.NavigationServiceRemoveWaypointResponse, error) {
	svc, ok := s.r.ServiceByName("navigation")
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

func (s *Server) imuByName(name string) (imu.IMU, error) {
	sensorDevice, ok := s.r.SensorByName(name)
	if !ok {
		return nil, errors.Errorf("no sensor with name (%s)", name)
	}
	return sensorDevice.(imu.IMU), nil
}

// IMUAngularVelocity returns the most recent angular velocity reading from the given IMU.
func (s *Server) IMUAngularVelocity(ctx context.Context, req *pb.IMUAngularVelocityRequest) (*pb.IMUAngularVelocityResponse, error) {
	imuDevice, err := s.imuByName(req.Name)
	if err != nil {
		return nil, err
	}
	vel, err := imuDevice.AngularVelocity(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IMUAngularVelocityResponse{
		AngularVelocity: &pb.AngularVelocity{
			X: vel.X,
			Y: vel.Y,
			Z: vel.Z,
		},
	}, nil
}

// IMUOrientation returns the most recent angular velocity reading from the given IMU.
func (s *Server) IMUOrientation(ctx context.Context, req *pb.IMUOrientationRequest) (*pb.IMUOrientationResponse, error) {
	imuDevice, err := s.imuByName(req.Name)
	if err != nil {
		return nil, err
	}
	orientation, err := imuDevice.Orientation(ctx)
	if err != nil {
		return nil, err
	}
	ea := orientation.EulerAngles()
	return &pb.IMUOrientationResponse{
		Orientation: &pb.EulerAngles{
			Roll:  ea.Roll,
			Pitch: ea.Pitch,
			Yaw:   ea.Yaw,
		},
	}, nil
}

func (s *Server) gpsByName(name string) (gps.GPS, error) {
	sensorDevice, ok := s.r.SensorByName(name)
	if !ok {
		return nil, errors.Errorf("no sensor with name (%s)", name)
	}
	return sensorDevice.(gps.GPS), nil
}

// GPSLocation returns the most recent location from the given GPS.
func (s *Server) GPSLocation(ctx context.Context, req *pb.GPSLocationRequest) (*pb.GPSLocationResponse, error) {
	gpsDevice, err := s.gpsByName(req.Name)
	if err != nil {
		return nil, err
	}
	loc, err := gpsDevice.Location(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSLocationResponse{
		Coordinate: &pb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
	}, nil
}

// GPSAltitude returns the most recent location from the given GPS.
func (s *Server) GPSAltitude(ctx context.Context, req *pb.GPSAltitudeRequest) (*pb.GPSAltitudeResponse, error) {
	gpsDevice, err := s.gpsByName(req.Name)
	if err != nil {
		return nil, err
	}
	alt, err := gpsDevice.Altitude(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSAltitudeResponse{
		Altitude: alt,
	}, nil
}

// GPSSpeed returns the most recent location from the given GPS.
func (s *Server) GPSSpeed(ctx context.Context, req *pb.GPSSpeedRequest) (*pb.GPSSpeedResponse, error) {
	gpsDevice, err := s.gpsByName(req.Name)
	if err != nil {
		return nil, err
	}
	speed, err := gpsDevice.Speed(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSSpeedResponse{
		SpeedKph: speed,
	}, nil
}

// GPSAccuracy returns the most recent location from the given GPS.
func (s *Server) GPSAccuracy(ctx context.Context, req *pb.GPSAccuracyRequest) (*pb.GPSAccuracyResponse, error) {
	gpsDevice, err := s.gpsByName(req.Name)
	if err != nil {
		return nil, err
	}
	horz, vert, err := gpsDevice.Accuracy(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSAccuracyResponse{
		HorizontalAccuracy: horz,
		VerticalAccuracy:   vert,
	}, nil
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

// InputControllerControls lists the inputs of an input.Controller
func (s *Server) InputControllerControls(ctx context.Context, req *pb.InputControllerControlsRequest) (*pb.InputControllerControlsResponse, error) {
	controller, ok := s.r.InputControllerByName(req.Controller)
	if !ok {
		return nil, errors.Errorf("no input controller with name (%s)", req.Controller)
	}

	controlList, err := controller.Controls(ctx)
	if err != nil {
		return nil, err
	}

	resp := &pb.InputControllerControlsResponse{}

	for _, control := range controlList {
		resp.Controls = append(resp.Controls, string(control))
	}

	return resp, nil
}

// InputControllerLastEvents returns the last input.Event (current state) of each control
func (s *Server) InputControllerLastEvents(ctx context.Context, req *pb.InputControllerLastEventsRequest) (*pb.InputControllerLastEventsResponse, error) {
	controller, ok := s.r.InputControllerByName(req.Controller)
	if !ok {
		return nil, errors.Errorf("no input controller with name (%s)", req.Controller)
	}

	eventsIn, err := controller.LastEvents(ctx)
	if err != nil {
		return nil, err
	}

	resp := &pb.InputControllerLastEventsResponse{}

	for _, eventIn := range eventsIn {
		resp.Events = append(resp.Events, &pb.InputControllerEvent{
			Time:    timestamppb.New(eventIn.Time),
			Event:   string(eventIn.Event),
			Control: string(eventIn.Control),
			Value:   eventIn.Value,
		})
	}

	return resp, nil
}

// InputControllerEventStream returns a stream of input.Event
func (s *Server) InputControllerEventStream(req *pb.InputControllerEventStreamRequest, server pb.RobotService_InputControllerEventStreamServer) error {
	controller, ok := s.r.InputControllerByName(req.Controller)
	if !ok {
		return errors.Errorf("no input controller with name (%s)", req.Controller)
	}
	eventsChan := make(chan *pb.InputControllerEvent, 1024)

	ctrlFunc := func(ctx context.Context, eventIn input.Event) {
		resp := &pb.InputControllerEvent{
			Time:    timestamppb.New(eventIn.Time),
			Event:   string(eventIn.Event),
			Control: string(eventIn.Control),
			Value:   eventIn.Value,
		}
		select {
		case eventsChan <- resp:
		case <-ctx.Done():
		}
	}

	for _, ev := range req.Events {
		var triggers []input.EventType
		for _, v := range ev.Events {
			triggers = append(triggers, input.EventType(v))
		}
		if len(triggers) > 0 {
			err := controller.RegisterControlCallback(server.Context(), input.Control(ev.Control), triggers, ctrlFunc)
			if err != nil {
				return err
			}
		}

		var cancelledTriggers []input.EventType
		for _, v := range ev.CancelledEvents {
			cancelledTriggers = append(cancelledTriggers, input.EventType(v))
		}
		if len(cancelledTriggers) > 0 {
			err := controller.RegisterControlCallback(server.Context(), input.Control(ev.Control), cancelledTriggers, nil)
			if err != nil {
				return err
			}
		}
	}

	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		case msg := <-eventsChan:
			err := server.Send(msg)
			if err != nil {
				return err
			}
		}
	}
}

// matrixToProto is a helper function to convert force matrix values from a 2-dimensional
// slice into protobuf format.
func matrixToProto(matrix [][]int) *pb.ForceMatrixMatrixResponse {

	rows := len(matrix)
	var cols int
	if rows != 0 {
		cols = len(matrix[0])
	}

	data := make([]uint32, 0, rows*cols)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			data = append(data, uint32(matrix[row][col]))
		}
	}

	return &pb.ForceMatrixMatrixResponse{Matrix: &pb.Matrix{
		Rows: uint32(rows),
		Cols: uint32(cols),
		Data: data,
	}}
}

// ForceMatrixMatrix returns a matrix of measured forces on a matrix force sensor.
func (s *Server) ForceMatrixMatrix(ctx context.Context, req *pb.ForceMatrixMatrixRequest) (*pb.ForceMatrixMatrixResponse, error) {
	forceMatrixDevice, err := s.forceMatrixByName(req.Name)
	if err != nil {
		return nil, err
	}
	matrix, err := forceMatrixDevice.Matrix(ctx)
	if err != nil {
		return nil, err
	}
	return matrixToProto(matrix), nil
}

func (s *Server) forceMatrixByName(name string) (forcematrix.ForceMatrix, error) {
	sensorDevice, ok := s.r.SensorByName(name)
	if !ok {
		return nil, errors.Errorf("no force matrix with name (%s)", name)
	}
	return sensorDevice.(forcematrix.ForceMatrix), nil
}

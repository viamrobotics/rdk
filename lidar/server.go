package lidar

import (
	"context"

	pb "go.viam.com/robotcore/proto/lidar/v1"

	"google.golang.org/protobuf/types/known/structpb"
)

type Server struct {
	pb.UnimplementedLidarServiceServer
	device Device
}

func NewServer(device Device) pb.LidarServiceServer {
	return &Server{device: device}
}

func (s *Server) Info(ctx context.Context, _ *pb.InfoRequest) (*pb.InfoResponse, error) {
	info, err := s.device.Info(ctx)
	if err != nil {
		return nil, err
	}
	str, err := structpb.NewStruct(info)
	if err != nil {
		return nil, err
	}
	return &pb.InfoResponse{Info: str}, nil
}

func (s *Server) Start(ctx context.Context, _ *pb.StartRequest) (*pb.StartResponse, error) {
	err := s.device.Start(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.StartResponse{}, nil
}

func (s *Server) Stop(ctx context.Context, _ *pb.StopRequest) (*pb.StopResponse, error) {
	err := s.device.Stop(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, nil
}

func (s *Server) Scan(ctx context.Context, req *pb.ScanRequest) (*pb.ScanResponse, error) {
	opts := ScanOptionsFromProto(req)
	ms, err := s.device.Scan(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &pb.ScanResponse{Measurements: MeasurementsToProto(ms)}, nil
}

func (s *Server) Range(ctx context.Context, _ *pb.RangeRequest) (*pb.RangeResponse, error) {
	r, err := s.device.Range(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.RangeResponse{Range: int64(r)}, nil
}

func (s *Server) Bounds(ctx context.Context, _ *pb.BoundsRequest) (*pb.BoundsResponse, error) {
	bounds, err := s.device.Bounds(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoundsResponse{X: int64(bounds.X), Y: int64(bounds.Y)}, nil
}

func (s *Server) AngularResolution(ctx context.Context, _ *pb.AngularResolutionRequest) (*pb.AngularResolutionResponse, error) {
	angRes, err := s.device.AngularResolution(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.AngularResolutionResponse{AngularResolution: angRes}, nil
}

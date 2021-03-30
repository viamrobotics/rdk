package compass

import (
	"context"

	pb "go.viam.com/robotcore/proto/sensor/compass/v1"
)

type Server struct {
	pb.UnimplementedCompassServiceServer
	device Device
}

func NewServer(device Device) pb.CompassServiceServer {
	return &Server{device: device}
}

func (s *Server) Heading(ctx context.Context, _ *pb.HeadingRequest) (*pb.HeadingResponse, error) {
	heading, err := s.device.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.HeadingResponse{Heading: heading}, nil
}

func (s *Server) StartCalibration(ctx context.Context, _ *pb.StartCalibrationRequest) (*pb.StartCalibrationResponse, error) {
	err := s.device.StartCalibration(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.StartCalibrationResponse{}, nil
}

func (s *Server) StopCalibration(ctx context.Context, _ *pb.StopCalibrationRequest) (*pb.StopCalibrationResponse, error) {
	err := s.device.StopCalibration(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.StopCalibrationResponse{}, nil
}

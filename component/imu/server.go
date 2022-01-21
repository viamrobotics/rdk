// Package imu contains a gRPC based IMU service server.
package imu

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from imu_subtype.proto.
type subtypeServer struct {
	pb.UnimplementedIMUServiceServer
	s subtype.Service
}

// NewServer constructs an imu gRPC service server.
func NewServer(s subtype.Service) pb.IMUServiceServer {
	return &subtypeServer{s: s}
}

// getIMU returns the imu specified, nil if not.
func (s *subtypeServer) getIMU(name string) (IMU, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no IMU with name (%s)", name)
	}
	imu, ok := resource.(IMU)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not an IMU", name)
	}
	return imu, nil
}

// ReadAngularVelocity returns the most recent angular velocity reading from the given IMU.
func (s *subtypeServer) ReadAngularVelocity(
	ctx context.Context,
	req *pb.IMUServiceReadAngularVelocityRequest,
) (*pb.IMUServiceReadAngularVelocityResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
	if err != nil {
		return nil, err
	}
	vel, err := imuDevice.ReadAngularVelocity(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IMUServiceReadAngularVelocityResponse{
		AngularVelocity: &pb.AngularVelocity{
			XDegsPerSec: vel.X,
			YDegsPerSec: vel.Y,
			ZDegsPerSec: vel.Z,
		},
	}, nil
}

// Orientation returns the most recent angular velocity reading from the given IMU.
func (s *subtypeServer) Orientation(ctx context.Context, req *pb.IMUServiceOrientationRequest) (*pb.IMUServiceOrientationResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
	if err != nil {
		return nil, err
	}
	o, err := imuDevice.Orientation(ctx)
	if err != nil {
		return nil, err
	}
	ea := o.EulerAngles()
	return &pb.IMUServiceOrientationResponse{
		Orientation: &pb.EulerAngles{
			Roll:  ea.Roll,
			Pitch: ea.Pitch,
			Yaw:   ea.Yaw,
		},
	}, nil
}

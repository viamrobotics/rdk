// Package imu contains a gRPC based IMU service server.
package imu

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/subtype"
)

// subtypeServer implements the contract from imu_subtype.proto
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

// IMUAngularVelocity returns the most recent angular velocity reading from the given IMU.
func (s *subtypeServer) IMUAngularVelocity(ctx context.Context, req *pb.IMUAngularVelocityRequest) (*pb.IMUAngularVelocityResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
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
func (s *subtypeServer) IMUOrientation(ctx context.Context, req *pb.IMUOrientationRequest) (*pb.IMUOrientationResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
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

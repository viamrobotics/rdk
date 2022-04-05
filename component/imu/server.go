// Package imu contains a gRPC based IMU service server.
package imu

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the IMUService from imu.proto.
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
		return nil, errors.Errorf("no IMU with paramName (%s)", name)
	}
	imu, ok := resource.(IMU)
	if !ok {
		return nil, errors.Errorf("resource with paramName (%s) is not an IMU", name)
	}
	return imu, nil
}

// readAngularVelocity returns the most recent angular velocity reading from the given IMU.
func (s *subtypeServer) ReadAngularVelocity(
	ctx context.Context,
	req *pb.ReadAngularVelocityRequest,
) (*pb.ReadAngularVelocityResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
	if err != nil {
		return nil, err
	}
	vel, err := imuDevice.ReadAngularVelocity(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ReadAngularVelocityResponse{
		AngularVelocity: &pb.AngularVelocity{
			XDegsPerSec: vel.X,
			YDegsPerSec: vel.Y,
			ZDegsPerSec: vel.Z,
		},
	}, nil
}

// Orientation returns the most recent angular velocity reading from the given IMU.
func (s *subtypeServer) ReadOrientation(
	ctx context.Context,
	req *pb.ReadOrientationRequest,
) (*pb.ReadOrientationResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
	if err != nil {
		return nil, err
	}
	o, err := imuDevice.ReadOrientation(ctx)
	if err != nil {
		return nil, err
	}
	ea := o.EulerAngles()
	return &pb.ReadOrientationResponse{
		Orientation: &pb.EulerAngles{
			RollDeg:  utils.RadToDeg(ea.Roll),
			PitchDeg: utils.RadToDeg(ea.Pitch),
			YawDeg:   utils.RadToDeg(ea.Yaw),
		},
	}, nil
}

func (s *subtypeServer) ReadAcceleration(
	ctx context.Context,
	req *pb.ReadAccelerationRequest,
) (*pb.ReadAccelerationResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
	if err != nil {
		return nil, err
	}
	acc, err := imuDevice.ReadAcceleration(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ReadAccelerationResponse{
		Acceleration: &pb.Acceleration{
			XMmPerSecPerSec: acc.X,
			YMmPerSecPerSec: acc.Y,
			ZMmPerSecPerSec: acc.Z,
		},
	}, nil
}

func (s *subtypeServer) ReadMagnetometer(
	ctx context.Context,
	req *pb.ReadMagnetometerRequest,
) (*pb.ReadMagnetometerResponse, error) {
	imuDevice, err := s.getIMU(req.Name)
	if err != nil {
		return nil, err
	}
	mag, err := imuDevice.ReadMagnetometer(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ReadMagnetometerResponse{
		Magnetometer: &pb.Magnetometer{
			XGauss: mag.X,
			YGauss: mag.Y,
			ZGauss: mag.Z,
		},
	}, nil
}

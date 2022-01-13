// Package gps contains a gRPC based GPS service subtypeServer.
package gps

import (
	"context"

	"github.com/pkg/errors"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from gps_subtype.proto.
type subtypeServer struct {
	pb.UnimplementedGPSServiceServer
	s subtype.Service
}

// NewServer constructs an gps gRPC service subtypeServer.
func NewServer(s subtype.Service) pb.GPSServiceServer {
	return &subtypeServer{s: s}
}

// getGPS returns the gps specified, nil if not.
func (s *subtypeServer) getGPS(name string) (GPS, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no GPS with name (%s)", name)
	}
	gps, ok := resource.(GPS)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a GPS", name)
	}
	return gps, nil
}

// Location returns the most recent location from the given GPS.
func (s *subtypeServer) Location(ctx context.Context, req *pb.GPSServiceLocationRequest) (*pb.GPSServiceLocationResponse, error) {
	gpsDevice, err := s.getGPS(req.Name)
	if err != nil {
		return nil, err
	}
	loc, err := gpsDevice.Location(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSServiceLocationResponse{
		Coordinate: &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
	}, nil
}

// Altitude returns the most recent location from the given GPS.
func (s *subtypeServer) Altitude(ctx context.Context, req *pb.GPSServiceAltitudeRequest) (*pb.GPSServiceAltitudeResponse, error) {
	gpsDevice, err := s.getGPS(req.Name)
	if err != nil {
		return nil, err
	}
	alt, err := gpsDevice.Altitude(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSServiceAltitudeResponse{
		Altitude: alt,
	}, nil
}

// Speed returns the most recent location from the given GPS.
func (s *subtypeServer) Speed(ctx context.Context, req *pb.GPSServiceSpeedRequest) (*pb.GPSServiceSpeedResponse, error) {
	gpsDevice, err := s.getGPS(req.Name)
	if err != nil {
		return nil, err
	}
	speed, err := gpsDevice.Speed(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSServiceSpeedResponse{
		SpeedKph: speed,
	}, nil
}

// Accuracy returns the most recent location from the given GPS.
func (s *subtypeServer) Accuracy(ctx context.Context, req *pb.GPSServiceAccuracyRequest) (*pb.GPSServiceAccuracyResponse, error) {
	gpsDevice, err := s.getGPS(req.Name)
	if err != nil {
		return nil, err
	}
	horz, vert, err := gpsDevice.Accuracy(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GPSServiceAccuracyResponse{
		HorizontalAccuracy: horz,
		VerticalAccuracy:   vert,
	}, nil
}

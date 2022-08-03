// Package gps contains a gRPC based GPS service subtypeServer.
package movementsensor

import (
	"context"

	"github.com/pkg/errors"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/gps/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the GPSService from gps.proto.
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

// ReadLocation returns the most recent location from the given GPS.
func (s *subtypeServer) ReadLocation(
	ctx context.Context,
	req *pb.ReadLocationRequest,
) (*pb.ReadLocationResponse, error) {
	gpsDevice, err := s.getGPS(req.Name)
	if err != nil {
		return nil, err
	}
	loc, err := gpsDevice.ReadLocation(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ReadLocationResponse{
		Coordinate: &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
	}, nil
}

// ReadAltitude returns the most recent location from the given GPS.
func (s *subtypeServer) ReadAltitude(
	ctx context.Context,
	req *pb.ReadAltitudeRequest,
) (*pb.ReadAltitudeResponse, error) {
	gpsDevice, err := s.getGPS(req.Name)
	if err != nil {
		return nil, err
	}
	alt, err := gpsDevice.ReadAltitude(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ReadAltitudeResponse{
		AltitudeMeters: alt,
	}, nil
}

// ReadSpeed returns the most recent location from the given GPS.
func (s *subtypeServer) ReadSpeed(ctx context.Context, req *pb.ReadSpeedRequest) (*pb.ReadSpeedResponse, error) {
	gpsDevice, err := s.getGPS(req.Name)
	if err != nil {
		return nil, err
	}
	speed, err := gpsDevice.ReadSpeed(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ReadSpeedResponse{
		SpeedMmPerSec: speed,
	}, nil
}

// Package navigation is the service that allows you to navigate along waypoints.
package navigation

import (
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"
)

// Paths is a slice of Path.
// type Paths []*Path

// Path describes a series of geo points the robot will travel through.
type Path struct {
	destinationWaypointID string
	geoPoints             []*geo.Point
}

// NewPath constructs a Path from a slice of geo.Points and ID.
func NewPath(id string, geoPoints []*geo.Point) *Path {
	return &Path{
		destinationWaypointID: id,
		geoPoints:             geoPoints,
	}
}

// DestinationWaypointID returns the ID of the Path.
func (p *Path) DestinationWaypointID() string {
	return p.destinationWaypointID
}

// GeoPoints returns the slice of geo.Points the Path is comprised of.
func (p *Path) GeoPoints() []*geo.Point {
	return p.geoPoints
}

// PathsToProto converts a slice of Path into an equivalent Protobuf message.
func PathsToProto(paths []*Path) []*pb.Path {
	var pbPaths []*pb.Path
	for _, path := range paths {
		pbPaths = append(pbPaths, PathToProto(path))
	}
	return pbPaths
}

// PathToProto converts the Path struct into an equivalent Protobuf message.
func PathToProto(path *Path) *pb.Path {
	var pbGeoPoints []*commonpb.GeoPoint
	for _, pt := range path.geoPoints {
		pbGeoPoints = append(pbGeoPoints, &commonpb.GeoPoint{
			Latitude: pt.Lat(), Longitude: pt.Lng(),
		})
	}
	return &pb.Path{
		DestinationWaypointId: path.destinationWaypointID,
		Geopoints:             pbGeoPoints,
	}
}

// Proto to Path TODO

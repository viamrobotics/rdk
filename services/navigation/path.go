// Package navigation is the service that allows you to navigate along waypoints.
package navigation

import (
	"errors"

	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"
)

var errNilPath = errors.New("cannot convert nil path")

// Path describes a series of geo points the robot will travel through.
type Path struct {
	destinationWaypointID primitive.ObjectID
	geoPoints             []*geo.Point
}

// NewPath constructs a Path from a slice of geo.Points and ID.
func NewPath(id primitive.ObjectID, geoPoints []*geo.Point) (*Path, error) {
	if len(geoPoints) == 0 {
		return nil, errors.New("cannot instantiate path with no geoPoints")
	}
	return &Path{
		destinationWaypointID: id,
		geoPoints:             geoPoints,
	}, nil
}

// DestinationWaypointID returns the ID of the Path.
func (p *Path) DestinationWaypointID() primitive.ObjectID {
	return p.destinationWaypointID
}

// GeoPoints returns the slice of geo.Points the Path is comprised of.
func (p *Path) GeoPoints() []*geo.Point {
	return p.geoPoints
}

// PathSliceToProto converts a slice of Path into an equivalent Protobuf message.
func PathSliceToProto(paths []*Path) ([]*pb.Path, error) {
	var pbPaths []*pb.Path
	for _, path := range paths {
		pbPath, err := PathToProto(path)
		if err != nil {
			return nil, err
		}
		pbPaths = append(pbPaths, pbPath)
	}
	return pbPaths, nil
}

// PathToProto converts the Path struct into an equivalent Protobuf message.
func PathToProto(path *Path) (*pb.Path, error) {
	if path == nil {
		return nil, errNilPath
	}
	var pbGeoPoints []*commonpb.GeoPoint
	for _, pt := range path.geoPoints {
		pbGeoPoints = append(pbGeoPoints, &commonpb.GeoPoint{
			Latitude: pt.Lat(), Longitude: pt.Lng(),
		})
	}
	return &pb.Path{
		DestinationWaypointId: path.destinationWaypointID.Hex(),
		Geopoints:             pbGeoPoints,
	}, nil
}

// ProtoSliceToPaths converts a slice of Path Protobuf messages into an equivalent struct.
func ProtoSliceToPaths(pbPaths []*pb.Path) ([]*Path, error) {
	var paths []*Path
	for _, pbPath := range pbPaths {
		path, err := ProtoToPath(pbPath)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// ProtoToPath converts the Path Protobuf message into an equivalent struct.
func ProtoToPath(path *pb.Path) (*Path, error) {
	if path == nil {
		return nil, errNilPath
	}
	geoPoints := []*geo.Point{}
	for _, pt := range path.GetGeopoints() {
		geoPoints = append(geoPoints, geo.NewPoint(pt.GetLatitude(), pt.GetLongitude()))
	}
	id, err := primitive.ObjectIDFromHex(path.GetDestinationWaypointId())
	if err != nil {
		return nil, err
	}

	return NewPath(id, geoPoints)
}

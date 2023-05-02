package referenceframe

import (
	geo "github.com/kellydunn/golang-geo"

	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
)

type GeoObstacle struct {
	location   *geo.Point
	geometries []spatialmath.Geometry
}

// TODO: Docs
func NewGeoObstacle(loc *geo.Point, geom []spatialmath.Geometry) *GeoObstacle {
	return &GeoObstacle{
		location:   loc,
		geometries: geom,
	}
}

// TODO: Docs
func GeoObstacleToProtobuf(geoObst *GeoObstacle) (*commonpb.GeoObstacle, error) {
	var convGeoms []*commonpb.Geometry
	for _, geometry := range geoObst.geometries {
		convGeoms = append(convGeoms, geometry.ToProtobuf())
	}
	return &commonpb.GeoObstacle{
		Location:   &commonpb.GeoPoint{Latitude: geoObst.location.Lat(), Longitude: geoObst.location.Lng()},
		Geometries: convGeoms,
	}, nil
}

// TODO: Docs
func GeoObstacleFromProtobuf(protoGeoObst *commonpb.GeoObstacle) (*GeoObstacle, error) {
	convPoint := geo.NewPoint(protoGeoObst.location.GetLatitude(), protoGeoObst.location.GetLongitude())
	convGeoms := []spatialmath.Geometry{}
	for _, protoGeom := range protoGeoObst.geometries {
		newGeom, err := spatialmath.NewGeometryFromProto(protoGeom)
		if err != nil {
			return nil, err
		}
		convGeoms = append(convGeoms, newGeom)
	}
	return NewGeoObstacle(convPoint, convGeoms), nil
}

// WIP - More methods on GeoObstacles?

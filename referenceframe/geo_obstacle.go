package referenceframe

import (
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
)

// GeoObstacle is a struct to store the location and geometric structure of an obstacle in a geospatial environment.
type GeoObstacle struct {
	location   *geo.Point
	geometries []spatialmath.Geometry
}

// NewGeoObstacle constructs a GeoObstacle from a geo.Point and a slice of Geometries.
func NewGeoObstacle(loc *geo.Point, geom []spatialmath.Geometry) *GeoObstacle {
	return &GeoObstacle{
		location:   loc,
		geometries: geom,
	}
}

// Location returns the locating coordinates of the GeoObstacle.
func (gob *GeoObstacle) Location() *geo.Point {
	return gob.location
}

// Geometries returns the geometries which comprise structure of the GeoObstacle.
func (gob *GeoObstacle) Geometries() []spatialmath.Geometry {
	return gob.geometries
}

// GeoObstacleToProtobuf converts the GeoObstacle struct into an equivalent Protobuf message.
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

// GeoObstacleFromProtobuf takes a Protobuf representation of a GeoObstacle and converts back into a Go struct.
func GeoObstacleFromProtobuf(protoGeoObst *commonpb.GeoObstacle) (*GeoObstacle, error) {
	convPoint := geo.NewPoint(protoGeoObst.GetLocation().GetLatitude(), protoGeoObst.GetLocation().GetLongitude())
	convGeoms := []spatialmath.Geometry{}
	for _, protoGeom := range protoGeoObst.GetGeometries() {
		newGeom, err := spatialmath.NewGeometryFromProto(protoGeom)
		if err != nil {
			return nil, err
		}
		convGeoms = append(convGeoms, newGeom)
	}
	return NewGeoObstacle(convPoint, convGeoms), nil
}

// GeoObstacleConfig specifies the format of GeoObstacles specified through the configuration file.
type GeoObstacleConfig struct {
	Location   *commonpb.GeoPoint   `json:"location"`
	Geometries []*commonpb.Geometry `json:"geometries"`
}

// NewGeoObstacleConfig takes a GeoObstacle and returns a GeoObstacleConfig.
func NewGeoObstacleConfig(geo GeoObstacle) (*GeoObstacleConfig, error) {
	protoGeom := []*commonpb.Geometry{}
	for _, geom := range geo.geometries {
		protoGeom = append(protoGeom, geom.ToProtobuf())
	}

	config := &GeoObstacleConfig{
		Location:   &commonpb.GeoPoint{Latitude: geo.location.Lat(), Longitude: geo.location.Lng()},
		Geometries: protoGeom,
	}

	return config, nil
}

// GeoObstaclesFromConfig takes a GeoObstacleConfig and returns a list of GeoObstacles.
func GeoObstaclesFromConfig(config GeoObstacleConfig) ([]*GeoObstacle, error) {
	var gobs []*GeoObstacle
	for _, navGeom := range config.Geometries {
		gob := GeoObstacle{}

		gob.location = geo.NewPoint(config.Location.Latitude, config.Location.Longitude)
		geom, err := spatialmath.NewGeometryFromProto(navGeom)
		if err != nil {
			return nil, err
		}
		gob.geometries = append(gob.geometries, geom)
		gobs = append(gobs, &gob)
	}
	return gobs, nil
}

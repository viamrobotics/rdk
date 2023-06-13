package spatialmath

import (
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
)

// GeoObstacle is a struct to store the location and geometric structure of an obstacle in a geospatial environment.
type GeoObstacle struct {
	location   *geo.Point
	geometries []Geometry
}

// NewGeoObstacle constructs a GeoObstacle from a geo.Point and a slice of Geometries.
func NewGeoObstacle(loc *geo.Point, geom []Geometry) *GeoObstacle {
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
func (gob *GeoObstacle) Geometries() []Geometry {
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
	convGeoms := []Geometry{}
	for _, protoGeom := range protoGeoObst.GetGeometries() {
		newGeom, err := NewGeometryFromProto(protoGeom)
		if err != nil {
			return nil, err
		}
		convGeoms = append(convGeoms, newGeom)
	}
	return NewGeoObstacle(convPoint, convGeoms), nil
}

// GeoObstacleConfig specifies the format of GeoObstacles specified through the configuration file.
type GeoObstacleConfig struct {
	Location   *commonpb.GeoPoint `json:"location"`
	Geometries []*GeometryConfig  `json:"geometries"`
}

// NewGeoObstacleConfig takes a GeoObstacle and returns a GeoObstacleConfig.
func NewGeoObstacleConfig(geo *GeoObstacle) (*GeoObstacleConfig, error) {
	geomCfgs := []*GeometryConfig{}
	for _, geom := range geo.geometries {
		gc, err := NewGeometryConfig(geom)
		if err != nil {
			return nil, err
		}
		geomCfgs = append(geomCfgs, gc)
	}

	config := &GeoObstacleConfig{
		Location:   &commonpb.GeoPoint{Latitude: geo.location.Lat(), Longitude: geo.location.Lng()},
		Geometries: geomCfgs,
	}

	return config, nil
}

// GeoObstaclesFromConfigs takes a GeoObstacleConfig and returns a list of GeoObstacles.
func GeoObstaclesFromConfigs(configs []*GeoObstacleConfig) ([]*GeoObstacle, error) {
	var gobs []*GeoObstacle
	for _, cfg := range configs {
		gob, err := GeoObstaclesFromConfig(cfg)
		if err != nil {
			return nil, err
		}
		gobs = append(gobs, gob...)
	}
	return gobs, nil
}

// GeoObstaclesFromConfig takes a GeoObstacleConfig and returns a list of GeoObstacles.
func GeoObstaclesFromConfig(config *GeoObstacleConfig) ([]*GeoObstacle, error) {
	var gobs []*GeoObstacle
	for _, navGeom := range config.Geometries {
		gob := GeoObstacle{}

		gob.location = geo.NewPoint(config.Location.Latitude, config.Location.Longitude)

		geom, err := navGeom.ParseConfig()
		if err != nil {
			return nil, err
		}
		gob.geometries = append(gob.geometries, geom)
		gobs = append(gobs, &gob)
	}
	return gobs, nil
}

// GetCartesianDistance calculates the latitude and longitide displacement between p and q in kilometers.
func GetCartesianDistance(p, q *geo.Point) (float64, float64) {
	mod := geo.NewPoint(p.Lat(), q.Lng())
	// Calculates the Haversine distance between two points in kilometers
	latDist := p.GreatCircleDistance(mod)
	lngDist := q.GreatCircleDistance(mod)
	return latDist, lngDist
}

// GeoPointToPose converts p into a spatialmath pose relative to lng = 0 = lat.
func GeoPointToPose(p *geo.Point) Pose {
	latDist, lngDist := GetCartesianDistance(geo.NewPoint(0, 0), p)
	// multiply by 1000000 to convert km to mm
	return NewPoseFromPoint(r3.Vector{latDist * 1e6, lngDist * 1e6, 0})
}

// GeoObstaclesToGeometries converts a list of GeoObstacles into a list of Geometries.
func GeoObstaclesToGeometries(obstacles []*GeoObstacle, worldOrigin r3.Vector) []Geometry {
	// we note that there are two transformations to be accounted for
	// when converting a GeoObstacle. Namely, the obstacle's pose needs to
	// transformed by the specified in GPS coordinates.
	geoms := []Geometry{}
	for _, v := range obstacles {
		obstacleOrigin := GeoPointToPose(v.location)
		relativeDestinationPt := r3.Vector{
			X: obstacleOrigin.Point().X - worldOrigin.X,
			Y: obstacleOrigin.Point().Y - worldOrigin.Y,
			Z: 0,
		}
		relativeDstPose := NewPoseFromPoint(relativeDestinationPt)
		for _, geom := range v.geometries {
			geo := geom.Transform(relativeDstPose)
			geoms = append(geoms, geo)
		}
	}
	return geoms
}

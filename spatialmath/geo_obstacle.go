package spatialmath

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/utils"
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
func GeoObstacleToProtobuf(geoObst *GeoObstacle) *commonpb.GeoObstacle {
	var convGeoms []*commonpb.Geometry
	for _, geometry := range geoObst.geometries {
		convGeoms = append(convGeoms, geometry.ToProtobuf())
	}
	return &commonpb.GeoObstacle{
		Location:   &commonpb.GeoPoint{Latitude: geoObst.location.Lat(), Longitude: geoObst.location.Lng()},
		Geometries: convGeoms,
	}
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

// GetCartesianDistance calculates the latitude and longitide displacement between p and q in millimeters.
// Note that this is an approximation since we are trying to project a point on a sphere onto a plane.
// The closer these points are the more accurate the approximation is.
func GetCartesianDistance(p, q *geo.Point) (float64, float64) {
	mod := geo.NewPoint(p.Lat(), q.Lng())
	// Calculate the Haversine distance between two points in kilometers, convert to mm
	distAlongLat := 1e6 * p.GreatCircleDistance(mod)
	distAlongLng := 1e6 * q.GreatCircleDistance(mod)
	return distAlongLat, distAlongLng
}

// GeoPoseToPose returns the pose of point with respect to origin.
func GeoPoseToPose(point, origin *GeoPose) Pose {
	localBearing := origin.Location().BearingTo(point.Location())
	absoluteBearing := localBearing - origin.Heading()

	latDist, lngDist := GetCartesianDistance(point.Location(), origin.Location())
	v := r3.Vector{X: latDist * 1e-6, Y: lngDist * 1e-6, Z: 0}

	newPoint := origin.Location().PointAtDistanceAndBearing(v.Norm(), absoluteBearing)

	// subtracting the point from the origin results in a right handed angle
	headingChange := normalizeAngle(origin.Heading() - point.Heading())
	return NewPose(GeoPointToPoint(newPoint, origin.Location()), &OrientationVectorDegrees{OZ: 1, Theta: headingChange})
}

// GeoPointToPoint returns the point (r3.Vector) which translates the origin to the destination geopoint
// Because the function we use to project a point on a spheroid to a plane is nonlinear, we linearize it about a specified origin point.
func GeoPointToPoint(point, origin *geo.Point) r3.Vector {
	latDist, lngDist := GetCartesianDistance(origin, point)
	azimuth := origin.BearingTo(point)

	switch {
	case azimuth >= 0 && azimuth <= 90:
		return r3.Vector{X: latDist, Y: lngDist, Z: 0}
	case azimuth > 90 && azimuth <= 180:
		return r3.Vector{X: latDist, Y: -lngDist, Z: 0}
	case azimuth >= -90 && azimuth < 0:
		return r3.Vector{X: -latDist, Y: lngDist, Z: 0}
	default:
		return r3.Vector{X: -latDist, Y: -lngDist, Z: 0}
	}
}

// GeoObstaclesToGeometries converts a list of GeoObstacles into a list of Geometries.
func GeoObstaclesToGeometries(obstacles []*GeoObstacle, origin *geo.Point) []Geometry {
	// we note that there are two transformations to be accounted for
	// when converting a GeoObstacle. Namely, the obstacle's pose needs to
	// transformed by the specified in GPS coordinates.
	geoms := []Geometry{}
	for _, v := range obstacles {
		relativePose := NewPoseFromPoint(GeoPointToPoint(v.location, origin))
		for _, geom := range v.geometries {
			geo := geom.Transform(relativePose)
			geoms = append(geoms, geo)
		}
	}
	return geoms
}

// GeoPose is a struct to store to location and heading in a geospatial environment.
type GeoPose struct {
	location *geo.Point
	heading  float64
}

// NewGeoPose constructs a GeoPose from a geo.Point and float64.
func NewGeoPose(loc *geo.Point, heading float64) *GeoPose {
	return &GeoPose{
		location: loc,
		heading:  heading,
	}
}

// Location returns the locating coordinates of the GeoPose.
func (gpo *GeoPose) Location() *geo.Point {
	return gpo.location
}

// Heading returns a number from [0-360) where 0 is north.
func (gpo *GeoPose) Heading() float64 {
	return gpo.heading
}

// PoseToGeoPose converts a pose (which are always in mm) into a GeoPose treating relativeTo as the origin.
func PoseToGeoPose(relativeTo *GeoPose, pose Pose) *GeoPose {
	fmt.Println(" ")
	// poses are always in mm but PointAtDistanceAndBearing expects the pose to be in km so we need to convert
	kmPoint := pose.Point().Mul(1e-6)

	// calculate the bearing (illustrated on the plot below as angle "x"), to the GeoPose (illustrated as "*")
	// as we are measuring x from the right side of the vertical axis this angle is left handed
	//       |   *
	//       |x /
	//       | /
	//       |/
	// -----------
	//       |
	//       |
	bearing := utils.RadToDeg(math.Atan2(kmPoint.X, kmPoint.Y))
	headingInWorld := relativeTo.Heading()

	// get the absolute bearing, i.e. the bearing of pose p from north
	absoluteBearing := normalizeAngle(bearing + headingInWorld)

	// get the new geopoint at distance poseMagnitude
	newPosition := relativeTo.Location().PointAtDistanceAndBearing(kmPoint.Norm(), absoluteBearing)

	// get the heading of pose p, this is a right-handed value
	headingRight := pose.Orientation().OrientationVectorDegrees().Theta

	// convert headingRight to be left-handed
	headingLeft := math.Mod(math.Abs(headingRight-360), 360)

	poseAbsoluteHeading := normalizeAngle(headingLeft + headingInWorld)

	// return the GeoPose at the new position with the absolute heading of pose p, i.e. the heading in the world
	return NewGeoPose(newPosition, poseAbsoluteHeading)
}

// normalizeAngle takes in an angle in degrees and returns an equivalent angle in the domain [0,360).
func normalizeAngle(degrees float64) float64 {
	normalized := math.Mod(degrees, 360)
	if degrees < 0 {
		normalized += 360
	}
	return normalized
}

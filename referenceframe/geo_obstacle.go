package referenceframe

import (
	"github.com/golang/geo/r3"
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

type GeoObstacleConfig struct {
	Location   *navLocation `json:"location"`
	Geometries []*navGeoms  `json:"obstacles"`
}

type navLocation struct {
	Lat float64 `json:"latitude"`
	Lng float64 `json:"longitude"`
}

type navGeoms struct {
	Center  *navGeomCenter `json:"center"`
	Box     *navBox        `json:"box"`
	Sphere  *navSphere     `json:"sphere"`
	Capsule *navCapsule    `json:"capsule"`
	Label   string         `json:"label"`
}

type navGeomCenter struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	OX    float64 `json:"o_x"`
	OY    float64 `json:"o_y"`
	OZ    float64 `json:"o_z"`
	Theta float64 `json:"theta"`
}

type navBox struct {
	BoxDims *navBoxDims `json:"dims_mm"`
}

type navBoxDims struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type navSphere struct {
	RadiusMM float64 `json:"radius_mm"`
}

type navCapsule struct {
	RadiusMM float64 `json:"radius_mm"`
	LengthMM float64 `json:"length_mm"`
}

// NewGeoObstacleConfig takes a GeoObstacle and returns a GeoObstacleConfig
func NewGeoObstacleConfig(geo GeoObstacle) (*GeoObstacleConfig, error) {
	config := &GeoObstacleConfig{}
	config.Location.Lat = geo.location.Lat()
	config.Location.Lng = geo.location.Lng()

	for _, geom := range geo.geometries {
		geo := navGeoms{}

		geo.Label = geom.Label()

		geomCfg, err := spatialmath.NewGeometryConfig(geom)
		if err != nil {
			return nil, err
		}

		geo.Center.X = geomCfg.X
		geo.Center.Y = geomCfg.Y
		geo.Center.Z = geomCfg.Z

		orientation, err := geomCfg.OrientationOffset.ParseConfig()
		if err != nil {
			return nil, err
		}

		geo.Center.OX = orientation.OrientationVectorDegrees().OX
		geo.Center.OY = orientation.OrientationVectorDegrees().OY
		geo.Center.OZ = orientation.OrientationVectorDegrees().OZ
		geo.Center.Theta = orientation.OrientationVectorDegrees().Theta

		switch geomCfg.Type {
		case spatialmath.BoxType:
			box := navBox{}
			box.BoxDims.X = geomCfg.X
			box.BoxDims.Y = geomCfg.Y
			box.BoxDims.Z = geomCfg.Z
			geo.Box = &box

		case spatialmath.SphereType:
			sphere := navSphere{}
			sphere.RadiusMM = geomCfg.R
			geo.Sphere = &sphere

		case spatialmath.CapsuleType:
			capsule := navCapsule{}
			capsule.RadiusMM = geomCfg.R
			capsule.LengthMM = geomCfg.L
			geo.Capsule = &capsule

		default:
			return nil, spatialmath.ErrGeometryTypeUnsupported
		}

		config.Geometries = append(config.Geometries, &geo)
	}

	return config, nil
}

// GeoObstaclesFromConfig takes a GeoObstacleConfig and returns a list of GeoObstacles
func GeoObstaclesFromConfig(config GeoObstacleConfig) ([]*GeoObstacle, error) {
	var gobs []*GeoObstacle
	for _, navGeom := range config.Geometries {
		var gob *GeoObstacle
		gob.location = geo.NewPoint(config.Location.Lat, config.Location.Lng)
		orien := spatialmath.NewOrientationVectorDegrees()
		orien.OX = navGeom.Center.OX
		orien.OY = navGeom.Center.OY
		orien.OZ = navGeom.Center.OZ
		orien.Theta = navGeom.Center.Theta
		offset := r3.Vector{navGeom.Center.X, navGeom.Center.Y, navGeom.Center.Z}
		pose := spatialmath.NewPose(offset, orien)

		if navGeom.Box != nil {
			boxDims := r3.Vector{navGeom.Box.BoxDims.X, navGeom.Box.BoxDims.Y, navGeom.Box.BoxDims.Z}
			box, err := spatialmath.NewBox(pose, boxDims, navGeom.Label)
			if err != nil {
				return nil, err
			}
			gob.geometries = append(gob.geometries, box)
		}

		if navGeom.Sphere != nil {
			sphere, err := spatialmath.NewSphere(pose, navGeom.Sphere.RadiusMM, navGeom.Label)
			if err != nil {
				return nil, err
			}
			gob.geometries = append(gob.geometries, sphere)
		}

		if navGeom.Capsule != nil {
			capsule, err := spatialmath.NewCapsule(pose, navGeom.Capsule.RadiusMM, navGeom.Capsule.LengthMM, navGeom.Label)
			if err != nil {
				return nil, err
			}
			gob.geometries = append(gob.geometries, capsule)
		}

		gobs = append(gobs, gob)
	}
	return gobs, nil
}

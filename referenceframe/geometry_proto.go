package referenceframe

import (
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

// NewGeometryFromProto instantiates a new Geometry from a protobuf Geometry message.
func NewGeometryFromProto(geometry *commonpb.Geometry) (spatialmath.Geometry, error) {
	if geometry.Center == nil {
		return nil, errors.New("cannot have nil pose for geometry")
	}
	pose := spatialmath.NewPoseFromProtobuf(geometry.Center)
	if box := geometry.GetBox().GetDimsMm(); box != nil {
		return spatialmath.NewBox(pose, r3.Vector{X: box.X, Y: box.Y, Z: box.Z}, geometry.Label)
	}
	if capsule := geometry.GetCapsule(); capsule != nil {
		return spatialmath.NewCapsule(pose, capsule.RadiusMm, capsule.LengthMm, geometry.Label)
	}
	if sphere := geometry.GetSphere(); sphere != nil {
		// Fallback to point if radius is 0
		if sphere.RadiusMm == 0 {
			return spatialmath.NewPoint(pose.Point(), geometry.Label), nil
		}
		return spatialmath.NewSphere(pose, sphere.RadiusMm, geometry.Label)
	}
	if mesh := geometry.GetMesh(); mesh != nil {
		return spatialmath.NewMeshFromProto(pose, mesh, geometry.Label)
	}
	if pointCloud := geometry.GetPointcloud(); pointCloud != nil {
		return pointcloud.NewPointCloudFromProto(pointCloud, geometry.Label)
	}
	return nil, errGeometryTypeUnsupported
}

// NewGeometriesFromProto converts a list of Geometries from protobuf.
func NewGeometriesFromProto(proto []*commonpb.Geometry) ([]spatialmath.Geometry, error) {
	if proto == nil {
		return nil, nil
	}
	geometries := []spatialmath.Geometry{}
	for _, geometry := range proto {
		g, err := NewGeometryFromProto(geometry)
		if err != nil {
			return nil, err
		}
		geometries = append(geometries, g)
	}
	return geometries, nil
}

// NewGeometriesToProto converts a list of Geometries to profobuf.
func NewGeometriesToProto(geometries []spatialmath.Geometry) []*commonpb.Geometry {
	var proto []*commonpb.Geometry
	for _, geometry := range geometries {
		proto = append(proto, geometry.ToProtobuf())
	}
	return proto
}

// GeoGeometryToProtobuf converts the GeoGeometry struct into an equivalent Protobuf message.
func GeoGeometryToProtobuf(geoObst *spatialmath.GeoGeometry) *commonpb.GeoGeometry {
	var convGeoms []*commonpb.Geometry
	for _, geometry := range geoObst.Geometries() {
		convGeoms = append(convGeoms, geometry.ToProtobuf())
	}
	return &commonpb.GeoGeometry{
		Location:   &commonpb.GeoPoint{Latitude: geoObst.Location().Lat(), Longitude: geoObst.Location().Lng()},
		Geometries: convGeoms,
	}
}

// GeoGeometryFromProtobuf takes a Protobuf representation of a GeoGeometry and converts back into a Go struct.
func GeoGeometryFromProtobuf(protoGeoObst *commonpb.GeoGeometry) (*spatialmath.GeoGeometry, error) {
	convPoint := geo.NewPoint(protoGeoObst.GetLocation().GetLatitude(), protoGeoObst.GetLocation().GetLongitude())
	convGeoms := []spatialmath.Geometry{}
	for _, protoGeom := range protoGeoObst.GetGeometries() {
		newGeom, err := NewGeometryFromProto(protoGeom)
		if err != nil {
			return nil, err
		}
		convGeoms = append(convGeoms, newGeom)
	}
	return spatialmath.NewGeoGeometry(convPoint, convGeoms), nil
}

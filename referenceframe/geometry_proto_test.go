package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

func TestGeometryToFromProtobuf(t *testing.T) {
	deg45 := math.Pi / 4
	testCases := []struct {
		name     string
		geometry spatialmath.Geometry
	}{
		{"box", spatialmath.MakeTestBox(&spatialmath.EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, "box")},
		{"sphere", spatialmath.MakeTestSphere(r3.Vector{3, 4, 5}, 10, "sphere")},
		{"capsule", spatialmath.MakeTestCapsule(&spatialmath.EulerAngles{0, 0, deg45}, r3.Vector{1, 2, 3}, 5, 20, "capsule")},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			newVol, err := NewGeometryFromProto(testCase.geometry.ToProtobuf())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, spatialmath.GeometriesAlmostEqual(testCase.geometry, newVol), test.ShouldBeTrue)
			test.That(t, testCase.geometry.Label(), test.ShouldEqual, testCase.name)
		})
	}

	// test that bad message does not generate error
	_, err := NewGeometryFromProto(&commonpb.Geometry{Center: spatialmath.PoseToProtobuf(spatialmath.NewZeroPose())})
	test.That(t, err.Error(), test.ShouldContainSubstring, errGeometryTypeUnsupported.Error())
}

func TestPointCloudGeometryToFromProtobuf(t *testing.T) {
	pc := pointcloud.MakeTestPointCloud("pointcloud")
	proto := pc.ToProtobuf()
	test.That(t, proto, test.ShouldNotBeNil)
	test.That(t, proto.Center, test.ShouldNotBeNil)
	test.That(t, proto.Label, test.ShouldEqual, "pointcloud")

	newVol, err := NewGeometryFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newVol, test.ShouldNotBeNil)
	test.That(t, newVol.Label(), test.ShouldEqual, "pointcloud")
	test.That(t, pc.Label(), test.ShouldEqual, newVol.Label())
	test.That(t, spatialmath.GeometriesAlmostEqual(pc, newVol), test.ShouldBeFalse)
	test.That(t, pc.Size(), test.ShouldEqual, 3)

	newVolPC, ok := newVol.(pointcloud.PointCloud)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, newVolPC.Size(), test.ShouldEqual, 3)
}

func TestMeshGeometryToFromProtobuf(t *testing.T) {
	mesh := spatialmath.MakeTestMesh(
		spatialmath.NewZeroOrientation(),
		r3.Vector{1, 1, 1},
		[]*spatialmath.Triangle{spatialmath.NewTriangle(r3.Vector{0, 0, 0}, r3.Vector{1, 0, 0}, r3.Vector{0, 1, 0})},
		"mesh",
	)
	_, err := NewGeometryFromProto(mesh.ToProtobuf())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported Mesh type")
	test.That(t, mesh.Label(), test.ShouldEqual, "mesh")
}

func TestNewGeometryFromProto(t *testing.T) {
	malformedGeom := commonpb.Geometry{}
	viamGeom, err := NewGeometryFromProto(&malformedGeom)
	test.That(t, viamGeom, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have nil pose for geometry"))

	properGeom := commonpb.Geometry{
		Center: &commonpb.Pose{OZ: 1},
		GeometryType: &commonpb.Geometry_Sphere{
			Sphere: &commonpb.Sphere{
				RadiusMm: 1,
			},
		},
	}
	viamGeom, err = NewGeometryFromProto(&properGeom)
	test.That(t, err, test.ShouldBeNil)
	sphereGeom, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 1, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, viamGeom, test.ShouldResemble, sphereGeom)
}

func TestAllGeometryTypesToProtobuf(t *testing.T) {
	deg45 := math.Pi / 4
	testCases := []struct {
		name     string
		geometry spatialmath.Geometry
	}{
		{"box", spatialmath.MakeTestBox(&spatialmath.EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, "box")},
		{"sphere", spatialmath.MakeTestSphere(r3.Vector{3, 4, 5}, 10, "sphere")},
		{"point", spatialmath.MakeTestPoint(r3.Vector{3, 4, 5}, "point")},
		{"capsule", spatialmath.MakeTestCapsule(&spatialmath.EulerAngles{0, 0, deg45}, r3.Vector{1, 2, 3}, 5, 20, "capsule")},
		{
			"mesh",
			spatialmath.MakeTestMesh(
				spatialmath.NewZeroOrientation(),
				r3.Vector{1, 1, 1},
				[]*spatialmath.Triangle{spatialmath.NewTriangle(r3.Vector{0, 0, 0}, r3.Vector{1, 0, 0}, r3.Vector{0, 1, 0})},
				"mesh",
			),
		},
		{"pointcloud", pointcloud.MakeTestPointCloud("pointcloud")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proto := testCase.geometry.ToProtobuf()
			test.That(t, proto, test.ShouldNotBeNil)
			test.That(t, proto.Center, test.ShouldNotBeNil)
			test.That(t, proto.Label, test.ShouldEqual, testCase.name)
		})
	}
}

func TestGeometryProtobufRoundTrip(t *testing.T) {
	deg45 := math.Pi / 4
	testCases := []struct {
		name     string
		geometry spatialmath.Geometry
	}{
		{"box", spatialmath.MakeTestBox(&spatialmath.EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, "box")},
		{"sphere", spatialmath.MakeTestSphere(r3.Vector{3, 4, 5}, 10, "sphere")},
		{"capsule", spatialmath.MakeTestCapsule(&spatialmath.EulerAngles{0, 0, deg45}, r3.Vector{1, 2, 3}, 5, 20, "capsule")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proto := testCase.geometry.ToProtobuf()
			test.That(t, proto, test.ShouldNotBeNil)

			newGeometry, err := NewGeometryFromProto(proto)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, newGeometry, test.ShouldNotBeNil)

			test.That(t, spatialmath.GeometriesAlmostEqual(testCase.geometry, newGeometry), test.ShouldBeTrue)
			test.That(t, testCase.geometry.Label(), test.ShouldEqual, newGeometry.Label())
		})
	}
}

func TestPointCloudGeometryProtobufRoundTrip(t *testing.T) {
	pc := pointcloud.MakeTestPointCloud("pointcloud")

	proto := pc.ToProtobuf()
	test.That(t, proto, test.ShouldNotBeNil)

	newGeometry, err := NewGeometryFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newGeometry, test.ShouldNotBeNil)
	test.That(t, pc.Label(), test.ShouldEqual, newGeometry.Label())

	newVolPC, ok := newGeometry.(pointcloud.PointCloud)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, newVolPC.Size(), test.ShouldEqual, 3)

	_, exists := newVolPC.At(0, 0, 0)
	test.That(t, exists, test.ShouldBeTrue)
	_, exists = newVolPC.At(1, 0, 0)
	test.That(t, exists, test.ShouldBeTrue)
	_, exists = newVolPC.At(0, 1, 0)
	test.That(t, exists, test.ShouldBeTrue)

	test.That(t, newVolPC.MetaData(), test.ShouldResemble, pc.MetaData())
}

func TestMeshGeometryProtobufRoundTrip(t *testing.T) {
	mesh := spatialmath.MakeTestMesh(
		spatialmath.NewZeroOrientation(),
		r3.Vector{1, 1, 1},
		[]*spatialmath.Triangle{spatialmath.NewTriangle(r3.Vector{0, 0, 0}, r3.Vector{1, 0, 0}, r3.Vector{0, 1, 0})},
		"mesh",
	)

	proto := mesh.ToProtobuf()
	test.That(t, proto, test.ShouldNotBeNil)

	_, err := NewGeometryFromProto(proto)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported Mesh type")
	test.That(t, mesh.Label(), test.ShouldEqual, "mesh")
}

func TestGeoGeometryToProtobuf(t *testing.T) {
	testSphere := spatialmath.MakeTestSphere(r3.Vector{0, 0, 0}, 5, "test_sphere")
	testGeoms := []spatialmath.Geometry{testSphere}
	testLatitude := 37.7749
	testLongitude := -122.4194
	testPoint := geo.NewPoint(testLatitude, testLongitude)
	testGeoObst := spatialmath.NewGeoGeometry(testPoint, testGeoms)

	convGeoObstProto := GeoGeometryToProtobuf(testGeoObst)
	test.That(t, convGeoObstProto, test.ShouldNotBeNil)
	test.That(t, testPoint.Lat(), test.ShouldEqual, convGeoObstProto.GetLocation().GetLatitude())
	test.That(t, testPoint.Lng(), test.ShouldEqual, convGeoObstProto.GetLocation().GetLongitude())
	test.That(t, len(testGeoms), test.ShouldEqual, len(convGeoObstProto.GetGeometries()))
}

func TestGeoGeometryFromProtobuf(t *testing.T) {
	testSphere := spatialmath.MakeTestSphere(r3.Vector{0, 0, 0}, 5, "test_sphere")
	testGeoms := []spatialmath.Geometry{testSphere}
	testLatitude := 37.7749
	testLongitude := -122.4194
	testPoint := geo.NewPoint(testLatitude, testLongitude)

	testProtobuf := &commonpb.GeoGeometry{
		Location:   &commonpb.GeoPoint{Latitude: testLatitude, Longitude: testLongitude},
		Geometries: []*commonpb.Geometry{testSphere.ToProtobuf()},
	}

	convGeoObst, err := GeoGeometryFromProtobuf(testProtobuf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, testPoint.Lat(), test.ShouldEqual, convGeoObst.Location().Lat())
	test.That(t, testPoint.Lng(), test.ShouldEqual, convGeoObst.Location().Lng())
	test.That(t, len(testGeoms), test.ShouldEqual, len(convGeoObst.Geometries()))
}

package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

func TestGeometryProtobufRoundTrip(t *testing.T) {
	deg45 := math.Pi / 4
	box, _ := spatialmath.NewBox(spatialmath.NewPose(r3.Vector{0, 0, 0}, &spatialmath.EulerAngles{0, 0, deg45}), r3.Vector{2, 2, 2}, "box")
	sphere, _ := spatialmath.NewSphere(spatialmath.NewPose(r3.Vector{3, 4, 5}, spatialmath.NewZeroOrientation()), 10, "sphere")
	point := spatialmath.NewPoint(r3.Vector{3, 4, 5}, "point")
	capsule, _ := spatialmath.NewCapsule(spatialmath.NewPose(r3.Vector{1, 2, 3}, &spatialmath.EulerAngles{0, 0, deg45}), 5, 20, "capsule")
	testCases := []struct {
		name     string
		geometry spatialmath.Geometry
	}{
		{"box", box},
		{"sphere", sphere},
		{"point", point},
		{"capsule", capsule},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proto := testCase.geometry.ToProtobuf()
			test.That(t, proto, test.ShouldNotBeNil)
			test.That(t, proto.Center, test.ShouldNotBeNil)
			test.That(t, proto.Label, test.ShouldEqual, testCase.name)

			newGeometry, err := NewGeometryFromProto(proto)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, newGeometry, test.ShouldNotBeNil)
			test.That(t, testCase.geometry.Label(), test.ShouldEqual, newGeometry.Label())
			test.That(t, spatialmath.GeometriesAlmostEqual(testCase.geometry, newGeometry), test.ShouldBeTrue)
		})
	}
}

func makeTestPointCloud(label string) *pointcloud.BasicOctree {
	pc := pointcloud.NewBasicPointCloud(3)
	err := pc.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pointcloud.NewBasicData())
	if err != nil {
		return nil
	}
	err = pc.Set(r3.Vector{X: 1, Y: 0, Z: 0}, pointcloud.NewBasicData())
	if err != nil {
		return nil
	}
	err = pc.Set(r3.Vector{X: 0, Y: 1, Z: 0}, pointcloud.NewBasicData())
	if err != nil {
		return nil
	}

	octree, err := pointcloud.ToBasicOctree(pc, 50)
	if err != nil {
		return nil
	}

	octree.SetLabel(label)
	return octree
}

func TestPointCloudProtobufRoundTrip(t *testing.T) {
	pc := makeTestPointCloud("pointcloud")

	proto := pc.ToProtobuf()
	test.That(t, proto, test.ShouldNotBeNil)
	test.That(t, proto.Center, test.ShouldNotBeNil)
	test.That(t, proto.Label, test.ShouldEqual, "pointcloud")

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

func TestMeshProtobufRoundTrip(t *testing.T) {
	// Create a complex mesh with various triangles for thorough testing
	triangles := []*spatialmath.Triangle{
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1000, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1000, Z: 0},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: -500, Y: -500, Z: 0},
			r3.Vector{X: 500, Y: -500, Z: 0},
			r3.Vector{X: 0, Y: 500, Z: 0},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 1000},
			r3.Vector{X: 1000, Y: 0, Z: 1000},
			r3.Vector{X: 500, Y: 1000, Z: 1000},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 123.456, Y: 789.012, Z: 345.678},
			r3.Vector{X: 456.789, Y: 123.456, Z: 678.901},
			r3.Vector{X: 789.012, Y: 456.789, Z: 123.456},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 10000, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 10000, Z: 0},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1, Z: 0},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: -1000, Y: -1000, Z: -1000},
			r3.Vector{X: -500, Y: -1000, Z: -1000},
			r3.Vector{X: -1000, Y: -500, Z: -1000},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 100, Y: 100, Z: -500},
			r3.Vector{X: 200, Y: 100, Z: 500},
			r3.Vector{X: 150, Y: 200, Z: 0},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1000, Y: 0, Z: 0},
			r3.Vector{X: 1000, Y: 1000, Z: 0},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 1000, Y: 1000, Z: 0},
			r3.Vector{X: 0, Y: 1000, Z: 0},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1000, Z: 0},
			r3.Vector{X: 0, Y: 1000, Z: 1000},
		),
		spatialmath.NewTriangle(
			r3.Vector{X: 0, Y: 0, Z: 0},
			r3.Vector{X: 0, Y: 1000, Z: 1000},
			r3.Vector{X: 0, Y: 0, Z: 1000},
		),
	}

	// Create mesh with a pose and label
	originalPose := spatialmath.NewPose(r3.Vector{X: 100, Y: 200, Z: 300}, spatialmath.NewZeroOrientation())
	originalMesh := spatialmath.NewMesh(originalPose, triangles, "test_mesh_from_triangles")

	// Convert to protobuf
	proto := originalMesh.ToProtobuf()
	test.That(t, proto, test.ShouldNotBeNil)
	test.That(t, proto.Label, test.ShouldEqual, "test_mesh_from_triangles")

	// Restore from protobuf
	restoredGeometry, err := NewGeometryFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	restoredMesh, ok := restoredGeometry.(*spatialmath.Mesh)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, restoredMesh.Label(), test.ShouldEqual, originalMesh.Label())
	test.That(t, spatialmath.PoseAlmostEqual(restoredMesh.Pose(), originalMesh.Pose()), test.ShouldBeTrue)
	test.That(t, len(restoredMesh.Triangles()), test.ShouldEqual, len(originalMesh.Triangles()))

	// Verify all triangles match
	originalTriangles := originalMesh.Triangles()
	restoredTriangles := restoredMesh.Triangles()
	for i, originalTri := range originalTriangles {
		restoredTri := restoredTriangles[i]
		origPoints := originalTri.Points()
		restoredPoints := restoredTri.Points()

		test.That(t, len(restoredPoints), test.ShouldEqual, len(origPoints))

		for j, origPoint := range origPoints {
			restoredPoint := restoredPoints[j]
			// The conversion from mm to meters and back can create micrometer-level float changes
			epsilon := 1e-4
			test.That(t, math.Abs(origPoint.X-restoredPoint.X), test.ShouldBeLessThan, epsilon)
			test.That(t, math.Abs(origPoint.Y-restoredPoint.Y), test.ShouldBeLessThan, epsilon)
			test.That(t, math.Abs(origPoint.Z-restoredPoint.Z), test.ShouldBeLessThan, epsilon)
		}
	}

	// Verify that the mesh can be converted to protobuf again
	secondProto := restoredMesh.ToProtobuf()
	test.That(t, secondProto, test.ShouldNotBeNil)
	test.That(t, secondProto.Label, test.ShouldEqual, originalMesh.Label())

	// Verify the protobuf content is the same
	test.That(t, secondProto.GetMesh().ContentType, test.ShouldEqual, proto.GetMesh().ContentType)
	test.That(t, len(secondProto.GetMesh().Mesh), test.ShouldEqual, len(proto.GetMesh().Mesh))
}

func TestNewGeometryFromProtoErrors(t *testing.T) {
	testCases := []struct {
		name        string
		geometry    *commonpb.Geometry
		expectedErr string
	}{
		{
			name:        "nil pose",
			geometry:    &commonpb.Geometry{},
			expectedErr: "cannot have nil pose for geometry",
		},
		{
			name: "unsupported geometry type",
			geometry: &commonpb.Geometry{
				Center: spatialmath.PoseToProtobuf(spatialmath.NewZeroPose()),
			},
			expectedErr: errGeometryTypeUnsupported.Error(),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			viamGeom, err := NewGeometryFromProto(testCase.geometry)
			test.That(t, viamGeom, test.ShouldBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, testCase.expectedErr)
		})
	}
}

func TestGeoGeometryProtobufRoundTrip(t *testing.T) {
	singleSphere, _ := spatialmath.NewSphere(spatialmath.NewPose(r3.Vector{0, 0, 0}, spatialmath.NewZeroOrientation()), 5, "test_sphere")
	multiSphere, _ := spatialmath.NewSphere(spatialmath.NewPose(r3.Vector{0, 0, 0}, spatialmath.NewZeroOrientation()), 5, "sphere1")
	multiBox, _ := spatialmath.NewBox(spatialmath.NewPose(r3.Vector{10, 0, 0}, spatialmath.NewZeroOrientation()), r3.Vector{2, 2, 2}, "box1")
	multiSphere.SetLabel("sphere1")
	multiBox.SetLabel("box1")

	testCases := []struct {
		name        string
		geometries  []spatialmath.Geometry
		latitude    float64
		longitude   float64
		description string
	}{
		{
			name:        "single sphere",
			geometries:  []spatialmath.Geometry{singleSphere},
			latitude:    37.7749,
			longitude:   -122.4194,
			description: "San Francisco",
		},
		{
			name: "multiple geometries",
			geometries: []spatialmath.Geometry{
				multiSphere,
				multiBox,
			},
			latitude:    40.7128,
			longitude:   -74.0060,
			description: "New York",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testPoint := geo.NewPoint(testCase.latitude, testCase.longitude)
			testGeoObst := spatialmath.NewGeoGeometry(testPoint, testCase.geometries)

			convGeoObstProto := GeoGeometryToProtobuf(testGeoObst)
			test.That(t, convGeoObstProto, test.ShouldNotBeNil)
			test.That(t, testPoint.Lat(), test.ShouldEqual, convGeoObstProto.GetLocation().GetLatitude())
			test.That(t, testPoint.Lng(), test.ShouldEqual, convGeoObstProto.GetLocation().GetLongitude())
			test.That(t, len(testCase.geometries), test.ShouldEqual, len(convGeoObstProto.GetGeometries()))

			convGeoObst, err := GeoGeometryFromProtobuf(convGeoObstProto)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, testPoint.Lat(), test.ShouldEqual, convGeoObst.Location().Lat())
			test.That(t, testPoint.Lng(), test.ShouldEqual, convGeoObst.Location().Lng())
			test.That(t, len(testCase.geometries), test.ShouldEqual, len(convGeoObst.Geometries()))
		})
	}
}

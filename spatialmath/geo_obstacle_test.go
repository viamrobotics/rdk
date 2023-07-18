package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
)

func TestGeoObstacles(t *testing.T) {
	testLatitude := 39.58836
	testLongitude := -105.64464
	testPoint := geo.NewPoint(testLatitude, testLongitude)
	testPose := NewPoseFromPoint(r3.Vector{2, 3, 4})
	testSphere, err := NewSphere(testPose, 100, "sphere")
	test.That(t, err, test.ShouldBeNil)
	testGeoms := []Geometry{testSphere}

	testGeoObst := NewGeoObstacle(testPoint, testGeoms)
	test.That(t, testPoint, test.ShouldResemble, testGeoObst.Location())
	test.That(t, testGeoms, test.ShouldResemble, testGeoObst.Geometries())

	t.Run("Conversion from GeoObstacle to Protobuf", func(t *testing.T) {
		convGeoObstProto := GeoObstacleToProtobuf(testGeoObst)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testPoint.Lat(), test.ShouldEqual, convGeoObstProto.GetLocation().GetLatitude())
		test.That(t, testPoint.Lng(), test.ShouldEqual, convGeoObstProto.GetLocation().GetLongitude())
		test.That(t, len(testGeoms), test.ShouldEqual, len(convGeoObstProto.GetGeometries()))
	})

	t.Run("Conversion from Protobuf to GeoObstacle", func(t *testing.T) {
		testProtobuf := &commonpb.GeoObstacle{
			Location:   &commonpb.GeoPoint{Latitude: testLatitude, Longitude: testLongitude},
			Geometries: []*commonpb.Geometry{testSphere.ToProtobuf()},
		}

		convGeoObst, err := GeoObstacleFromProtobuf(testProtobuf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testPoint, test.ShouldResemble, convGeoObst.Location())
		test.That(t, testGeoms, test.ShouldResemble, convGeoObst.Geometries())
	})

	// test forward and backward conversion from GeoObstacleConfig to GeoObstacle
	gc, err := NewGeometryConfig(testSphere)
	test.That(t, err, test.ShouldBeNil)

	gobCfg := GeoObstacleConfig{
		Location:   &commonpb.GeoPoint{Latitude: testLatitude, Longitude: testLongitude},
		Geometries: []*GeometryConfig{gc},
	}

	t.Run("Conversion from GeoObstacle to GeoObstacleConfig", func(t *testing.T) {
		conv, err := NewGeoObstacleConfig(testGeoObst)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, testPoint.Lat(), test.ShouldEqual, conv.Location.Latitude)
		test.That(t, testPoint.Lng(), test.ShouldEqual, conv.Location.Longitude)
		test.That(t, conv.Geometries, test.ShouldResemble, []*GeometryConfig{gc})
	})

	t.Run("Conversion from GeoObstacleConfig to GeoObstacle", func(t *testing.T) {
		conv, err := GeoObstaclesFromConfig(&gobCfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(conv), test.ShouldEqual, 1)
		test.That(t, conv[0].location, test.ShouldResemble, testGeoObst.location)
		test.That(t, conv[0].geometries, test.ShouldResemble, testGeoObst.geometries)
	})
}

func TestConvertGeoPointToPose(t *testing.T) {
	gp := geo.NewPoint(0, 0)
	pose := GeoPointToPose(gp)
	test.That(t, R3VectorAlmostEqual(pose.Point(), r3.Vector{0, 0, 0}, 0.1), test.ShouldBeTrue)

	gp = geo.NewPoint(0.0000009, 0.0000009)
	pose = GeoPointToPose(gp)
	test.That(t, R3VectorAlmostEqual(pose.Point(), r3.Vector{100, 100, 0}, 0.1), test.ShouldBeTrue)

	gp = geo.NewPoint(0, 0.0000009)
	pose = GeoPointToPose(gp)
	test.That(t, R3VectorAlmostEqual(pose.Point(), r3.Vector{100, 0, 0}, 0.1), test.ShouldBeTrue)

	gp = geo.NewPoint(-0.0000009, 0.0000009)
	pose = GeoPointToPose(gp)
	test.That(t, R3VectorAlmostEqual(pose.Point(), r3.Vector{100, -100, 0}, 0.1), test.ShouldBeTrue)

	gp = geo.NewPoint(-0.0000009, 0)
	pose = GeoPointToPose(gp)
	test.That(t, R3VectorAlmostEqual(pose.Point(), r3.Vector{0, -100, 0}, 0.1), test.ShouldBeTrue)

	gp = geo.NewPoint(-0.0000009, -0.0000009)
	pose = GeoPointToPose(gp)
	test.That(t, R3VectorAlmostEqual(pose.Point(), r3.Vector{-100, -100, 0}, 0.1), test.ShouldBeTrue)

	gp = geo.NewPoint(0, -0.0000009)
	pose = GeoPointToPose(gp)
	test.That(t, R3VectorAlmostEqual(pose.Point(), r3.Vector{-100, 0, 0}, 0.1), test.ShouldBeTrue)
}

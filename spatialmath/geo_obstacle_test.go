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
		convGeoObstProto, err := GeoObstacleToProtobuf(testGeoObst)
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
	gob := NewGeoObstacle(testPoint, []Geometry{testSphere})

	gobCfg := GeoObstacleConfig{
		Location:   &commonpb.GeoPoint{Latitude: testLatitude, Longitude: testLongitude},
		Geometries: []*commonpb.Geometry{testSphere.ToProtobuf()},
	}

	t.Run("Conversion from GeoObstacle to GeoObstacleConfig", func(t *testing.T) {
		conv, err := NewGeoObstacleConfig(gob)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, testPoint.Lat(), test.ShouldEqual, conv.Location.Latitude)
		test.That(t, testPoint.Lng(), test.ShouldEqual, conv.Location.Longitude)
		test.That(t, conv.Geometries, test.ShouldResemble, []*commonpb.Geometry{testSphere.ToProtobuf()})
	})

	t.Run("Conversion from GeoObstacleConfig to GeoObstacle", func(t *testing.T) {
		conv, err := GeoObstaclesFromConfig(&gobCfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(conv), test.ShouldEqual, 1)
		test.That(t, conv[0].location, test.ShouldResemble, gob.location)
		test.That(t, conv[0].geometries, test.ShouldResemble, gob.geometries)
	})

	t.Run("Conversion from GeoObstacle to Geometry", func(t *testing.T) {
		geoms := GeoObstaclesToGeometries([]*GeoObstacle{gob})
		test.That(t, len(geoms), test.ShouldEqual, 1)
		test.That(t, geoms[0].Pose(), test.ShouldResemble, Compose(GeoPointToPose(testPoint), testPose))
	})
}

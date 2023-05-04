package referenceframe

import (
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/spatialmath"
)

func TestGeoObstacles(t *testing.T) {
	testLatitude := 39.58836
	testLongitude := -105.64464
	testPoint := geo.NewPoint(testLatitude, testLongitude)
	testSphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 100, "sphere")
	test.That(t, err, test.ShouldBeNil)
	testGeoms := []spatialmath.Geometry{testSphere}

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
}

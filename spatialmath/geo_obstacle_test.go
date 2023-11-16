package spatialmath

import (
	"fmt"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
)

func TestGeoPose(t *testing.T) {
	origin := geo.NewPoint(0, 0)
	testCases := []struct {
		*geo.Point
		r3.Vector
	}{
		{geo.NewPoint(9e-9, 9e-9), r3.Vector{1, 1, 0}},
		{geo.NewPoint(0, 9e-9), r3.Vector{1, 0, 0}},
		{geo.NewPoint(-9e-9, 9e-9), r3.Vector{1, -1, 0}},
		{geo.NewPoint(9e-9, 0), r3.Vector{0, 1, 0}},
		{geo.NewPoint(0, 0), r3.Vector{0, 0, 0}},
		{geo.NewPoint(-9e-9, -9e-9), r3.Vector{-1, -1, 0}},
		{geo.NewPoint(0, -9e-9), r3.Vector{-1, 0, 0}},
		{geo.NewPoint(9e-9, -9e-9), r3.Vector{-1, 1, 0}},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			point := GeoPointToPoint(tc.Point, origin)
			test.That(t, R3VectorAlmostEqual(point, tc.Vector, 0.1), test.ShouldBeTrue)
		})
	}
}

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

func TestPoseToGeoPoint(t *testing.T) {
	type testCase struct {
		msg             string
		relativeTo      GeoPose
		p               Pose
		expectedGeoPose GeoPose
	}
	mmToMoveOneDegree := 1.1119492664455873e+08

	// east := &OrientationVectorDegrees{OZ: 1, Theta: 270}
	// northeast := &OrientationVectorDegrees{OZ: 1, Theta: 315}
	// west := &OrientationVectorDegrees{OZ: 1, Theta: 90}
	// south := &OrientationVectorDegrees{OZ: 1, Theta: 180}

	tcs := []testCase{
		// {
		// 	msg:             "zero geopose & pose outputs zero geopose",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewZeroPose(),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 0), 0),
		// },
		// {
		// 	msg:             "zero geopoint with non zero heading & zero pose outputs same geopose",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 90),
		// 	p:               NewZeroPose(),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 0), 90),
		// },
		// {
		// 	msg:             "zero geopose with pose that turns east results in zero geopoint heading east",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewPose(r3.Vector{}, east),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 0), 90),
		// },
		// {
		// 	msg:             "nonzero geopose with pose that turns west results in same geopoint heading west",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(50, 50), 0),
		// 	p:               NewPose(r3.Vector{}, west),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(50, 50), 270),
		// },
		// {
		// 	msg:             "nonzero geopose facing west with pose that turns east results in same geopoint heading north",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(50, 50), 270),
		// 	p:               NewPose(r3.Vector{}, east),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(50, 50), 0),
		// },
		// {
		// 	msg:             "non zero geopose & zero pose outputs same non zero geopose",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(40.770301, -73.977308), 90),
		// 	p:               NewZeroPose(),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(40.770301, -73.977308), 90),
		// },
		// {
		// 	msg:             "zero geopose & pose that moves one lat degree north outputs +1 lat degree diff geopose",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewPose(r3.Vector{X: 0, Y: mmToMoveOneDegree, Z: 0}, NewZeroOrientation()),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(1, 0), 0),
		// },
		// {
		// 	msg:             "zero geopose & pose that moves one lng degree outputs 1 lat degree diff geopose",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewPose(r3.Vector{X: mmToMoveOneDegree, Y: 0, Z: 0}, NewZeroOrientation()),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 1), 0),
		// },
		// {
		// 	msg:             "zero geopose & pose that moves 10 lat degrees north outputs +10 lat degree diff geopose",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewPose(r3.Vector{X: 0, Y: mmToMoveOneDegree * 10, Z: 0}, NewZeroOrientation()),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(10, 0), 0),
		// },
		// {
		// 	msg:             "zero geopose & pose that moves 10 lng degrees east outputs +10 lng degree diff geopose",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewPose(r3.Vector{X: mmToMoveOneDegree * 10, Y: 0, Z: 0}, NewZeroOrientation()),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 10), 0),
		// },
		// {
		// 	msg:             "zero geopose & a pose that moves 1 lat degree north with a south orientation outputs +1 lat degree diff geopose facing south",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewPose(r3.Vector{X: 0, Y: mmToMoveOneDegree, Z: 0}, south),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(1, 0), 180),
		// },
		// {
		// 	msg:             "zero geopose & a pose that moves 1 lat degree south with an east orientation outputs -1 lat degree diff geopose facing east",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 0),
		// 	p:               NewPose(r3.Vector{X: 0, Y: -mmToMoveOneDegree, Z: 0}, east),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(-1, 0), 90),
		// },
		// {
		// 	msg:             "zero geopose heading south & a pose that rotates east, outputs zero geopose facing west",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 180),
		// 	p:               NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, east),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 0), 270),
		// },
		// {
		// 	msg:             "zero geopose heading south & a pose that rotates east, outputs zero geopose facing west even when 360 is added multiple times",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 180+360+360+360+360),
		// 	p:               NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, east),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 0), 270),
		// },
		// {
		// 	msg:             "zero geopose heading south & a pose that rotates east, outputs zero geopose facing west",
		// 	relativeTo:      *NewGeoPose(geo.NewPoint(0, 0), 180-360-360-360-360),
		// 	p:               NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, east),
		// 	expectedGeoPose: *NewGeoPose(geo.NewPoint(0, 0), 270),
		// },
		{
			msg:             "1,5 lat long facing west & a pose that rotates northeast & translates 2 lat deg & 3 lng degrees arrives at 3,8 facing north west",
			relativeTo:      *NewGeoPose(geo.NewPoint(1, 5), 270-360-360-360-360),
			p:               NewPose(r3.Vector{X: mmToMoveOneDegree, Y: mmToMoveOneDegree, Z: 0}, NewZeroOrientation()),
			expectedGeoPose: *NewGeoPose(geo.NewPoint(1.9997968273479143, 3.9994413235922375), 270),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.msg, func(t *testing.T) {
			gp := PoseToGeoPose(tc.relativeTo, tc.p)
			t.Logf("gp: %#v %#v\n", gp.Location(), gp.Heading())
			test.That(t, gp.Heading(), test.ShouldAlmostEqual, tc.expectedGeoPose.Heading())
			test.That(t, gp.Location().Lat(), test.ShouldAlmostEqual, tc.expectedGeoPose.Location().Lat())
			test.That(t, gp.Location().Lng(), test.ShouldAlmostEqual, tc.expectedGeoPose.Location().Lng())
			geoPointToPose := GeoPoseToPose(gp, tc.relativeTo)
			t.Logf("geoPointToPose.Point(): %#v, geoPointToPose.Orientation().OrientationVectorDegrees().: %#v\n", geoPointToPose.Point(), geoPointToPose.Orientation().OrientationVectorDegrees())
			t.Logf("tc.p.Point(): %#v tc.p.Orientation().OrientationVectorDegrees().: %#v\n", tc.p.Point(), tc.p.Orientation().OrientationVectorDegrees())
			test.That(t, PoseAlmostEqualEps(geoPointToPose, tc.p, 1), test.ShouldBeTrue)
		})
	}
}

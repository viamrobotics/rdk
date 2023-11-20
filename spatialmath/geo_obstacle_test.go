package spatialmath

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestGeoPointToPoint(t *testing.T) {
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

func TestPoseToGeoPose(t *testing.T) {
	type testCase struct {
		name                        string
		relativeTo, expectedGeoPose *GeoPose
		pose                        Pose
	}

	// degree of accuracy to expect on results
	mmTol := 1e-3
	gpsTol := 1e-6

	// The number of mm required to move one one thousandth of a degree long or lat from the GPS point (0, 0)
	mmToOneThousandthDegree := 1.1119492664455873e+05

	// values are right handed - north is 0 degrees
	LHNortheast := 45.
	LHEast := 90.
	LHSouth := 180.
	LHWest := 270.
	LHNorthwest := 315.

	// values are right handed - north is 0 degrees
	RHNortheast := 360 - LHNortheast
	RHEast := 360 - LHEast
	RHSouth := 360 - LHSouth
	RHWest := 360 - LHWest

	tcs := []testCase{
		{
			name:            "zero geopose + zero pose = zero geopose",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), 0),
			pose:            NewZeroPose(),
			expectedGeoPose: NewGeoPose(geo.NewPoint(0, 0), 0),
		},
		{
			name:            "zero geopose heading west + zero pose = original geopose",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), LHWest),
			pose:            NewZeroPose(),
			expectedGeoPose: NewGeoPose(geo.NewPoint(0, 0), LHWest),
		},
		{
			name:            "zero geopose + pose heading east = zero geopoint heading east",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), 0),
			pose:            NewPose(r3.Vector{}, &OrientationVectorDegrees{OZ: 1, Theta: RHEast}),
			expectedGeoPose: NewGeoPose(geo.NewPoint(0, 0), LHEast),
		},
		{
			name:            "nonzero geopose + pose heading west = in same geopoint heading west",
			relativeTo:      NewGeoPose(geo.NewPoint(50, 50), 0),
			pose:            NewPose(r3.Vector{}, &OrientationVectorDegrees{OZ: 1, Theta: RHWest}),
			expectedGeoPose: NewGeoPose(geo.NewPoint(50, 50), LHWest),
		},
		{
			name:            "nonzero geopose heading west + pose heading east = same geopoint heading north",
			relativeTo:      NewGeoPose(geo.NewPoint(50, 50), LHWest),
			pose:            NewPose(r3.Vector{}, &OrientationVectorDegrees{OZ: 1, Theta: RHEast}),
			expectedGeoPose: NewGeoPose(geo.NewPoint(50, 50), 0),
		},
		{
			name:            "zero geopose + pose that moves 0.001 degree north = 0.001 degree diff geopose",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), 0),
			pose:            NewPose(r3.Vector{X: 0, Y: mmToOneThousandthDegree, Z: 0}, NewZeroOrientation()),
			expectedGeoPose: NewGeoPose(geo.NewPoint(1e-3, 0), 0),
		},
		{
			name:            "zero geopose + pose that moves 0.001 degree east = 0.001 degree diff geopose",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), 0),
			pose:            NewPose(r3.Vector{X: mmToOneThousandthDegree, Y: 0, Z: 0}, NewZeroOrientation()),
			expectedGeoPose: NewGeoPose(geo.NewPoint(0, 1e-3), 0),
		},
		{
			name: "zero geopose + pose that moves 0.001 lat degree north with a south orientation = " +
				"0.001 lat degree diff geopose facing south",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), 0),
			pose:            NewPose(r3.Vector{X: 0, Y: mmToOneThousandthDegree, Z: 0}, &OrientationVectorDegrees{OZ: 1, Theta: RHSouth}),
			expectedGeoPose: NewGeoPose(geo.NewPoint(1e-3, 0), LHSouth),
		},
		{
			name: "zero geopose + pose that moves 0.001 lat degree south with an east orientation = " +
				"0.001 lat degree diff geopose facing east",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), 0),
			pose:            NewPose(r3.Vector{X: 0, Y: -mmToOneThousandthDegree, Z: 0}, &OrientationVectorDegrees{OZ: 1, Theta: RHEast}),
			expectedGeoPose: NewGeoPose(geo.NewPoint(-1e-3, 0), LHEast),
		},
		{
			name: "zero geopose heading south + pose that rotates east = zero geopose facing west" +
				"even when 360 is added multiple times",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), LHSouth+360+360+360+360),
			pose:            NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, &OrientationVectorDegrees{OZ: 1, Theta: RHEast}),
			expectedGeoPose: NewGeoPose(geo.NewPoint(0, 0), LHWest),
		},
		{
			name:            "zero geopose heading south + pose that rotates east = zero geopose facing west",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), LHSouth-360-360-360-360),
			pose:            NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, &OrientationVectorDegrees{OZ: 1, Theta: LHEast}),
			expectedGeoPose: NewGeoPose(geo.NewPoint(0, 0), LHWest),
		},
		{
			name:       "zero geopose heading northwest + pose that rotates northeast",
			relativeTo: NewGeoPose(geo.NewPoint(0, 0), LHNorthwest),
			pose: NewPose(
				r3.Vector{X: mmToOneThousandthDegree, Y: mmToOneThousandthDegree, Z: 0},
				&OrientationVectorDegrees{OZ: 1, Theta: RHNortheast},
			),
			expectedGeoPose: NewGeoPose(geo.NewPoint(0, math.Sqrt2*1e-3), 0),
		},
		{
			name:       "zero geopose heading north + pose that rotates northeast",
			relativeTo: NewGeoPose(geo.NewPoint(0, 0), 0),
			pose: NewPose(
				r3.Vector{X: mmToOneThousandthDegree, Y: mmToOneThousandthDegree, Z: 0},
				&OrientationVectorDegrees{OZ: 1, Theta: RHNortheast},
			),
			expectedGeoPose: NewGeoPose(geo.NewPoint(1e-3, 1e-3), LHNortheast),
		},
		{
			name:            "zero geopose heading east + pose that rotates north",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), LHWest),
			pose:            NewPose(r3.Vector{X: mmToOneThousandthDegree, Y: mmToOneThousandthDegree, Z: 0}, NewZeroOrientation()),
			expectedGeoPose: NewGeoPose(geo.NewPoint(-1e-3, 1e-3), LHWest),
		},
		{
			name:            "zero geopose heading east",
			relativeTo:      NewGeoPose(geo.NewPoint(1e-3, 5e-3), LHEast),
			pose:            NewPose(r3.Vector{X: mmToOneThousandthDegree, Y: mmToOneThousandthDegree, Z: 0}, NewZeroOrientation()),
			expectedGeoPose: NewGeoPose(geo.NewPoint(2e-3, 4e-3), LHEast),
		},
		{
			name:            "zero geopose heading west",
			relativeTo:      NewGeoPose(geo.NewPoint(0, 0), LHWest),
			pose:            NewPose(r3.Vector{X: mmToOneThousandthDegree, Y: mmToOneThousandthDegree, Z: 0}, NewZeroOrientation()),
			expectedGeoPose: NewGeoPose(geo.NewPoint(-1e-3, 1e-3), LHWest),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			gp := PoseToGeoPose(*tc.relativeTo, tc.pose)
			t.Logf("gp: %#v %#v\n", gp.Location(), gp.Heading())
			t.Logf("tc: %#v %#v\n", tc.expectedGeoPose.Location(), tc.expectedGeoPose.Heading())
			test.That(t, gp.Heading(), test.ShouldAlmostEqual, tc.expectedGeoPose.Heading())
			test.That(t, utils.Float64AlmostEqual(gp.Location().Lat(), tc.expectedGeoPose.Location().Lat(), gpsTol), test.ShouldBeTrue)
			test.That(t, utils.Float64AlmostEqual(gp.Location().Lng(), tc.expectedGeoPose.Location().Lng(), gpsTol), test.ShouldBeTrue)
			geoPointToPose := GeoPoseToPose(gp, *tc.relativeTo)
			msga := "geoPointToPose.Point(): %#v, geoPointToPose.Orientation().OrientationVectorDegrees().: %#v\n"
			t.Logf(msga, geoPointToPose.Point(), geoPointToPose.Orientation().OrientationVectorDegrees())
			msgb := "tc.p.Point(): %#v tc.p.Orientation().OrientationVectorDegrees().: %#v\n"
			t.Logf(msgb, tc.pose.Point(), tc.pose.Orientation().OrientationVectorDegrees())
			test.That(t, PoseAlmostEqualEps(geoPointToPose, tc.pose, mmTol), test.ShouldBeTrue)
		})
	}
}

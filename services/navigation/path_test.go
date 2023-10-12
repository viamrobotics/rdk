package navigation_test

import (
	"errors"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/services/navigation"
)

func TestPaths(t *testing.T) {
	// create valid path
	path, err := navigation.NewPath("test", []*geo.Point{geo.NewPoint(0, 0)})
	test.That(t, err, test.ShouldBeNil)

	// create valid pb.path
	pbPath := &pb.Path{
		DestinationWaypointId: "test",
		Geopoints:             []*commonpb.GeoPoint{{Latitude: 0, Longitude: 0}},
	}

	// test converting path to pb.path
	convertedPath, err := navigation.PathToProto(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convertedPath, test.ShouldResemble, pbPath)

	// test converting pb.path to path
	convertedProtoPath, err := navigation.ProtoToPath(pbPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convertedProtoPath, test.ShouldResemble, path)

	// test creating invalid path
	shouldBeNil, err := navigation.NewPath("test", nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, shouldBeNil, test.ShouldBeNil)

	// test converting slice of malformed pbPath to slice of path
	malformedPbSlice := []*pb.Path{
		{
			DestinationWaypointId: "malformed",
			Geopoints:             nil,
		},
		nil,
	}
	_, err = navigation.ProtoSliceToPaths(malformedPbSlice)
	test.That(t, err, test.ShouldBeError, errors.New("cannot convert nil path"))
}

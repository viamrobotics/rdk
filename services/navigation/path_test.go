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
	t.Parallel()
	t.Run("convert to and from proto", func(t *testing.T) {
		t.Parallel()
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
	})

	t.Run("creating invalid path", func(t *testing.T) {
		t.Parallel()
		shouldBeNil, err := navigation.NewPath("test", nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, shouldBeNil, test.ShouldBeNil)
	})

	t.Run("converting slice of nil pbPath to slice of path", func(t *testing.T) {
		t.Parallel()
		nilSlice := []*pb.Path{nil}
		_, err := navigation.ProtoSliceToPaths(nilSlice)
		test.That(t, err, test.ShouldBeError, errors.New("cannot convert nil path"))
	})

	t.Run("converting slice of pb path with nil geoPoints", func(t *testing.T) {
		t.Parallel()
		malformedPath := []*pb.Path{
			{
				DestinationWaypointId: "malformed",
				Geopoints:             nil,
			},
		}
		_, err := navigation.ProtoSliceToPaths(malformedPath)
		test.That(t, err, test.ShouldBeError, errors.New("cannot instantiate path with no geoPoints"))
	})

	t.Run("converting slice of pb path with nil id", func(t *testing.T) {
		t.Parallel()
		malformedPath := []*pb.Path{
			{
				DestinationWaypointId: "",
				Geopoints:             []*commonpb.GeoPoint{{Latitude: 0, Longitude: 0}},
			},
		}
		_, err := navigation.ProtoSliceToPaths(malformedPath)
		test.That(t, err, test.ShouldBeError, errors.New("cannot instantiate path with no destinationWaypointID"))
	})
}

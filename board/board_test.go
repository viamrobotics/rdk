package board

import (
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/test"
)

func TestFlipDirection(t *testing.T) {
	test.That(t, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED)
	test.That(t, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	test.That(t, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
}

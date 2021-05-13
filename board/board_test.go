package board

import (
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/core/proto/api/v1"
)

func TestFlipDirection(t *testing.T) {
	test.That(t, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED)
	test.That(t, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	test.That(t, FlipDirection(pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
}

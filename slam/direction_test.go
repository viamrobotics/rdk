package slam

import (
	"fmt"
	"testing"

	pb "go.viam.com/core/proto/slam/v1"

	"go.viam.com/test"
)

func TestDirectionFromXY(t *testing.T) {
	for i, tc := range []struct {
		x, y                  int
		viewWidth, viewHeight int
		expected              pb.Direction
	}{
		{0, 0, 0, 0, pb.Direction_DIRECTION_RIGHT}, // bogus for views with area < 0
		{0, 0, 2, 2, pb.Direction_DIRECTION_UP},
		{1, 0, 2, 2, pb.Direction_DIRECTION_DOWN},
		{0, 1, 2, 2, pb.Direction_DIRECTION_LEFT},
		{1, 1, 2, 2, pb.Direction_DIRECTION_RIGHT},
		{200, 200, 800, 600, pb.Direction_DIRECTION_UP},
		{700, 200, 800, 600, pb.Direction_DIRECTION_DOWN},
		{5, 301, 800, 600, pb.Direction_DIRECTION_LEFT},
		{450, 301, 800, 600, pb.Direction_DIRECTION_RIGHT},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			dir := DirectionFromXY(tc.x, tc.y, tc.viewWidth, tc.viewHeight)
			test.That(t, dir, test.ShouldEqual, tc.expected)
		})
	}
}

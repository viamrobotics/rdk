package slam

import (
	"fmt"
	"testing"

	"github.com/edaniels/test"
)

func TestDirectionFromXY(t *testing.T) {
	for i, tc := range []struct {
		x, y                  int
		viewWidth, viewHeight int
		expected              Direction
	}{
		{0, 0, 0, 0, DirectionRight}, // bogus for views with area < 0
		{0, 0, 2, 2, DirectionUp},
		{1, 0, 2, 2, DirectionDown},
		{0, 1, 2, 2, DirectionLeft},
		{1, 1, 2, 2, DirectionRight},
		{200, 200, 800, 600, DirectionUp},
		{700, 200, 800, 600, DirectionDown},
		{5, 301, 800, 600, DirectionLeft},
		{450, 301, 800, 600, DirectionRight},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			dir := DirectionFromXY(tc.x, tc.y, tc.viewWidth, tc.viewHeight)
			test.That(t, dir, test.ShouldEqual, tc.expected)
		})
	}
}

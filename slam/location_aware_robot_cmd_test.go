package slam

import (
	"errors"
	"fmt"
	"testing"

	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
)

func TestHandleClick(t *testing.T) {
	t.Run("unknown click mode", func(t *testing.T) {
		harness := newTestHarness(t)
		larBot := harness.bot
		larBot.clientClickMode = "who"
		_, err := larBot.HandleClick(0, 0, 10, 10)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
	})

	t.Run("move mode", func(t *testing.T) {
		harness := newTestHarness(t)
		larBot := harness.bot
		larBot.clientClickMode = clientClickModeMove
		injectBase := &inject.Base{Device: larBot.baseDevice}
		larBot.baseDevice = injectBase
		err1 := errors.New("whoops")
		injectBase.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
			return err1
		}
		_, err := larBot.HandleClick(1, 2, 3, 4)
		test.That(t, err, test.ShouldWrap, err1)

		for i, tc := range []struct {
			x, y                  int
			viewWidth, viewHeight int
			expectedDir           Direction
			expectedX             int
			expectedY             int
		}{
			{0, 0, 0, 0, DirectionRight, 70, 50}, // bogus for views with area < 0
			{0, 0, 2, 2, DirectionUp, 50, 30},
			{1, 0, 2, 2, DirectionDown, 50, 70},
			{0, 1, 2, 2, DirectionLeft, 30, 50},
		} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				harness := newTestHarness(t)
				larBot := harness.bot
				larBot.clientClickMode = clientClickModeMove
				ret, err := larBot.HandleClick(tc.x, tc.y, tc.viewWidth, tc.viewHeight)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, ret, test.ShouldContainSubstring, fmt.Sprintf("(%d, %d)", tc.expectedX, tc.expectedY))
				test.That(t, ret, test.ShouldContainSubstring, string(tc.expectedDir))
			})
		}
	})
}

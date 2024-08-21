package board_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
)

func TestValidatePWMDutyCycle(t *testing.T) {
	// Normal values are unchanged
	val, err := board.ValidatePWMDutyCycle(0.5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, val, test.ShouldEqual, 0.5)

	// No negative values
	val, err = board.ValidatePWMDutyCycle(-1.0)
	test.That(t, err, test.ShouldNotBeNil)

	// Values slightly over 100% get rounded down
	val, err = board.ValidatePWMDutyCycle(1.005)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, val, test.ShouldEqual, 1.0)

	// No values well over 100%
	val, err = board.ValidatePWMDutyCycle(2.0)
	test.That(t, err, test.ShouldNotBeNil)
}

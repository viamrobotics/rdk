package builtin

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestTeleopMoveOptions_NilWhenNoCaps(t *testing.T) {
	test.That(t, teleopMoveOptions(nil), test.ShouldBeNil)
	test.That(t, teleopMoveOptions(&Config{}), test.ShouldBeNil)
}

func TestTeleopMoveOptions_VelAndAccConvertDegsToRads(t *testing.T) {
	opts := teleopMoveOptions(&Config{
		TeleopMaxVelDegsPerSec:        90,
		TeleopMaxAccDegsPerSecSquared: 180,
	})
	test.That(t, opts, test.ShouldNotBeNil)
	test.That(t, opts.MaxVelRads, test.ShouldAlmostEqual, math.Pi/2, 1e-9)
	test.That(t, opts.MaxAccRads, test.ShouldAlmostEqual, math.Pi, 1e-9)
}

func TestConfigValidate_RejectsNegativeCaps(t *testing.T) {
	cases := []Config{
		{TeleopMaxVelDegsPerSec: -1},
		{TeleopMaxAccDegsPerSecSquared: -1},
	}
	for _, c := range cases {
		_, _, err := c.Validate("/foo")
		test.That(t, err, test.ShouldNotBeNil)
	}
}

func TestConfigValidate_AcceptsZeroAndPositiveCaps(t *testing.T) {
	_, _, err := (&Config{}).Validate("/foo")
	test.That(t, err, test.ShouldBeNil)
	_, _, err = (&Config{
		TeleopMaxVelDegsPerSec:        90,
		TeleopMaxAccDegsPerSecSquared: 180,
	}).Validate("/foo")
	test.That(t, err, test.ShouldBeNil)
}

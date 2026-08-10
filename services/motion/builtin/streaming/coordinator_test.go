// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"testing"

	"go.viam.com/test"
)

func TestStreamOptionsDefaultsAndValidate(t *testing.T) {
	c := NewDefaultOptions()
	test.That(t, c.TargetRunwayInArmMs, test.ShouldEqual, defaultTargetRunwayInArmMs)
	test.That(t, c.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, c.VelLimitDegPerSec, test.ShouldEqual, defaultVelLimitDegPerSec)
	test.That(t, c.AccelLimitDegPerSec2, test.ShouldEqual, defaultAccelLimitDegPerSec2)
	test.That(t, c.Validate(), test.ShouldBeNil)

	test.That(t, (&StreamOptions{SendToArmIntervalMs: 10, TargetRunwayInArmMs: -1}).Validate(), test.ShouldNotBeNil)
	// A zero send interval is invalid (division by zero when converting to Hz).
	test.That(t, (&StreamOptions{SendToArmIntervalMs: 0, TargetRunwayInArmMs: 10}).Validate(), test.ShouldNotBeNil)
	test.That(t, (&StreamOptions{SendToArmIntervalMs: 10, VelLimitDegPerSec: -1}).Validate(), test.ShouldNotBeNil)
}

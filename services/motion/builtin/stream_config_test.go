package builtin

import (
	"testing"

	"go.viam.com/test"
)

func TestStreamConfigApplyDefaultsAndValidate(t *testing.T) {
	c := streamConfig{}
	c.ApplyDefaults()
	test.That(t, c.BufferAheadInArmMs, test.ShouldEqual, defaultBufferAheadInArmMs)
	test.That(t, c.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, c.VelLimitDegPerSec, test.ShouldEqual, defaultVelLimitDegPerSec)
	test.That(t, c.AccelLimitDegPerSec2, test.ShouldEqual, defaultAccelLimitDegPerSec2)
	test.That(t, c.Validate(), test.ShouldBeNil)

	test.That(t, (&streamConfig{SendToArmIntervalMs: 10, BufferAheadInArmMs: -1}).Validate(), test.ShouldNotBeNil)
	// A zero send interval is invalid (division by zero when converting to Hz).
	test.That(t, (&streamConfig{SendToArmIntervalMs: 0, BufferAheadInArmMs: 10}).Validate(), test.ShouldNotBeNil)
	test.That(t, (&streamConfig{SendToArmIntervalMs: 10, VelLimitDegPerSec: -1}).Validate(), test.ShouldNotBeNil)
}

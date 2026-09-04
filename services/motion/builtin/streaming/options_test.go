// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"testing"

	"go.viam.com/test"
)

func TestStreamOptionsDefaultsAndValidate(t *testing.T) {
	valid := NewDefaultOptions()
	test.That(t, valid.TargetRunwayInArmMs, test.ShouldEqual, defaultTargetRunwayInArmMs)
	test.That(t, valid.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, valid.VelLimitDegPerSec, test.ShouldEqual, defaultVelLimitDegPerSec)
	test.That(t, valid.AccelLimitDegPerSec2, test.ShouldEqual, defaultAccelLimitDegPerSec2)
	test.That(t, valid.MaxTrajexRunwayMs, test.ShouldEqual, 0)
	test.That(t, valid.Validate(), test.ShouldBeNil)

	// Backpressure is opt-in; a positive cap also validates.
	withCap := valid
	withCap.MaxTrajexRunwayMs = 200
	test.That(t, withCap.Validate(), test.ShouldBeNil)

	// The zero value does not validate.
	test.That(t, (&StreamOptions{}).Validate(), test.ShouldNotBeNil)

	// Each Validate rule, violated in isolation against the otherwise-valid base.
	for _, tc := range []struct {
		name   string
		mutate func(*StreamOptions)
	}{
		{"zero runway", func(o *StreamOptions) { o.TargetRunwayInArmMs = 0 }},
		{"negative runway", func(o *StreamOptions) { o.TargetRunwayInArmMs = -1 }},
		{"zero send interval", func(o *StreamOptions) { o.SendToArmIntervalMs = 0 }},
		{"negative send interval", func(o *StreamOptions) { o.SendToArmIntervalMs = -1 }},
		{"send interval not less than runway", func(o *StreamOptions) { o.SendToArmIntervalMs = o.TargetRunwayInArmMs }},
		{"zero vel limit", func(o *StreamOptions) { o.VelLimitDegPerSec = 0 }},
		{"negative vel limit", func(o *StreamOptions) { o.VelLimitDegPerSec = -1 }},
		{"zero accel limit", func(o *StreamOptions) { o.AccelLimitDegPerSec2 = 0 }},
		{"negative accel limit", func(o *StreamOptions) { o.AccelLimitDegPerSec2 = -1 }},
		{"negative max trajex runway", func(o *StreamOptions) { o.MaxTrajexRunwayMs = -1 }},
	} {
		bad := valid
		tc.mutate(&bad)
		test.That(t, bad.Validate(), test.ShouldNotBeNil)
	}
}

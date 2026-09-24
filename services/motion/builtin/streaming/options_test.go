// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/services/motion"
)

func TestNewStreamOptionsOverrides(t *testing.T) {
	runway, window := int32(80), int32(0)
	opts := NewStreamOptions(motion.TempStreamOptions{
		ArmSideTargetRunwayMs: &runway,
		DiagnosticsWindowSecs: &window,
		MoveOptions:           &arm.MoveOptions{MaxAccRads: 2, MaxVelRadsJoints: []float64{1, 2}},
	})
	test.That(t, opts.ArmSideTargetRunwayMs, test.ShouldEqual, 80)
	test.That(t, opts.DiagnosticsWindowSecs, test.ShouldEqual, 0)
	test.That(t, opts.MoveOptions.MaxAccRads, test.ShouldEqual, 2.0)
	test.That(t, opts.MoveOptions.MaxVelRadsJoints, test.ShouldResemble, []float64{1, 2})
	// nil arguments and zero scalar limits keep the defaults.
	test.That(t, opts.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, opts.MoveOptions.MaxVelRads, test.ShouldEqual, defaultVelLimitRadPerSec)
}

func TestStreamOptionsDefaultsAndValidate(t *testing.T) {
	valid := NewStreamOptions(motion.TempStreamOptions{})
	test.That(t, valid.ArmSideTargetRunwayMs, test.ShouldEqual, defaultArmSideTargetRunwayMs)
	test.That(t, valid.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, valid.MoveOptions.MaxVelRads, test.ShouldEqual, defaultVelLimitRadPerSec)
	test.That(t, valid.MoveOptions.MaxAccRads, test.ShouldEqual, defaultAccelLimitRadPerSec2)
	test.That(t, valid.DiagnosticsWindowSecs, test.ShouldEqual, defaultDiagnosticsWindowSecs)
	test.That(t, valid.Validate(), test.ShouldBeNil)

	// A zero diagnostics window is valid: it disables window-detail retention only.
	disabled := valid
	disabled.DiagnosticsWindowSecs = 0
	test.That(t, disabled.Validate(), test.ShouldBeNil)

	// The zero value does not validate.
	test.That(t, (&StreamOptions{}).Validate(), test.ShouldNotBeNil)

	perJoint := valid
	perJoint.MoveOptions.MaxVelRads = 0
	perJoint.MoveOptions.MaxVelRadsJoints = []float64{1, 2}
	perJoint.MoveOptions.MaxAccRads = 0
	perJoint.MoveOptions.MaxAccRadsJoints = []float64{1, 2}
	test.That(t, perJoint.Validate(), test.ShouldBeNil)

	// Each Validate rule, violated in isolation against the otherwise-valid base.
	for _, tc := range []struct {
		name   string
		mutate func(*StreamOptions)
	}{
		{"zero runway", func(o *StreamOptions) { o.ArmSideTargetRunwayMs = 0 }},
		{"negative runway", func(o *StreamOptions) { o.ArmSideTargetRunwayMs = -1 }},
		{"zero send interval", func(o *StreamOptions) { o.SendToArmIntervalMs = 0 }},
		{"negative send interval", func(o *StreamOptions) { o.SendToArmIntervalMs = -1 }},
		{"send interval not less than runway", func(o *StreamOptions) { o.SendToArmIntervalMs = o.ArmSideTargetRunwayMs }},
		{"zero vel limit", func(o *StreamOptions) { o.MoveOptions.MaxVelRads = 0 }},
		{"negative vel limit", func(o *StreamOptions) { o.MoveOptions.MaxVelRads = -1 }},
		{"zero accel limit", func(o *StreamOptions) { o.MoveOptions.MaxAccRads = 0 }},
		{"negative accel limit", func(o *StreamOptions) { o.MoveOptions.MaxAccRads = -1 }},
		{"zero entry in per-joint vel limits", func(o *StreamOptions) { o.MoveOptions.MaxVelRadsJoints = []float64{1, 0} }},
		{"negative entry in per-joint vel limits", func(o *StreamOptions) { o.MoveOptions.MaxVelRadsJoints = []float64{1, -1} }},
		{"zero entry in per-joint accel limits", func(o *StreamOptions) { o.MoveOptions.MaxAccRadsJoints = []float64{1, 0} }},
		{"negative entry in per-joint accel limits", func(o *StreamOptions) { o.MoveOptions.MaxAccRadsJoints = []float64{1, -1} }},
		{"unsupported tcp speed limit", func(o *StreamOptions) { tcp := 0.1; o.MoveOptions.MaxTCPSpeedMPerSec = &tcp }},
		{"negative diagnostics window", func(o *StreamOptions) { o.DiagnosticsWindowSecs = -1 }},
	} {
		bad := valid
		tc.mutate(&bad)
		test.That(t, bad.Validate(), test.ShouldNotBeNil)
	}
}

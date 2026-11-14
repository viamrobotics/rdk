// Package streaming implements the ability to stream joint positions to an arm resource.
package streaming

import (
	"math"
	"testing"
	"time"

	"go.viam.com/test"
	vprotoutils "go.viam.com/utils/protoutils"

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
	test.That(t, opts.ArmSideTargetRunway, test.ShouldEqual, 80*time.Millisecond)
	test.That(t, opts.DiagnosticsWindow, test.ShouldEqual, 0)
	test.That(t, opts.MoveOptions.MaxAccRads, test.ShouldEqual, 2.0)
	test.That(t, opts.MoveOptions.MaxVelRadsJoints, test.ShouldResemble, []float64{1, 2})
	// nil arguments and zero scalar limits keep the defaults.
	test.That(t, opts.SendToArmInterval, test.ShouldEqual, defaultSendToArmInterval)
	test.That(t, opts.MoveOptions.MaxVelRads, test.ShouldEqual, defaultVelLimitRadPerSec)
}

func TestStreamOptionsDefaultsAndValidate(t *testing.T) {
	valid := NewStreamOptions(motion.TempStreamOptions{})
	test.That(t, valid.ArmSideTargetRunway, test.ShouldEqual, defaultArmSideTargetRunway)
	test.That(t, valid.SendToArmInterval, test.ShouldEqual, defaultSendToArmInterval)
	test.That(t, valid.MoveOptions.MaxVelRads, test.ShouldEqual, defaultVelLimitRadPerSec)
	test.That(t, valid.MoveOptions.MaxAccRads, test.ShouldEqual, defaultAccelLimitRadPerSec2)
	test.That(t, valid.DiagnosticsWindow, test.ShouldEqual, defaultDiagnosticsWindow)
	test.That(t, valid.Validate(), test.ShouldBeNil)

	// A zero diagnostics window is valid: it disables window-detail retention only.
	disabled := valid
	disabled.DiagnosticsWindow = 0
	test.That(t, disabled.Validate(), test.ShouldBeNil)

	// The zero value does not validate.
	test.That(t, (&StreamOptions{}).Validate(), test.ShouldNotBeNil)

	perJoint := valid
	perJoint.MoveOptions.MaxVelRads = 0
	perJoint.MoveOptions.MaxVelRadsJoints = []float64{1, 2}
	perJoint.MoveOptions.MaxAccRads = 0
	perJoint.MoveOptions.MaxAccRadsJoints = []float64{1, 2}
	test.That(t, perJoint.Validate(), test.ShouldBeNil)

	tcpCapped := valid
	tcp := 0.1
	tcpCapped.MoveOptions.MaxTCPSpeedMPerSec = &tcp
	test.That(t, tcpCapped.Validate(), test.ShouldBeNil)

	// Each Validate rule, violated in isolation against the otherwise-valid base.
	for _, tc := range []struct {
		name   string
		mutate func(*StreamOptions)
	}{
		{"zero runway", func(o *StreamOptions) { o.ArmSideTargetRunway = 0 }},
		{"negative runway", func(o *StreamOptions) { o.ArmSideTargetRunway = -1 }},
		{"zero send interval", func(o *StreamOptions) { o.SendToArmInterval = 0 }},
		{"negative send interval", func(o *StreamOptions) { o.SendToArmInterval = -1 }},
		{"send interval not less than runway", func(o *StreamOptions) { o.SendToArmInterval = o.ArmSideTargetRunway }},
		{"zero vel limit", func(o *StreamOptions) { o.MoveOptions.MaxVelRads = 0 }},
		{"negative vel limit", func(o *StreamOptions) { o.MoveOptions.MaxVelRads = -1 }},
		{"zero accel limit", func(o *StreamOptions) { o.MoveOptions.MaxAccRads = 0 }},
		{"negative accel limit", func(o *StreamOptions) { o.MoveOptions.MaxAccRads = -1 }},
		{"zero entry in per-joint vel limits", func(o *StreamOptions) { o.MoveOptions.MaxVelRadsJoints = []float64{1, 0} }},
		{"negative entry in per-joint vel limits", func(o *StreamOptions) { o.MoveOptions.MaxVelRadsJoints = []float64{1, -1} }},
		{"zero entry in per-joint accel limits", func(o *StreamOptions) { o.MoveOptions.MaxAccRadsJoints = []float64{1, 0} }},
		{"negative entry in per-joint accel limits", func(o *StreamOptions) { o.MoveOptions.MaxAccRadsJoints = []float64{1, -1} }},
		{"zero tcp speed limit", func(o *StreamOptions) { tcp := 0.0; o.MoveOptions.MaxTCPSpeedMPerSec = &tcp }},
		{"negative tcp speed limit", func(o *StreamOptions) { tcp := -0.1; o.MoveOptions.MaxTCPSpeedMPerSec = &tcp }},
		{"negative diagnostics window", func(o *StreamOptions) { o.DiagnosticsWindow = -1 }},
	} {
		bad := valid
		tc.mutate(&bad)
		test.That(t, bad.Validate(), test.ShouldNotBeNil)
	}
}

func TestMaxTargetsDroppedInARowDueToTrajexExtendError(t *testing.T) {
	extra := func(v interface{}) map[string]interface{} {
		return map[string]interface{}{maxTargetsDroppedInARowDueToTrajexExtendErrorKey: v}
	}

	n, err := MaxTargetsDroppedInARowDueToTrajexExtendError(nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, 0)

	n, err = MaxTargetsDroppedInARowDueToTrajexExtendError(extra(float64(7)))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, 7)

	for _, v := range []interface{}{int32(3), 3, int64(3), "3", 1.5, -1.0, math.NaN()} {
		_, err = MaxTargetsDroppedInARowDueToTrajexExtendError(extra(v))
		test.That(t, err, test.ShouldNotBeNil)
	}

	opts := NewStreamOptions(motion.TempStreamOptions{})
	opts.MaxTargetsDroppedInARowDueToTrajexExtendError = -1
	test.That(t, opts.Validate(), test.ShouldNotBeNil)
}

func TestForceRebaseIfExtendLatencyExceeds(t *testing.T) {
	d, err := ForceRebaseIfExtendLatencyExceeds(nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d, test.ShouldEqual, 0)

	d, err = ForceRebaseIfExtendLatencyExceeds(map[string]interface{}{forceRebaseIfExtendLatencyExceedsMsKey: float64(15)})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d, test.ShouldEqual, 15*time.Millisecond)

	for _, v := range []interface{}{int32(15), 15, int64(15), "15", 1.5, -1.0, math.NaN()} {
		_, err = ForceRebaseIfExtendLatencyExceeds(map[string]interface{}{forceRebaseIfExtendLatencyExceedsMsKey: v})
		test.That(t, err, test.ShouldNotBeNil)
	}

	opts := NewStreamOptions(motion.TempStreamOptions{})
	opts.ForceRebaseIfExtendLatencyExceeds = -1
	test.That(t, opts.Validate(), test.ShouldNotBeNil)
}

func TestExtraValuesSurviveTheMotionServiceRoundTrip(t *testing.T) {
	sent := map[string]interface{}{
		maxTargetsDroppedInARowDueToTrajexExtendErrorKey: int32(7),
		forceRebaseIfExtendLatencyExceedsMsKey:           int32(15),
	}
	pb, err := vprotoutils.StructToStructPb(sent)
	test.That(t, err, test.ShouldBeNil)
	received := pb.AsMap()
	test.That(t, received[maxTargetsDroppedInARowDueToTrajexExtendErrorKey], test.ShouldHaveSameTypeAs, float64(0))

	n, err := MaxTargetsDroppedInARowDueToTrajexExtendError(received)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, 7)
	d, err := ForceRebaseIfExtendLatencyExceeds(received)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, d, test.ShouldEqual, 15*time.Millisecond)
}

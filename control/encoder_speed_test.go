package control

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

func TestEncoderSpeedConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	for _, tc := range []struct {
		name  string
		ticks interface{}
		err   string
	}{
		// Block configs are decoded from JSON, so numbers arrive as float64 rather than int.
		{"json float64", float64(1000), ""},
		{"native int", 1000, ""},
		{"non-integral", 10.5, "must be a whole number"},
		{"wrong type", "1000", "must be a number"},
		{"zero", float64(0), "must be nonzero"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := BlockConfig{
				Name:      "E1",
				Type:      "encoderToRpm",
				Attribute: utils.AttributeMap{"ticks_per_revolution": tc.ticks},
				DependsOn: []string{"A"},
			}
			var blk Block
			var err error
			test.That(t, func() { blk, err = newEncoderSpeed(cfg, logger) }, test.ShouldNotPanic)
			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, blk.(*encoderToRPM).ticksPerRevolution, test.ShouldEqual, 1000)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
			}
		})
	}
}

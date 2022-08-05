package vision

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestColorDetector(t *testing.T) {
	inp := &DetectorConfig{
		Name: "my_color_detector",
		Type: "color",
		Parameters: config.AttributeMap{
			"segment_size": 150000,
			"tolerance":    0.44,
			"detect_color": "#4F3815",
			"extraneous":   "whatever",
		},
	}
	ctx := context.Background()
	reg := make(detectorMap)
	testlog := golog.NewLogger("testlog")
	err := registerColorDetector(ctx, reg, inp, testlog)
	test.That(t, err, test.ShouldBeNil)
	_, err = reg.detectorLookup("my_color_detector")
	test.That(t, err, test.ShouldBeNil)

	// with error - bad parameters
	inp.Name = "will_fail"
	inp.Parameters["tolerance"] = 4.0 // value out of range
	err = registerColorDetector(ctx, reg, inp, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "tolerance must be between")

	// with error - nil entry
	err = registerColorDetector(ctx, reg, nil, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")

	// with error - nil parameters
	inp.Parameters = nil
	err = registerColorDetector(ctx, reg, inp, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")
}

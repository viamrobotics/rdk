package builtin

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/vision"
)

func TestColorDetector(t *testing.T) {
	inp := &vision.VisModelConfig{
		Name: "my_color_detector",
		Type: "color_detector",
		Parameters: config.AttributeMap{
			"segment_size_px": 150000,
			"hue_tolerance_pct":   0.44,
			"detect_color":    "#4F3815",
			"extraneous":      "whatever",
		},
	}
	ctx := context.Background()
	reg := make(modelMap)
	testlog := golog.NewLogger("testlog")
	err := registerColorDetector(ctx, reg, inp, testlog)
	test.That(t, err, test.ShouldBeNil)
	_, err = reg.modelLookup("my_color_detector")
	test.That(t, err, test.ShouldBeNil)

	// with error - bad parameters
	inp.Name = "will_fail"
	inp.Parameters["hue_tolerance_pct"] = 4.0 // value out of range
	err = registerColorDetector(ctx, reg, inp, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "hue_tolerance_pct must be between")

	// with error - nil entry
	err = registerColorDetector(ctx, reg, nil, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")

	// with error - nil parameters
	inp.Parameters = nil
	err = registerColorDetector(ctx, reg, inp, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")
}

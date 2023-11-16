package transform

import (
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
)

type alignImageHelper struct {
	name                string
	config              AlignConfig
	expectedColorOutput []image.Point
	expectedDepthOutput []image.Point
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func makeTestCases() []alignImageHelper {
	cases := []alignImageHelper{
		{
			name: "base_case",
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{200, 100},
				DepthWarpPoints: []image.Point{{15, 57}, {47, 23}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: rimage.ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: rimage.ArrayToPoints([]image.Point{{0, 0}, {105, 99}}),
		},
		{
			name: "rotated case",
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{100, 200},
				DepthWarpPoints: []image.Point{{42, 15}, {76, 47}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: rimage.ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: rotatePoints(rimage.ArrayToPoints([]image.Point{{0, 0}, {99, 105}})),
		},
		{
			name: "scaled case",
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{150, 75},
				DepthWarpPoints: []image.Point{{11, 43}, {35, 17}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: rimage.ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: rimage.ArrayToPoints([]image.Point{{0, 0}, {79, 74}}),
		},
		{
			name: "scaled+rotated case",
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{75, 150},
				DepthWarpPoints: []image.Point{{31, 11}, {57, 35}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: rimage.ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: rotatePoints(rimage.ArrayToPoints([]image.Point{{0, 0}, {74, 79}})),
		},
	}
	return cases
}

func expectedImageAlignOutput(t *testing.T, a alignImageHelper, logger logging.Logger) {
	t.Helper()
	colorOutput, depthOutput, err := ImageAlign(
		a.config.ColorInputSize,
		a.config.ColorWarpPoints,
		a.config.DepthInputSize,
		a.config.DepthWarpPoints,
		logger,
	)
	test.That(t, err, test.ShouldBeNil)
	// If scaling changes expected pixel boundaries by 1 pixel, that can be explained by rounding
	for i := range colorOutput {
		Xdiff := Abs(colorOutput[i].X - a.expectedColorOutput[i].X)
		Ydiff := Abs(colorOutput[i].Y - a.expectedColorOutput[i].Y)
		test.That(t, Xdiff, test.ShouldBeLessThanOrEqualTo, 1)
		test.That(t, Ydiff, test.ShouldBeLessThanOrEqualTo, 1)
	}
	for i := range depthOutput {
		Xdiff := Abs(depthOutput[i].X - a.expectedDepthOutput[i].X)
		Ydiff := Abs(depthOutput[i].Y - a.expectedDepthOutput[i].Y)
		test.That(t, Xdiff, test.ShouldBeLessThanOrEqualTo, 1)
		test.That(t, Ydiff, test.ShouldBeLessThanOrEqualTo, 1)
	}
}

func TestAlignImage(t *testing.T) {
	cases := makeTestCases()
	for _, c := range cases {
		logger := logging.NewTestLogger(t)
		t.Run(c.name, func(t *testing.T) {
			expectedImageAlignOutput(t, c, logger)
		})
	}
}

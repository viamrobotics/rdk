package rimage

import (
	"fmt"
	"image"
	"testing"

	"github.com/edaniels/golog"
)

type alignImageHelper struct {
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
		{ //base case
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{200, 100},
				DepthWarpPoints: []image.Point{{15, 57}, {47, 23}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: ArrayToPoints([]image.Point{{0, 0}, {105, 99}}),
		},
		{ //rotated case
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{100, 200},
				DepthWarpPoints: []image.Point{{42, 15}, {76, 47}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: rotatePoints(ArrayToPoints([]image.Point{{0, 0}, {99, 105}})),
		},
		{ //scaled case
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{150, 75},
				DepthWarpPoints: []image.Point{{11, 43}, {35, 17}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: ArrayToPoints([]image.Point{{0, 0}, {79, 74}}),
		},
		{ //scaled+rotated case
			config: AlignConfig{
				ColorInputSize:  image.Point{120, 240},
				ColorWarpPoints: []image.Point{{29, 82}, {61, 48}},
				DepthInputSize:  image.Point{75, 150},
				DepthWarpPoints: []image.Point{{31, 11}, {57, 35}},
				OutputSize:      image.Point{50, 50},
			},
			expectedColorOutput: ArrayToPoints([]image.Point{{14, 25}, {119, 124}}),
			expectedDepthOutput: rotatePoints(ArrayToPoints([]image.Point{{0, 0}, {74, 79}})),
		},
	}
	return cases
}

func expectedImageAlignOutput(a alignImageHelper, logger golog.Logger) (bool, error) {

	colorOutput, depthOutput, err := ImageAlign(a.config.ColorInputSize, a.config.ColorWarpPoints, a.config.DepthInputSize, a.config.DepthWarpPoints, logger)
	if err != nil {
		return false, err
	}
	// If scaling changes expected pixel boundaries by 1 pixel, that can be explained by rounding
	for i := range colorOutput {
		Xdiff := Abs(colorOutput[i].X - a.expectedColorOutput[i].X)
		Ydiff := Abs(colorOutput[i].Y - a.expectedColorOutput[i].Y)
		if Xdiff > 1 || Ydiff > 1 {
			return false, fmt.Errorf("color got: %v color exp: %v", colorOutput, a.expectedColorOutput)
		}
	}
	for i := range depthOutput {
		Xdiff := Abs(depthOutput[i].X - a.expectedDepthOutput[i].X)
		Ydiff := Abs(depthOutput[i].Y - a.expectedDepthOutput[i].Y)
		if Xdiff > 1 || Ydiff > 1 {
			return false, fmt.Errorf("depth got: %v depth exp: %v", depthOutput, a.expectedDepthOutput)
		}
	}
	return true, nil
}

func TestAlignImage(t *testing.T) {
	cases := makeTestCases()
	for _, c := range cases {
		logger := golog.NewTestLogger(t)
		ok, err := expectedImageAlignOutput(c, logger)
		if !ok {
			t.Error(err)
		}
	}

}

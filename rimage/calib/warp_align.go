package calib

import (
	"fmt"
	"image"

	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
)

var (
	IntelConfig = AlignConfig{
		ColorInputSize:  image.Point{1280, 720},
		ColorWarpPoints: []image.Point{{0, 0}, {1196, 720}},

		DepthInputSize:  image.Point{1024, 768},
		DepthWarpPoints: []image.Point{{67, 100}, {1019, 665}},

		OutputSize: image.Point{640, 360},
	}
)

type AlignConfig struct {
	ColorInputSize  image.Point // this validates input size
	ColorWarpPoints []image.Point

	DepthInputSize  image.Point // this validates output size
	DepthWarpPoints []image.Point

	WarpFromCommon bool

	OutputSize image.Point
}

func (config AlignConfig) ComputeWarpFromCommon(logger golog.Logger) (*AlignConfig, error) {

	colorPoints, depthPoints, err := ImageAlign(
		config.ColorInputSize,
		config.ColorWarpPoints,
		config.DepthInputSize,
		config.DepthWarpPoints,
		logger,
	)

	if err != nil {
		return nil, err
	}

	return &AlignConfig{
		ColorInputSize:  config.ColorInputSize,
		ColorWarpPoints: rimage.ArrayToPoints(colorPoints),
		DepthInputSize:  config.DepthInputSize,
		DepthWarpPoints: rimage.ArrayToPoints(depthPoints),
		OutputSize:      config.OutputSize,
	}, nil
}

func (config AlignConfig) CheckValid() error {
	if config.ColorInputSize.X == 0 ||
		config.ColorInputSize.Y == 0 {
		return fmt.Errorf("invalid ColorInputSize %#v", config.ColorInputSize)
	}

	if config.DepthInputSize.X == 0 ||
		config.DepthInputSize.Y == 0 {
		return fmt.Errorf("invalid DepthInputSize %#v", config.DepthInputSize)
	}

	if config.OutputSize.X == 0 || config.OutputSize.Y == 0 {
		return fmt.Errorf("invalid OutputSize %v", config.OutputSize)
	}

	if len(config.ColorWarpPoints) != 2 && len(config.ColorWarpPoints) != 4 {
		return fmt.Errorf("invalid ColorWarpPoints, has to be 2 or 4 is %d", len(config.ColorWarpPoints))
	}

	if len(config.DepthWarpPoints) != 2 && len(config.DepthWarpPoints) != 4 {
		return fmt.Errorf("invalid DepthWarpPoints, has to be 2 or 4 is %d", len(config.DepthWarpPoints))
	}

	return nil
}

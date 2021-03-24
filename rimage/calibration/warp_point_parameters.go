package calibration

import (
	"fmt"
	"image"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
)

var (
	IntelConfig = rimage.AlignConfig{
		ColorInputSize:  image.Point{1280, 720},
		ColorWarpPoints: []image.Point{{0, 0}, {1196, 720}},

		DepthInputSize:  image.Point{1024, 768},
		DepthWarpPoints: []image.Point{{67, 100}, {1019, 665}},

		OutputSize: image.Point{640, 360},
	}
)

func NewDepthColorTransformsFromWarp(attrs api.AttributeMap, logger golog.Logger) (*rimage.DepthColorTransforms, error) {
	var config *rimage.AlignConfig
	var err error

	if attrs.Has("config") {
		config = attrs["config"].(*rimage.AlignConfig)
	} else if attrs["make"] == "intel515" {
		config = &IntelConfig
	} else {
		return nil, fmt.Errorf("no aligntmnt config")
	}

	dst := rimage.ArrayToPoints([]image.Point{{0, 0}, {config.OutputSize.X, config.OutputSize.Y}})

	if config.WarpFromCommon {
		config, err = config.ComputeWarpFromCommon(logger)
		if err != nil {
			return nil, err
		}
	}

	colorPoints := rimage.ArrayToPoints(config.ColorWarpPoints)
	depthPoints := rimage.ArrayToPoints(config.DepthWarpPoints)

	colorTransform := rimage.GetPerspectiveTransform(colorPoints, dst)
	depthTransform := rimage.GetPerspectiveTransform(depthPoints, dst)

	return &rimage.DepthColorTransforms{colorTransform, depthTransform, config}, nil
}

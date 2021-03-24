package calib

import (
	"fmt"
	"image"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/pointcloud"
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

type WarpPointTransforms struct {
	ColorTransform, DepthTransform rimage.TransformationMatrix
	*AlignConfig                   // anonymous fields
}

func (dct *WarpPointTransforms) ToPointCloudWithColor(ii *rimage.ImageWithDepth, logger golog.Logger) (*pointcloud.PointCloud, error) {
	return nil, fmt.Errorf("method ToPointCloudWithColor not implemented for WarpPointTransforms")
}

func (dct *WarpPointTransforms) ToAlignedImageWithDepth(ii *rimage.ImageWithDepth, logger golog.Logger) (*rimage.ImageWithDepth, error) {

	if ii.Color.Width() != dct.ColorInputSize.X ||
		ii.Color.Height() != dct.ColorInputSize.Y ||
		ii.Depth.Width() != dct.DepthInputSize.X ||
		ii.Depth.Height() != dct.DepthInputSize.Y {
		return nil, fmt.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %#v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), dct.AlignConfig)
	}
	ii.Depth.Smooth() // TODO(erh): maybe instead of this I should change warp to let the user determine how to average

	c2 := rimage.WarpImage(ii, dct.ColorTransform, dct.OutputSize)
	dm2 := ii.Depth.Warp(dct.DepthTransform, dct.OutputSize)

	return &rimage.ImageWithDepth{c2, &dm2}, nil
}

func NewDepthColorTransformsFromWarp(attrs api.AttributeMap, logger golog.Logger) (*WarpPointTransforms, error) {
	var config *AlignConfig
	var err error

	if attrs.Has("config") {
		config = attrs["config"].(*AlignConfig)
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

	return &WarpPointTransforms{colorTransform, depthTransform, config}, nil
}

package calib

import (
	"fmt"
	"image"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
)

type DepthColorWarpTransforms struct {
	ColorTransform, DepthTransform rimage.TransformationMatrix
	*AlignConfig                   // anonymous fields
}

func (dct *DepthColorWarpTransforms) ToPointCloudWithColor(ii *rimage.ImageWithDepth, logger golog.Logger) (*pointcloud.PointCloud, error) {
	return nil, fmt.Errorf("method ToPointCloudWithColor not implemented for DepthColorWarpTransforms")
}

func (dct *DepthColorWarpTransforms) ToAlignedImageWithDepth(ii *rimage.ImageWithDepth, logger golog.Logger) (*rimage.ImageWithDepth, error) {

	if ii.Color.Width() != dct.ColorInputSize.X ||
		ii.Color.Height() != dct.ColorInputSize.Y ||
		ii.Depth.Width() != dct.DepthInputSize.X ||
		ii.Depth.Height() != dct.DepthInputSize.Y {
		return nil, fmt.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %#v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), dct.AlignConfig)
	}

	if dct.Smooth {
		ii.Depth.Smooth() // TODO(erh): maybe instead of this I should change warp to let the user determine how to average
	}

	c2 := rimage.WarpImage(ii, dct.ColorTransform, dct.OutputSize)
	dm2 := ii.Depth.Warp(dct.DepthTransform, dct.OutputSize)

	return &rimage.ImageWithDepth{c2, &dm2}, nil
}

func NewDepthColorWarpTransforms(attrs api.AttributeMap, logger golog.Logger) (*DepthColorWarpTransforms, error) {
	var config *AlignConfig
	var err error

	if attrs.Has("config") {
		config = attrs["config"].(*AlignConfig)
	} else {
		return nil, fmt.Errorf("no alignment config")
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

	return &DepthColorWarpTransforms{colorTransform, depthTransform, config}, nil
}

package calibration

import (
	"context"
	"fmt"
	"image"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
)

var (
	alignCurrentlyWriting = false
	IntelConfig           = rimage.AlignConfig{
		ColorInputSize:  image.Point{1280, 720},
		ColorWarpPoints: []image.Point{{0, 0}, {1196, 720}},

		DepthInputSize:  image.Point{1024, 768},
		DepthWarpPoints: []image.Point{{67, 100}, {1019, 665}},

		OutputSize: image.Point{640, 360},
	}
)

type DepthColorTransforms struct {
	colorTransform, depthTransform rimage.TransformationMatrix

	config *rimage.AlignConfig
	debug  bool
	logger golog.Logger
}

func NewDepthColorTransforms(attrs api.AttributeMap, logger golog.Logger) (*DepthColorTransforms, error) {
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

	return &DepthColorTransforms{colorTransform, depthTransform, config, attrs.GetBool("debug", false), logger}, nil
}

func (dct *DepthColorTransforms) AlignColorAndDepth(ctx context.Context, ii *rimage.ImageWithDepth, logger golog.Logger) (*rimage.ImageWithDepth, error) {
	_, span := trace.StartSpan(ctx, "AlignColorAndDepth")
	defer span.End()

	if dct.debug {
		if !alignCurrentlyWriting {
			alignCurrentlyWriting = true
			go func() {
				defer func() { alignCurrentlyWriting = false }()
				fn := fmt.Sprintf("data/align-test-%d.both.gz", time.Now().Unix())
				err := ii.WriteTo(fn)
				if err != nil {
					logger.Debugf("error writing debug file: %s", err)
				} else {
					logger.Debugf("wrote debug file to %s", fn)
				}
			}()
		}
	}

	if ii.Color.Width() != dct.config.ColorInputSize.X ||
		ii.Color.Height() != dct.config.ColorInputSize.Y ||
		ii.Depth.Width() != dct.config.DepthInputSize.X ||
		ii.Depth.Height() != dct.config.DepthInputSize.Y {
		return nil, fmt.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %#v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), dct.config)
	}
	ii.Depth.Smooth() // TODO(erh): maybe instead of this I should change warp to let the user determine how to average

	c2 := rimage.WarpImage(ii, dct.colorTransform, dct.config.OutputSize)
	dm2 := ii.Depth.Warp(dct.depthTransform, dct.config.OutputSize)

	return &rimage.ImageWithDepth{c2, &dm2}, nil
}

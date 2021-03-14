package rimage

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.opencensus.io/trace"

	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterCamera("depthComposed", func(r api.Robot, config api.Component) (gostream.ImageSource, error) {
		colorName := config.Attributes.GetString("color")
		color := r.CameraByName(colorName)
		if color == nil {
			return nil, fmt.Errorf("cannot find color camera (%s)", colorName)
		}

		depthName := config.Attributes.GetString("depth")
		depth := r.CameraByName(depthName)
		if depth == nil {
			return nil, fmt.Errorf("cannot find depth camera (%s)", depthName)
		}
		return NewDepthComposed(color, depth)
	})
}

type depthComposed struct {
	color gostream.ImageSource
	depth gostream.ImageSource
}

func NewDepthComposed(color, depth gostream.ImageSource) (gostream.ImageSource, error) {
	if color == nil {
		return nil, fmt.Errorf("need color")
	}
	if depth == nil {
		return nil, fmt.Errorf("need depth")
	}
	return &depthComposed{color, depth}, nil
}

func (dc *depthComposed) Close() error {
	// TODO(erh): who owns these?
	return nil
}

func (dc *depthComposed) Next(ctx context.Context) (image.Image, func(), error) {
	c, cCloser, err := dc.color.Next(ctx)
	if err != nil {
		return nil, nil, err
	}

	defer cCloser()

	d, dCloser, err := dc.depth.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer dCloser()

	aligned, err := intel515align(ctx, &ImageWithDepth{ConvertImage(c), imageToDepthMap(d)})

	return aligned, func() {}, err

}

func rectToPoints(r image.Rectangle) []image.Point {
	return []image.Point{
		r.Min,
		{r.Max.X, r.Min.Y},
		{r.Min.X, r.Max.Y},
		r.Max,
	}
}

type alignConfig struct {
	ColorInputSize  image.Point // this validates input size
	ColorWarpPoints image.Rectangle

	DepthInputSize  image.Point // this validates output size
	DepthWarpPoints image.Rectangle

	OutputSize image.Point
}

var (
	intelCurrentlyWriting = false
	intelConfig           = alignConfig{
		ColorInputSize:  image.Point{1280, 720},
		ColorWarpPoints: image.Rect(0, 0, 1196, 720),

		DepthInputSize:  image.Point{1024, 768},
		DepthWarpPoints: image.Rect(67, 100, 1019, 665),

		OutputSize: image.Point{640, 360},
	}
)

func intel515align(ctx context.Context, ii *ImageWithDepth) (*ImageWithDepth, error) {
	return alignColorAndDepth(ctx, ii, intelConfig)
}

func alignColorAndDepth(ctx context.Context, ii *ImageWithDepth, config alignConfig) (*ImageWithDepth, error) {
	_, span := trace.StartSpan(ctx, "Intel515Align")
	defer span.End()

	if false {
		if !intelCurrentlyWriting {
			intelCurrentlyWriting = true
			go func() {
				defer func() { intelCurrentlyWriting = false }()
				fn := fmt.Sprintf("data/align-test-%d.both.gz", time.Now().Unix())
				err := ii.WriteTo(fn)
				if err != nil {
					golog.Global.Debugf("error writing debug file: %s", err)
				} else {
					golog.Global.Debugf("wrote debug file to %s", fn)
				}
			}()
		}
		return ii, nil
	}

	if ii.Color.Width() != config.ColorInputSize.X ||
		ii.Color.Height() != config.ColorInputSize.Y ||
		ii.Depth.Width() != config.DepthInputSize.X ||
		ii.Depth.Height() != config.DepthInputSize.Y {
		return nil, fmt.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), config)
	}

	newBounds := image.Rect(0, 0, config.OutputSize.X, config.OutputSize.Y)

	dst := rectToPoints(newBounds)

	depthPoints := rectToPoints(config.DepthWarpPoints)
	colorPoints := rectToPoints(config.ColorWarpPoints)

	c2 := WarpImage(ii, GetPerspectiveTransform(colorPoints, dst), newBounds.Max)
	dm2 := ii.Depth.Warp(GetPerspectiveTransform(depthPoints, dst), newBounds.Max)

	return &ImageWithDepth{c2, &dm2}, nil
}

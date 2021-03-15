package rimage

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.opencensus.io/trace"

	"github.com/mitchellh/mapstructure"

	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterCamera("depthComposed", func(r api.Robot, config api.Component) (gostream.ImageSource, error) {
		attrs := config.Attributes

		colorName := attrs.GetString("color")
		color := r.CameraByName(colorName)
		if color == nil {
			return nil, fmt.Errorf("cannot find color camera (%s)", colorName)
		}

		depthName := attrs.GetString("depth")
		depth := r.CameraByName(depthName)
		if depth == nil {
			return nil, fmt.Errorf("cannot find depth camera (%s)", depthName)
		}

		return NewDepthComposed(color, depth, config.Attributes)
	})

	api.Register(api.ComponentTypeCamera, "depthComposed", "config", func(val interface{}) (interface{}, error) {
		config := &alignConfig{}
		err := mapstructure.Decode(val, config)
		if err == nil {
			err = config.checkValid()
		}
		return config, err
	})
}

type DepthComposed struct {
	color, depth                   gostream.ImageSource
	colorTransform, depthTransform TransformationMatrix

	config *alignConfig
	debug  bool
}

func NewDepthComposed(color, depth gostream.ImageSource, attrs api.AttributeMap) (*DepthComposed, error) {
	var config *alignConfig
	var err error

	if attrs.Has("config") {
		config = attrs["config"].(*alignConfig)
	} else if attrs["make"] == "intel515" {
		config = &intelConfig
	} else {
		return nil, fmt.Errorf("no aligntmnt config")
	}

	dst := arrayToPoints([]image.Point{{0, 0}, {config.OutputSize.X, config.OutputSize.Y}})

	if config.WarpFromCommon {
		config, err = config.computeWarpFromCommon()
		if err != nil {
			return nil, err
		}
	}

	colorPoints := arrayToPoints(config.ColorWarpPoints)
	depthPoints := arrayToPoints(config.DepthWarpPoints)

	colorTransform := GetPerspectiveTransform(colorPoints, dst)
	depthTransform := GetPerspectiveTransform(depthPoints, dst)

	return &DepthComposed{color, depth, colorTransform, depthTransform, config, attrs.GetBool("debug", false)}, nil
}

func (dc *DepthComposed) Close() error {
	// TODO(erh): who owns these?
	return nil
}

func convertImageToDepthMap(img image.Image) (*DepthMap, error) {
	switch ii := img.(type) {
	case *ImageWithDepth:
		return ii.Depth, nil
	case *image.Gray16:
		return imageToDepthMap(ii), nil
	default:
		return nil, fmt.Errorf("don't know how to make DepthMap from %T", img)
	}
}

func (dc *DepthComposed) Next(ctx context.Context) (image.Image, func(), error) {
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

	dm, err := convertImageToDepthMap(d)
	if err != nil {
		return nil, nil, err
	}

	aligned, err := dc.alignColorAndDepth(ctx, &ImageWithDepth{ConvertImage(c), dm})

	return aligned, func() {}, err

}

func arrayToPoints(pts []image.Point) []image.Point {

	if len(pts) == 4 {
		return pts
	}

	if len(pts) == 2 {
		r := image.Rectangle{pts[0], pts[1]}
		return []image.Point{
			r.Min,
			{r.Max.X, r.Min.Y},
			r.Max,
			{r.Min.X, r.Max.Y},
		}
	}

	panic(fmt.Errorf("invalid number of points passed to arrayToPoints %d", len(pts)))
}

type alignConfig struct {
	ColorInputSize  image.Point // this validates input size
	ColorWarpPoints []image.Point

	DepthInputSize  image.Point // this validates output size
	DepthWarpPoints []image.Point

	WarpFromCommon bool

	OutputSize image.Point
}

func (config alignConfig) computeWarpFromCommon() (*alignConfig, error) {

	colorPoints, depthPoints, err := ImageAlign(
		config.ColorInputSize,
		config.ColorWarpPoints,
		config.DepthInputSize,
		config.DepthWarpPoints,
	)

	if err != nil {
		return nil, err
	}

	return &alignConfig{
		ColorInputSize:  config.ColorInputSize,
		ColorWarpPoints: arrayToPoints(colorPoints),
		DepthInputSize:  config.DepthInputSize,
		DepthWarpPoints: arrayToPoints(depthPoints),
		OutputSize:      config.OutputSize,
	}, nil
}

func (config alignConfig) checkValid() error {
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

var (
	alignCurrentlyWriting = false
	intelConfig           = alignConfig{
		ColorInputSize:  image.Point{1280, 720},
		ColorWarpPoints: []image.Point{{0, 0}, {1196, 720}},

		DepthInputSize:  image.Point{1024, 768},
		DepthWarpPoints: []image.Point{{67, 100}, {1019, 665}},

		OutputSize: image.Point{640, 360},
	}
)

func (dc *DepthComposed) alignColorAndDepth(ctx context.Context, ii *ImageWithDepth) (*ImageWithDepth, error) {
	_, span := trace.StartSpan(ctx, "alignColorAndDepth")
	defer span.End()

	if dc.debug {
		if !alignCurrentlyWriting {
			alignCurrentlyWriting = true
			go func() {
				defer func() { alignCurrentlyWriting = false }()
				fn := fmt.Sprintf("data/align-test-%d.both.gz", time.Now().Unix())
				err := ii.WriteTo(fn)
				if err != nil {
					golog.Global.Debugf("error writing debug file: %s", err)
				} else {
					golog.Global.Debugf("wrote debug file to %s", fn)
				}
			}()
		}
	}

	if ii.Color.Width() != dc.config.ColorInputSize.X ||
		ii.Color.Height() != dc.config.ColorInputSize.Y ||
		ii.Depth.Width() != dc.config.DepthInputSize.X ||
		ii.Depth.Height() != dc.config.DepthInputSize.Y {
		return nil, fmt.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %#v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), dc.config)
	}

	ii.Depth.Smooth() // TODO(erh): maybe instead of this I should change warp to let the user determine how to average

	c2 := WarpImage(ii, dc.colorTransform, dc.config.OutputSize)
	dm2 := ii.Depth.Warp(dc.depthTransform, dc.config.OutputSize)

	return &ImageWithDepth{c2, &dm2}, nil
}

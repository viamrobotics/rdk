package imagesource

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/core/component/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"
	"go.opencensus.io/trace"
)

func init() {
	registry.RegisterComponent(camera.Subtype, "depthComposed", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		attrs := config.Attributes

		colorName := attrs.String("color")
		color, ok := r.CameraByName(colorName)
		if !ok {
			return nil, errors.Errorf("cannot find color camera (%s)", colorName)
		}

		depthName := attrs.String("depth")
		depth, ok := r.CameraByName(depthName)
		if !ok {
			return nil, errors.Errorf("cannot find depth camera (%s)", depthName)
		}

		dc, err := NewDepthComposed(color, depth, config.Attributes, logger)
		if err != nil {
			return nil, err
		}
		return &camera.ImageSource{ImageSource: dc}, nil
	}})

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "depthComposed", "warp", func(val interface{}) (interface{}, error) {
		warp := &transform.AlignConfig{}
		err := mapstructure.Decode(val, warp)
		if err == nil {
			err = warp.CheckValid()
		}
		return warp, err
	})

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "depthComposed", "intrinsic_extrinsic", func(val interface{}) (interface{}, error) {
		matrices := &transform.DepthColorIntrinsicsExtrinsics{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: matrices})
		if err != nil {
			return nil, err
		}
		err = decoder.Decode(val)
		if err == nil {
			err = matrices.CheckValid()
		}
		return matrices, err
	})

	config.RegisterComponentAttributeConverter(config.ComponentTypeCamera, "depthComposed", "homography", func(val interface{}) (interface{}, error) {
		homography := &transform.RawPinholeCameraHomography{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: homography})
		if err != nil {
			return nil, err
		}
		err = decoder.Decode(val)
		if err == nil {
			err = homography.CheckValid()
		}
		return homography, err
	})
}

var alignCurrentlyWriting = false

// depthComposed TODO
type depthComposed struct {
	color, depth     gostream.ImageSource
	alignmentCamera  rimage.CameraSystem
	projectionCamera rimage.CameraSystem
	aligned          bool
	debug            bool
	logger           golog.Logger
}

// NewDepthComposed TODO
func NewDepthComposed(color, depth gostream.ImageSource, attrs config.AttributeMap, logger golog.Logger) (gostream.ImageSource, error) {
	var alignCamera rimage.CameraSystem
	var projectCamera rimage.CameraSystem
	var err error

	if attrs.Has("intrinsic_extrinsic") {
		alignCamera, err = transform.NewDepthColorIntrinsicsExtrinsics(attrs)
		if err != nil {
			return nil, err
		}
		projectCamera = alignCamera
	} else if attrs.Has("homography") {
		conf := attrs["homography"].(*transform.RawPinholeCameraHomography)
		alignCamera, err = transform.NewPinholeCameraHomography(conf)
		if err != nil {
			return nil, err
		}
		projectCamera = alignCamera
	} else if attrs.Has("warp") {
		conf := attrs["warp"].(*transform.AlignConfig)
		alignCamera, err = transform.NewDepthColorWarpTransforms(conf, logger)
		if err != nil {
			return nil, err
		}
		projectCamera = alignCamera
	} else {
		return nil, errors.New("no camera system config")
	}
	return &depthComposed{color, depth, alignCamera, projectCamera, attrs.Bool("aligned", false), attrs.Bool("debug", false), logger}, nil
}

// Close does nothing.
func (dc *depthComposed) Close() error {
	// TODO(erh): who owns these?
	return nil
}

// Next TODO
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

	dm, err := rimage.ConvertImageToDepthMap(d)
	if err != nil {
		return nil, nil, err
	}

	ii := rimage.MakeImageWithDepth(rimage.ConvertImage(c), dm, dc.aligned, dc.alignmentCamera)

	_, span := trace.StartSpan(ctx, "AlignImageWithDepth")
	defer span.End()
	if dc.debug {
		if !alignCurrentlyWriting {
			alignCurrentlyWriting = true
			utils.PanicCapturingGo(func() {
				defer func() { alignCurrentlyWriting = false }()
				fn := artifact.MustNewPath(fmt.Sprintf("rimage/imagesource/align-test-%d.both.gz", time.Now().Unix()))
				err := ii.WriteTo(fn)
				if err != nil {
					dc.logger.Debugf("error writing debug file: %s", err)
				} else {
					dc.logger.Debugf("wrote debug file to %s", fn)
				}
			})
		}
	}
	aligned, err := dc.alignmentCamera.AlignImageWithDepth(ii)
	if err != nil {
		return nil, nil, err
	}
	aligned.SetCameraSystem(dc.projectionCamera)

	return aligned, func() {}, err

}

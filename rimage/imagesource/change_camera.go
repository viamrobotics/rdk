package imagesource

import (
	"context"
	"image"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/mitchellh/mapstructure"

	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterCamera("changeCameraSystem", registry.Camera{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error) {
		return newChangeCameraSystem(r, config)
	}})

	config.RegisterAttributeConverter(config.ComponentTypeCamera, "changeCameraSystem", "matrices", func(val interface{}) (interface{}, error) {
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
}

// CameraSystemChanger changes the camera system of the ImageWithDepth. Might be necessary if the system
// that does the alignment is different from the system that does the projection.
type CameraSystemChanger struct {
	source    gostream.ImageSource
	NewCamera rimage.CameraSystem
}

// Close closes the source
func (os *CameraSystemChanger) Close() error {
	return nil
}

// Next changes the CameraSystem of the ImageWithDepth to the system in the config file.
func (os *CameraSystemChanger) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := os.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	ii.SetCameraSystem(os.NewCamera)
	return ii, func() {}, nil
}

func newChangeCameraSystem(r robot.Robot, config config.Component) (camera.Camera, error) {
	var cam rimage.CameraSystem
	var err error

	attrs := config.Attributes
	source, ok := r.CameraByName(attrs.String("source"))
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", source)
	}
	if attrs.Has("matrices") {
		cam, err = transform.NewDepthColorIntrinsicsExtrinsics(attrs)
	} else {
		return nil, errors.New("no camera system config")
	}
	if err != nil {
		return nil, err
	}
	return &camera.ImageSource{&CameraSystemChanger{source, cam}}, nil
}

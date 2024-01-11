// Package rtsp implements an RTSP camera client for RDK
package rtsp

import (
	"context"
	"image"
	"sync"

	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/ffmpeg"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

var model = resource.DefaultModelFamily.WithModel("rtsp")

func init() {
	resource.RegisterComponent(camera.API, model, resource.Registration[camera.Camera, *Config]{
		Constructor: func(
			ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (camera.Camera, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			return NewRTSPCamera(ctx, conf.ResourceName(), newConf, logger)
		},
	})
}

// Config are the config attributes for an RTSP camera model.
type Config struct {
	Address          string                             `json:"rtsp_address"`
	IntrinsicParams  *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParams *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

// Validate checks to see if the attributes of the model are valid.
func (conf *Config) Validate(_ string) ([]string, error) {
	_, err := url.Parse(conf.Address)
	if err != nil {
		return nil, err
	}
	if conf.IntrinsicParams != nil {
		if err := conf.IntrinsicParams.CheckValid(); err != nil {
			return nil, err
		}
	}
	if conf.DistortionParams != nil {
		if err := conf.DistortionParams.CheckValid(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// rtsp contains the rtsp client, and the reader function that fulfills the camera interface.
type rtsp struct {
	gostream.VideoReader
	cancelCtx               context.Context
	cancelFunc              context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
}

// Close closes the camera. It always returns nil, but because of Close() interface, it needs to return an error.
func (r *rtsp) Close(_ context.Context) error {
	r.cancelFunc()
	r.activeBackgroundWorkers.Wait()

	return nil
}

type imgError struct {
	image.Image
	error
}

// Reading from an RTSP stream as fast as possible prevents the write queue from filling up which results in the stream
// dropping the session and EOF errors. This should only be a problem for recorded streams or streams that feeds us
// frames faster than we can process them.
func (r *rtsp) startStreaming(ctx context.Context, addr string) (chan imgError, error) {
	imgCh := make(chan imgError)

	dec, err := NewDecoder(ctx, addr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot start rtsp stream")
	}

	r.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		defer dec.Close()
		for {
			select {
			case <-r.cancelCtx.Done():
				return
			default:
			}

			img, err := dec.Decode(r.cancelCtx)
			select {
			case <-r.cancelCtx.Done():
				return
			case imgCh <- imgError{img, err}:
			default:
			}
		}
	}, r.activeBackgroundWorkers.Done)

	return imgCh, nil
}

// NewRTSPCamera creates a camera client using RTSP given the server URL.
func NewRTSPCamera(ctx context.Context, name resource.Name, conf *Config, logger logging.Logger) (camera.Camera, error) {
	ffmpeg.NetworkInit(ctx)

	r := &rtsp{}
	r.cancelCtx, r.cancelFunc = context.WithCancel(context.Background())
	imgCh, err := r.startStreaming(ctx, conf.Address)
	if err != nil {
		return nil, errors.Wrap(err, "cannot start RTSP stream")
	}

	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		imgErr := <-imgCh
		return imgErr.Image, func() {}, imgErr.error
	})

	r.VideoReader = reader
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(conf.IntrinsicParams, conf.DistortionParams)
	src, err := camera.NewVideoSourceFromReader(ctx, r, &cameraModel, camera.ColorStream)
	if err != nil {
		return nil, err
	}
	return camera.FromVideoSource(name, src, logger), nil
}

// Package ffmpeg provides an implementation for ffmpeg based cameras
package ffmpeg

import (
	"context"
	"image"
	"image/jpeg"
	"io"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	ffmpeg "github.com/u2takey/ffmpeg-go"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
	viamutils "go.viam.com/utils"
)

// ffmpegAttrs is the attribute struct for ffmpeg cameras.
type ffmpegAttrs struct {
	*camera.AttrConfig
	URL string `json:"url"`
}

const model = "ffmpeg"

func init() {
	registry.RegisterComponent(camera.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
			attrs, ok := cfg.ConvertedAttributes.(*ffmpegAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, cfg.ConvertedAttributes)
			}
			return NewFFmpegCamera(attrs, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(
		camera.SubtypeName,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf ffmpegAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*ffmpegAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		},
		&ffmpegAttrs{},
	)
}

func NewFFmpegCamera(attrs *ffmpegAttrs, logger golog.Logger) (camera.Camera, error) {
	// launch thread to run ffmpeg and pull images from the url and put them into the pipe
	ctx, cancel := context.WithCancel(context.Background())

	ffCam := &ffmpegCamera{
		cancelFunc: cancel,
	}

	var ffmpegErr atomic.Value
	in, out := io.Pipe()
	ffCam.activeBackgroundWorkers.Add(1)
	viamutils.ManagedGo(func() {
		stream := ffmpeg.
			Input(attrs.URL).
			Output("pipe:", ffmpeg.KwArgs{"update": 1, "format": "image2"})
		stream.Context = ctx

		ffmpegErr.Store(stream.WithOutput(out).Run())
	}, ffCam.activeBackgroundWorkers.Done)

	// launch thread to consume images from the pipe and store the latest in shared memory
	var latestFrame atomic.Value
	var gotFirstFrameOnce bool
	gotFirstFrame := make(chan struct{})
	ffCam.activeBackgroundWorkers.Add(1)
	viamutils.ManagedGo(func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if ffmpegErr.Load() != nil {
				return
			}
			img, err := jpeg.Decode(in)
			if err != nil {
				continue
			}
			latestFrame.Store(img)
			if !gotFirstFrameOnce {
				close(gotFirstFrame)
				gotFirstFrameOnce = true
			}
		}
	}, ffCam.activeBackgroundWorkers.Done)

	// when next image is requested simply load the image from where it is stored in shared memory
	imgSourceFunc := gostream.ImageSourceFunc(func(ctx context.Context) (image.Image, func(), error) {
		if ffmpegErr.Load() != nil {
			return nil, nil, ffmpegErr.Load().(error)
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-gotFirstFrame:
		}
		return latestFrame.Load().(image.Image), func() {}, nil
	})

	ffCam.ImageSource = imgSourceFunc
	return camera.New(ffCam, attrs.AttrConfig, nil)
}

type ffmpegCamera struct {
	gostream.ImageSource
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func (fc *ffmpegCamera) Close() {
	fc.cancelFunc()
	fc.activeBackgroundWorkers.Wait()
}

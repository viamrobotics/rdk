// Package ffmpeg provides an implementation for ffmpeg based cameras
package ffmpeg

import (
	"context"
	"image"
	"image/jpeg"
	"io"
	"os/exec"
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

type ffmpegCamera struct {
	gostream.ImageSource
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func NewFFmpegCamera(attrs *ffmpegAttrs, logger golog.Logger) (camera.Camera, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ffCam := &ffmpegCamera{cancelFunc: cancel}
	var ffmpegErr atomic.Value
	in, out := io.Pipe()

	// launch thread to run ffmpeg and pull images from the url and put them into the pipe
	ffCam.activeBackgroundWorkers.Add(1)
	viamutils.ManagedGo(func() {
		defer cancel()
		stream := ffmpeg.
			Input(attrs.URL).
			Output("pipe:", ffmpeg.KwArgs{"update": 1, "format": "image2"})
		stream.Context = ctx
		ffmpegErr.Store(stream.WithOutput(out).Run())
	}, ffCam.activeBackgroundWorkers.Done)

	var latestFrame atomic.Value
	var gotFirstFrameOnce bool
	gotFirstFrame := make(chan struct{})

	// launch thread to consume images from the pipe and store the latest in shared memory
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
	ourCtx := ctx
	ffCam.ImageSource = gostream.ImageSourceFunc(func(ctx context.Context) (image.Image, func(), error) {
		if ffmpegErr.Load() != nil {
			return nil, nil, ffmpegErr.Load().(error)
		}
		select {
		case <-ourCtx.Done():
			if ffmpegErr.Load() != nil {
				return nil, nil, ffmpegErr.Load().(error)
			}
			return nil, nil, ourCtx.Err()
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-gotFirstFrame:
		}
		return latestFrame.Load().(image.Image), func() {}, nil
	})

	return camera.New(ffCam, attrs.AttrConfig, nil)
}

func (fc *ffmpegCamera) Close() {
	fc.cancelFunc()
	fc.activeBackgroundWorkers.Wait()
}

// package ffmpeg provides an implementation for ffmpeg based cameras
package ffmpeg

import (
	"context"
	"image"
	"image/jpeg"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"

	ffmpeg "github.com/u2takey/ffmpeg-go"
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
			return NewFFmpegCamera(ctx, attrs, logger)
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

func NewFFmpegCamera(ctx context.Context, attrs *ffmpegAttrs, logger golog.Logger) (camera.Camera, error) {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// launch thread to run ffmpeg and pull images from the url and put them into the pipe
	var ffmpegErr atomic.Value
	var activeProcesses sync.WaitGroup
	in, out := io.Pipe()
	activeProcesses.Add(1)
	go func() {
		defer activeProcesses.Done()
		stream := ffmpeg.Input(attrs.URL).Output("pipe:", ffmpeg.KwArgs{"update": 1, "format": "image2"}).WithOutput(out)
		stream.Context = ctxWithCancel
		ffmpegErr.Store(stream.Run())
	}()

	// TODO(rb): this is a hacky workaround to keep race conditions from happening
	time.Sleep(time.Second)

	// launch thread to consume images from the pipe and store the latest in shared memory
	var latestFrame atomic.Value
	gotFirstFrameOnce := false
	gotFirstFrame := make(chan struct{})
	activeProcesses.Add(1)
	go func() {
		defer activeProcesses.Done()
		for {
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
	}()

	closeFunc := func() {
		if cancel != nil {
			cancel()
			cancel = nil
			activeProcesses.Wait()
		}
	}

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
		return latestFrame.Load().(image.Image), closeFunc, nil
	})

	return camera.New(imgSourceFunc, attrs.AttrConfig, nil)
}

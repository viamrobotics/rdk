package rtsp

import (
	"context"
	"image"
	"net/url"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

const model = "rtsp"

func init() {
	registry.RegisterComponent(camera.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
			attrs, ok := cfg.ConvertedAttributes.(*RtspAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, cfg.ConvertedAttributes)
			}
			return NewRTSPCamera(ctx, attrs, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(
		camera.SubtypeName,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf RtspAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*RtspAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&RtspAttrs{},
	)
}

type RtspAttrs struct {
	Address          `json:"rtsp_address"`
	IntrinsicParams  *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParams *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

type rtspCamera struct {
	gostream.VideoReader
	cancelFunc              context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
}

func NewRTSPCamera(ctx context.Context, attrs *RtspAttrs, logger golog.Logger) (camera.Camera, error) {
	cameraModel = &transform.PinholeCameraModel{attrs.IntrinsicParams, attrs.DistortionParams}
	// make a channel to put images into from the Client
	// turn the output into an image, put it in the channel
	c := gortsplib.Client{}
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// convert RTP packets into NALUs
		nalus, _, err := rtpDec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		for _, nalu := range nalus {
			// convert NALUs into RGBA frames
			img, err := h264RawDec.decode(nalu)
			if err != nil {
				panic(err)
			}

			// wait for a frame
			if img == nil {
				continue
			}
	}
	// parse URL
	u, err := url.Parse(attrs.Address)
	if err != nil {
		return nil, err
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	rtspCam := &rtspCamera{cancelFunc: cancel}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	// read the image from the channel
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		select {
		case <-cancelCtx.Done():
			return nil, nil, cancelCtx.Err()
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-gotFirstFrame:
		}
		return latestFrame.Load().(image.Image), func() {}, nil
	})
	rtspCam.VideoReader = reader
	return camera.NewFromReader(ctx, rtspCam, cameraModel, camera.ColorStream)
}

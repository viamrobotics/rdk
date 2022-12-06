package rtsp

import (
	"context"
	"image"
	"net/url"
	"sync/atomic"

	"github.com/aler9/gortsplib"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
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

// rtspCamera contains the rtsp client, and the reader function that fulfills the camera interface
type rtspCamera struct {
	gostream.VideoReader
	client  gortsplib.Client
	decoder *h264Decoder
}

func (rc *rtspCamera) Close(ctx context.Context) error {
	err := rc.client.Close()
	if err != nil {
		return err
	}
	rc.decoder.Close()
	return nil
}

func NewRTSPCamera(ctx context.Context, attrs *RtspAttrs, logger golog.Logger) (camera.Camera, error) {
	cameraModel = &transform.PinholeCameraModel{attrs.IntrinsicParams, attrs.DistortionParams}
	c := gortsplib.Client{}
	// parse URL
	u, err := url.Parse(attrs.Address)
	if err != nil {
		return nil, err
	}
	// find published tracks
	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		return nil, err
	}

	// find the H264 track
	track := func() *gortsplib.TrackH264 {
		for _, track := range tracks {
			if track, ok := track.(*gortsplib.TrackH264); ok {
				return track
			}
		}
		return nil
	}()
	if track == nil {
		return nil, errors.Errorf("H264 track not found in rtsp camera %s", u)
	}
	// get the RTP decoder and the h264 decoder
	rtpDec, h264RawDec, err := rtpH264Decoder(track)
	if err != nil {
		return nil, err
	}
	// On packet retreival, turn it into an image, and store it in shared memory
	var latestFrame atomic.Value
	var gotFirstFrameOnce bool
	gotFirstFrame := make(chan struct{})
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// convert RTP packets into NALUs
		nalus, _, err := rtpDec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		for _, nalu := range nalus {
			// convert NALUs into RGBA frames
			img, err := h264RawDec.Decode(nalu)
			if err != nil {
				panic(err)
			}

			// wait for a frame
			if img == nil {
				continue
			}
			latestFrame.Store(img)
			if !gotFirstFrameOnce {
				close(gotFirstFrame)
				gotFirstFrameOnce = true
			}
		}
	}
	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	// setup and read the H264 track only
	err = c.SetupAndPlay(gortsplib.Tracks{track}, baseURL)
	if err != nil {
		return nil, err
	}
	// read the image from shared memory when it is requested
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
	// define and return the camera
	rtspCam := &rtspCamera{reader, c, h264RawDec}
	return camera.NewFromReader(ctx, rtspCam, cameraModel, camera.ColorStream)
}

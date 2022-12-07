// Package rtsp implements an RTSP camera client for RDK
package rtsp

import (
	"context"
	"image"
	"sync/atomic"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

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
			attrs, ok := cfg.ConvertedAttributes.(*Attrs)
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
			var conf Attrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*Attrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&Attrs{},
	)
}

// Attrs are the config attributes for an RTSP camera model.
type Attrs struct {
	Address          string                             `json:"rtsp_address"`
	IntrinsicParams  *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParams *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

// rtspCamera contains the rtsp client, and the reader function that fulfills the camera interface.
type rtspCamera struct {
	gostream.VideoReader
	client     *gortsplib.Client
	decoder    *h264Decoder
	cancelFunc context.CancelFunc
}

// Close closes the camera.
func (rc *rtspCamera) Close(ctx context.Context) error {
	rc.cancelFunc()
	err := rc.client.Close()
	if err != nil {
		return err
	}
	return rc.decoder.Close()
}

// NewRTSPCamera creates a camera client for an RTSP given given the server URL.
// Right now, only supports servers that have H264 video tracks.
func NewRTSPCamera(ctx context.Context, attrs *Attrs, logger golog.Logger) (camera.Camera, error) {
	c := &gortsplib.Client{}
	// parse URL
	u, err := url.Parse(attrs.Address)
	if err != nil {
		return nil, err
	}
	// connect to the server - be sure to close it if setup fails.
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	// find published tracks
	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		return nil, multierr.Combine(err, c.Close())
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
		err := errors.Errorf("H264 track not found in rtsp camera %s", u)
		return nil, multierr.Combine(err, c.Close())
	}
	// get the RTP->h264 decoder and the h264->image.Image decoder
	rtpDec, h264RawDec, err := rtpH264Decoder(track)
	if err != nil {
		return nil, multierr.Combine(err, c.Close())
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
	// setup and read the H264 track only
	err = c.SetupAndPlay(gortsplib.Tracks{track}, baseURL)
	if err != nil {
		return nil, multierr.Combine(err, h264RawDec.Close(), c.Close())
	}
	// read the image from shared memory when it is requested
	cancelCtx, cancel := context.WithCancel(context.Background())
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
	rtspCam := &rtspCamera{reader, c, h264RawDec, cancel}
	cameraModel := &transform.PinholeCameraModel{attrs.IntrinsicParams, attrs.DistortionParams}
	return camera.NewFromReader(ctx, rtspCam, cameraModel, camera.ColorStream)
}

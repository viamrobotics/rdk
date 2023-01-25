// Package rtsp implements an RTSP camera client for RDK
package rtsp

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"strings"
	"sync/atomic"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/liberrors"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/rtp"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("rtsp")

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
		camera.Subtype,
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

// Validate checks to see if the attributes of the model are valid.
func (at *Attrs) Validate() error {
	if prefix := strings.HasPrefix(at.Address, "rtsp://"); !prefix {
		return errors.New(`rtsp_address must begin with "rtsp://"`)
	}
	if err := at.IntrinsicParams.CheckValid(); err != nil {
		return err
	}
	if err := at.DistortionParams.CheckValid(); err != nil {
		return err
	}
	return nil
}

// rtspCamera contains the rtsp client, and the reader function that fulfills the camera interface.
type rtspCamera struct {
	gostream.VideoReader
	client     *gortsplib.Client
	cancelFunc context.CancelFunc
}

// Close closes the camera.
func (rc *rtspCamera) Close(ctx context.Context) error {
	var clientTerminated liberrors.ErrClientTerminated
	rc.cancelFunc()
	if err := rc.client.Close(); err != nil && !errors.Is(err, clientTerminated) {
		return err
	}
	return nil
}

// NewRTSPCamera creates a camera client for an RTSP given given the server URL.
// Right now, only supports servers that have MJPEG video tracks.
func NewRTSPCamera(ctx context.Context, attrs *Attrs, logger golog.Logger) (camera.Camera, error) {
	c := &gortsplib.Client{}
	// parse URL
	u, err := url.Parse(attrs.Address)
	if err != nil {
		return nil, err
	}
	// connect to the server - be sure to close it if setup fails.
	var clientSuccessful bool
	var clientErr error
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !clientSuccessful {
			clientErr = c.Close()
		}
	}()
	// find published tracks
	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		return nil, multierr.Combine(err, c.Close())
	}

	// find the MJPEG track
	var mjpeg *format.MJPEG
	media := tracks.FindFormat(&mjpeg)
	if media == nil {
		err := errors.Errorf("MJPEG track not found in rtsp camera %s", u)
		return nil, multierr.Combine(err, clientErr)
	}
	// get the RTP->MJPEG decoder
	rtpDec := mjpeg.CreateDecoder()

	// Setup the MJPEG track
	_, err = c.Setup(media, baseURL, 0, 0)
	if err != nil {
		return nil, multierr.Combine(err, clientErr)
	}
	// On packet retreival, turn it into an image, and store it in shared memory
	var latestFrame atomic.Value
	var gotFirstFrameOnce bool
	gotFirstFrame := make(chan struct{})
	c.OnPacketRTP(media, mjpeg, func(pkt *rtp.Packet) {
		// convert RTP packets into NALUs
		encoded, _, err := rtpDec.Decode(pkt)
		if err != nil {
			return
		}
		// convert JPEG images to image.Image
		img, err := jpeg.Decode(bytes.NewReader(encoded))
		if err != nil {
			panic(err)
		}

		// wait for a frame
		if img == nil {
			return
		}
		latestFrame.Store(img)
		if !gotFirstFrameOnce {
			close(gotFirstFrame)
			gotFirstFrameOnce = true
		}
	})
	// play the track
	_, err = c.Play(nil)
	if err != nil {
		return nil, multierr.Combine(err, clientErr)
	}
	clientSuccessful = true
	// read the image from shared memory when it is requested
	cancelCtx, cancel := context.WithCancel(context.Background())
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		select {
		case <-cancelCtx.Done():
			return nil, nil, cancelCtx.Err()
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}
		<-gotFirstFrame // block until you get the first frame
		return latestFrame.Load().(image.Image), func() {}, nil
	})
	// define and return the camera
	rtspCam := &rtspCamera{reader, c, cancel}
	cameraModel := &transform.PinholeCameraModel{attrs.IntrinsicParams, attrs.DistortionParams}
	return camera.NewFromReader(ctx, rtspCam, cameraModel, camera.ColorStream)
}

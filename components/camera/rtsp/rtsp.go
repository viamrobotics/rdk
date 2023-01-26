// Package rtsp implements an RTSP camera client for RDK
package rtsp

import (
	"context"
	"image"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/liberrors"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/rtp"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"

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
	u                       *url.URL
	client                  *gortsplib.Client
	cancelCtx               context.Context
	cancelFunc              context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
	latestFrame             atomic.Value
	logger                  golog.Logger
}

// Close closes the camera.
func (rc *rtspCamera) Close(ctx context.Context) error {
	var clientTerminated liberrors.ErrClientTerminated
	rc.cancelFunc()
	rc.activeBackgroundWorkers.Wait()
	if err := rc.client.Close(); err != nil && !errors.Is(err, clientTerminated) {
		return err
	}
	return nil
}

// clientReconnectBackgroundWorker checks every 5 sec to see if the client is connected to the server, and reconnects if not.
func (rc *rtspCamera) clientReconnectBackgroundWorker() {
	rc.activeBackgroundWorkers.Add(1)
	ticker := time.NewTicker(5 * time.Second)
	goutils.ManagedGo(func() {
		for {
			select {
			case <-rc.cancelCtx.Done():
				return
			case <-ticker.C:
				// check if the client is still connected with an OPTIONS request
				res, err := rc.client.Options(rc.u)
				if err != nil && (errors.Is(err, liberrors.ErrClientTerminated{}) ||
					strings.Contains(err.Error(), "EOF") ||
					strings.Contains(err.Error(), "connection refused") ||
					strings.Contains(err.Error(), "broken pipe")) {
					rc.logger.Warnf("The rtsp client for %s has error %s", rc.u, err)
					if err = restartClient(rc); err != nil {
						rc.logger.Warnf("cannot reconnect to rtsp server: %s", err)
					} else {
						rc.logger.Infof("reconnected to rtsp server %s", rc.u)
					}
				} else if res != nil && res.StatusCode != base.StatusOK {
					rc.logger.Warnf("The rtsp server responded with %s, trying to reconnect", res.StatusCode)
					if err = restartClient(rc); err != nil {
						rc.logger.Warnf("cannot reconnect to rtsp server: %s", err)
					} else {
						rc.logger.Infof("reconnected to rtsp server %s", rc.u)
					}
				}
			}
		}
	}, rc.activeBackgroundWorkers.Done)
}

// NewRTSPCamera creates a camera client for an RTSP given given the server URL.
// Right now, only supports servers that have MJPEG video tracks.
func NewRTSPCamera(ctx context.Context, attrs *Attrs, logger golog.Logger) (camera.Camera, error) {
	// parse URL
	u, err := url.Parse(attrs.Address)
	if err != nil {
		return nil, err
	}
	client := &gortsplib.Client{}
	// connect to the server - be sure to close it if setup fails.
	var clientSuccessful bool
	var clientErr error
	err = client.Start(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !clientSuccessful {
			clientErr = client.Close()
		}
	}()
	// get MJPEG decodings
	mjpegFormat, mjpegDecoder := mjpegDecoding()

	// find published tracks, and MJPEG specifically
	tracks, baseURL, _, err := client.Describe(u)
	if err != nil {
		return nil, multierr.Combine(err, clientErr)
	}
	track := tracks.FindFormat(&mjpegFormat)
	if track == nil {
		err := errors.Errorf("MJPEG track not found in rtsp camera %s", u)
		return nil, multierr.Combine(err, clientErr)
	}
	// Setup the MJPEG track
	_, err = client.Setup(track, baseURL, 0, 0)
	if err != nil {
		return nil, multierr.Combine(err, clientErr)
	}
	// define the camera
	rtspCam := &rtspCamera{
		u:      u,
		logger: logger,
	}
	var gotFirstFrameOnce bool
	gotFirstFrame := make(chan struct{})
	// On packet retreival, turn it into an image, and store it in shared memory
	client.OnPacketRTP(track, mjpegFormat, func(pkt *rtp.Packet) {
		img, err := mjpegDecoder(pkt)
		if err != nil {
			return
		}
		// wait for a frame
		if img == nil {
			return
		}
		rtspCam.latestFrame.Store(img)
		if !gotFirstFrameOnce {
			close(gotFirstFrame)
			gotFirstFrameOnce = true
		}
	})
	// play the track
	_, err = client.Play(nil)
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
		return rtspCam.latestFrame.Load().(image.Image), func() {}, nil
	})
	// define and return the camera
	rtspCam.VideoReader = reader
	rtspCam.client = client
	rtspCam.cancelCtx = cancelCtx
	rtspCam.cancelFunc = cancel
	cameraModel := &transform.PinholeCameraModel{attrs.IntrinsicParams, attrs.DistortionParams}
	rtspCam.clientReconnectBackgroundWorker()
	return camera.NewFromReader(ctx, rtspCam, cameraModel, camera.ColorStream)
}

func restartClient(rc *rtspCamera) error {
	err := rc.client.Close()
	if err != nil {
		rc.logger.Debugf("error closing while trying to restart client: %s", err)
	}
	// replace the client with a new one
	client := &gortsplib.Client{}
	rc.client = client
	// restart the client
	err = rc.client.Start(rc.u.Scheme, rc.u.Host)
	if err != nil {
		return err
	}
	// get MJPEG decodings
	mjpegFormat, mjpegDecoder := mjpegDecoding()

	// find published tracks, and MJPEG specifically
	tracks, baseURL, _, err := rc.client.Describe(rc.u)
	if err != nil {
		return err
	}
	track := tracks.FindFormat(&mjpegFormat)
	if track == nil {
		return errors.New("MJPEG track not found")
	}
	// Setup the MJPEG track
	_, err = rc.client.Setup(track, baseURL, 0, 0)
	if err != nil {
		return err
	}
	// On packet retreival, turn it into an image, and store it in shared memory
	rc.client.OnPacketRTP(track, mjpegFormat, func(pkt *rtp.Packet) {
		img, err := mjpegDecoder(pkt)
		if err != nil {
			return
		}
		// wait for a frame
		if img == nil {
			return
		}
		rc.latestFrame.Store(img)
	})
	// play the track
	_, err = rc.client.Play(nil)
	if err != nil {
		return err
	}
	return nil
}

//go:build !no_media

// Package rtsp implements an RTSP camera client for RDK
package rtsp

import (
	"context"
	"image"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/liberrors"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/edaniels/golog"
	"github.com/pion/rtp"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

var model = resource.DefaultModelFamily.WithModel("rtsp")

func init() {
	resource.RegisterComponent(camera.API, model, resource.Registration[camera.Camera, *Config]{
		Constructor: func(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (camera.Camera, error) {
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
func (conf *Config) Validate(path string) ([]string, error) {
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

// rtspCamera contains the rtsp client, and the reader function that fulfills the camera interface.
type rtspCamera struct {
	gostream.VideoReader
	u                       *url.URL
	client                  *gortsplib.Client
	cancelCtx               context.Context
	cancelFunc              context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
	gotFirstFrameOnce       bool
	gotFirstFrame           chan struct{}
	latestFrame             atomic.Pointer[image.Image]
	logger                  golog.Logger
}

// Close closes the camera. It always returns nil, but because of Close() interface, it needs to return an error.
func (rc *rtspCamera) Close(ctx context.Context) error {
	rc.cancelFunc()
	rc.activeBackgroundWorkers.Wait()
	if err := rc.client.Close(); err != nil && !errors.Is(err, liberrors.ErrClientTerminated{}) {
		rc.logger.Infow("error while closing rtsp client:", "error", err)
	}
	return nil
}

// clientReconnectBackgroundWorker checks every 5 sec to see if the client is connected to the server, and reconnects if not.
func (rc *rtspCamera) clientReconnectBackgroundWorker() {
	rc.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		for {
			if ok := goutils.SelectContextOrWait(rc.cancelCtx, 5*time.Second); ok {
				// use an OPTIONS request to see if the server is still responding to requests
				res, err := rc.client.Options(rc.u)
				badState := false
				if err != nil && (errors.Is(err, liberrors.ErrClientTerminated{}) ||
					errors.Is(err, io.EOF) ||
					errors.Is(err, syscall.EPIPE) ||
					errors.Is(err, syscall.ECONNREFUSED)) {
					rc.logger.Warnw("The rtsp client encountered an error, trying to reconnect", "url", rc.u, "error", err)
					badState = true
				} else if res != nil && res.StatusCode != base.StatusOK {
					rc.logger.Warnw("The rtsp server responded with non-OK status", "url", rc.u, "status code", res.StatusCode)
					badState = true
				}
				if badState {
					if err = rc.reconnectClient(); err != nil {
						rc.logger.Warnw("cannot reconnect to rtsp server", "error", err)
					} else {
						rc.logger.Infow("reconnected to rtsp server", "url", rc.u)
					}
				}
			} else {
				return
			}
		}
	}, rc.activeBackgroundWorkers.Done)
}

// reconnectClient reconnects the RTSP client to the streaming server by closing the old one and starting a new one.
func (rc *rtspCamera) reconnectClient() (err error) {
	if rc == nil {
		return errors.New("rtspCamera is nil")
	}
	if rc.client != nil {
		err := rc.client.Close()
		if err != nil {
			rc.logger.Debugw("error while closing rtsp client:", "error", err)
		}
	}
	// replace the client with a new one, but close it if setup is not successful
	client := &gortsplib.Client{}
	rc.client = client
	var clientSuccessful bool
	defer func() {
		if !clientSuccessful {
			if errClose := rc.client.Close(); errClose != nil {
				err = multierr.Combine(err, errClose)
			}
		}
	}()
	err = rc.client.Start(rc.u.Scheme, rc.u.Host)
	if err != nil {
		return err
	}
	mjpegFormat, mjpegDecoder := mjpegDecoding()
	tracks, baseURL, _, err := rc.client.Describe(rc.u)
	if err != nil {
		return err
	}
	track := tracks.FindFormat(&mjpegFormat)
	if track == nil {
		return errors.New("MJPEG track not found")
	}
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
		if img == nil {
			return
		}
		rc.latestFrame.Store(&img)
		if !rc.gotFirstFrameOnce {
			rc.gotFirstFrameOnce = true
			close(rc.gotFirstFrame)
		}
	})
	_, err = rc.client.Play(nil)
	if err != nil {
		return err
	}
	clientSuccessful = true
	return nil
}

// NewRTSPCamera creates a camera client using RTSP given the server URL.
// Right now, only supports servers that have MJPEG video tracks.
func NewRTSPCamera(ctx context.Context, name resource.Name, conf *Config, logger golog.Logger) (camera.Camera, error) {
	u, err := url.Parse(conf.Address)
	if err != nil {
		return nil, err
	}
	gotFirstFrame := make(chan struct{})
	rtspCam := &rtspCamera{
		u:             u,
		logger:        logger,
		gotFirstFrame: gotFirstFrame,
	}
	err = rtspCam.reconnectClient()
	if err != nil {
		return nil, err
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		select { // First select block always ensures the cancellations are listened to.
		case <-cancelCtx.Done():
			return nil, nil, cancelCtx.Err()
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}
		select { // if gotFirstFrame is closed, this case will almost always fire and not respect the cancelation.
		case <-cancelCtx.Done():
			return nil, nil, cancelCtx.Err()
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-rtspCam.gotFirstFrame:
		}
		latest := rtspCam.latestFrame.Load()
		if latest == nil {
			return nil, func() {}, errors.New("no frame yet")
		}
		return *latest, func() {}, nil
	})
	rtspCam.VideoReader = reader
	rtspCam.cancelCtx = cancelCtx
	rtspCam.cancelFunc = cancel
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(conf.IntrinsicParams, conf.DistortionParams)
	rtspCam.clientReconnectBackgroundWorker()
	src, err := camera.NewVideoSourceFromReader(ctx, rtspCam, &cameraModel, camera.ColorStream)
	if err != nil {
		return nil, err
	}
	return camera.FromVideoSource(name, src, logger), nil
}

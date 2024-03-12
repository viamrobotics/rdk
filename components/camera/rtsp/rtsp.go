// Package rtsp implements an RTSP camera client for RDK
package rtsp

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
	"github.com/pion/rtp"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/rtsp/formatprocessor"
	"go.viam.com/rdk/components/camera/rtsp/unit"
	"go.viam.com/rdk/gostream"
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
	H264Passthrough  bool                               `json:"h264_passthrough"`
	IntrinsicParams  *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParams *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
}

// Validate checks to see if the attributes of the model are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	_, err := base.ParseURL(conf.Address)
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

type (
	unitSubscriberFunc func(unit.Unit) error
	rtspCamera         struct {
		gostream.VideoReader
		u                       *base.URL
		client                  *gortsplib.Client
		cancelCtx               context.Context
		cancelFunc              context.CancelFunc
		activeBackgroundWorkers sync.WaitGroup
		gotFirstFrameOnce       bool
		gotFirstFrame           chan struct{}
		latestFrame             atomic.Pointer[image.Image]
		logger                  logging.Logger
		h264Passthrough         bool
		subsMu                  sync.RWMutex
		subs                    map[*camera.StreamSubscription]unitSubscriberFunc
	}
)

// Close closes the camera. It always returns nil, but because of Close() interface, it needs to return an error.
func (rc *rtspCamera) Close(ctx context.Context) error {
	rc.cancelFunc()
	rc.client.Close()
	rc.activeBackgroundWorkers.Wait()
	rc.unsubscribeAll()
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
		rc.client.Close()
	}
	// replace the client with a new one, but close it if setup is not successful
	client := &gortsplib.Client{}
	rc.client = client
	// On packet retreival, turn it into an image, and store it in shared memory
	rc.client.OnPacketLost = func(err error) {
		rc.logger.Warnf("OnPacketLost: err: %s", err.Error())
	}
	rc.client.OnTransportSwitch = func(err error) {
		rc.logger.Infof("OnTransportSwitch: err: %s", err.Error())
	}
	rc.client.OnDecodeError = func(err error) {
		rc.logger.Warnf("OnDecodeError: err: %s", err.Error())
	}
	rc.logger.Info("calling start")
	err = rc.client.Start(rc.u.Scheme, rc.u.Host)
	if err != nil {
		return err
	}
	var clientSuccessful bool
	defer func() {
		if !clientSuccessful {
			rc.client.Close()
		}
	}()
	rc.logger.Info("REQ Describe")
	desc, resp, err := rc.client.Describe(rc.u)
	if err != nil {
		return err
	}
	ms := []description.Media{}
	for _, m := range desc.Medias {
		ms = append(ms, *m)
	}
	rc.logger.Infof("RES Describe, desc: %# ", desc)
	rc.logger.Infof("RES Describe, ms: %#v", ms)
	rc.logger.Infof("RES Describe, resp: %#v ", resp)

	var forma format.Format
	var media *description.Media
	var fp formatprocessor.Processor
	var onPacketRTPFunc func(pkt *rtp.Packet)
	if rc.h264Passthrough {
		var f *format.H264
		media = desc.FindFormat(&f)
		if media == nil {
			return errors.New("H264 media not found")
		}
		forma = f

		fp, err = formatprocessor.New(1472, forma, true)
		if err != nil {
			return err
		}
		onPacketRTPFunc = func(pkt *rtp.Packet) {
			pts, ok := rc.client.PacketPTS(media, pkt)
			if !ok {
				return
			}
			ntp := time.Now()
			// NOTE(NickS): Why is this false?
			u, err := fp.ProcessRTPPacket(pkt, ntp, pts, false)
			if err != nil {
				rc.logger.Debug(err.Error())
				return
			}
			rc.subsMu.RLock()
			defer rc.subsMu.RUnlock()
			if len(rc.subs) == 0 {
				// no subscribers, dropping packets on the floor
				return
			}
			// Publish the newly received packet Unit to all subscribers
			for sub, cb := range rc.subs {
				if err := sub.Publish(func() error { return cb(u) }); err != nil {
					rc.logger.Debug("RTP packet dropped due to %s", err.Error())
				}
			}
		}
	} else {
		var f *format.MJPEG
		media = desc.FindFormat(&f)
		if media == nil {
			return errors.New("MJPEG track not found")
		}
		forma = f
		// get the RTP->MJPEG decoder
		rtpMjpegDec, err := f.CreateDecoder()
		if err != nil {
			return err
		}

		onPacketRTPFunc = func(pkt *rtp.Packet) {
			jpegEncodedBytes, err := rtpMjpegDec.Decode(pkt)
			if err != nil {
				err = errors.Wrap(err, "rtp to mjpeg decoding failed")
				rc.logger.Debug(err.Error())
				return
			}

			img, err := jpeg.Decode(bytes.NewReader(jpegEncodedBytes))
			if err != nil {
				err = errors.Wrap(err, "jpeg to image.Image decoding failed")
				rc.logger.Debug(err.Error())
				return
			}
			if img == nil {
				rc.logger.Debug("jpeg.Decode returned no error but also no image")
				return
			}
			rc.latestFrame.Store(&img)
			if !rc.gotFirstFrameOnce {
				rc.gotFirstFrameOnce = true
				close(rc.gotFirstFrame)
			}
		}
	}
	rc.logger.Infof("url: %s, media: %#v", desc.BaseURL.String(), media)
	res, err := rc.client.Setup(desc.BaseURL, media, 0, 0)
	if err != nil {
		rc.logger.Fatal(err.Error())
		return err
	}
	rc.logger.Infof("Setup: res: %#v", res)

	rc.client.OnPacketRTP(media, forma, onPacketRTPFunc)
	_, err = rc.client.Play(nil)
	if err != nil {
		return err
	}
	clientSuccessful = true
	return nil
}

// SubscribeRTP registers the PacketCallback which will be called when there are new packets.
// NOTE: Packets may be dropped before calling packetsCB if the rate new packets are received by
// the VideoCodecStream is greater than the rate the subscriber consumes them.
func (rc *rtspCamera) SubscribeRTP(r *camera.StreamSubscription, packetsCB camera.PacketCallback) error {
	webrtcPayloadMaxSize := 1188 // 1200 - 12 (RTP header)
	encoder := &rtph264.Encoder{
		PayloadType:    96,
		PayloadMaxSize: webrtcPayloadMaxSize,
	}

	if err := encoder.Init(); err != nil {
		return err
	}

	var firstReceived bool
	var lastPTS time.Duration
	// OnPacketRTP will call this unitSubscriberFunc for all subscribers.
	// unitSubscriberFunc will then convert the Unit into a slice of
	// WebRTC compliant RTP packets & call packetsCB, which will
	// allow the caller of SubscribeRTP to handle the packets.
	// This is intended to free the SubscribeRTP caller from needing
	// to care about how to transform RTSP compliant RTP packets into
	// WebRTC compliant RTP packets.
	unitSubscriberFunc := func(u unit.Unit) error {
		tunit, ok := u.(*unit.H264)
		if !ok {
			return errors.New("(*unit.H264) type conversion error")
		}

		// If we have no AUs we can't encode packets.
		if tunit.AU == nil {
			return nil
		}

		if !firstReceived {
			firstReceived = true
		} else if tunit.PTS < lastPTS {
			return errors.New("WebRTC doesn't support H264 streams with B-frames")
		}
		lastPTS = tunit.PTS

		pkts, err := encoder.Encode(tunit.AU)
		if err != nil {
			// If there is an Encode error we just drop the packets.
			return nil //nolint:nilerr
		}

		if len(pkts) == 0 {
			// If no packets can be encoded from the AU, there is no need to call the subscriber's callback.
			return nil
		}

		for _, pkt := range pkts {
			pkt.Timestamp += tunit.RTPPackets[0].Timestamp
		}

		return packetsCB(pkts)
	}

	rc.subsMu.Lock()
	defer rc.subsMu.Unlock()
	rc.subs[r] = unitSubscriberFunc
	r.Start()
	return nil
}

func (rc *rtspCamera) unsubscribeAll() {
	rc.subsMu.Lock()
	defer rc.subsMu.Unlock()
	for r := range rc.subs {
		r.Stop()
		delete(rc.subs, r)
	}
}

// Unsubscribe deregisters the StreamSubscription's callback.
func (rc *rtspCamera) Unsubscribe(r *camera.StreamSubscription) {
	rc.subsMu.Lock()
	defer rc.subsMu.Unlock()
	r.Stop()
	delete(rc.subs, r)
}

// NewRTSPCamera creates a camera client using RTSP given the server URL.
// Right now, only supports servers that have MJPEG video tracks.
func NewRTSPCamera(ctx context.Context, name resource.Name, conf *Config, logger logging.Logger) (camera.Camera, error) {
	u, err := base.ParseURL(conf.Address)
	if err != nil {
		return nil, err
	}
	rtspCam := &rtspCamera{
		u:               u,
		h264Passthrough: conf.H264Passthrough,
		logger:          logger,
		subs:            make(map[*camera.StreamSubscription]unitSubscriberFunc),
		gotFirstFrame:   make(chan struct{}),
	}
	err = rtspCam.reconnectClient()
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	f := func(ctx context.Context) (image.Image, func(), error) {
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
	}

	if rtspCam.h264Passthrough {
		f = func(ctx context.Context) (image.Image, func(), error) {
			return nil, func() {}, errors.New("builtin RTSP camera.GetImage method unimplemented when H264Passthrough enabled")
		}
	}

	reader := gostream.VideoReaderFunc(f)

	rtspCam.VideoReader = reader
	rtspCam.cancelCtx = cancelCtx
	rtspCam.cancelFunc = cancel
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(conf.IntrinsicParams, conf.DistortionParams)
	rtspCam.clientReconnectBackgroundWorker()
	src, err := camera.NewVideoSourceFromReader(ctx, rtspCam, &cameraModel, camera.ColorStream, rtspCam.h264Passthrough)
	if err != nil {
		return nil, err
	}
	return camera.FromVideoSource(name, src, logger), nil
}

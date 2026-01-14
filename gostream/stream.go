// Package gostream implements a simple server for serving video streams over WebRTC.
package gostream

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/rtp"
	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils"

	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	utils2 "go.viam.com/rdk/utils"
)

const (
	defaultTargetFrameRate = 20
)

// A Stream is sink that accepts any image frames for the purpose
// of displaying in a WebRTC video track.
type Stream interface {
	internalStream

	Name() string

	// Start starts processing frames.
	Start()
	WriteRTP(pkt *rtp.Packet) error

	// Ready signals that there is at least one client connected and that
	// streams are ready for input. The returned context should be used for
	// signaling that streaming is no longer ready.
	StreamingReady() (<-chan struct{}, context.Context)

	InputVideoFrames(props prop.Video) (chan<- MediaReleasePair[image.Image], error)

	// Stop stops further processing of frames.
	Stop()
}

type internalStream interface {
	VideoTrackLocal() (webrtc.TrackLocal, bool)
}

// MediaReleasePair associates a media with a corresponding
// function to release its resources once the receiver of a
// pair is finished with the media.
type MediaReleasePair[T any] struct {
	Media   T
	Release func()
}

// NewStream returns a newly configured stream that can begin to handle
// new connections.
func NewStream(config StreamConfig, logger logging.Logger) (Stream, error) {
	if config.VideoEncoderFactory == nil {
		return nil, errors.New("video encoder factory must be set")
	}
	if config.TargetFrameRate == 0 {
		config.TargetFrameRate = defaultTargetFrameRate
	}

	name := config.Name
	if name == "" {
		name = uuid.NewString()
	}

	var trackLocal *trackLocalStaticSample
	if config.VideoEncoderFactory != nil {
		trackLocal = newVideoTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: config.VideoEncoderFactory.MIMEType()},
			"video",
			name,
		)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	bs := &basicStream{
		name:             name,
		config:           config,
		streamingReadyCh: make(chan struct{}),

		videoTrackLocal: trackLocal,
		inputImageChan:  make(chan MediaReleasePair[image.Image]),
		outputVideoChan: make(chan []byte),

		logger:            logger,
		shutdownCtx:       ctx,
		shutdownCtxCancel: cancelFunc,
	}

	return bs, nil
}

type basicStream struct {
	mu               sync.RWMutex
	name             string
	config           StreamConfig
	started          bool
	streamingReadyCh chan struct{}

	videoTrackLocal *trackLocalStaticSample
	inputImageChan  chan MediaReleasePair[image.Image]
	outputVideoChan chan []byte
	videoEncoder    codec.VideoEncoder

	shutdownCtx             context.Context
	shutdownCtxCancel       func()
	activeBackgroundWorkers sync.WaitGroup
	logger                  logging.Logger
}

func (bs *basicStream) Name() string {
	return bs.name
}

func (bs *basicStream) Start() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.started {
		return
	}
	bs.started = true
	close(bs.streamingReadyCh)
	// add 2 actviate background workers for the processInput and output frames routines
	bs.activeBackgroundWorkers.Add(2)
	utils.ManagedGo(bs.processInputFrames, bs.activeBackgroundWorkers.Done)
	utils.ManagedGo(bs.processOutputFrames, bs.activeBackgroundWorkers.Done)
}

// NOTE: (Nick S) This only writes video RTP packets
// if we also need to support writing audio RTP packets, we should split
// this method into WriteVideoRTP and WriteAudioRTP.
func (bs *basicStream) WriteRTP(pkt *rtp.Packet) error {
	return bs.videoTrackLocal.rtpTrack.WriteRTP(pkt)
}

func (bs *basicStream) Stop() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !bs.started {
		close(bs.streamingReadyCh)
	}

	bs.started = false
	bs.shutdownCtxCancel()
	bs.activeBackgroundWorkers.Wait()
	if bs.videoEncoder != nil {
		if err := bs.videoEncoder.Close(); err != nil {
			bs.logger.Error(err)
		}
	}

	// reset
	bs.outputVideoChan = make(chan []byte)
	ctx, cancelFunc := context.WithCancel(context.Background())
	bs.shutdownCtx = ctx
	bs.shutdownCtxCancel = cancelFunc
	bs.streamingReadyCh = make(chan struct{})
}

func (bs *basicStream) StreamingReady() (<-chan struct{}, context.Context) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.streamingReadyCh, bs.shutdownCtx
}

func (bs *basicStream) InputVideoFrames(props prop.Video) (chan<- MediaReleasePair[image.Image], error) {
	if bs.config.VideoEncoderFactory == nil {
		return nil, errors.New("no video in stream")
	}
	return bs.inputImageChan, nil
}

func (bs *basicStream) VideoTrackLocal() (webrtc.TrackLocal, bool) {
	return bs.videoTrackLocal, bs.videoTrackLocal != nil
}

func (bs *basicStream) processInputFrames() {
	frameLimiterDur := time.Second / time.Duration(bs.config.TargetFrameRate)
	defer close(bs.outputVideoChan)
	var dx, dy int
	ticker := time.NewTicker(frameLimiterDur)
	defer ticker.Stop()
	for {
		select {
		case <-bs.shutdownCtx.Done():
			return
		default:
		}
		select {
		case <-bs.shutdownCtx.Done():
			return
		case <-ticker.C:
		}
		var framePair MediaReleasePair[image.Image]
		select {
		case framePair = <-bs.inputImageChan:
		case <-bs.shutdownCtx.Done():
			return
		}
		if framePair.Media == nil {
			continue
		}
		var initErr bool
		func() {
			if framePair.Release != nil {
				defer framePair.Release()
			}

			var encodedFrame []byte

			if frame, ok := framePair.Media.(*rimage.LazyEncodedImage); ok && frame.MIMEType() == utils2.MimeTypeH264 {
				encodedFrame = frame.RawData() // nothing to do; already encoded
			} else {
				var bounds image.Rectangle
				var boundsError any
				func() {
					defer func() {
						if paniced := recover(); paniced != nil {
							boundsError = paniced
						}
					}()

					bounds = framePair.Media.Bounds()
				}()

				if boundsError != nil {
					bs.logger.Errorw("Getting frame bounds failed", "err", fmt.Sprintf("%s", boundsError))
					// Dan: It's unclear why we get this error. There's reason to believe this pops
					// up when a camera is reconfigured/removed. In which case I'd expect the
					// `basicStream` to soon be closed. Making this `initErr = true` assignment to
					// exit the `processInputFrames` goroutine unnecessary. But I'm choosing to be
					// conservative for the worst case. Where we may be in a permanent bad state and
					// (until we understand the problem better) we would spew logs until the user
					// stops the stream.
					initErr = true
					return
				}

				newDx, newDy := bounds.Dx(), bounds.Dy()
				if bs.videoEncoder == nil || dx != newDx || dy != newDy {
					dx, dy = newDx, newDy
					bs.logger.Infow("detected new image bounds", "width", dx, "height", dy)

					if err := bs.initVideoCodec(dx, dy); err != nil {
						bs.logger.Error(err)
						initErr = true
						return
					}
				}

				// thread-safe because the size is static
				var err error
				encodedFrame, err = bs.videoEncoder.Encode(bs.shutdownCtx, framePair.Media)
				if err != nil {
					bs.logger.Error(err)
					return
				}
			}

			if encodedFrame != nil {
				select {
				case <-bs.shutdownCtx.Done():
					return
				case bs.outputVideoChan <- encodedFrame:
				}
			}
		}()
		if initErr {
			return
		}
	}
}

func (bs *basicStream) processOutputFrames() {
	framesSent := 0
	for outputFrame := range bs.outputVideoChan {
		select {
		case <-bs.shutdownCtx.Done():
			return
		default:
		}
		now := time.Now()
		if err := bs.videoTrackLocal.WriteData(outputFrame); err != nil {
			bs.logger.Errorw("error writing frame", "error", err)
		}
		framesSent++
		if Debug {
			bs.logger.Debugw("wrote sample", "frames_sent", framesSent, "write_time", time.Since(now))
		}
	}
}

func (bs *basicStream) initVideoCodec(width, height int) error {
	var err error
	bs.videoEncoder, err = bs.config.VideoEncoderFactory.New(width, height, bs.config.TargetFrameRate, bs.logger)
	return err
}

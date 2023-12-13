// Package gostream implements a simple server for serving video streams over WebRTC.
package gostream

import (
	"context"
	"errors"
	"image"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	// register screen drivers.
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/pion/webrtc/v3"
	"go.viam.com/utils"

	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/rimage"
	utils2 "go.viam.com/rdk/utils"
)

// A Stream is sink that accepts any image frames for the purpose
// of displaying in a WebRTC video track.
type Stream interface {
	internalStream

	Name() string

	// Start starts processing frames.
	Start()

	// Ready signals that there is at least one client connected and that
	// streams are ready for input. The returned context should be used for
	// signaling that streaming is no longer ready.
	StreamingReady() (<-chan struct{}, context.Context)

	InputVideoFrames(props prop.Video) (chan<- MediaReleasePair[image.Image], error)

	InputAudioChunks(props prop.Audio) (chan<- MediaReleasePair[wave.Audio], error)

	// Stop stops further processing of frames.
	Stop()
}

type internalStream interface {
	VideoTrackLocal() (webrtc.TrackLocal, bool)
	AudioTrackLocal() (webrtc.TrackLocal, bool)
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
func NewStream(config StreamConfig) (Stream, error) {
	logger := config.Logger
	if logger == nil {
		logger = golog.Global()
	}
	if config.VideoEncoderFactory == nil && config.AudioEncoderFactory == nil {
		return nil, errors.New("at least one audio or video encoder factory must be set")
	}
	if config.TargetFrameRate == 0 {
		config.TargetFrameRate = codec.DefaultKeyFrameInterval
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

	var audioTrackLocal *trackLocalStaticSample
	if config.AudioEncoderFactory != nil {
		audioTrackLocal = newAudioTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: config.AudioEncoderFactory.MIMEType()},
			"audio",
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

		audioTrackLocal: audioTrackLocal,
		inputAudioChan:  make(chan MediaReleasePair[wave.Audio]),
		outputAudioChan: make(chan []byte),

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

	audioTrackLocal *trackLocalStaticSample
	inputAudioChan  chan MediaReleasePair[wave.Audio]
	outputAudioChan chan []byte
	audioEncoder    codec.AudioEncoder

	// audioLatency specifies how long in between audio samples. This must be guaranteed
	// by all streamed audio.
	audioLatency    time.Duration
	audioLatencySet bool

	shutdownCtx             context.Context
	shutdownCtxCancel       func()
	activeBackgroundWorkers sync.WaitGroup
	logger                  golog.Logger
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
	bs.activeBackgroundWorkers.Add(4)
	utils.ManagedGo(bs.processInputFrames, bs.activeBackgroundWorkers.Done)
	utils.ManagedGo(bs.processOutputFrames, bs.activeBackgroundWorkers.Done)
	utils.ManagedGo(bs.processInputAudioChunks, bs.activeBackgroundWorkers.Done)
	utils.ManagedGo(bs.processOutputAudioChunks, bs.activeBackgroundWorkers.Done)
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
	if bs.audioEncoder != nil {
		bs.audioEncoder.Close()
	}

	// reset
	bs.outputVideoChan = make(chan []byte)
	bs.outputAudioChan = make(chan []byte)
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

func (bs *basicStream) InputAudioChunks(props prop.Audio) (chan<- MediaReleasePair[wave.Audio], error) {
	if bs.config.AudioEncoderFactory == nil {
		return nil, errors.New("no audio in stream")
	}
	bs.mu.Lock()
	if bs.audioLatencySet && bs.audioLatency != props.Latency {
		return nil, errors.New("cannot stream audio source with different latencies")
	}
	bs.audioLatencySet = true
	bs.audioLatency = props.Latency
	bs.mu.Unlock()
	return bs.inputAudioChan, nil
}

func (bs *basicStream) VideoTrackLocal() (webrtc.TrackLocal, bool) {
	return bs.videoTrackLocal, bs.videoTrackLocal != nil
}

func (bs *basicStream) AudioTrackLocal() (webrtc.TrackLocal, bool) {
	return bs.audioTrackLocal, bs.audioTrackLocal != nil
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
				bounds := framePair.Media.Bounds()
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

func (bs *basicStream) processInputAudioChunks() {
	defer close(bs.outputAudioChan)
	var samplingRate, channels int
	for {
		select {
		case <-bs.shutdownCtx.Done():
			return
		default:
		}
		var audioChunkPair MediaReleasePair[wave.Audio]
		select {
		case audioChunkPair = <-bs.inputAudioChan:
		case <-bs.shutdownCtx.Done():
			return
		}
		if audioChunkPair.Media == nil {
			continue
		}
		var initErr bool
		func() {
			if audioChunkPair.Release != nil {
				defer audioChunkPair.Release()
			}

			info := audioChunkPair.Media.ChunkInfo()
			newSamplingRate, newChannels := info.SamplingRate, info.Channels
			if samplingRate != newSamplingRate || channels != newChannels {
				samplingRate, channels = newSamplingRate, newChannels
				bs.logger.Infow("detected new audio info", "sampling_rate", samplingRate, "channels", channels)

				bs.audioTrackLocal.setAudioLatency(bs.audioLatency)
				if err := bs.initAudioCodec(samplingRate, channels); err != nil {
					bs.logger.Error(err)
					initErr = true
					return
				}
			}

			encodedChunk, ready, err := bs.audioEncoder.Encode(bs.shutdownCtx, audioChunkPair.Media)
			if err != nil {
				bs.logger.Error(err)
				return
			}
			if ready && encodedChunk != nil {
				select {
				case <-bs.shutdownCtx.Done():
					return
				case bs.outputAudioChan <- encodedChunk:
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

func (bs *basicStream) processOutputAudioChunks() {
	chunksSent := 0
	for outputChunk := range bs.outputAudioChan {
		select {
		case <-bs.shutdownCtx.Done():
			return
		default:
		}
		now := time.Now()
		if err := bs.audioTrackLocal.WriteData(outputChunk); err != nil {
			bs.logger.Errorw("error writing audio chunk", "error", err)
		}
		chunksSent++
		if Debug {
			bs.logger.Debugw("wrote sample", "chunks_sent", chunksSent, "write_time", time.Since(now))
		}
	}
}

func (bs *basicStream) initVideoCodec(width, height int) error {
	var err error
	bs.videoEncoder, err = bs.config.VideoEncoderFactory.New(width, height, bs.config.TargetFrameRate, bs.logger)
	return err
}

func (bs *basicStream) initAudioCodec(sampleRate, channelCount int) error {
	var err error
	if bs.audioEncoder != nil {
		bs.audioEncoder.Close()
	}
	bs.audioEncoder, err = bs.config.AudioEncoderFactory.New(sampleRate, channelCount, bs.audioLatency, bs.logger)
	return err
}

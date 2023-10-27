// Package opus contains the opus video codec.
package opus

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pion/mediadevices/pkg/codec/opus"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"go.viam.com/utils"

	ourcodec "go.viam.com/rdk/gostream/codec"
)

type encoder struct {
	codec                   codec.ReadCloser
	chunkCh                 chan wave.Audio
	encodedCh               chan encodedData
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// Gives suitable results. Probably want to make this configurable this in the future.
const bitrate = 32000

type encodedData struct {
	data []byte
	err  error
}

// NewEncoder returns an Opus encoder that can encode images of the given width and height. It will
// also ensure that it produces key frames at the given interval.
func NewEncoder(sampleRate, channelCount int, latency time.Duration, logger golog.Logger) (ourcodec.AudioEncoder, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	enc := &encoder{
		chunkCh:    make(chan wave.Audio, 1),
		encodedCh:  make(chan encodedData, 1),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	var builder codec.AudioEncoderBuilder
	params, err := opus.NewParams()
	if err != nil {
		return nil, err
	}
	builder = &params
	params.BitRate = bitrate
	params.Latency = opus.Latency(latency)

	codec, err := builder.BuildAudioEncoder(enc, prop.Media{
		Audio: prop.Audio{
			Latency:      latency,
			SampleRate:   sampleRate,
			ChannelCount: channelCount,
		},
	})
	if err != nil {
		return nil, err
	}
	enc.codec = codec

	enc.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			if cancelCtx.Err() != nil {
				return
			}
			data, release, err := enc.codec.Read()
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			release()

			select {
			case <-cancelCtx.Done():
				return
			case enc.encodedCh <- encodedData{dataCopy, err}:
			}
		}
	}, func() {
		defer enc.activeBackgroundWorkers.Done()
		close(enc.encodedCh)
	})

	return enc, nil
}

// Read returns an audio chunk for codec to process.
func (a *encoder) Read() (chunk wave.Audio, release func(), err error) {
	if err := a.cancelCtx.Err(); err != nil {
		return nil, func() {}, err
	}

	select {
	case <-a.cancelCtx.Done():
		return nil, func() {}, io.EOF
	case chunk := <-a.chunkCh:
		return chunk, func() {}, nil
	}
}

// Encode asks the codec to process the given audio chunk.
func (a *encoder) Encode(ctx context.Context, chunk wave.Audio) ([]byte, bool, error) {
	defer func() {
		select {
		case <-ctx.Done():
			return
		case <-a.cancelCtx.Done():
			return
		case a.chunkCh <- chunk:
		}
	}()
	if err := a.cancelCtx.Err(); err != nil {
		return nil, false, err
	}
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case <-a.cancelCtx.Done():
		return nil, false, a.cancelCtx.Err()
	case encoded := <-a.encodedCh:
		if encoded.err != nil {
			return nil, false, encoded.err
		}
		return encoded.data, true, nil
	default:
		return nil, false, nil
	}
}

func (a *encoder) Close() {
	a.cancelFunc()
	a.activeBackgroundWorkers.Wait()
}

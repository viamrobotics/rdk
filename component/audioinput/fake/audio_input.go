// Package fake implements a fake audio input.
package fake

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"

	"go.viam.com/rdk/component/audioinput"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

var _ = audioinput.AudioInput(&audioInput{})

func init() {
	registry.RegisterComponent(
		audioinput.Subtype,
		"fake",
		registry.Component{Constructor: func(
			_ context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			ticker := time.NewTicker(latencyMillis * time.Millisecond)
			input := &audioInput{
				Name:   config.Name,
				toneHz: 440,
				cancel: func() {
					cancelFunc()
					ticker.Stop()
				},
				cancelCtx: cancelCtx,
				tickerC:   ticker.C,
			}
			as := gostream.NewAudioSource(gostream.AudioReaderFunc(input.Read), prop.Audio{
				ChannelCount:  channelCount,
				SampleRate:    samplingRate,
				IsBigEndian:   audioinput.HostEndian == binary.BigEndian,
				IsInterleaved: true,
				Latency:       time.Millisecond * latencyMillis,
			})
			input.AudioSource = as
			return audioinput.NewFromSource(input)
		}})
}

// audioInput is a fake audioinput that always returns the same chunk.
type audioInput struct {
	gostream.AudioSource
	mu        sync.Mutex
	Name      string
	step      int
	toneHz    float64
	cancel    func()
	cancelCtx context.Context
	tickerC   <-chan time.Time
}

const (
	latencyMillis = 20
	samplingRate  = 48000
	channelCount  = 1
)

func (i *audioInput) Read(ctx context.Context) (wave.Audio, func(), error) {
	select {
	case <-i.cancelCtx.Done():
		return nil, nil, i.cancelCtx.Err()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	select {
	case <-i.cancelCtx.Done():
		return nil, nil, i.cancelCtx.Err()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-i.tickerC:
	}
	const length = samplingRate * latencyMillis / 1000
	const numChunks = samplingRate / length
	angle := math.Pi * 2 / (float64(length) * numChunks)

	i.mu.Lock()
	toneHz := i.toneHz
	i.mu.Unlock()

	i.step = (i.step + 1) % numChunks
	chunk := wave.NewFloat32Interleaved(wave.ChunkInfo{
		Len:          length,
		Channels:     channelCount,
		SamplingRate: samplingRate,
	})

	for sample := 0; sample < length; sample++ {
		val := wave.Float32Sample(math.Sin(angle * toneHz * (float64((length * i.step) + sample))))
		chunk.Set(sample, 0, val)
	}
	return chunk, func() {}, nil
}

func (i *audioInput) MediaProperties(_ context.Context) (prop.Audio, error) {
	return prop.Audio{
		ChannelCount:  channelCount,
		SampleRate:    samplingRate,
		IsBigEndian:   audioinput.HostEndian == binary.BigEndian,
		IsInterleaved: true,
		Latency:       time.Millisecond * latencyMillis,
	}, nil
}

// Do allows setting of tone.
func (i *audioInput) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	newTone, ok := cmd["set_tone_hz"].(float64)
	if !ok {
		return map[string]interface{}{}, nil
	}
	oldTone := i.toneHz
	i.toneHz = newTone
	return map[string]interface{}{"prev_tone_hz": oldTone}, nil
}

// Close stops the generator routine.
func (i *audioInput) Close(ctx context.Context) error {
	i.cancel()
	return i.AudioSource.Close(ctx)
}

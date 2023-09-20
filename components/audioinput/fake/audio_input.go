//go:build !no_cgo

// Package fake implements a fake audio input.
package fake

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/viamrobotics/gostream"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/resource"
)

func init() {
	resource.RegisterComponent(
		audioinput.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[audioinput.AudioInput, resource.NoNativeConfig]{Constructor: func(
			_ context.Context,
			_ resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (audioinput.AudioInput, error) {
			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			var condMu sync.RWMutex
			cond := sync.NewCond(condMu.RLocker())
			input := &audioInput{
				Named:     conf.ResourceName().AsNamed(),
				toneHz:    440,
				cancel:    cancelFunc,
				cancelCtx: cancelCtx,
				cond:      cond,
			}
			input.activeBackgroundWorkers.Add(1)
			utils.ManagedGo(func() {
				ticker := time.NewTicker(latencyMillis * time.Millisecond)
				for {
					if !utils.SelectContextOrWaitChan(cancelCtx, ticker.C) {
						return
					}
					atomic.AddInt64(&input.step, 1)
					cond.Broadcast()
				}
			}, input.activeBackgroundWorkers.Done)
			as := gostream.NewAudioSource(gostream.AudioReaderFunc(input.Read), prop.Audio{
				ChannelCount:  channelCount,
				SampleRate:    samplingRate,
				IsBigEndian:   audioinput.HostEndian == binary.BigEndian,
				IsInterleaved: true,
				Latency:       time.Millisecond * latencyMillis,
			})
			input.AudioSource = as
			return audioinput.FromAudioSource(conf.ResourceName(), input)
		}})
}

// audioInput is a fake audioinput that always returns the same chunk.
type audioInput struct {
	resource.Named
	resource.TriviallyReconfigurable
	gostream.AudioSource
	mu                      sync.RWMutex
	step                    int64
	toneHz                  float64
	cancel                  func()
	cancelCtx               context.Context
	activeBackgroundWorkers sync.WaitGroup
	cond                    *sync.Cond
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

	i.cond.L.Lock()
	i.cond.Wait()
	i.cond.L.Unlock()

	select {
	case <-i.cancelCtx.Done():
		return nil, nil, i.cancelCtx.Err()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	const length = samplingRate * latencyMillis / 1000
	const numChunks = samplingRate / length
	angle := math.Pi * 2 / (float64(length) * numChunks)

	i.mu.RLock()
	toneHz := i.toneHz
	i.mu.RUnlock()

	step := int(atomic.LoadInt64(&i.step) % numChunks)
	chunk := wave.NewFloat32Interleaved(wave.ChunkInfo{
		Len:          length,
		Channels:     channelCount,
		SamplingRate: samplingRate,
	})

	for sample := 0; sample < length; sample++ {
		val := wave.Float32Sample(math.Sin(angle * toneHz * (float64((length * step) + sample))))
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

// DoCommand allows setting of tone.
func (i *audioInput) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
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
	i.activeBackgroundWorkers.Wait()
	i.cond.L.Lock()
	i.cond.Signal()
	i.cond.L.Unlock()
	return i.AudioSource.Close(ctx)
}

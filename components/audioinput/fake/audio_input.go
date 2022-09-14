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
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var _ = audioinput.AudioInput(&audioInput{})

func init() {
	registry.RegisterComponent(
		audioinput.Subtype,
		resource.NewDefaultModel("fake"),
		registry.Component{Constructor: func(
			_ context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			cancelCtx, cancelFunc := context.WithCancel(context.Background())
			condMu := &sync.RWMutex{}
			cond := sync.NewCond(condMu.RLocker())
			input := &audioInput{
				Name:      config.Name,
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
			return audioinput.NewFromSource(input)
		}})
}

// audioInput is a fake audioinput that always returns the same chunk.
type audioInput struct {
	gostream.AudioSource
	mu                      sync.RWMutex
	Name                    string
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
	return i.AudioSource.Close(ctx)
}

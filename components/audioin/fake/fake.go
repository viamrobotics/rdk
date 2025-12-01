// Package fake implements a fake audio in.
package fake

import (
	"context"
	"fmt"
	"math"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/audioin"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterComponent(
		audioin.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[audioin.AudioIn, *Config]{Constructor: NewAudioIn})
}

const (
	defaultSampleRate = 44100
	defaultChannels   = 1
)

// AudioIn is a fake AudioIn
type AudioIn struct {
	resource.Named
	resource.AlwaysRebuild
	Geometry        []spatialmath.Geometry
	logger          logging.Logger
	sampleRate      int
	numChannels     int
	supportedCodecs []string
	workers         goutils.StoppableWorkers
}

// A Config describes the configuration of a fake board and all of its connected parts.
type Config struct {
	SampleRate  int `json:"sample_rate,omitempty"`
	NumChannels int `json:"num_channels,omitempty"`
}

// Validate validates the config.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	if conf.SampleRate != 0 && conf.SampleRate <= 0 {
		return nil, nil, fmt.Errorf("sample_rate must be greater than 0 if provided, got %d", conf.SampleRate)
	}
	if conf.NumChannels != 0 && conf.NumChannels <= 0 {
		return nil, nil, fmt.Errorf("num_channels must be greater than 0 if provided, got %d", conf.NumChannels)
	}
	return nil, nil, nil
}

// NewAudioIn instantiates a new AudioIn of the fake model type.
func NewAudioIn(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger) (audioin.AudioIn, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	sampleRate := defaultSampleRate
	if newConf.SampleRate > 0 {
		sampleRate = newConf.SampleRate
	}

	numChannels := defaultChannels
	if newConf.NumChannels > 0 {
		numChannels = newConf.NumChannels
	}

	a := &AudioIn{
		Named:           conf.ResourceName().AsNamed(),
		Geometry:        []spatialmath.Geometry{},
		logger:          logger,
		sampleRate:      sampleRate,
		numChannels:     numChannels,
		supportedCodecs: []string{"pcm16"},
		workers:         *goutils.NewBackgroundStoppableWorkers(),
	}

	return a, nil
}

func (a *AudioIn) generateAudioChunk(sequence int32, currentTime time.Time) *audioin.AudioChunk {
	chunkDurationMs := 100 // 100ms per chunk
	samplesPerChunk := a.sampleRate * chunkDurationMs / 1000

	// Allocate buffer for PCM16 audio data filled with zeros (silence)
	// Each sample is 2 bytes (int16), and we have numChannels channels
	chunkData := make([]byte, samplesPerChunk*2*a.numChannels)

	startTimeNs := currentTime.UnixNano()
	chunkDurationNs := int64(chunkDurationMs * 1e6)
	endTimeNs := startTimeNs + chunkDurationNs

	return &audioin.AudioChunk{
		AudioData:                 chunkData,
		AudioInfo:                 &rutils.AudioInfo{Codec: "pcm16", SampleRateHz: int32(a.sampleRate), NumChannels: int32(a.numChannels)},
		StartTimestampNanoseconds: startTimeNs,
		EndTimestampNanoseconds:   endTimeNs,
		Sequence:                  sequence,
	}
}

// GetAudio streams audio to the client
func (a *AudioIn) GetAudio(ctx context.Context,
	codec string, durationSeconds float32,
	previousTimestampNs int64,
	extra map[string]interface{}) (
	chan *audioin.AudioChunk, error,
) {
	chunkChan := make(chan *audioin.AudioChunk)

	a.workers.Add(func(ctx context.Context) {
		defer close(chunkChan)

		chunkDurationMs := 100 // 100ms per chunk
		samplesPerChunk := a.sampleRate * chunkDurationMs / 1000
		sampleOffset := int(previousTimestampNs * int64(a.sampleRate) / 1e9)
		var sequence int32

		// Determine start time: either use previousTimestampNs or current time
		var chunkTime time.Time
		if previousTimestampNs > 0 {
			chunkTime = time.Unix(0, previousTimestampNs)
		} else {
			chunkTime = time.Now()
		}

		// Calculate number of chunks to generate
		var totalChunks int
		if durationSeconds > 0 {
			totalChunks = int(math.Ceil(float64(durationSeconds) * 1000 / float64(chunkDurationMs)))
		} else {
			totalChunks = -1 // infinite stream
		}

		ticker := time.NewTicker(time.Duration(chunkDurationMs) * time.Millisecond)
		defer ticker.Stop()

		chunksGenerated := 0
		for {
			// Generate audio chunk with current timestamp
			chunk := a.generateAudioChunk(sequence, chunkTime)

			// Send chunk to channel
			select {
			case chunkChan <- chunk:
				sampleOffset += samplesPerChunk
				sequence++
				chunksGenerated++
				// Advance time by chunk duration
				chunkTime = chunkTime.Add(time.Duration(chunkDurationMs) * time.Millisecond)

				// Check if we've generated enough chunks
				if totalChunks > 0 && chunksGenerated >= totalChunks {
					return
				}

			case <-ctx.Done():
				return
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	})
	return chunkChan, nil
}

// Properties returns the audios's properties.
func (a *AudioIn) Properties(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
	return rutils.Properties{
		SupportedCodecs: a.supportedCodecs,
		SampleRateHz:    int32(a.sampleRate),
		NumChannels:     int32(a.numChannels),
	}, nil
}

// Geometries returns the geometries associated with the fake audio.
func (a *AudioIn) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return a.Geometry, nil
}

// Close does nothing
func (a *AudioIn) Close(ctx context.Context) error {
	a.workers.Stop()
	return nil
}

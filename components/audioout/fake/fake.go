// Package fake implements a fake audio out.
package fake

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/components/audioout"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterComponent(
		audioout.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[audioout.AudioOut, resource.NoNativeConfig]{Constructor: NewAudioOut})
}

// AudioOut is a fake AudioOut that simulates audio playback.
type AudioOut struct {
	resource.Named
	resource.TriviallyReconfigurable
	Geometry        []spatialmath.Geometry
	logger          logging.Logger
	sampleRate      int
	numChannels     int
	supportedCodecs []string
}

// NewAudioOut instantiates a new AudioOut of the fake model type.
func NewAudioOut(_ context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger) (audioout.AudioOut, error) {
	a := &AudioOut{
		Named:           conf.ResourceName().AsNamed(),
		Geometry:        []spatialmath.Geometry{},
		logger:          logger,
		sampleRate:      44100,
		numChannels:     1,
		supportedCodecs: []string{"pcm16"},
	}

	return a, nil
}

// Play simulates playing audio by blocking for the duration of playback.
func (a *AudioOut) Play(ctx context.Context, data []byte, info *utils.AudioInfo, extra map[string]interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("no audio data provided")
	}

	if info == nil {
		return fmt.Errorf("audio info is required")
	}

	if info.Codec != "pcm16" {
		return fmt.Errorf("codec %s not supported, only pcm16 is supported", info.Codec)
	}

	if info.NumChannels <= 0 || info.SampleRateHz <= 0 {
		return fmt.Errorf("invalid audio info, sample rate and num channels must be above zero")
	}

	bytesPerSample := 2 // 16-bit = 2 bytes
	numSamples := len(data) / (bytesPerSample * int(info.NumChannels))
	duration := float64(numSamples) / float64(info.SampleRateHz)

	a.logger.Infof("Fake audio playback: codec=%s, channels=%d, sample_rate=%d, samples=%d, duration=%.3fs, bytes=%d",
		info.Codec, info.NumChannels, info.SampleRateHz, numSamples, duration, len(data))

	// Block for the duration of the audio playback to simulate real playback
	select {
	case <-time.After(time.Duration(duration * float64(time.Second))):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Properties returns the audio output's properties.
func (a *AudioOut) Properties(ctx context.Context, extra map[string]interface{}) (utils.Properties, error) {
	return utils.Properties{
		SupportedCodecs: a.supportedCodecs,
		SampleRateHz:    int32(a.sampleRate),
		NumChannels:     int32(a.numChannels),
	}, nil
}

// Geometries returns the geometries associated with the fake audio output.
func (a *AudioOut) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return a.Geometry, nil
}

// Close does nothing.
func (a *AudioOut) Close(ctx context.Context) error {
	return nil
}

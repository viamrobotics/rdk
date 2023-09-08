//go:build !notc

package inject

import (
	"context"

	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/resource"
)

// AudioInput is an injected audio input.
type AudioInput struct {
	audioinput.AudioInput
	name       resource.Name
	DoFunc     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StreamFunc func(
		ctx context.Context,
		errHandlers ...gostream.ErrorHandler,
	) (gostream.AudioStream, error)
	MediaPropertiesFunc func(ctx context.Context) (prop.Audio, error)
	CloseFunc           func(ctx context.Context) error
}

// NewAudioInput returns a new injected audio input.
func NewAudioInput(name string) *AudioInput {
	return &AudioInput{name: audioinput.Named(name)}
}

// Name returns the name of the resource.
func (ai *AudioInput) Name() resource.Name {
	return ai.name
}

// Stream calls the injected Stream or the real version.
func (ai *AudioInput) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.AudioStream, error) {
	if ai.StreamFunc == nil {
		return ai.AudioInput.Stream(ctx, errHandlers...)
	}
	return ai.StreamFunc(ctx, errHandlers...)
}

// MediaProperties calls the injected MediaProperties or the real version.
func (ai *AudioInput) MediaProperties(ctx context.Context) (prop.Audio, error) {
	if ai.MediaPropertiesFunc != nil {
		return ai.MediaPropertiesFunc(ctx)
	}
	if ai.AudioInput != nil {
		return ai.AudioInput.MediaProperties(ctx)
	}
	return prop.Audio{}, errors.Wrap(ctx.Err(), "media properties unavailable")
}

// Close calls the injected Close or the real version.
func (ai *AudioInput) Close(ctx context.Context) error {
	if ai.CloseFunc == nil {
		if ai.AudioInput == nil {
			return nil
		}
		return ai.AudioInput.Close(ctx)
	}
	return ai.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (ai *AudioInput) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if ai.DoFunc == nil {
		return ai.AudioInput.DoCommand(ctx, cmd)
	}
	return ai.DoFunc(ctx, cmd)
}

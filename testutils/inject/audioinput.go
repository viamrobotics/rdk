package inject

import (
	"context"

	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/audioinput"
)

// AudioInput is an injected audio input.
type AudioInput struct {
	audioinput.AudioInput
	DoFunc     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StreamFunc func(
		ctx context.Context,
		extra map[string]interface{},
		errHandlers ...gostream.ErrorHandler,
	) (gostream.AudioStream, error)
	MediaPropertiesFunc func(ctx context.Context, extra map[string]interface{}) (prop.Audio, error)
	CloseFunc           func(ctx context.Context) error
}

// Stream calls the injected Stream or the real version.
func (ai *AudioInput) Stream(
	ctx context.Context,
	extra map[string]interface{},
	errHandlers ...gostream.ErrorHandler,
) (gostream.AudioStream, error) {
	if ai.StreamFunc == nil {
		return ai.AudioInput.Stream(ctx, extra, errHandlers...)
	}
	return ai.StreamFunc(ctx, extra, errHandlers...)
}

// MediaProperties calls the injected MediaProperties or the real version.
func (ai *AudioInput) MediaProperties(ctx context.Context, extra map[string]interface{}) (prop.Audio, error) {
	if ai.MediaPropertiesFunc == nil {
		return ai.AudioInput.MediaProperties(ctx, extra)
	}
	return ai.MediaPropertiesFunc(ctx, extra)
}

// Close calls the injected Close or the real version.
func (ai *AudioInput) Close(ctx context.Context) error {
	if ai.CloseFunc == nil {
		return utils.TryClose(ctx, ai.AudioInput)
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

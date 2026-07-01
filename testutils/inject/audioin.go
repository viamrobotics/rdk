package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/audioin"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// AudioIn is an injected audioin component.
type AudioIn struct {
	audioin.AudioIn
	name         resource.Name
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc   func(ctx context.Context) (map[string]interface{}, error)
	GetAudioFunc func(ctx context.Context, codec string, durationSeconds float32, previousTimestampNs int64, extra map[string]interface{}) (
		chan *audioin.AudioChunk, error)
	PropertiesFunc func(ctx context.Context, extra map[string]interface{}) (utils.Properties, error)
	CloseFunc      func(ctx context.Context) error
}

// NewAudioIn returns a new injected audio in.
func NewAudioIn(name string) *AudioIn {
	return &AudioIn{name: audioin.Named(name)}
}

// Name returns the name of the resource.
func (a *AudioIn) Name() resource.Name {
	return a.name
}

// GetAudio calls the injected GetAudio or the real version.
func (a *AudioIn) GetAudio(ctx context.Context, codec string, durationSeconds float32, previousTimestampNs int64,
	extra map[string]interface{}) (chan *audioin.AudioChunk, error,
) {
	if a.GetAudioFunc == nil {
		return errtrace.Wrap2(a.AudioIn.GetAudio(ctx, codec, durationSeconds, previousTimestampNs, extra))
	}
	return errtrace.Wrap2(a.GetAudioFunc(ctx, codec, durationSeconds, previousTimestampNs, extra))
}

// Properties returns the injected Properties or the real version.
func (a *AudioIn) Properties(ctx context.Context, extra map[string]interface{}) (utils.Properties, error) {
	if a.PropertiesFunc == nil {
		return errtrace.Wrap2(a.AudioIn.Properties(ctx, extra))
	}
	return errtrace.Wrap2(a.PropertiesFunc(ctx, extra))
}

// DoCommand returns the injected docommand or the real version.
func (a *AudioIn) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return errtrace.Wrap2(a.AudioIn.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(a.DoFunc(ctx, cmd))
}

// Close calls the injected Close or the real version.
func (a *AudioIn) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		if a.AudioIn == nil {
			return nil
		}
		return errtrace.Wrap(a.AudioIn.Close(ctx))
	}
	return errtrace.Wrap(a.CloseFunc(ctx))
}

// Status calls the injected Status or the real version.
func (a *AudioIn) Status(ctx context.Context) (map[string]interface{}, error) {
	if a.StatusFunc != nil {
		return errtrace.Wrap2(a.StatusFunc(ctx))
	}
	if a.AudioIn != nil {
		return errtrace.Wrap2(a.AudioIn.Status(ctx))
	}
	return map[string]interface{}{}, nil
}

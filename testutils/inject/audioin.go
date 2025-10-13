package inject

import (
	"context"

	"go.viam.com/rdk/components/audioin"
	"go.viam.com/rdk/resource"
)

// AudioIn is an injected audioin component.
type AudioIn struct {
	audioin.AudioIn
	name           resource.Name
	DoFunc         func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetAudioFunc   func(ctx context.Context, codec string, durationSeconds float32, previousTimestamp int64, extra map[string]interface{}) (chan *audioin.AudioChunk, error)
	PropertiesFunc func(ctx context.Context, extra map[string]interface{}) (audioin.Properties, error)
}

// NewAudioInput returns a new injected audio in.
func NewAudioIn(name string) *AudioIn {
	return &AudioIn{name: audioin.Named(name)}
}

// Name returns the name of the resource.
func (ai *AudioIn) Name() resource.Name {
	return ai.name
}

// GetAudio calls the injected GetAudio or the real version.
func (a *AudioIn) GetAudio(ctx context.Context, codec string, durationSeconds float32, previousTimestamp int64, extra map[string]interface{}) (chan *audioin.AudioChunk, error) {
	if a.GetAudioFunc == nil {
		return a.AudioIn.GetAudio(ctx, codec, durationSeconds, previousTimestamp, extra)
	}
	return a.GetAudioFunc(ctx, codec, durationSeconds, previousTimestamp, extra)
}

// Properties returns the injected Properties or the real version.
func (a *AudioIn) Properties(ctx context.Context, extra map[string]interface{}) (audioin.Properties, error) {
	if a.PropertiesFunc == nil {
		return a.AudioIn.Properties(ctx, extra)
	}
	return a.PropertiesFunc(ctx, extra)
}

// Docommand returns the injected docommand or the real version
func (a *AudioIn) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return a.AudioIn.DoCommand(ctx, cmd)
	}
	return a.DoFunc(ctx, cmd)
}

package inject

import (
	"context"

	"go.viam.com/rdk/components/audioout"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// AudioOut is an injected AudioOut.
type AudioOut struct {
	audioout.AudioOut
	name           resource.Name
	DoFunc         func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	PlayFunc       func(ctx context.Context, data []byte, info *utils.AudioInfo, extra map[string]interface{}) error
	PropertiesFunc func(ctx context.Context, extra map[string]interface{}) (utils.Properties, error)
	CloseFunc      func(ctx context.Context) error
}

// NewAudioOut returns a new injected AudioOut.
func NewAudioOut(name string) *AudioOut {
	return &AudioOut{name: audioout.Named(name)}
}

// Name returns the name of the resource.
func (a *AudioOut) Name() resource.Name {
	return a.name
}

// DoCommand calls the injected DoCommand or the real version.
func (a *AudioOut) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if a.DoFunc == nil {
		return a.AudioOut.DoCommand(ctx, cmd)
	}
	return a.DoFunc(ctx, cmd)
}

// Properties calls the injected Properties or the real version.
func (a *AudioOut) Properties(ctx context.Context, extra map[string]interface{}) (utils.Properties, error) {
	if a.PropertiesFunc == nil {
		return a.AudioOut.Properties(ctx, extra)
	}
	return a.PropertiesFunc(ctx, extra)
}

// Play calls the injected Play or the real version.
func (a *AudioOut) Play(ctx context.Context, data []byte, info *utils.AudioInfo, extra map[string]interface{}) error {
	if a.PlayFunc == nil {
		return a.AudioOut.Play(ctx, data, info, extra)
	}
	return a.PlayFunc(ctx, data, info, extra)
}

// Close calls the injected Close or the real version.
func (a *AudioOut) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		if a.AudioOut == nil {
			return nil
		}
		return a.AudioOut.Close(ctx)
	}
	return a.CloseFunc(ctx)
}

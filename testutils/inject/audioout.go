package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/audioout"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// AudioOut is an injected AudioOut.
type AudioOut struct {
	audioout.AudioOut
	name           resource.Name
	DoFunc         func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc     func(ctx context.Context) (map[string]interface{}, error)
	PlayFunc       func(ctx context.Context, data []byte, info *utils.AudioInfo, extra map[string]interface{}) error
	PlayStreamFunc func(ctx context.Context, info *utils.AudioInfo, chunks <-chan []byte, extra map[string]interface{}) error
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
		return errtrace.Wrap2(a.AudioOut.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(a.DoFunc(ctx, cmd))
}

// Properties calls the injected Properties or the real version.
func (a *AudioOut) Properties(ctx context.Context, extra map[string]interface{}) (utils.Properties, error) {
	if a.PropertiesFunc == nil {
		return errtrace.Wrap2(a.AudioOut.Properties(ctx, extra))
	}
	return errtrace.Wrap2(a.PropertiesFunc(ctx, extra))
}

// Play calls the injected Play or the real version.
func (a *AudioOut) Play(ctx context.Context, data []byte, info *utils.AudioInfo, extra map[string]interface{}) error {
	if a.PlayFunc == nil {
		return errtrace.Wrap(a.AudioOut.Play(ctx, data, info, extra))
	}
	return errtrace.Wrap(a.PlayFunc(ctx, data, info, extra))
}

// PlayStream calls the injected PlayStream or the real version.
func (a *AudioOut) PlayStream(ctx context.Context, info *utils.AudioInfo, chunks <-chan []byte, extra map[string]interface{}) error {
	if a.PlayStreamFunc == nil {
		return errtrace.Wrap(a.AudioOut.PlayStream(ctx, info, chunks, extra))
	}
	return errtrace.Wrap(a.PlayStreamFunc(ctx, info, chunks, extra))
}

// Close calls the injected Close or the real version.
func (a *AudioOut) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		if a.AudioOut == nil {
			return nil
		}
		return errtrace.Wrap(a.AudioOut.Close(ctx))
	}
	return errtrace.Wrap(a.CloseFunc(ctx))
}

// Status calls the injected Status or the real version.
func (a *AudioOut) Status(ctx context.Context) (map[string]interface{}, error) {
	if a.StatusFunc != nil {
		return errtrace.Wrap2(a.StatusFunc(ctx))
	}
	if a.AudioOut != nil {
		return errtrace.Wrap2(a.AudioOut.Status(ctx))
	}
	return map[string]interface{}{}, nil
}

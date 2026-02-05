package inject

import (
	"context"
	"time"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/video"
)

// Video is an injected video service for testing.
type Video struct {
	video.Service
	name         resource.Name
	GetVideoFunc func(
		ctx context.Context,
		startTime, endTime time.Time,
		videoCodec, videoContainer string,
		extra map[string]interface{},
	) (chan *video.Chunk, error)
	DoFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// NewVideo returns a new injected video service.
func NewVideo(name string) *Video {
	return &Video{name: video.Named(name)}
}

// Name returns the name of the injected video service.
func (v *Video) Name() resource.Name {
	return v.name
}

// GetVideo calls the injected GetVideoFunc if set, otherwise calls the embedded Service's GetVideo method.
func (v *Video) GetVideo(
	ctx context.Context,
	startTime, endTime time.Time,
	videoCodec, videoContainer string,
	extra map[string]interface{},
) (chan *video.Chunk, error) {
	if v.GetVideoFunc == nil {
		return v.Service.GetVideo(ctx, startTime, endTime, videoCodec, videoContainer, extra)
	}
	return v.GetVideoFunc(ctx, startTime, endTime, videoCodec, videoContainer, extra)
}

// DoCommand calls the injected DoFunc if set, otherwise calls the embedded Service's DoCommand method.
func (v *Video) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if v.DoFunc == nil {
		return v.Service.DoCommand(ctx, cmd)
	}
	return v.DoFunc(ctx, cmd)
}

// Ensure the mock implements the interface.
var _ video.Service = (*Video)(nil)

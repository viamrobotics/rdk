package inject

import (
	"context"
	"io"
	"time"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/video"
)

type Video struct {
	video.Service
	name         resource.Name
	GetVideoFunc func(
		ctx context.Context,
		startTime time.Time,
		endTime time.Time,
		videoCodec string,
		extra map[string]interface{},
		w io.Writer,
	) error
	DoFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// NewVideo returns a new injected video service.
func NewVideo(name string) *Video {
	return &Video{name: video.Named(name)}
}

func (v *Video) Name() resource.Name {
	return v.name
}

func (v *Video) GetVideo(
	ctx context.Context,
	startTime, endTime time.Time,
	videoCodec, videoContainer, requestID string,
	extra map[string]interface{},
	w io.Writer,
) error {
	if v.GetVideoFunc == nil {
		return v.Service.GetVideo(ctx, startTime, endTime, videoCodec, videoContainer, requestID, extra, w)
	}
	return v.GetVideoFunc(ctx, startTime, endTime, videoCodec, extra, w)
}

func (v *Video) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if v.DoFunc == nil {
		return v.Service.DoCommand(ctx, cmd)
	}
	return v.DoFunc(ctx, cmd)
}

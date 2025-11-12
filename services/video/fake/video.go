// Package fake contains a fake video service implementation for testing.
package fake

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/video"
)

const (
	chunkCount = 6
	chunkSize  = 512
	interval   = 40 * time.Millisecond
)

// Model identifies this fake video service implementation.
var Model = resource.DefaultModelFamily.WithModel("fake")

func init() {
	// Register the fake implementation so the blank import builds & makes it available.
	resource.RegisterService(
		video.API,
		Model,
		resource.Registration[video.Service, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (video.Service, error) {
				return &Video{
					Named:  conf.ResourceName().AsNamed(),
					logger: logger,
				}, nil
			},
		},
	)
}

// Video is a fake video service implementation for testing.
type Video struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
}

// GetVideo is a fake implementation that writes dummy video data to the provided writer.
func (fv *Video) GetVideo(
	ctx context.Context,
	startTime, endTime time.Time,
	videoCodec, videoContainer, requestID string,
	extra map[string]interface{},
	w io.Writer,
) error {
	fv.logger.Debug("fake GetVideo",
		"start", startTime,
		"end", endTime,
		"codec", videoCodec,
		"container", videoContainer,
		"requestID", requestID,
		"extra_len", len(extra),
	)

	payload := make([]byte, chunkSize)
	for i := range payload {
		payload[i] = byte((i * 17) % 251)
	}

	// simulate streaming by sending chunks at intervals
	t := time.NewTicker(interval)
	defer t.Stop()
	for i := 0; i < chunkCount; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			header := []byte(fmt.Sprintf("fake-%s-%02d\n", requestID, i))
			if _, err := w.Write(header); err != nil {
				return err
			}
			if _, err := w.Write(payload); err != nil {
				return err
			}
		}
	}
	return nil
}

// DoCommand is a fake implementation that returns the command as the result.
func (fv *Video) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

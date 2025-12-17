// Package fake contains a fake video service implementation for testing.
package fake

import (
	"context"
	"fmt"
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
	videoCodec, videoContainer string,
	extra map[string]interface{},
) (chan *video.Chunk, error) {
	fv.logger.Debug("fake GetVideo",
		"start", startTime,
		"end", endTime,
		"codec", videoCodec,
		"container", videoContainer,
		"extra_len", len(extra),
	)

	ch := make(chan *video.Chunk, 1)

	go func() {
		defer close(ch)

		// fixed-size payload pattern
		payload := make([]byte, chunkSize)
		for i := range payload {
			payload[i] = byte((i * 17) % 251)
		}

		t := time.NewTicker(interval)
		defer t.Stop()

		for i := 0; i < chunkCount; i++ {
			select {
			case <-ctx.Done():
				fv.logger.Debug("fake video context canceled", "err", ctx.Err())
				return
			case <-t.C:
				// prepend a tiny header so chunks are distinct
				header := []byte(fmt.Sprintf("fake-video-%02d\n", i))
				data := make([]byte, len(header)+len(payload))
				copy(data, header)
				copy(data[len(header):], payload)

				chunk := &video.Chunk{
					Data:      data,
					Container: videoContainer,
				}
				select {
				case <-ctx.Done():
					return
				case ch <- chunk:
				}
			}
		}
	}()

	return ch, nil
}

// DoCommand is a fake implementation that returns the command as the result.
func (fv *Video) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

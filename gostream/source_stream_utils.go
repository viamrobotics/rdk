package gostream

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
)

// StreamVideoSource streams the given video source to the stream forever until context signals cancellation.
func StreamVideoSource(ctx context.Context, vs VideoSource, stream Stream, logger logging.Logger) error {
	return streamMediaSource(ctx, vs, stream, func(ctx context.Context, frameErr error) {
		logger.Debugw("error getting frame", "error", frameErr)
	}, stream.InputVideoFrames, logger)
}

// StreamAudioSource streams the given video source to the stream forever until context signals cancellation.
func StreamAudioSource(ctx context.Context, as AudioSource, stream Stream, logger logging.Logger) error {
	return streamMediaSource(ctx, as, stream, func(ctx context.Context, frameErr error) {
		logger.Debugw("error getting frame", "error", frameErr)
	}, stream.InputAudioChunks, logger)
}

// StreamVideoSourceWithErrorHandler streams the given video source to the stream forever
// until context signals cancellation, frame errors are sent via the error handler.
func StreamVideoSourceWithErrorHandler(
	ctx context.Context, vs VideoSource, stream Stream, errHandler ErrorHandler, logger logging.Logger,
) error {
	return streamMediaSource(ctx, vs, stream, errHandler, stream.InputVideoFrames, logger)
}

// StreamAudioSourceWithErrorHandler streams the given audio source to the stream forever
// until context signals cancellation, audio errors are sent via the error handler.
func StreamAudioSourceWithErrorHandler(
	ctx context.Context, as AudioSource, stream Stream, errHandler ErrorHandler, logger logging.Logger,
) error {
	return streamMediaSource(ctx, as, stream, errHandler, stream.InputAudioChunks, logger)
}

// streamMediaSource will stream a source of media forever to the stream until the given context tells it to cancel.
func streamMediaSource[T, U any](
	ctx context.Context,
	ms MediaSource[T],
	stream Stream,
	errHandler ErrorHandler,
	inputChan func(props U) (chan<- MediaReleasePair[T], error),
	logger logging.Logger,
) error {
	streamLoop := func() error {
		readyCh, readyCtx := stream.StreamingReady()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-readyCh:
		}
		var props U
		if provider, ok := ms.(MediaPropertyProvider[U]); ok {
			var err error
			props, err = provider.MediaProperties(ctx)
			if err != nil {
				logger.Debugw("no properties found for media; will assume empty", "error", err)
			}
		} else {
			logger.Debug("no properties found for media; will assume empty")
		}
		input, err := inputChan(props)
		if err != nil {
			return err
		}
		mediaStream, err := ms.Stream(ctx, errHandler)
		if err != nil {
			return err
		}
		defer func() {
			utils.UncheckedError(mediaStream.Close(ctx))
		}()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-readyCtx.Done():
				return nil
			default:
			}
			media, release, err := mediaStream.Next(ctx)
			if err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-readyCtx.Done():
				return nil
			case input <- MediaReleasePair[T]{media, release}:
			}
		}
	}
	for {
		if err := streamLoop(); err != nil {
			return err
		}
	}
}

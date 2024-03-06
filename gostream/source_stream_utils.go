package gostream

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils"
)

// StreamVideoSource streams the given video source to the stream forever until context signals cancellation.
func StreamVideoSource(ctx context.Context, vs VideoSource, stream Stream, logger logging.Logger) error {
	return streamMediaSource(ctx, vs, stream, func(ctx context.Context, frameErr error) {
		golog.Global().Debugw("error getting frame", "error", frameErr)
	}, stream.InputVideoFrames, logger)
}

// StreamAudioSource streams the given video source to the stream forever until context signals cancellation.
func StreamAudioSource(ctx context.Context, as AudioSource, stream Stream, logger logging.Logger) error {
	return streamMediaSource(ctx, as, stream, func(ctx context.Context, frameErr error) {
		golog.Global().Debugw("error getting frame", "error", frameErr)
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
	logger.Infof("streamMediaSource BEGIN %s", stream.Name())
	defer logger.Infof("streamMediaSource END %s", stream.Name())
	streamLoop := func() error {
		logger.Infof("streamMediaSource streamLoop: %s", stream.Name())
		readyCh, readyCtx := stream.StreamingReady()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-readyCh:
		}
		logger.Infof("streamMediaSource streamLoop: %s ready", stream.Name())
		var props U
		if provider, ok := ms.(MediaPropertyProvider[U]); ok {
			var err error
			props, err = provider.MediaProperties(ctx)
			if err != nil {
				golog.Global().Debugw("no properties found for media; will assume empty", "error", err)
			}
		} else {
			golog.Global().Debug("no properties found for media; will assume empty")
		}
		input, err := inputChan(props)
		if err != nil {
			return err
		}
		logger.Infof("streamMediaSource streamLoop: %s ready", stream.Name())
		mediaStream, err := ms.Stream(ctx, errHandler)
		if err != nil {
			return err
		}
		logger.Infof("streamMediaSource streamLoop: %s has mediaStream", stream.Name())
		defer func() {
			logger.Infof("streamMediaSource streamLoop: %s mediaStream.Close()", stream.Name())
			utils.UncheckedError(mediaStream.Close(ctx))
		}()
		i := 0
		for {
			i++
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-readyCtx.Done():
				return nil
			default:
			}
			logger.Infof("streamMediaSource streamLoop: %s, iteration %d calling Next()", stream.Name(), i)
			media, release, err := mediaStream.Next(ctx)
			if err != nil {
				logger.Infof("streamMediaSource streamLoop: %s, iteration %d called Next() but got error", stream.Name(), i)
				continue
			}
			logger.Infof("streamMediaSource streamLoop: %s, iteration %d called Next()", stream.Name(), i)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-readyCtx.Done():
				return nil
			case input <- MediaReleasePair[T]{media, release}:
			}
			logger.Infof("streamMediaSource streamLoop: %s, iteration %d wrote input", stream.Name(), i)
		}
	}
	i := 0
	for {
		i++
		if err := streamLoop(); err != nil {
			logger.Infof("streamMediaSource streamLoop err: %s", err.Error())
			return err
		}
		logger.Infof("streamMediaSource streamLoop called %d times", i)
	}
}

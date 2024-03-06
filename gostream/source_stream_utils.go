package gostream

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
)

// StreamVideoSource streams the given video source to the stream forever until context signals cancellation.
func StreamVideoSource(ctx context.Context, vs VideoSource, stream Stream) error {
	return streamMediaSource(ctx, vs, stream, func(ctx context.Context, frameErr error) {
		golog.Global().Debugw("error getting frame", "error", frameErr)
	}, stream.InputVideoFrames)
}

// StreamAudioSource streams the given video source to the stream forever until context signals cancellation.
func StreamAudioSource(ctx context.Context, as AudioSource, stream Stream) error {
	return streamMediaSource(ctx, as, stream, func(ctx context.Context, frameErr error) {
		golog.Global().Debugw("error getting frame", "error", frameErr)
	}, stream.InputAudioChunks)
}

// StreamVideoSourceWithErrorHandler streams the given video source to the stream forever
// until context signals cancellation, frame errors are sent via the error handler.
func StreamVideoSourceWithErrorHandler(
	ctx context.Context, vs VideoSource, stream Stream, errHandler ErrorHandler,
) error {
	return streamMediaSource(ctx, vs, stream, errHandler, stream.InputVideoFrames)
}

// StreamAudioSourceWithErrorHandler streams the given audio source to the stream forever
// until context signals cancellation, audio errors are sent via the error handler.
func StreamAudioSourceWithErrorHandler(
	ctx context.Context, as AudioSource, stream Stream, errHandler ErrorHandler,
) error {
	return streamMediaSource(ctx, as, stream, errHandler, stream.InputAudioChunks)
}

// streamMediaSource will stream a source of media forever to the stream until the given context tells it to cancel.
func streamMediaSource[T, U any](
	ctx context.Context,
	ms MediaSource[T],
	stream Stream,
	errHandler ErrorHandler,
	inputChan func(props U) (chan<- MediaReleasePair[T], error),
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
				golog.Global().Debugw("no properties found for media; will assume empty", "error", err)
			}
		} else {
			golog.Global().Debug("no properties found for media; will assume empty")
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

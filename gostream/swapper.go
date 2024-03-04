package gostream

import (
	"context"
	"image"
	"sync"

	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

type (
	// A HotSwappableMediaSource allows for continuous streaming of media of
	// swappable underlying media sources.
	HotSwappableMediaSource[T, U any] interface {
		MediaSource[T]
		MediaPropertyProvider[U]
		Swap(src MediaSource[T])
	}

	// A HotSwappableVideoSource allows for continuous streaming of video of
	// swappable underlying video sources.
	HotSwappableVideoSource = HotSwappableMediaSource[image.Image, prop.Video]

	// A HotSwappableAudioSource allows for continuous streaming of audio of
	// swappable underlying audio sources.
	HotSwappableAudioSource = HotSwappableMediaSource[wave.Audio, prop.Audio]
)

type hotSwappableMediaSource[T, U any] struct {
	mu        sync.RWMutex
	src       MediaSource[T]
	cancelCtx context.Context
	cancel    func()
}

// NewHotSwappableMediaSource returns a hot swappable media source.
func NewHotSwappableMediaSource[T, U any](src MediaSource[T]) HotSwappableMediaSource[T, U] {
	swapper := &hotSwappableMediaSource[T, U]{}
	swapper.Swap(src)
	return swapper
}

// NewHotSwappableVideoSource returns a hot swappable video source.
func NewHotSwappableVideoSource(src VideoSource) HotSwappableVideoSource {
	return NewHotSwappableMediaSource[image.Image, prop.Video](src)
}

// NewHotSwappableAudioSource returns a hot swappable audio source.
func NewHotSwappableAudioSource(src AudioSource) HotSwappableAudioSource {
	return NewHotSwappableMediaSource[wave.Audio, prop.Audio](src)
}

var errSwapperClosed = errors.New("hot swapper closed or uninitialized")

// Stream returns a stream that is tolerant to the underlying media source changing.
func (swapper *hotSwappableMediaSource[T, U]) Stream(
	ctx context.Context,
	errHandlers ...ErrorHandler,
) (MediaStream[T], error) {
	swapper.mu.RLock()
	defer swapper.mu.RUnlock()

	if swapper.src == nil {
		return nil, errSwapperClosed
	}

	stream := &hotSwappableMediaSourceStream[T, U]{
		parent:      swapper,
		errHandlers: errHandlers,
		cancelCtx:   swapper.cancelCtx,
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if err := stream.init(ctx); err != nil {
		return nil, err
	}

	return stream, nil
}

// Swap replaces the underlying media source with the given one and signals to all
// streams that a new source is available.
func (swapper *hotSwappableMediaSource[T, U]) Swap(newSrc MediaSource[T]) {
	swapper.mu.Lock()
	defer swapper.mu.Unlock()
	if swapper.src == newSrc {
		// they are the same so lets not cause any interruptions.
		return
	}

	if swapper.cancel != nil {
		swapper.cancel()
	}

	swapper.src = newSrc
	cancelCtx, cancel := context.WithCancel(context.Background())
	swapper.cancelCtx = cancelCtx
	swapper.cancel = cancel
}

// MediaProperties attempts to return media properties for the source, if they exist.
func (swapper *hotSwappableMediaSource[T, U]) MediaProperties(ctx context.Context) (U, error) {
	swapper.mu.RLock()
	defer swapper.mu.RUnlock()

	var zero U
	if swapper.src == nil {
		return zero, errSwapperClosed
	}

	if provider, ok := swapper.src.(MediaPropertyProvider[U]); ok {
		return provider.MediaProperties(ctx)
	}
	return zero, nil
}

// Close unsets the underlying media source and signals all streams to close.
func (swapper *hotSwappableMediaSource[T, U]) Close(ctx context.Context) error {
	swapper.Swap(nil)
	return nil
}

type hotSwappableMediaSourceStream[T, U any] struct {
	mu          sync.Mutex
	parent      *hotSwappableMediaSource[T, U]
	errHandlers []ErrorHandler
	stream      MediaStream[T]
	cancelCtx   context.Context
}

func (cs *hotSwappableMediaSourceStream[T, U]) init(ctx context.Context) error {
	var err error
	if cs.stream != nil {
		utils.UncheckedError(cs.stream.Close(ctx))
		cs.stream = nil
	}
	cs.parent.mu.RLock()
	defer cs.parent.mu.RUnlock()
	cs.cancelCtx = cs.parent.cancelCtx
	if cs.parent.src == nil {
		return errSwapperClosed
	}
	cs.stream, err = cs.parent.src.Stream(ctx, cs.errHandlers...)
	return err
}

func (cs *hotSwappableMediaSourceStream[T, U]) checkStream(ctx context.Context) error {
	if cs.stream != nil && cs.cancelCtx.Err() == nil {
		return nil
	}
	return cs.init(ctx)
}

func (cs *hotSwappableMediaSourceStream[T, U]) Next(ctx context.Context) (T, func(), error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := cs.checkStream(ctx); err != nil {
		var zero T
		return zero, nil, err
	}
	return cs.stream.Next(ctx)
}

func (cs *hotSwappableMediaSourceStream[T, U]) Close(ctx context.Context) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.stream == nil {
		return nil
	}
	return cs.stream.Close(ctx)
}

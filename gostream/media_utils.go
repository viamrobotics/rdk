package gostream

import (
	"context"
	"sync"

	"go.uber.org/multierr"
)

// NewEmbeddedMediaStream returns a media stream from a media source that is
// intended to be embedded/composed by another source. It defers the creation
// of its media stream.
func NewEmbeddedMediaStream[T, U any](src MediaSource[T]) MediaStream[T] {
	return &embeddedMediaStream[T, U]{src: src}
}

type embeddedMediaStream[T, U any] struct {
	mu     sync.Mutex
	src    MediaSource[T]
	stream MediaStream[T]
}

func (ems *embeddedMediaStream[T, U]) initStream(ctx context.Context) error {
	if ems.stream != nil {
		return nil
	}
	stream, err := ems.src.Stream(ctx)
	if err != nil {
		return err
	}
	ems.stream = stream
	return nil
}

func (ems *embeddedMediaStream[T, U]) Next(ctx context.Context) (T, func(), error) {
	ems.mu.Lock()
	defer ems.mu.Unlock()
	if err := ems.initStream(ctx); err != nil {
		var zero T
		return zero, nil, err
	}
	return ems.stream.Next(ctx)
}

func (ems *embeddedMediaStream[T, U]) Close(ctx context.Context) error {
	ems.mu.Lock()
	defer ems.mu.Unlock()
	if ems.stream == nil {
		return nil
	}
	return ems.stream.Close(ctx)
}

// NewEmbeddedMediaStreamFromReader returns a media stream from a media reader that is
// intended to be embedded/composed by another source. It defers the creation
// of its media stream.
func NewEmbeddedMediaStreamFromReader[T, U any](reader MediaReader[T], p U) MediaStream[T] {
	src := newMediaSource[T](nil, MediaReaderFunc[T](reader.Read), p)
	stream := NewEmbeddedMediaStream[T, U](src)
	return &embeddedMediaReaderStream[T, U]{
		src:    src,
		stream: stream,
	}
}

type embeddedMediaReaderStream[T, U any] struct {
	src    MediaSource[T]
	stream MediaStream[T]
}

func (emrs *embeddedMediaReaderStream[T, U]) Next(ctx context.Context) (T, func(), error) {
	return emrs.stream.Next(ctx)
}

func (emrs *embeddedMediaReaderStream[T, U]) Close(ctx context.Context) error {
	return multierr.Combine(emrs.stream.Close(ctx), emrs.src.Close(ctx))
}

type contextValue byte

const contextValueMIMETypeHint contextValue = iota

// WithMIMETypeHint provides a hint to readers that media should be encoded to
// this type.
func WithMIMETypeHint(ctx context.Context, mimeType string) context.Context {
	return context.WithValue(ctx, contextValueMIMETypeHint, mimeType)
}

// MIMETypeHint gets the hint of what MIME type to use in encoding; if nothing is
// set, the default provided is used.
func MIMETypeHint(ctx context.Context, defaultType string) string {
	if val, ok := ctx.Value(contextValueMIMETypeHint).(string); ok {
		if val == "" {
			return defaultType
		}
		return val
	}
	return defaultType
}

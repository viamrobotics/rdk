package gostream

import (
	"context"
	"image"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/prop"
)

type (
	// An VideoReader is anything that can read and recycle video data.
	VideoReader = MediaReader[image.Image]

	// An VideoReaderFunc is a helper to turn a function into an VideoReader.
	VideoReaderFunc = MediaReaderFunc[image.Image]

	// A VideoSource is responsible for producing images when requested. A source
	// should produce the image as quickly as possible and introduce no rate limiting
	// of its own as that is handled internally.
	VideoSource = MediaSource[image.Image]

	// An VideoStream streams video forever until closed.
	VideoStream = MediaStream[image.Image]

	// VideoPropertyProvider providers information about a video source.
	VideoPropertyProvider = MediaPropertyProvider[prop.Video]
)

// NewVideoSource instantiates a new video source.
func NewVideoSource(r VideoReader, p prop.Video) VideoSource {
	return newMediaSource(nil, r, p)
}

// NewVideoSourceForDriver instantiates a new video source and references the given driver.
func NewVideoSourceForDriver(d driver.Driver, r VideoReader, p prop.Video) VideoSource {
	return newMediaSource(d, r, p)
}

// ReadImage gets a single image from a video source. Using this has less of a guarantee
// than VideoSource.Stream that the Nth image follows the N-1th image.
func ReadImage(ctx context.Context, source VideoSource) (image.Image, func(), error) {
	return ReadMedia(ctx, source)
}

// NewEmbeddedVideoStream returns a video stream from a video source that is
// intended to be embedded/composed by another source. It defers the creation
// of its stream.
func NewEmbeddedVideoStream(src VideoSource) VideoStream {
	return NewEmbeddedMediaStream[image.Image, prop.Video](src)
}

// NewEmbeddedVideoStreamFromReader returns a video stream from a video reader that is
// intended to be embedded/composed by another source. It defers the creation
// of its stream.
func NewEmbeddedVideoStreamFromReader(reader VideoReader) VideoStream {
	return NewEmbeddedMediaStreamFromReader(reader, prop.Video{})
}

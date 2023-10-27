package gostream

import (
	"context"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
)

type (
	// An AudioReader is anything that can read and recycle audio data.
	AudioReader = MediaReader[wave.Audio]

	// An AudioReaderFunc is a helper to turn a function into an AudioReader.
	AudioReaderFunc = MediaReaderFunc[wave.Audio]

	// An AudioSource is responsible for producing audio chunks when requested. A source
	// should produce the chunk as quickly as possible and introduce no rate limiting
	// of its own as that is handled internally.
	AudioSource = MediaSource[wave.Audio]

	// An AudioStream streams audio forever until closed.
	AudioStream = MediaStream[wave.Audio]

	// AudioPropertyProvider providers information about an audio source.
	AudioPropertyProvider = MediaPropertyProvider[prop.Audio]
)

// NewAudioSource instantiates a new audio read closer.
func NewAudioSource(r AudioReader, p prop.Audio) AudioSource {
	return newMediaSource(nil, r, p)
}

// NewAudioSourceForDriver instantiates a new audio read closer and references the given
// driver.
func NewAudioSourceForDriver(d driver.Driver, r AudioReader, p prop.Audio) AudioSource {
	return newMediaSource(d, r, p)
}

// ReadAudio gets a single audio wave from an audio source. Using this has less of a guarantee
// than AudioSource.Stream that the Nth wave follows the N-1th wave.
func ReadAudio(ctx context.Context, source AudioSource) (wave.Audio, func(), error) {
	return ReadMedia(ctx, source)
}

// NewEmbeddedAudioStream returns an audio stream from an audio source that is
// intended to be embedded/composed by another source. It defers the creation
// of its stream.
func NewEmbeddedAudioStream(src AudioSource) AudioStream {
	return NewEmbeddedMediaStream[wave.Audio, prop.Audio](src)
}

// NewEmbeddedAudioStreamFromReader returns an audio stream from an audio reader that is
// intended to be embedded/composed by another source. It defers the creation
// of its stream.
func NewEmbeddedAudioStreamFromReader(reader AudioReader) AudioStream {
	return NewEmbeddedMediaStreamFromReader(reader, prop.Audio{})
}

//go:build cgo && !android

package ffmpeg

import (
	"testing"

	"go.viam.com/test"
)

func TestEncoderIsAvailable(t *testing.T) {
	isAvailable := EncoderIsAvailable("foo")
	test.That(t, isAvailable, test.ShouldBeFalse)
}

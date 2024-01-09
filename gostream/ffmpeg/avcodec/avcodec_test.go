//go:build cgo && ((linux && !android) || (darwin && arm64))

package avcodec

import (
	"testing"

	"go.viam.com/test"
)

func TestEncoderIsAvailable(t *testing.T) {
	isAvailable := EncoderIsAvailable("foo")
	test.That(t, isAvailable, test.ShouldBeFalse)
}

//go:build cgo && linux && !(arm || android)

package avcodec

import (
	"testing"

	"go.viam.com/test"
)

func TestEncoderIsAvailable(t *testing.T) {
	isAvailable := EncoderIsAvailable("foo")
	test.That(t, isAvailable, test.ShouldBeFalse)
}

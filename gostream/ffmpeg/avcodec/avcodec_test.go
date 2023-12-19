//go:build cgo && linux && !(arm || android)

package avcodec

import (
	"go.viam.com/test"
	"testing"
)

func TestEncoderIsAvailable(t *testing.T) {
	isAvailable := EncoderIsAvailable("foo")
	test.That(t, isAvailable, test.ShouldBeFalse)
}

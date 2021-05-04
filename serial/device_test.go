package serial

import (
	"testing"

	"go.viam.com/test"
)

func TestOpenDevice(t *testing.T) {
	_, err := OpenDevice("")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such")
}

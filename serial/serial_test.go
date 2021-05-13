package serial

import (
	"testing"

	"go.viam.com/test"
)

func TestOpen(t *testing.T) {
	_, err := Open("")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such")
}

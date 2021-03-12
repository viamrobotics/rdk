package utils

import (
	"testing"

	"github.com/edaniels/test"
)

func TestTryReserveRandomPort(t *testing.T) {
	p, err := TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, p, test.ShouldBeGreaterThan, 0)
}

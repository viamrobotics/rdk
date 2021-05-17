package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestTryReserveRandomPort(t *testing.T) {
	p, err := TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, p, test.ShouldBeGreaterThan, 0)
}

func TestGetAllLocalIPv4s(t *testing.T) {
	ips, err := GetAllLocalIPv4s()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ips, test.ShouldNotBeEmpty)
}

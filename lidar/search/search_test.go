// +build !linux,!darwin

package search

import (
	"testing"

	"github.com/edaniels/test"
)

func TestDevices(t *testing.T) {
	test.That(t, Devices(), test.ShouldBeEmpty)
}

// +build !linux,!darwin

package search

import (
	"testing"

	"go.viam.com/test"
)

func TestDevices(t *testing.T) {
	test.That(t, Devices(), test.ShouldBeEmpty)
}

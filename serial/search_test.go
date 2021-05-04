// +build !linux,!darwin

package serial

import (
	"testing"

	"go.viam.com/test"
)

func TestSearchDevices(t *testing.T) {
	test.That(t, SearchDevices(SearchFilter{}), test.ShouldBeEmpty)
}

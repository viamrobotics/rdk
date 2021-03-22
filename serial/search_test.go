// +build !linux,!darwin

package serial

import (
	"testing"

	"github.com/edaniels/test"
)

func TestSearchDevices(t *testing.T) {
	test.That(t, SearchDevices(SearchFilter{}), test.ShouldBeEmpty)
}

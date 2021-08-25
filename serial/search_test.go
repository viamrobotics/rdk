//go:build !linux && !darwin
// +build !linux,!darwin

package serial

import (
	"testing"

	"go.viam.com/test"
)

func TestSearch(t *testing.T) {
	test.That(t, Search(SearchFilter{}), test.ShouldBeEmpty)
}

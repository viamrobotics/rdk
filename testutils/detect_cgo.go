//go:build !no_cgo

package testutils

import "testing"

// SkipNoCgo skips the test if the no_cgo tag is present.
func SkipNoCgo(t *testing.T) {
	return
}

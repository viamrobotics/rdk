//go:build !no_media

package vision

import (
	"testing"

	testutilsext "go.viam.com/utils/testutils/ext"
)

// TestMain is used to control the execution of all tests run within this package (including _test packages).
func TestMain(m *testing.M) {
	testutilsext.VerifyTestMain(m)
}

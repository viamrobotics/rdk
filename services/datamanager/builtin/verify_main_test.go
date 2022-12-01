package builtin

import (
	testutilsext "go.viam.com/utils/testutils/ext"
	"testing"
)

// TestMain is used to control the execution of all tests run within this package (including _test packages).
func TestMain(m *testing.M) {
	testutilsext.VerifyTestMain(m)
}

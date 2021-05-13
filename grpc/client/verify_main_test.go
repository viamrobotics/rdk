package client

import (
	"testing"

	"go.viam.com/core/testutils"
)

// TestMain is used to control the execution of all tests run within this package (including _test packages)
func TestMain(m *testing.M) {
	testutils.VerifyTestMain(m)
}

// Package main_test tests that the rtppassthrough example builds successfully.
package main_test

import (
	"testing"

	"go.viam.com/rdk/testutils"
)

func TestRTPPassthroughBuild(t *testing.T) {
	testutils.BuildTempModule(t, "examples/customresources/demos/rtppassthrough")
}

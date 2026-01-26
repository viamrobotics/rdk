// Package main_test tests that the rtppassthrough example builds successfully.
package main_test

import (
	"testing"

	"go.viam.com/rdk/testutils"
	"go.viam.com/test"

)

func TestRTPPassthroughBuild(t *testing.T) {
	modPath := testutils.BuildTempModule(t, "examples/customresources/demos/rtppassthrough")
	test.That(t, modPath, test.ShouldNotBeEmpty)
}

// Package main tests that the simplemodule example builds successfully.
package main_test

import (
	"testing"

	"go.viam.com/rdk/testutils"
	"go.viam.com/test"

)

func TestSimpleModuleBuild(t *testing.T) {
	modPath := testutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	test.That(t, modPath, test.ShouldNotBeEmpty)
}

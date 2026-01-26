// Package main tests that the simplemodule example builds successfully.
package main_test

import (
	"testing"

	"go.viam.com/rdk/testutils"
)

func TestSimpleModuleBuild(t *testing.T) {
	testutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
}

func TestSimpleModuleClientBuild(t *testing.T) {
	testutils.BuildTempModule(t, "examples/customresources/demos/simplemodule/client")
}

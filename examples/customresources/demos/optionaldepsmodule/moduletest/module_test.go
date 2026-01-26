// Package main tests that the optionaldepsmodule example builds successfully.
package main_test

import (
	"testing"

	"go.viam.com/rdk/testutils"
	"go.viam.com/test"

)

func TestOptionalDepsModuleBuild(t *testing.T) {
	modPath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")
	test.That(t, modPath, test.ShouldNotBeEmpty)
}

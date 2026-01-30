// Package main tests that the optionaldepsmodule example builds successfully.
package main_test

import (
	"testing"

	"go.viam.com/rdk/testutils"
)

func TestOptionalDepsModuleBuild(t *testing.T) {
	testutils.VerifyDirectoryBuilds(t, "examples/customresources/demos/optionaldepsmodule")
}

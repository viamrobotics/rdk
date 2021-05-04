package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestExecuteShellCommand(t *testing.T) {
	res, err := ExecuteShellCommand("ls")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeEmpty)
}

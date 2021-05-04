package utils

import (
	"testing"

	"github.com/edaniels/test"
)

func TestExecuteShellCommand(t *testing.T) {
	res, err := ExecuteShellCommand("ls")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeEmpty)
}

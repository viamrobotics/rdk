package utils

import (
	"testing"
)

func TestExecuteShellCommand(t *testing.T) {
	res, err := ExecuteShellCommand("ls")
	if err != nil {
		t.Fatal(err)
	}

	if len(res) == 0 {
		t.Error("no results")
	}
}

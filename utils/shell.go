package utils

import (
	"os/exec"
	"strings"
)

func ExecuteShellCommand(name string, arg ...string) ([]string, error) {
	cmd := exec.Command(name, arg...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return strings.Split(string(out), "\n"), nil
}

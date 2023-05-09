package testutils

import (
	"fmt"
	"os/exec"

	"go.uber.org/multierr"

	"go.viam.com/rdk/utils"
)

// BuildInDir will run "go build ." in the provided RDK directory and return
// any build related errors.
func BuildInDir(dir string) error {
	builder := exec.Command("go", "build", ".")
	builder.Dir = utils.ResolveFile(dir)
	out, err := builder.CombinedOutput()
	if len(out) != 0 {
		return multierr.Combine(err, fmt.Errorf(`output from "go build .": %s`, out))
	}
	return nil
}

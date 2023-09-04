package testutils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"go.uber.org/multierr"

	"go.viam.com/rdk/utils"
)

// BuildTempModule will run "go build ." in the provided RDK directory and return
// the path to the built temporary file and any build related errors.
func BuildTempModule(tb testing.TB, dir string) (string, error) {
	tb.Helper()
	modPath := filepath.Join(tb.TempDir(), filepath.Base(dir))
	//nolint:gosec
	builder := exec.Command("go", "build", "-o", modPath, ".")
	builder.Dir = utils.ResolveFile(dir)
	out, err := builder.CombinedOutput()
	if len(out) != 0 || err != nil {
		return modPath, multierr.Combine(err, fmt.Errorf(`output from "go build .": %s`, out))
	}
	return modPath, nil
}

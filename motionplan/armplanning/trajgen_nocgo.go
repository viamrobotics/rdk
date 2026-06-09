//go:build windows || no_cgo

package armplanning

import (
	"github.com/pkg/errors"

	"go.viam.com/rdk/services/mlmodel"
)

// newTrajGenBackend errors on non-cgo builds. Trajectory generation depends on the trajex cgo
// library, which is excluded from no_cgo and windows builds. The cgo counterpart that returns a
// working backend lives in trajgen_cgo.go.
func newTrajGenBackend() (mlmodel.Service, error) {
	return nil, errors.New("trajectory generation requires a cgo build (trajex backend unavailable under no_cgo)")
}

//go:build !windows && !no_cgo

package armplanning

import (
	trajexrdk "github.com/viam-modules/trajex/go/totg/rdk"

	"go.viam.com/rdk/services/mlmodel"
)

// newTrajGenBackend returns the in-process, cgo-backed trajex TOTG service used as the
// trajectory generator. It satisfies mlmodel.Service, so the rest of the traj-gen pipeline
// (inferTrajGen) consumes it unchanged.
//
// This file is the only place in rdk that references trajex; the dependency is confined to
// armplanning behind a cgo build tag. Callers (e.g. the builtin motion service) never import
// trajex. The no_cgo / windows counterpart lives in trajgen_nocgo.go.
func newTrajGenBackend() (mlmodel.Service, error) {
	return trajexrdk.NewService(mlmodel.Named("trajex-totg")), nil
}

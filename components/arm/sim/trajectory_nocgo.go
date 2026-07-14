//go:build windows || no_cgo || !viam_rdk_cgo_have_cxx20_rt

package sim

import (
	"go.viam.com/rdk/logging"
)

// newTrajectoryGenerator returns the no-cgo fallback. The fake ignores
// accelLimit and pathTolerance; see fakeTrajectoryGenerator's docs.
func newTrajectoryGenerator(logger logging.Logger) trajectoryGenerator {
	return newFakeTrajectoryGenerator(logger)
}

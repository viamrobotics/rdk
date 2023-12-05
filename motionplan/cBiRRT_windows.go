//go:build windows

package motionplan

import (
	"math/rand"

	"github.com/pkg/errors"
	"go.viam.com/rdk/logging"

	"go.viam.com/rdk/referenceframe"
)

// TODO(RSDK-1772): support motion planning on windows
func newCBiRRTMotionPlanner(
	frame referenceframe.Frame,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	return nil, errors.New("motion planning is not yet supported on Windows")
}

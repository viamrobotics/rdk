package streaming

import (
	"time"
)

// pvat is one sampled position/velocity/acceleration/time point, the unit passed from the
// trajex session to the arm stream. Kept here rather than in the build-tagged trajex files so
// that the arm stream, which needs no trajex support, builds (and its tests run) without the
// trajex build tag.
type pvat struct {
	positions     []float64
	velocities    []float64
	accelerations []float64
	time          time.Duration
}

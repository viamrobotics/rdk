package sim

import (
	"context"
	"errors"
	"math"
	"sync"

	"go.viam.com/rdk/logging"
)

// plannedTrajectory is the uniformly-sampled trajectory shape produced by a
// trajectoryGenerator and consumed by simulatedArm.updateForTime.
type plannedTrajectory struct {
	// sampleTimes is [n_samples] in seconds, evenly spaced from 0 to duration.
	sampleTimes []float64
	// sampleConfigs is row-major [n_samples, n_dof].
	sampleConfigs []float64
	nDof          int
}

// trajectoryGenerator abstracts trajectory planning so simulatedArm can compile
// with or without cgo. The cgo build uses a trajex TOTG-backed generator
// (trajectory_cgo.go); the no-cgo build uses fakeTrajectoryGenerator.
//
// Implementations should:
//   - Internally deduplicate adjacent waypoints (defensively).
//   - Apply velLimit and accelLimit uniformly across all joints.
//   - Honor pathTolerance if able; otherwise surface the limitation via the logger.
type trajectoryGenerator interface {
	Plan(
		ctx context.Context,
		waypoints [][]float64,
		velLimit, accelLimit float64,
		pathTolerance float64,
	) (*plannedTrajectory, error)
}

// defaultDedupToleranceRads matches the trajex CAPI default. Adjacent waypoints
// within this max-per-joint absolute distance are collapsed to a single point.
const defaultDedupToleranceRads = 1e-5

// dedupWaypoints removes consecutive waypoints whose max per-joint absolute
// distance is at most `tol`. Returns a fresh slice.
func dedupWaypoints(waypoints [][]float64, tol float64) [][]float64 {
	if len(waypoints) <= 1 {
		return waypoints
	}
	out := make([][]float64, 1, len(waypoints))
	out[0] = waypoints[0]
	for i := 1; i < len(waypoints); i++ {
		if maxAbsDiff(waypoints[i], out[len(out)-1]) > tol {
			out = append(out, waypoints[i])
		}
	}
	return out
}

// maxAbsDiff returns the maximum absolute per-element difference between two
// equal-length slices. Panics on length mismatch.
func maxAbsDiff(a, b []float64) float64 {
	if len(a) != len(b) {
		panic("maxAbsDiff: length mismatch")
	}
	var m float64
	for i := range a {
		d := math.Abs(a[i] - b[i])
		if d > m {
			m = d
		}
	}
	return m
}

// fakeSamplingFreqHz matches the trajex CAPI default so that updateForTime's
// uniform-grid lookup behaves identically across build variants.
const fakeSamplingFreqHz = 100.0

// fakeTrajectoryGenerator is the no-cgo fallback. It implements the legacy
// "scale joint speeds so all joints arrive together" algorithm, applied per
// segment with an implicit stop at every interior waypoint (no blending).
// It ignores accelLimit -- motion is instantaneously at full speed -- and
// pathTolerance, emitting a one-shot warning the first time pathTolerance > 0
// is seen so the caller knows blending isn't honored.
type fakeTrajectoryGenerator struct {
	logger logging.Logger

	pathToleranceWarn sync.Once
}

func newFakeTrajectoryGenerator(logger logging.Logger) *fakeTrajectoryGenerator {
	return &fakeTrajectoryGenerator{logger: logger}
}

func (g *fakeTrajectoryGenerator) Plan(
	_ context.Context,
	waypoints [][]float64,
	velLimit, _ float64,
	pathTolerance float64,
) (*plannedTrajectory, error) {
	if velLimit <= 0 {
		return nil, errors.New("velLimit must be positive")
	}
	if len(waypoints) == 0 {
		return nil, errors.New("at least one waypoint is required")
	}
	if pathTolerance > 0 {
		g.pathToleranceWarn.Do(func() {
			g.logger.Warn(
				"sim arm fake trajectory generator ignores path-tolerance; " +
					"trajectory will pass exactly through every waypoint with no blending",
			)
		})
	}

	// Defensive dedup so direct callers can pass un-cleaned input. Trajex does
	// the same internally; sim.go also dedups so it can short-circuit the
	// "already at target" case before calling Plan at all.
	waypoints = dedupWaypoints(waypoints, defaultDedupToleranceRads)

	nDof := len(waypoints[0])

	// Single-waypoint trivial case: nothing to plan, one-sample trajectory.
	if len(waypoints) == 1 {
		out := make([]float64, nDof)
		copy(out, waypoints[0])
		return &plannedTrajectory{
			sampleTimes:   []float64{0.0},
			sampleConfigs: out,
			nDof:          nDof,
		}, nil
	}

	// Segment durations under the "all joints finish simultaneously" rule:
	// segment_duration = max(|joint excursion|) / velLimit.
	nSegments := len(waypoints) - 1
	segmentDurs := make([]float64, nSegments)
	var totalDuration float64
	for i := 0; i < nSegments; i++ {
		d := maxAbsDiff(waypoints[i+1], waypoints[i]) / velLimit
		segmentDurs[i] = d
		totalDuration += d
	}

	// totalDuration == 0 should already be impossible post-dedup, but be defensive.
	if totalDuration == 0 {
		out := make([]float64, nDof)
		copy(out, waypoints[0])
		return &plannedTrajectory{
			sampleTimes:   []float64{0.0},
			sampleConfigs: out,
			nDof:          nDof,
		}, nil
	}

	// Sample uniformly across the whole trajectory at fakeSamplingFreqHz,
	// matching the trajex sampler formula so updateForTime treats both
	// trajectories identically.
	nSamples := int(math.Ceil(totalDuration*fakeSamplingFreqHz)) + 1
	dt := totalDuration / float64(nSamples-1)

	sampleTimes := make([]float64, nSamples)
	sampleConfigs := make([]float64, nSamples*nDof)

	// Monotonic cursor over segments. segStart is the elapsed time at the start
	// of waypoints[segIdx].
	segIdx := 0
	segStart := 0.0
	for k := 0; k < nSamples; k++ {
		t := float64(k) * dt
		if k == nSamples-1 {
			// Pin the final sample to exact totalDuration to avoid float drift.
			t = totalDuration
		}

		for segIdx < nSegments-1 && t > segStart+segmentDurs[segIdx] {
			segStart += segmentDurs[segIdx]
			segIdx++
		}

		alpha := (t - segStart) / segmentDurs[segIdx]
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 1 {
			alpha = 1
		}

		sampleTimes[k] = t
		startWp := waypoints[segIdx]
		endWp := waypoints[segIdx+1]
		for j := 0; j < nDof; j++ {
			sampleConfigs[k*nDof+j] = startWp[j] + alpha*(endWp[j]-startWp[j])
		}
	}

	return &plannedTrajectory{
		sampleTimes:   sampleTimes,
		sampleConfigs: sampleConfigs,
		nDof:          nDof,
	}, nil
}

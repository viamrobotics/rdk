package streaming

import (
	"context"
	"sync"
	"time"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/testutils/inject"
)

type fakeStreamRecorder struct {
	mu      sync.Mutex
	batches [][]arm.TrajectoryPoint
}

func (r *fakeStreamRecorder) get() [][]arm.TrajectoryPoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.batches
}

// newFakeStreamingArm builds a fake arm whose Kinematics(ctx) reports dof joints, each limited to
// velDegPerSec/accelDegPerSec2, for use with Run (which queries them via
// referenceframe.TrajectoryLimits). Tests that exercise armStream directly rather than through
// Run don't need these to be accurate; any values will do.
func newFakeStreamingArm(dof int, velDegPerSec, accelDegPerSec2 float64) (*inject.Arm, *fakeStreamRecorder) {
	rec := &fakeStreamRecorder{}
	inj := inject.NewArm("test-arm")
	inj.KinematicsFunc = func(ctx context.Context) (referenceframe.Model, error) {
		return testModel(dof, velDegPerSec, accelDegPerSec2)
	}
	inj.MoveThroughJointPositionsStreamedFunc = func(
		ctx context.Context,
		batches <-chan []arm.TrajectoryPoint,
		responses chan<- arm.Response,
		extra map[string]interface{},
	) error {
		// Honor the interface contract: return only once the trajectory is done, not
		// when the input stream closes. Playback is simulated against a wall clock
		// anchored at the first batch; after the input stream closes, wait out
		// whatever trajectory time remains (or bail on ctx cancellation, like a real
		// driver interrupting a move).
		var started time.Time
		var lastPointTime time.Duration
		for batch := range batches {
			if started.IsZero() {
				started = time.Now()
			}
			rec.mu.Lock()
			rec.batches = append(rec.batches, batch)
			rec.mu.Unlock()
			if len(batch) > 0 {
				lastPointTime = batch[len(batch)-1].Time
			}
		}
		if started.IsZero() {
			return nil
		}
		remaining := lastPointTime - time.Since(started)
		if remaining <= 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(remaining):
			return nil
		}
	}
	return inj, rec
}

package builtin

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/testutils/inject"
)

const chaserTestStep = 10 * time.Millisecond

func newTestChaser(start float64) *jointChaser {
	return newJointChaser([]float64{start}, []float64{1.5}, []float64{8}, defaultTeleopChaseGainPerSec, defaultTeleopChaseFeedforward)
}

// checkLimits steps the chaser n times, asserting on every step that velocity and acceleration
// stay within the limits, that acceleration never jumps by more than the jerk bound allows, and
// that position is the integral of velocity. It returns the positions visited.
func checkLimits(t *testing.T, c *jointChaser, n int, each func(i int)) []float64 {
	t.Helper()
	h := chaserTestStep.Seconds()
	maxAccStep := c.accMax[0]*h/teleopChaseAccelRampTime.Seconds() + 1e-9
	positions := make([]float64, 0, n)
	for i := range n {
		if each != nil {
			each(i)
		}
		prevPos, prevAcc := c.pos[0], c.acc[0]
		c.step(chaserTestStep)
		test.That(t, math.Abs(c.vel[0]), test.ShouldBeLessThanOrEqualTo, c.velMax[0]+1e-9)
		test.That(t, math.Abs(c.acc[0]), test.ShouldBeLessThanOrEqualTo, c.accMax[0]+1e-9)
		test.That(t, math.Abs(c.acc[0]-prevAcc), test.ShouldBeLessThanOrEqualTo, maxAccStep)
		test.That(t, c.pos[0]-prevPos, test.ShouldAlmostEqual, c.vel[0]*h, 1e-12)
		positions = append(positions, c.pos[0])
	}
	return positions
}

func TestJointChaserStepToTarget(t *testing.T) {
	// A large step: the chaser should saturate, then settle on the target without passing it.
	c := newTestChaser(0)
	c.setTarget([]float64{1.0}, time.Now())
	positions := checkLimits(t, c, 400, nil)
	peak := 0.0
	for _, p := range positions {
		peak = math.Max(peak, p)
	}
	test.That(t, peak, test.ShouldBeLessThan, 1.0+0.005)
	test.That(t, c.pos[0], test.ShouldAlmostEqual, 1.0, 1e-3)
	test.That(t, c.atRest(), test.ShouldBeTrue)
}

func TestJointChaserFollowsMovingTarget(t *testing.T) {
	// A target moving steadily at a third of the speed limit, updated every step: once the
	// chaser has caught up it should move at the target's speed, close behind it, without the
	// stop-and-go of a planner that stops at each target.
	c := newTestChaser(0)
	const targetVel = 0.5
	now := time.Now()
	target := 0.0
	var lateVels []float64
	checkLimits(t, c, 300, func(i int) {
		target += targetVel * chaserTestStep.Seconds()
		now = now.Add(chaserTestStep)
		c.setTarget([]float64{target}, now)
		if i >= 150 {
			lateVels = append(lateVels, c.vel[0])
		}
	})
	for _, v := range lateVels {
		test.That(t, v, test.ShouldAlmostEqual, targetVel, 0.02)
	}
	// With full velocity feedforward the steady-state lag is small: well under 30ms of motion.
	test.That(t, target-c.pos[0], test.ShouldBeLessThan, targetVel*0.03)
	test.That(t, target-c.pos[0], test.ShouldBeGreaterThan, -targetVel*0.03)
}

func TestJointChaserTargetStops(t *testing.T) {
	// The target moves, then stops arriving. The chaser should come to rest on the last target,
	// even though it was matching the target's speed.
	c := newTestChaser(0)
	now := time.Now()
	target := 0.0
	checkLimits(t, c, 150, func(_ int) {
		target += 1.0 * chaserTestStep.Seconds()
		now = now.Add(chaserTestStep)
		c.setTarget([]float64{target}, now)
	})
	positions := checkLimits(t, c, 300, nil)
	peak := 0.0
	for _, p := range positions {
		peak = math.Max(peak, p)
	}
	// A target that stops dead cannot be stopped on: the chaser was moving with it. It must not
	// run on further than the distance it takes to brake from the target's speed.
	brakingDistance := 1.0 * 1.0 / (2 * teleopChaseBrakeFraction * c.accMax[0])
	test.That(t, peak-target, test.ShouldBeLessThan, brakingDistance)
	test.That(t, c.atRest(), test.ShouldBeTrue)
	test.That(t, c.pos[0], test.ShouldAlmostEqual, target, 1e-3)
}

func TestJointChaserReversal(t *testing.T) {
	// A target that reverses direction: limits must hold through the reversal.
	c := newTestChaser(0)
	now := time.Now()
	checkLimits(t, c, 600, func(i int) {
		now = now.Add(chaserTestStep)
		c.setTarget([]float64{0.6 * math.Sin(float64(i)*chaserTestStep.Seconds()*3)}, now)
	})
}

// recordingStreamArm is an arm whose MoveThroughJointPositionsStreamed records every point it is
// sent and returns when its batches channel is closed.
type recordingStreamArm struct {
	*inject.Arm
	mu     sync.Mutex
	points []arm.TrajectoryPoint
	stops  int
}

func newRecordingStreamArm() *recordingStreamArm {
	r := &recordingStreamArm{Arm: inject.NewArm("test-arm")}
	r.StopFunc = func(context.Context, map[string]interface{}) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.stops++
		return nil
	}
	r.MoveThroughJointPositionsStreamedFunc = func(
		ctx context.Context, batches <-chan []arm.TrajectoryPoint, _ chan<- arm.Response, _ map[string]interface{},
	) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case batch, ok := <-batches:
				if !ok {
					return nil
				}
				r.mu.Lock()
				r.points = append(r.points, batch...)
				r.mu.Unlock()
			}
		}
	}
	return r
}

func (r *recordingStreamArm) snapshot() ([]arm.TrajectoryPoint, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]arm.TrajectoryPoint(nil), r.points...), r.stops
}

func chaseTestOptions(velMax, accMax float64) streaming.StreamOptions {
	return streaming.StreamOptions{
		ArmSideTargetRunway: 30 * time.Millisecond,
		SendToArmInterval:   10 * time.Millisecond,
		MoveOptions: arm.MoveOptions{
			MaxVelRadsJoints: []float64{velMax, velMax},
			MaxAccRadsJoints: []float64{accMax, accMax},
		},
	}
}

func TestTeleopChaseRun(t *testing.T) {
	logger := logging.NewTestLogger(t)
	a := newRecordingStreamArm()
	const velMax, accMax = 1.5, 8.0
	opts := chaseTestOptions(velMax, accMax)
	tc := teleopChase{gain: defaultTeleopChaseGainPerSec, feedforward: defaultTeleopChaseFeedforward}

	targets := make(chan []referenceframe.Input, 4)
	seed := []referenceframe.Input{0.1, -0.2}
	goal := []referenceframe.Input{0.4, -0.5}
	done := make(chan error, 1)
	go func() {
		done <- tc.run(context.Background(), logger, a, opts, targets, seed, diagnostics.New(0))
	}()

	// Hold at the seed for a moment: the stream must keep sending while there is no target.
	time.Sleep(100 * time.Millisecond)
	held, _ := a.snapshot()
	test.That(t, len(held), test.ShouldBeGreaterThan, 5)

	targets <- goal
	time.Sleep(600 * time.Millisecond)
	close(targets)
	select {
	case err := <-done:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(10 * time.Second):
		t.Fatal("chaser did not finish after its targets were closed")
	}

	points, stops := a.snapshot()
	// A stream that ends normally leaves the arm settled; it is not stopped.
	test.That(t, stops, test.ShouldEqual, 0)
	// The first point is the starting state at time zero.
	test.That(t, points[0].Time, test.ShouldEqual, time.Duration(0))
	test.That(t, points[0].Positions, test.ShouldResemble, seed)
	test.That(t, points[0].Constraints.Velocities, test.ShouldResemble, []float64{0, 0})
	for i := 1; i < len(points); i++ {
		// Time advances by exactly one interval per point, and every point is within limits.
		test.That(t, points[i].Time-points[i-1].Time, test.ShouldEqual, opts.SendToArmInterval)
		for j := range seed {
			test.That(t, math.Abs(points[i].Constraints.Velocities[j]), test.ShouldBeLessThanOrEqualTo, velMax+1e-9)
			test.That(t, math.Abs(points[i].Constraints.Accelerations[j]), test.ShouldBeLessThanOrEqualTo, accMax+1e-9)
		}
	}
	// It ends at rest on the last target.
	last := points[len(points)-1]
	for j := range goal {
		test.That(t, last.Positions[j], test.ShouldAlmostEqual, goal[j], 1e-3)
		test.That(t, last.Constraints.Velocities[j], test.ShouldAlmostEqual, 0, 2e-3)
	}
}

func TestTeleopChaseRunStopsArmOnCancelAndError(t *testing.T) {
	logger := logging.NewTestLogger(t)
	tc := teleopChase{gain: defaultTeleopChaseGainPerSec, feedforward: defaultTeleopChaseFeedforward}
	seed := []referenceframe.Input{0, 0}

	// Cancellation stops the arm.
	a := newRecordingStreamArm()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- tc.run(ctx, logger, a, chaseTestOptions(1.5, 8), make(chan []referenceframe.Input), seed, diagnostics.New(0))
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	test.That(t, <-done, test.ShouldBeError, context.Canceled)
	_, stops := a.snapshot()
	test.That(t, stops, test.ShouldEqual, 1)

	// So does the arm ending the stream with an error.
	failing := newRecordingStreamArm()
	failing.MoveThroughJointPositionsStreamedFunc = func(
		context.Context, <-chan []arm.TrajectoryPoint, chan<- arm.Response, map[string]interface{},
	) error {
		return errors.New("arm fault")
	}
	err := tc.run(context.Background(), logger, failing, chaseTestOptions(1.5, 8), make(chan []referenceframe.Input), seed, diagnostics.New(0))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "arm fault")
	_, stops = failing.snapshot()
	test.That(t, stops, test.ShouldEqual, 1)

	// Limits that do not match the arm's joint count are refused before anything is sent.
	bad := chaseTestOptions(1.5, 8)
	bad.MoveOptions.MaxVelRadsJoints = []float64{1.5}
	err = tc.run(context.Background(), logger, newRecordingStreamArm(), bad, make(chan []referenceframe.Input), seed, diagnostics.New(0))
	test.That(t, err, test.ShouldNotBeNil)
}

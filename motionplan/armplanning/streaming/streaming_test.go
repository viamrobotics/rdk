package streaming

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/testutils/inject"
)

const testDOF = 6

// fastConfig streams with a small buffer so end-to-end tests finish quickly.
func fastConfig() Config {
	return Config{
		BufferAheadInArmMs:   50,
		SendToArmIntervalMs:  10,
		VelLimitDegPerSec:    30,
		AccelLimitDegPerSec2: 60,
	}
}

// armRecorder is an inject.Arm streaming callback that records every trajectory
// point it receives and counts how many streams were opened.
type armRecorder struct {
	mu      sync.Mutex
	streams int
	points  []arm.TrajectoryPoint

	// enough is closed once total received points reaches minPoints (if set).
	minPoints int
	enough    chan struct{}
	enoughOne sync.Once

	// wantStreams is closed once streams reaches the given count (if set).
	streamTarget int
	wantStreams  chan struct{}
	streamsOne   sync.Once
}

func (r *armRecorder) fn(
	_ context.Context,
	batches <-chan []arm.TrajectoryPoint,
	_ chan<- arm.Response,
	_ map[string]interface{},
) error {
	r.mu.Lock()
	r.streams++
	streams := r.streams
	r.mu.Unlock()
	if r.wantStreams != nil && streams >= r.streamTarget {
		r.streamsOne.Do(func() { close(r.wantStreams) })
	}

	for batch := range batches {
		r.mu.Lock()
		r.points = append(r.points, batch...)
		total := len(r.points)
		r.mu.Unlock()
		if r.enough != nil && total >= r.minPoints {
			r.enoughOne.Do(func() { close(r.enough) })
		}
	}
	return nil
}

func ramp(dof int, joint0 float64) Target {
	p := make([]referenceframe.Input, dof)
	p[0] = referenceframe.Input(joint0)
	return Target{Positions: p}
}

func TestConfigApplyDefaultsAndValidate(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()
	test.That(t, c.BufferAheadInArmMs, test.ShouldEqual, defaultBufferAheadInArmMs)
	test.That(t, c.SendToArmIntervalMs, test.ShouldEqual, defaultSendToArmIntervalMs)
	test.That(t, c.VelLimitDegPerSec, test.ShouldEqual, defaultVelLimitDegPerSec)
	test.That(t, c.AccelLimitDegPerSec2, test.ShouldEqual, defaultAccelLimitDegPerSec2)
	test.That(t, c.Validate(), test.ShouldBeNil)

	test.That(t, (&Config{SendToArmIntervalMs: 10, BufferAheadInArmMs: -1}).Validate(), test.ShouldNotBeNil)
	// A zero send interval is invalid (division by zero when converting to Hz).
	test.That(t, (&Config{SendToArmIntervalMs: 0, BufferAheadInArmMs: 10}).Validate(), test.ShouldNotBeNil)
	test.That(t, (&Config{SendToArmIntervalMs: 10, VelLimitDegPerSec: -1}).Validate(), test.ShouldNotBeNil)
}

// TestArmStreamAdd exercises the strictly-increasing-time contract and the
// rad->deg conversion in armStream.add without touching trajex or the arm.
func TestArmStreamAdd(t *testing.T) {
	s := &armStream{}

	// First point may have zero time.
	test.That(t, s.add(pvat{
		positions:     []float64{0.1, 0.2},
		velocities:    []float64{math.Pi, 0},
		accelerations: []float64{0, math.Pi / 2},
		time:          0,
	}), test.ShouldBeNil)

	// Strictly-increasing time is accepted.
	test.That(t, s.add(pvat{
		positions:     []float64{0.3, 0.4},
		velocities:    []float64{0, 0},
		accelerations: []float64{0, 0},
		time:          10 * time.Millisecond,
	}), test.ShouldBeNil)

	// Zero dt after the first point is rejected.
	test.That(t, s.add(pvat{
		positions: []float64{0.3, 0.4}, velocities: []float64{0, 0}, accelerations: []float64{0, 0},
		time: 10 * time.Millisecond,
	}), test.ShouldNotBeNil)

	// Negative dt is rejected.
	test.That(t, s.add(pvat{
		positions: []float64{0.3, 0.4}, velocities: []float64{0, 0}, accelerations: []float64{0, 0},
		time: 5 * time.Millisecond,
	}), test.ShouldNotBeNil)

	// Only the two accepted points were appended.
	test.That(t, len(s.points), test.ShouldEqual, 2)
	test.That(t, s.points[0].Time, test.ShouldEqual, time.Duration(0))
	test.That(t, s.points[1].Time, test.ShouldEqual, 10*time.Millisecond)
	// pi rad/s -> 180 deg/s.
	test.That(t, s.points[0].Constraints.Velocities[0], test.ShouldAlmostEqual, 180.0)
}

// TestStreamJointTargetsStreamsToArm feeds a continuous ramp and confirms the
// arm receives a strictly-increasing-time joint trajectory, then that cancel
// unwinds the pipeline promptly.
func TestStreamJointTargetsStreamsToArm(t *testing.T) {
	rec := &armRecorder{minPoints: 20, enough: make(chan struct{})}
	a := inject.NewArm("test")
	a.MoveThroughJointPositionsStreamedFunc = rec.fn

	seed := make([]referenceframe.Input, testDOF)
	targets := make(chan Target)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- StreamJointTargets(ctx, a, seed, targets, fastConfig()) }()

	go func() {
		for i := 1; ; i++ {
			select {
			case <-ctx.Done():
				return
			case targets <- ramp(testDOF, float64(i)*0.02):
			}
		}
	}()

	select {
	case <-rec.enough:
	case <-time.After(5 * time.Second):
		t.Fatal("arm did not receive enough streamed points")
	}
	cancel()

	select {
	case err := <-done:
		test.That(t, err == nil || errors.Is(err, context.Canceled), test.ShouldBeTrue)
	case <-time.After(5 * time.Second):
		t.Fatal("StreamJointTargets did not return after cancel")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	test.That(t, len(rec.points) > 0, test.ShouldBeTrue)
	last := time.Duration(-1)
	for _, p := range rec.points {
		test.That(t, len(p.Positions), test.ShouldEqual, testDOF)
		test.That(t, p.Time > last, test.ShouldBeTrue)
		last = p.Time
	}
}

// TestStreamJointTargetsFlushOpensNewStream confirms a Flush target ends the
// current arm stream and opens a fresh one.
func TestStreamJointTargetsFlushOpensNewStream(t *testing.T) {
	rec := &armRecorder{streamTarget: 2, wantStreams: make(chan struct{})}
	a := inject.NewArm("test")
	a.MoveThroughJointPositionsStreamedFunc = rec.fn

	seed := make([]referenceframe.Input, testDOF)
	targets := make(chan Target)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- StreamJointTargets(ctx, a, seed, targets, fastConfig()) }()

	go func() {
		send := func(t Target) bool {
			select {
			case <-ctx.Done():
				return false
			case targets <- t:
				return true
			}
		}
		for i := 1; i <= 5; i++ {
			if !send(ramp(testDOF, float64(i)*0.02)) {
				return
			}
		}
		flush := ramp(testDOF, 0.12)
		flush.Flush = true
		if !send(flush) {
			return
		}
		for i := 1; ; i++ {
			if !send(ramp(testDOF, 0.12+float64(i)*0.02)) {
				return
			}
		}
	}()

	select {
	case <-rec.wantStreams:
	case <-time.After(5 * time.Second):
		t.Fatal("Flush did not open a second arm stream")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StreamJointTargets did not return after cancel")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	test.That(t, rec.streams >= 2, test.ShouldBeTrue)
}

// TestStreamJointTargetsGracefulClose confirms that closing the targets channel
// drains the trajectory to the arm and returns nil.
func TestStreamJointTargetsGracefulClose(t *testing.T) {
	rec := &armRecorder{}
	a := inject.NewArm("test")
	a.MoveThroughJointPositionsStreamedFunc = rec.fn

	seed := make([]referenceframe.Input, testDOF)
	targets := make(chan Target)

	done := make(chan error, 1)
	go func() { done <- StreamJointTargets(context.Background(), a, seed, targets, fastConfig()) }()

	targets <- ramp(testDOF, 0.05)
	close(targets)

	select {
	case err := <-done:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(5 * time.Second):
		t.Fatal("StreamJointTargets did not return after graceful close")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	test.That(t, len(rec.points) > 0, test.ShouldBeTrue)
	final := float64(rec.points[len(rec.points)-1].Positions[0])
	test.That(t, math.Abs(final-0.05) < 1e-2, test.ShouldBeTrue)
}

package streaming

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/testutils/inject"
)

func TestArmStreamerSendsPointsAndReportsRunway(t *testing.T) {
	got := make(chan []arm.TrajectoryPoint, 4)
	a := &inject.Arm{}
	a.MoveThroughJointPositionsStreamedFunc = func(ctx context.Context, b <-chan []arm.TrajectoryPoint,
		_ chan<- arm.Response, _ map[string]interface{},
	) error {
		for batch := range b {
			got <- batch
		}
		return nil
	}
	s := NewArmStreamer(context.Background(), a, diagnostics.New(0))
	test.That(t, s.Runway(), test.ShouldEqual, time.Duration(0))
	err := s.Send(context.Background(), []Point{{
		Positions: []float64{0.1, 0.2}, Velocities: []float64{0, 0},
		Accelerations: []float64{0, 0}, Time: 0,
	}})
	test.That(t, err, test.ShouldBeNil)
	var batch []arm.TrajectoryPoint
	select {
	case batch = <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for batch")
	}
	test.That(t, batch[0].Positions, test.ShouldResemble, []float64{0.1, 0.2})
	err = s.Send(context.Background(), []Point{{
		Positions: []float64{0.3, 0.4}, Velocities: []float64{0, 0},
		Accelerations: []float64{0, 0}, Time: 200 * time.Millisecond,
	}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.Runway(), test.ShouldBeGreaterThan, time.Duration(0))
	test.That(t, s.Runway(), test.ShouldBeLessThanOrEqualTo, 200*time.Millisecond)
	test.That(t, s.Close(), test.ShouldBeNil)
}

func TestArmStreamerSendAfterStreamEnds(t *testing.T) {
	a := &inject.Arm{}
	a.MoveThroughJointPositionsStreamedFunc = func(context.Context, <-chan []arm.TrajectoryPoint,
		chan<- arm.Response, map[string]interface{},
	) error {
		return errors.New("arm stopped")
	}
	s := NewArmStreamer(context.Background(), a, diagnostics.New(0))
	select {
	case <-s.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Done")
	}
	err := s.Send(context.Background(), []Point{{Positions: []float64{0}, Velocities: []float64{0}, Accelerations: []float64{0}}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "arm stopped")
}

package sim

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestMoveThroughJointPositionsStreamed(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	resConf := resource.Config{
		Name:  "arm",
		API:   arm.API,
		Model: Model,
		ConvertedAttributes: &Config{
			Model: "lite6",
			// Large speed plus simulated time so each blocking per-point move completes on its own.
			Speed:        100.0,
			SimulateTime: true,
		},
	}
	simArmI, err := NewArm(ctx, nil, resConf, logger)
	test.That(t, err, test.ShouldBeNil)
	simArm := simArmI.(*simulatedArm)
	defer func() {
		test.That(t, simArm.Close(ctx), test.ShouldBeNil)
	}()

	batches := make(chan []arm.TrajectoryPoint)
	responses := make(chan arm.Response)
	errCh := make(chan error, 1)
	go func() {
		errCh <- simArm.MoveThroughJointPositionsStreamed(ctx, batches, responses, nil)
	}()

	respCount := 0
	drained := make(chan struct{})
	go func() {
		for range responses {
			respCount++
		}
		close(drained)
	}()

	final := []float64{1, -2, 0, 0, 0, 0}
	batches <- []arm.TrajectoryPoint{{Positions: []float64{0.5, -1, 0, 0, 0, 0}}}
	batches <- []arm.TrajectoryPoint{{Positions: final}}
	close(batches)

	test.That(t, <-errCh, test.ShouldBeNil)
	close(responses)
	<-drained

	// One ack per batch, and the arm moved through to the final point.
	test.That(t, respCount, test.ShouldEqual, 2)
	got, err := simArm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, final)
}

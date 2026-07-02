package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

func TestMoveThroughJointPositionsStreamed(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{Name: "testArm", ConvertedAttributes: &Config{ArmModel: ur5eModel}}
	a, err := NewArm(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	m, err := a.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	dof := len(m.DoF())

	final := make([]referenceframe.Input, dof)
	for i := range final {
		final[i] = 0.1 * float64(i)
	}

	batches := make(chan []arm.TrajectoryPoint)
	responses := make(chan arm.Response)
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.MoveThroughJointPositionsStreamed(ctx, batches, responses, nil)
	}()

	respCount := 0
	drained := make(chan struct{})
	go func() {
		for range responses {
			respCount++
		}
		close(drained)
	}()

	batches <- []arm.TrajectoryPoint{{Positions: make([]referenceframe.Input, dof)}}
	batches <- []arm.TrajectoryPoint{{Positions: final}}
	close(batches)

	test.That(t, <-errCh, test.ShouldBeNil)
	close(responses)
	<-drained

	// One ack per batch, and the fake teleported to the final point.
	test.That(t, respCount, test.ShouldEqual, 2)
	got, err := a.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	for i := range final {
		test.That(t, got[i], test.ShouldAlmostEqual, final[i])
	}
}

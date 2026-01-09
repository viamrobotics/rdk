package sim

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestBasic(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	resConf := resource.Config{
		Name:  "arm",
		API:   arm.API,
		Model: Model,
		ConvertedAttributes: &Config{
			Model: "lite6",
			Speed: 1.0, // radians per second
		},
	}

	simArmI, err := NewArm(ctx, nil, resConf, logger)
	test.That(t, err, test.ShouldBeNil)
	simArm := simArmI.(*SimulatedArm)

	// Assert the starting joint position is all zeroes.
	currInputs, err := simArm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currInputs, test.ShouldResemble, []float64{0, 0, 0, 0, 0, 0})
	test.That(t, simArm.operation.isMoving(), test.ShouldBeFalse)

	// Set up a move that will take "2 seconds" to complete. `MoveToJointPositions` is blocking and
	// time must be advanced manually. Hence the goroutine.
	moveFuture := make(chan struct{})
	go func() {
		err := simArm.MoveToJointPositions(ctx, []float64{1, -2, 0, 0, 0, 0}, nil)
		test.That(t, err, test.ShouldBeNil)
		close(moveFuture)
	}()

	// Time hasn't advanced, assert the current inputs remain at 0.
	currInputs, err = simArm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currInputs, test.ShouldResemble, []float64{0, 0, 0, 0, 0, 0})
	// But we are now considered "moving".
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		isMoving, err := simArm.IsMoving(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, isMoving, test.ShouldBeTrue)
	})

	// Advance time by one second.
	clock := simArm.lastUpdated
	clock = clock.Add(time.Second)
	simArm.UpdateForTime(clock)

	// Assert the joint position for first two joints changed to `1`.
	currInputs, err = simArm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currInputs, test.ShouldResemble, []float64{1, -1, 0, 0, 0, 0})
	test.That(t, simArm.operation.isMoving(), test.ShouldBeTrue)
	select {
	case <-moveFuture:
		t.Fatal("Background goroutine calling `MoveToJointPositions` has incorrectly returned.")
	default:
	}

	// Advance time by another second. Assert the joint position matches the target. Assert the
	// operation is considered done.
	clock = clock.Add(time.Second)
	simArm.UpdateForTime(clock)

	currInputs, err = simArm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currInputs, test.ShouldResemble, []float64{1, -2, 0, 0, 0, 0})
	test.That(t, simArm.operation.isMoving(), test.ShouldBeFalse)
	select {
	case <-moveFuture:
	case <-time.After(time.Second):
		t.Fatal("Background goroutine calling `MoveToJointPositions` has not returned.")
	}
}

func TestStop(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	resConf := resource.Config{
		Name:  "arm",
		API:   arm.API,
		Model: Model,
		ConvertedAttributes: &Config{
			Model: "lite6",
			Speed: 1.0, // radians per second
		},
	}

	simArmI, err := NewArm(ctx, nil, resConf, logger)
	test.That(t, err, test.ShouldBeNil)
	simArm := simArmI.(*SimulatedArm)

	// Set up a move that will take "2 seconds" to complete. `MoveToJointPositions` is blocking and
	// time must be advanced manually. Hence the goroutine.
	moveFuture := make(chan error)
	go func() {
		err := simArm.MoveToJointPositions(ctx, []float64{1, -2, 0, 0, 0, 0}, nil)
		moveFuture <- err
		close(moveFuture)
	}()

	// Time hasn't advanced, assert the current inputs remain at 0.
	currInputs, err := simArm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currInputs, test.ShouldResemble, []float64{0, 0, 0, 0, 0, 0})
	// But we are now considered "moving".
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		isMoving, err := simArm.IsMoving(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, isMoving, test.ShouldBeTrue)
	})

	err = simArm.Stop(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	select {
	case moveErr := <-moveFuture:
		test.That(t, moveErr, test.ShouldNotBeNil)
		test.That(t, moveErr.Error(), test.ShouldEqual, "stopped before reaching target")
	case <-time.After(time.Second):
		t.Fatal("Background goroutine calling `MoveToJointPositions` has not returned.")
	}
}

func TestTimeSimulation(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	resConf := resource.Config{
		Name:  "arm",
		API:   arm.API,
		Model: Model,
		ConvertedAttributes: &Config{
			Model: "lite6",
			// We make `Speed` large such that `MoveToJointPositions` finishes quickly.
			Speed: 100.0,

			// Set up a background goroutine that advances time.
			SimulateTime: true,
		},
	}

	simArmI, err := NewArm(ctx, nil, resConf, logger)
	test.That(t, err, test.ShouldBeNil)
	simArm := simArmI.(*SimulatedArm)
	defer func() {
		// When `SimulateTime` is true, we must call `Close` to wait on background goroutines.
		err = simArm.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	}()

	// Assert the starting joint position is all zeroes.
	currInputs, err := simArm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currInputs, test.ShouldResemble, []float64{0, 0, 0, 0, 0, 0})
	test.That(t, simArm.operation.isMoving(), test.ShouldBeFalse)

	// Set up a move that will take a few milliseconds to complete. `MoveToJointPositions` is
	// blocking, but we asked the arm to simulate time. Hence we can assert this function returns
	// without further intervention.
	err = simArm.MoveToJointPositions(ctx, []float64{1, -2, 0, 0, 0, 0}, nil)
	test.That(t, err, test.ShouldBeNil)
}

package gpio

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

func TestEncodedMotorControls(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// create inject motor
	// note, all test files should have an inject motor and an inject
	// board in the future
	fakeMotor := &Motor{
		maxRPM:    100,
		logger:    logger,
		opMgr:     operation.NewSingleOperationManager(),
		motorType: DirectionPwm,
	}

	vals := newState()

	// create an inject encoder
	enc := injectEncoder(vals)

	// create an encoded motor
	conf := resource.Config{
		Name: motorName,
		ConvertedAttributes: &Config{
			Encoder:          encoderName,
			TicksPerRotation: 1,
			ControlParameters: &motorPIDConfig{
				P: 1,
				I: 2,
				D: 0,
			},
		},
	}

	m, err := setupMotorWithControls(context.Background(), fakeMotor, enc, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	cm, ok := m.(*controlledMotor)
	test.That(t, ok, test.ShouldBeTrue)

	defer func() {
		test.That(t, cm.Close(context.Background()), test.ShouldBeNil)
	}()

	t.Run("encoded motor controls test loop exists", func(t *testing.T) {
		test.That(t, cm.SetRPM(context.Background(), 10, nil), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, cm.loop, test.ShouldNotBeNil)
		})
		test.That(t, cm.Stop(context.Background(), nil), test.ShouldBeNil)
	})
}

func TestControlledMotorCreation(t *testing.T) {
	logger := logging.NewTestLogger(t)
	// create an encoded motor
	conf := resource.Config{
		Name: motorName,
		ConvertedAttributes: &Config{
			BoardName: boardName,
			Pins: PinConfig{
				Direction: "1",
				PWM:       "2",
			},
			Encoder:          encoderName,
			TicksPerRotation: 1,
			ControlParameters: &motorPIDConfig{
				P: 1,
				I: 2,
				D: 0,
			},
		},
	}

	deps := make(resource.Dependencies)

	vals := newState()
	deps[encoder.Named(encoderName)] = injectEncoder(vals)
	deps[board.Named(boardName)] = injectBoard()

	m, err := createNewMotor(context.Background(), deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	cm, ok := m.(*controlledMotor)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, cm.enc.Name().ShortName(), test.ShouldEqual, encoderName)
	test.That(t, cm.real.Name().ShortName(), test.ShouldEqual, motorName)

	// test DoCommand
	expectedPID := control.PIDConfig{P: 0.1, I: 2.0, D: 0.0}
	cm.tunedVals = &[]control.PIDConfig{expectedPID, {}}
	expectedeMap := make(map[string]interface{})
	expectedeMap["get_tuned_pid"] = (fmt.Sprintf("{p: %v, i: %v, d: %v, type: %v} ",
		expectedPID.P, expectedPID.I, expectedPID.D, expectedPID.Type))

	req := make(map[string]interface{})
	req["get_tuned_pid"] = true
	resp, err := m.DoCommand(context.Background(), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, expectedeMap)

	emptyMap := make(map[string]interface{})
	req["get_tuned_pid"] = false
	resp, err = cm.DoCommand(context.Background(), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, emptyMap)
}

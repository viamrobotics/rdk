package arduino

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/encoder"
	"go.viam.com/rdk/config"
)

func TestArduinoPWM(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	for i, tc := range []struct {
		conf config.Config
		err  string
	}{
		{
			config.Config{
				Components: []config.Component{
					{
						Name:  "m1",
						Model: "arduino",
						Type:  motor.SubtypeName,
						ConvertedAttributes: &motor.Config{
							Pins: motor.PinConfig{
								PWM:          "5",
								A:            "6",
								B:            "7",
								EnablePinLow: "8",
							},
							TicksPerRotation: 2000,
							PWMFreq:          2000,
						},
					},
					{
						Name:  "e1",
						Model: "arduino",
						Type:  encoder.SubtypeName,
						ConvertedAttributes: &EncoderConfig{
							Pins: EncoderPins{
								A:            "3",
								B:            "2",
							},
							TicksPerRotation: 2000,
							MotorName: "m1",
						},
					},
				},
			},
			"",
		},
		{
			config.Config{
				Components: []config.Component{
					{
						Name:  "m1",
						Model: "arduino",
						Type:  motor.SubtypeName,
						ConvertedAttributes: &motor.Config{
							Pins: motor.PinConfig{
								A:            "6",
								B:            "7",
								EnablePinLow: "8",
							},
							TicksPerRotation: 2000,
							PWMFreq:          2000,
						},
					},
					{
						Name:  "e1",
						Model: "arduino",
						Type:  encoder.SubtypeName,
						ConvertedAttributes: &EncoderConfig{
							Pins: EncoderPins{
								A:            "3",
								B:            "2",
							},
							TicksPerRotation: 2000,
							MotorName: "m1",
						},
					},
				},
			},
			"",
		},
		{
			config.Config{
				Components: []config.Component{
					{
						Name:  "m1",
						Model: "arduino",
						Type:  motor.SubtypeName,
						ConvertedAttributes: &motor.Config{
							Pins: motor.PinConfig{
								PWM:       "5",
								Direction: "10",
							},
							TicksPerRotation: 2000,
							PWMFreq:          2000,
						},
					},
					{
						Name:  "e1",
						Model: "arduino",
						Type:  encoder.SubtypeName,
						ConvertedAttributes: &EncoderConfig{
							Pins: EncoderPins{
								A:            "3",
								B:            "2",
							},
							TicksPerRotation: 2000,
							MotorName: "m1",
						},
					},
				},
			},
			"",
		},
		{
			config.Config{
				Components: []config.Component{
					{
						Name:  "m1",
						Model: "arduino",
						Type:  motor.SubtypeName,
						ConvertedAttributes: &motor.Config{
							Pins: motor.PinConfig{
								PWM:          "35",
								A:            "6",
								B:            "7",
								EnablePinLow: "8",
							},
							TicksPerRotation: 2000,
							PWMFreq:          2000,
						},
					},
					{
						Name:  "e1",
						Model: "arduino",
						Type:  encoder.SubtypeName,
						ConvertedAttributes: &EncoderConfig{
							Pins: EncoderPins{
								A:            "3",
								B:            "2",
							},
							TicksPerRotation: 2000,
							MotorName: "m1",
						},
					},
				},
			},
			"couldn't set pwm freq for pin",
		},
	} {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			b, err := newArduino(&board.Config{}, logger)
			if err != nil && strings.HasPrefix(err.Error(), "found ") {
				t.Skip()
				return
			}
			test.That(t, err, test.ShouldBeNil)

			ecfg := tc.conf.Components[1].ConvertedAttributes.(*EncoderConfig)
			ePins := ecfg.Pins.(*EncoderPins)

			_, err = configureMotorForBoard(
				ctx,
				b,
				tc.conf.Components[0],
				tc.conf.Components[0].ConvertedAttributes.(*motor.Config),
				&Encoder{board: b, A: ePins.A, B: ePins.B, name: ecfg.MotorName, ticksPerRotation: int64(ecfg.TicksPerRotation)},
			)

			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err.Error(),
					test.ShouldContainSubstring, tc.err)
				return
			}
			test.That(t, b, test.ShouldNotBeNil)

			err = b.SetPWMFreq(ctx, "7", 2000)
			test.That(t, err, test.ShouldBeNil)
			err = b.SetPWMFreq(ctx, "45", 2000)
			test.That(t, err, test.ShouldNotBeNil)
			err = b.SetPWMFreq(ctx, "-5", 2000)
			test.That(t, err, test.ShouldNotBeNil)

			err = b.SetPWM(ctx, "7", 0.03)
			test.That(t, err, test.ShouldBeNil)
			err = b.SetPWM(ctx, "45", 0.03)
			test.That(t, err, test.ShouldNotBeNil)
			err = b.SetPWM(ctx, "-5", 0.03)
			test.That(t, err, test.ShouldNotBeNil)

			defer b.Close()
		})
	}
}

// Test the A/B/PWM style IO.
func TestArduinoMotorABPWM(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := config.Config{
		Components: []config.Component{
			{
				Name:  "m1",
				Model: "arduino",
				Type:  motor.SubtypeName,
				ConvertedAttributes: &motor.Config{
					Pins: motor.PinConfig{
						PWM:          "11",
						A:            "37",
						B:            "39",
						EnablePinLow: "-1",
					},
					TicksPerRotation: 980,
				},
			},
			{
				Name:  "e1",
				Model: "arduino",
				Type:  encoder.SubtypeName,
				ConvertedAttributes: &EncoderConfig{
					Pins: EncoderPins{
						A:            "20",
						B:            "21",
					},
					TicksPerRotation: 980,
					MotorName: "m1",
				},
			},
		},
	}
	b, err := newArduino(&board.Config{}, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {
		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	ecfg := cfg.Components[1].ConvertedAttributes.(*EncoderConfig)
	ePins := ecfg.Pins.(*EncoderPins)

	m, err := configureMotorForBoard(
				context.Background(), 
				b, 
				cfg.Components[0], 
				cfg.Components[0].ConvertedAttributes.(*motor.Config),
				&Encoder{board: b, A: ePins.A, B: ePins.B, name: ecfg.MotorName, ticksPerRotation: int64(ecfg.TicksPerRotation)},
			)
	test.That(t, err, test.ShouldBeNil)

	arduinoMotorTests(ctx, t, m)
}

// Test the DIR/PWM style IO.
//nolint:dupl
func TestArduinoMotorDirPWM(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := config.Config{
		Components: []config.Component{
			{
				Name:  "m1",
				Model: "arduino",
				Type:  motor.SubtypeName,
				ConvertedAttributes: &motor.Config{
					Pins: motor.PinConfig{
						PWM:          "5",
						Direction:    "6",
						EnablePinLow: "7",
					},
					TicksPerRotation: 2000,
				},
			},
			{
				Name:  "e1",
				Model: "arduino",
				Type:  encoder.SubtypeName,
				ConvertedAttributes: &EncoderConfig{
					Pins: EncoderPins{
						A:            "3",
						B:            "2",
					},
					TicksPerRotation: 2000,
					MotorName: "m1",
				},
			},
		},
	}
	b, err := newArduino(&board.Config{}, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {
		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	ecfg := cfg.Components[1].ConvertedAttributes.(*EncoderConfig)
	ePins := ecfg.Pins.(*EncoderPins)

	m, err := configureMotorForBoard(
		context.Background(), 
		b, 
		cfg.Components[0], 
		cfg.Components[0].ConvertedAttributes.(*motor.Config),
		&Encoder{board: b, A: ePins.A, B: ePins.B, name: ecfg.MotorName, ticksPerRotation: int64(ecfg.TicksPerRotation)},
	)
	test.That(t, err, test.ShouldBeNil)

	arduinoMotorTests(ctx, t, m)
}

// Test the A/B only style IO.
//nolint:dupl
func TestArduinoMotorAB(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := config.Config{
		Components: []config.Component{
			{
				Name:  "m1",
				Model: "arduino",
				Type:  motor.SubtypeName,
				ConvertedAttributes: &motor.Config{
					Pins: motor.PinConfig{
						A:            "5",
						B:            "6",
						EnablePinLow: "7",
					},
					TicksPerRotation: 2000,
				},
			},
			{
				Name:  "e1",
				Model: "arduino",
				Type:  encoder.SubtypeName,
				ConvertedAttributes: &EncoderConfig{
					Pins: EncoderPins{
						A:            "3",
						B:            "2",
					},
					TicksPerRotation: 2000,
					MotorName: "m1",
				},
			},
		},
	}
	b, err := newArduino(&board.Config{}, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {
		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	ecfg := cfg.Components[1].ConvertedAttributes.(*EncoderConfig)
	ePins := ecfg.Pins.(*EncoderPins)

	m, err := configureMotorForBoard(
		context.Background(), 
		b, 
		cfg.Components[0], 
		cfg.Components[0].ConvertedAttributes.(*motor.Config),
		&Encoder{board: b, A: ePins.A, B: ePins.B, name: ecfg.MotorName, ticksPerRotation: int64(ecfg.TicksPerRotation)},
	)
	test.That(t, err, test.ShouldBeNil)

	arduinoMotorTests(ctx, t, m)
}

func arduinoMotorTests(ctx context.Context, t *testing.T, m motor.Motor) {
	t.Helper()

	t.Run("arduino motor features include position support", func(t *testing.T) {
		features, err := m.GetFeatures(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
	})

	t.Run("ardunio motor Go positive powerPct", func(t *testing.T) {
		startPos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.SetPower(ctx, 0.9)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos, err := m.GetPosition(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos-startPos, test.ShouldBeGreaterThan, 10)
		})

		test.That(t, m.Stop(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor Go negtive powerPct", func(t *testing.T) {
		startPos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.SetPower(ctx, -0.9)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos, err := m.GetPosition(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos-startPos, test.ShouldBeLessThan, -10)
		})

		test.That(t, m.Stop(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with positive rpm and positive revolutions", func(t *testing.T) {
		startPos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, 20, 1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err := m.IsPowered(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldBeFalse)

			pos, err := m.GetPosition(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos-startPos, test.ShouldBeGreaterThan, 1)
		})

		test.That(t, m.Stop(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with negative rpm and positive revolutions", func(t *testing.T) {
		startPos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, -20, 1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err := m.IsPowered(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldBeFalse)

			pos, err := m.GetPosition(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos-startPos, test.ShouldBeLessThan, -1)
		})

		test.That(t, m.Stop(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with positive rpm and negative revolutions", func(t *testing.T) {
		startPos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, 20, -1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err := m.IsPowered(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldBeFalse)

			pos, err := m.GetPosition(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos-startPos, test.ShouldBeLessThan, -1)
		})

		test.That(t, m.Stop(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with negative rpm and negative revolutions", func(t *testing.T) {
		startPos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, -20, -1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err := m.IsPowered(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldBeFalse)

			pos, err := m.GetPosition(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos-startPos, test.ShouldBeGreaterThan, 1)
		})

		test.That(t, m.Stop(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor Zero with positive offset", func(t *testing.T) {
		err := m.ResetZeroPosition(ctx, 2.0)
		test.That(t, err, test.ShouldBeNil)

		pos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 2.0)
	})

	t.Run("ardunio motor Zero with negative offset", func(t *testing.T) {
		err := m.ResetZeroPosition(ctx, -2.0)
		test.That(t, err, test.ShouldBeNil)

		pos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, -2.0)
	})
}

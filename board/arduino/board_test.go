package arduino

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
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
						Model: modelName,
						Type:  config.ComponentTypeMotor,
						ConvertedAttributes: &motor.Config{
							Pins: map[string]string{
								"pwm": "5",
								"a":   "6",
								"b":   "7",
								"en":  "8",
							},
							Encoder:          "3",
							EncoderB:         "2",
							TicksPerRotation: 2000,
							PWMFreq:          2000,
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
						Model: modelName,
						Type:  config.ComponentTypeMotor,
						ConvertedAttributes: &motor.Config{
							Pins: map[string]string{
								"a":  "6",
								"b":  "7",
								"en": "8",
							},
							Encoder:          "3",
							EncoderB:         "2",
							TicksPerRotation: 2000,
							PWMFreq:          2000,
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
						Model: modelName,
						Type:  config.ComponentTypeMotor,
						ConvertedAttributes: &motor.Config{
							Pins: map[string]string{
								"pwm": "5",
								"dir": "10",
							},
							Encoder:          "3",
							EncoderB:         "2",
							TicksPerRotation: 2000,
							PWMFreq:          2000,
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
						Model: modelName,
						Type:  config.ComponentTypeMotor,
						ConvertedAttributes: &motor.Config{
							Pins: map[string]string{
								"pwm": "35",
								"a":   "6",
								"b":   "7",
								"en":  "8",
							},
							Encoder:          "3",
							EncoderB:         "2",
							TicksPerRotation: 2000,
							PWMFreq:          2000,
						},
					},
				},
			},
			"couldn't set pwm freq for pin",
		},
	} {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			b, err := newArduino(ctx, &board.Config{}, logger)
			if err != nil && strings.HasPrefix(err.Error(), "found ") {

				t.Skip()
				return
			}
			test.That(t, err, test.ShouldBeNil)

			_, err = b.configureMotor(tc.conf.Components[0], tc.conf.Components[0].ConvertedAttributes.(*motor.Config))

			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err.Error(),
					test.ShouldContainSubstring, tc.err)
				return
			}
			test.That(t, b, test.ShouldNotBeNil)
			err = b.PWMSetFreq(ctx, "7", 2000)
			test.That(t, err, test.ShouldBeNil)
			err = b.PWMSetFreq(ctx, "45", 2000)
			test.That(t, err, test.ShouldNotBeNil)
			err = b.PWMSetFreq(ctx, "-5", 2000)
			test.That(t, err, test.ShouldNotBeNil)
			defer b.Close()
		})
	}
}

// Test the A/B/PWM style IO
func TestArduinoMotorABPWM(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := config.Config{
		Components: []config.Component{
			{
				Name:  "m1",
				Model: modelName,
				Type:  config.ComponentTypeMotor,
				ConvertedAttributes: &motor.Config{
					Pins: map[string]string{
						"pwm": "11",
						"a":   "37",
						"b":   "39",
						"en":  "-1",
					},
					Encoder:          "20",
					EncoderB:         "21",
					TicksPerRotation: 980,
				},
			},
		},
	}
	b, err := newArduino(ctx, &board.Config{}, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {
		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	m, err := b.configureMotor(cfg.Components[0], cfg.Components[0].ConvertedAttributes.(*motor.Config))
	test.That(t, err, test.ShouldBeNil)

	arduinoMotorTests(ctx, t, m)

}

// Test the DIR/PWM style IO
func TestArduinoMotorDirPWM(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := config.Config{
		Components: []config.Component{
			{
				Name:  "m1",
				Model: modelName,
				Type:  config.ComponentTypeMotor,
				ConvertedAttributes: &motor.Config{
					Pins: map[string]string{
						"pwm": "5",
						"dir": "6",
						"en":  "7",
					},
					Encoder:          "3",
					EncoderB:         "2",
					TicksPerRotation: 2000,
				},
			},
		},
	}
	b, err := newArduino(ctx, &board.Config{}, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {

		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	m, err := b.configureMotor(cfg.Components[0], cfg.Components[0].ConvertedAttributes.(*motor.Config))
	test.That(t, err, test.ShouldBeNil)

	arduinoMotorTests(ctx, t, m)
}

// Test the A/B only style IO
func TestArduinoMotorAB(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := config.Config{
		Components: []config.Component{
			{
				Name:  "m1",
				Model: modelName,
				Type:  config.ComponentTypeMotor,
				ConvertedAttributes: &motor.Config{
					Pins: map[string]string{
						"a":  "5",
						"b":  "6",
						"en": "7",
					},
					Encoder:          "3",
					EncoderB:         "2",
					TicksPerRotation: 2000,
				},
			},
		},
	}
	b, err := newArduino(ctx, &board.Config{}, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {

		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	m, err := b.configureMotor(cfg.Components[0], cfg.Components[0].ConvertedAttributes.(*motor.Config))
	test.That(t, err, test.ShouldBeNil)

	arduinoMotorTests(ctx, t, m)
}

func arduinoMotorTests(ctx context.Context, t *testing.T, m motor.Motor) {

	t.Run("ardunio motor Go positive powerPct", func(t *testing.T) {
		startPos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.Go(ctx, 0.9)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos, err := m.Position(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos-startPos, test.ShouldBeGreaterThan, 10)
		})

		test.That(t, m.Off(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor Go negtive powerPct", func(t *testing.T) {
		startPos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.Go(ctx, -0.9)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos, err := m.Position(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos-startPos, test.ShouldBeLessThan, -10)
		})

		test.That(t, m.Off(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with positive rpm and positive revolutions", func(t *testing.T) {
		startPos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, 20, 1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			on, err := m.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, on, test.ShouldBeFalse)

			pos, err := m.Position(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
		})

		test.That(t, m.Off(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with negative rpm and positive revolutions", func(t *testing.T) {
		startPos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, -20, 1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			on, err := m.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, on, test.ShouldBeFalse)

			pos, err := m.Position(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos-startPos, test.ShouldBeLessThan, -1)
		})

		test.That(t, m.Off(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with positive rpm and negative revolutions", func(t *testing.T) {
		startPos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, 20, -1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			on, err := m.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, on, test.ShouldBeFalse)

			pos, err := m.Position(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos-startPos, test.ShouldBeLessThan, -1)
		})

		test.That(t, m.Off(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor GoFor with negative rpm and negative revolutions", func(t *testing.T) {
		startPos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)

		err = m.GoFor(ctx, -20, -1.5)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			on, err := m.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, on, test.ShouldBeFalse)

			pos, err := m.Position(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
		})

		test.That(t, m.Off(ctx), test.ShouldBeNil)
	})

	t.Run("ardunio motor Zero with positive offset", func(t *testing.T) {
		err := m.Zero(ctx, 2.0)
		test.That(t, err, test.ShouldBeNil)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 2.0)
	})

	t.Run("ardunio motor Zero with negative offset", func(t *testing.T) {
		err := m.Zero(ctx, -2.0)
		test.That(t, err, test.ShouldBeNil)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, -2.0)
	})
}

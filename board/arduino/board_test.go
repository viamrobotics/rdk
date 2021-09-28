package arduino

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
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
						"pwm": "5",
						"a":   "6",
						"b":   "7",
						"en":  "8",
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

	startPos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 20, 1.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
	})

	err = m.Off(ctx)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 500*time.Millisecond)

	err = m.Zero(ctx, 2.0)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	err = m.GoTo(ctx, 50, 0.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeLessThan, 1)
	})

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

	startPos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 20, 1.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
	})

	err = m.Off(ctx)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 500*time.Millisecond)

	err = m.Zero(ctx, 2.0)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	err = m.GoTo(ctx, 50, 0.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeLessThan, 1)
	})

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

	startPos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 20, 1.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
	})

	err = m.Off(ctx)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 500*time.Millisecond)

	err = m.Zero(ctx, 2.0)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	err = m.GoTo(ctx, 50, 0.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeLessThan, 1)
	})

}

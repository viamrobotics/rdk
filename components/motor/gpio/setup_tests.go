package gpio

import (
	"context"
	"testing"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

func TestPinConfigMotorType(t *testing.T) {
	tests := []struct {
		name        string
		config      PinConfig
		wantType    MotorType
		wantErrText string
	}{
		{
			name: "valid ABPwm",
			config: PinConfig{
				A:   "pin1",
				B:   "pin2",
				PWM: "pwm1",
			},
			wantType: ABPwm,
		},
		{
			name: "valid AB",
			config: PinConfig{
				A: "pin1",
				B: "pin2",
			},
			wantType: AB,
		},
		{
			name: "valid DirectionPwm",
			config: PinConfig{
				Direction: "dir1",
				PWM:       "pwm1",
			},
			wantType: DirectionPwm,
		},
		{
			name: "invalid A without B",
			config: PinConfig{
				A: "pin1",
			},
			wantErrText: "motor pin config has specified pin A but not pin B",
		},
		{
			name: "invalid B without A",
			config: PinConfig{
				B: "pin2",
			},
			wantErrText: "motor pin config has specified pin B but not pin A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			motorType, err := tt.config.MotorType("test/path")
			if tt.wantErrText != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.wantErrText)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, motorType, test.ShouldEqual, tt.wantType)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantDeps    []string
		wantErrText string
	}{
		{
			name: "valid with encoder",
			config: Config{
				BoardName: "board1",
				Pins: PinConfig{
					A:   "pin1",
					B:   "pin2",
					PWM: "pwm1",
				},
				Encoder:          "encoder1",
				TicksPerRotation: 100,
			},
			wantDeps: []string{"board1", "encoder1"},
		},
		{
			name: "missing board name",
			config: Config{
				Pins: PinConfig{
					A:   "pin1",
					B:   "pin2",
					PWM: "pwm1",
				},
			},
			wantErrText: "board is required",
		},
		{
			name: "invalid pin config",
			config: Config{
				BoardName: "board1",
				Pins: PinConfig{
					A: "pin1",
				},
			},
			wantErrText: "motor pin config has specified pin A but not pin B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, err := tt.config.Validate("test/path")
			if tt.wantErrText != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.wantErrText)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, deps, test.ShouldResemble, tt.wantDeps)
			}
		})
	}
}

func TestCreateNewMotor(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	tests := []struct {
		name        string
		config      resource.Config
		setupMocks  func() resource.Dependencies
		wantErrText string
	}{
		{
			name: "valid motor without encoder",
			config: resource.Config{
				Name: "test_motor",
				ConvertedAttributes: &Config{
					BoardName: "test_board",
					Pins: PinConfig{
						A:   "pin1",
						B:   "pin2",
						PWM: "pwm1",
					},
					MaxRPM:  100,
					PWMFreq: 4000,
				},
			},
			setupMocks: func() resource.Dependencies {
				deps := testutils.NewMockDependencies(t)
				b := &inject.Board{
					GPIOPins: map[string]*inject.GPIOPin{
						"pin1": {},
						"pin2": {},
						"pwm1": {},
					},
				}
				deps.Register("test_board", b)
				return deps
			},
		},
		{
			name: "valid motor with encoder",
			config: resource.Config{
				Name: "test_motor",
				ConvertedAttributes: &Config{
					BoardName: "test_board",
					Pins: PinConfig{
						A:   "pin1",
						B:   "pin2",
						PWM: "pwm1",
					},
					Encoder:          "test_encoder",
					TicksPerRotation: 100,
					MaxRPM:           100,
					PWMFreq:          4000,
				},
			},
			setupMocks: func() resource.Dependencies {
				deps := testutils.NewMockDependencies(t)

				// Setup board
				b := &fakeboard.Board{
					GPIOPins: map[string]*fakeboard.GPIOPin{
						"pin1": {},
						"pin2": {},
						"pwm1": {},
					},
				}
				deps.Register("test_board", b)

				// Setup encoder
				mockEncoder := &fakeencoder.Encoder{
					PropertiesFunc: func(context.Context, map[string]interface{}) (encoder.Properties, error) {
						return encoder.Properties{
							TicksCountSupported: true,
						}, nil
					},
				}
				deps.Register("test_encoder", mockEncoder)
				return deps
			},
		},
		{
			name: "invalid board configuration",
			config: resource.Config{
				Name: "test_motor",
				ConvertedAttributes: &Config{
					BoardName: "",
					Pins: PinConfig{
						A:   "pin1",
						B:   "pin2",
						PWM: "pwm1",
					},
				},
			},
			setupMocks: func() resource.Dependencies {
				return testutils.NewMockDependencies(t)
			},
			wantErrText: "board is required",
		},
		{
			name: "encoder without ticks count support",
			config: resource.Config{
				Name: "test_motor",
				ConvertedAttributes: &Config{
					BoardName: "test_board",
					Pins: PinConfig{
						A:   "pin1",
						B:   "pin2",
						PWM: "pwm1",
					},
					Encoder:          "test_encoder",
					TicksPerRotation: 100,
					MaxRPM:           100,
				},
			},
			setupMocks: func() resource.Dependencies {
				deps := resource.Dependencies{}

				// Setup board
				b := &inject.Board{
					GPIOPinByNameFunc: func(name string) (board.GPIOPin, error) {
						return inject.GPIOPin{}.GPIOPin, nil
					},
				}
				deps[resource.Name{"test_board"}] = b

				// Setup encoder without ticks count support
				mockEncoder := &inject.Encoder{
					PropertiesFunc: func(context.Context, map[string]interface{}) (encoder.Properties, error) {
						return encoder.Properties{
							TicksCountSupported: false,
						}, nil
					},
				}
				deps.Register("test_encoder", mockEncoder)
				return deps
			},
			wantErrText: "encoder does not support ticks count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := tt.setupMocks()

			motor, err := createNewMotor(ctx, deps, tt.config, logger)

			if tt.wantErrText != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.wantErrText)
				test.That(t, motor, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, motor, test.ShouldNotBeNil)

				// Test motor stop after creation
				err = motor.Stop(ctx, nil)
				test.That(t, err, test.ShouldBeNil)

				// Verify motor state
				on, powerPct, err := motor.IsPowered(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, on, test.ShouldBeFalse)
				test.That(t, powerPct, test.ShouldEqual, 0)
			}
		})
	}
}

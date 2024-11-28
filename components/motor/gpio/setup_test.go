package gpio

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
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
		{
			name: "invalid dir without PWM",
			config: PinConfig{
				Direction: "dir1",
			},
			wantErrText: "motor pin config has direction pin but needs PWM pin",
		},
		{
			name: "invalid PWM without dir",
			config: PinConfig{
				PWM: "pwm1",
			},
			wantErrText: "motor pin config has PWM pin but needs either a direction pin, or A and B pins",
		},
		{
			name:        "invalid no pins",
			config:      PinConfig{},
			wantErrText: "motor pin config devoid of pin definitions (A, B, Direction, PWM are all missing)",
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
			wantErrText: resource.NewConfigValidationFieldRequiredError("test/path", "board").Error(),
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
		{
			name: "missing ticks per rotation",
			config: Config{
				BoardName: "board1",
				Pins: PinConfig{
					A:   "pin1",
					B:   "pin2",
					PWM: "pwm1",
				},
				Encoder: "encoder1",
			},
			wantErrText: resource.NewConfigValidationError("test/path", errors.New("ticks_per_rotation should be positive or zero")).Error(),
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
				Name:                "test_motor",
				ConvertedAttributes: &Config{BoardName: "test_board", Pins: PinConfig{A: "pin1", B: "pin2", PWM: "pwm1"}, MaxRPM: 100, PWMFreq: 4000},
			},
			setupMocks: func() resource.Dependencies {
				deps := resource.Dependencies{}
				b := inject.NewBoard("test_board")
				b.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
					return &inject.GPIOPin{
						SetFunc: func(ctx context.Context, high bool, extra map[string]interface{}) error { return nil },
					}, nil
				}

				deps[resource.NewName(board.API, "test_board")] = b

				return deps
			},
			wantErrText: "",
		},
		{
			name: "valid motor with encoder",
			config: resource.Config{
				Name: "test_motor",
				ConvertedAttributes: &Config{
					BoardName: "test_board",
					Pins:      PinConfig{A: "pin1", B: "pin2", PWM: "pwm1"}, Encoder: "test_encoder", TicksPerRotation: 100, MaxRPM: 100, PWMFreq: 4000,
				},
			},
			setupMocks: func() resource.Dependencies {
				deps := resource.Dependencies{}
				b := inject.NewBoard("test_board")
				b.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
					return &inject.GPIOPin{
						SetFunc:    func(ctx context.Context, high bool, extra map[string]interface{}) error { return nil },
						SetPWMFunc: func(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error { return nil },
					}, nil
				}
				mockEncoder := inject.NewEncoder("test_encoder")
				mockEncoder.PropertiesFunc = func(context.Context, map[string]interface{}) (encoder.Properties, error) {
					return encoder.Properties{TicksCountSupported: true}, nil
				}
				mockEncoder.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
					return nil, errors.ErrUnsupported
				}

				deps[resource.NewName(board.API, "test_board")] = b
				deps[resource.NewName(encoder.API, "test_encoder")] = mockEncoder
				return deps
			},
			wantErrText: "",
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
				return resource.Dependencies{}
			},
			wantErrText: "expected board name in config for motor",
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

				// Setup encoder without ticks count support
				mockEncoder := &inject.Encoder{
					PropertiesFunc: func(context.Context, map[string]interface{}) (encoder.Properties, error) {
						return encoder.Properties{
							TicksCountSupported: false,
						}, nil
					},
					DoFunc: func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
						return nil, errors.ErrUnsupported
					},
				}
				deps[resource.NewName(board.API, "test_board")] = b
				deps[resource.NewName(encoder.API, "test_encoder")] = mockEncoder
				return deps
			},
			wantErrText: "need an encoder that supports Ticks",
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

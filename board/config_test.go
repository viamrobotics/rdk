package board

import (
	"fmt"
	"testing"

	"go.viam.com/test"
)

func TestConfigValidate(t *testing.T) {
	var emptyConfig Config
	err := emptyConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig := Config{
		Name: "foo",
	}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.Motors = []MotorConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.motors.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.Motors = []MotorConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.Servos = []ServoConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.servos.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.Servos = []ServoConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.Analogs = []AnalogConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.analogs.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.Analogs = []AnalogConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.DigitalInterrupts = []DigitalInterruptConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.DigitalInterrupts = []DigitalInterruptConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
}

func TestConfigMerge(t *testing.T) {
	for i, tc := range []struct {
		left     *Config
		right    *Config
		expected *Config
		err      string
	}{
		{
			&Config{},
			&Config{},
			&Config{},
			"",
		},
		{
			&Config{
				Name: "one",
			},
			&Config{
				Name: "two",
			},
			&Config{},
			"expected board names",
		},
		{
			&Config{
				Model: "one",
			},
			&Config{
				Model: "two",
			},
			&Config{},
			"expected board models",
		},
		{
			&Config{
				Motors: []MotorConfig{
					{Name: "motor1"},
					{Name: "motor2"},
				},
				Servos: []ServoConfig{
					{Name: "servo1"},
					{Name: "servo2"},
				},
				Analogs: []AnalogConfig{
					{Name: "analog1"},
					{Name: "analog2"},
				},
				DigitalInterrupts: []DigitalInterruptConfig{
					{Name: "digital1"},
					{Name: "digital2"},
				},
			},
			&Config{
				Motors: []MotorConfig{
					{Name: "motor3"},
				},
				Servos: []ServoConfig{
					{Name: "servo3"},
				},
				Analogs: []AnalogConfig{
					{Name: "analog3"},
				},
				DigitalInterrupts: []DigitalInterruptConfig{
					{Name: "digital3"},
				},
			},
			&Config{
				Motors: []MotorConfig{
					{Name: "motor1"},
					{Name: "motor2"},
					{Name: "motor3"},
				},
				Servos: []ServoConfig{
					{Name: "servo1"},
					{Name: "servo2"},
					{Name: "servo3"},
				},
				Analogs: []AnalogConfig{
					{Name: "analog1"},
					{Name: "analog2"},
					{Name: "analog3"},
				},
				DigitalInterrupts: []DigitalInterruptConfig{
					{Name: "digital1"},
					{Name: "digital2"},
					{Name: "digital3"},
				},
			},
			"",
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			conf, err := tc.left.Merge(tc.right)
			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
				return
			}

			test.That(t, conf, test.ShouldResemble, tc.expected)
		})
	}
}

func TestDiffToConfig(t *testing.T) {
	for i, tc := range []struct {
		diff     ConfigDiff
		expected *Config
		err      string
	}{
		{
			ConfigDiff{
				Added:    &Config{},
				Modified: &Config{},
			},
			&Config{},
			"",
		},
		{
			ConfigDiff{
				Added: &Config{
					Name: "one",
				},
				Modified: &Config{
					Name: "two",
				},
			},
			&Config{},
			"expected board names",
		},
		{
			ConfigDiff{
				Added: &Config{
					Model: "one",
				},
				Modified: &Config{
					Model: "two",
				},
			},
			&Config{},
			"expected board models",
		},
		{
			ConfigDiff{
				Added: &Config{
					Motors: []MotorConfig{
						{Name: "motor1"},
						{Name: "motor2"},
					},
					Servos: []ServoConfig{
						{Name: "servo1"},
						{Name: "servo2"},
					},
					Analogs: []AnalogConfig{
						{Name: "analog1"},
						{Name: "analog2"},
					},
					DigitalInterrupts: []DigitalInterruptConfig{
						{Name: "digital1"},
						{Name: "digital2"},
					},
				},
				Modified: &Config{
					Motors: []MotorConfig{
						{Name: "motor3"},
					},
					Servos: []ServoConfig{
						{Name: "servo3"},
					},
					Analogs: []AnalogConfig{
						{Name: "analog3"},
					},
					DigitalInterrupts: []DigitalInterruptConfig{
						{Name: "digital3"},
					},
				},
			},
			&Config{
				Motors: []MotorConfig{
					{Name: "motor1"},
					{Name: "motor2"},
					{Name: "motor3"},
				},
				Servos: []ServoConfig{
					{Name: "servo1"},
					{Name: "servo2"},
					{Name: "servo3"},
				},
				Analogs: []AnalogConfig{
					{Name: "analog1"},
					{Name: "analog2"},
					{Name: "analog3"},
				},
				DigitalInterrupts: []DigitalInterruptConfig{
					{Name: "digital1"},
					{Name: "digital2"},
					{Name: "digital3"},
				},
			},
			"",
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			conf, err := tc.diff.ToConfig()
			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
				return
			}

			test.That(t, conf, test.ShouldResemble, tc.expected)
		})
	}
}

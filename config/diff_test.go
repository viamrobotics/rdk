package config_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/utils/pexec"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	functionvm "go.viam.com/core/function/vm"
	"go.viam.com/core/testutils/inject"
)

func init() {
	injectEngine1 := &inject.Engine{}
	injectEngine1.ValidateSourceFunc = func(_ string) error {
		return nil
	}
	functionvm.RegisterEngine(functionvm.EngineName("foo"), func() (functionvm.Engine, error) {
		return injectEngine1, nil
	})
	functionvm.RegisterEngine(functionvm.EngineName("bar"), func() (functionvm.Engine, error) {
		return injectEngine1, nil
	})
	functionvm.RegisterEngine(functionvm.EngineName("baz"), func() (functionvm.Engine, error) {
		return injectEngine1, nil
	})
}

func TestDiffConfigs(t *testing.T) {
	config1 := config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remote1",
				Address: "addr1",
				Prefix:  false,
			},
			{
				Name:    "remote2",
				Address: "addr2",
				Prefix:  false,
			},
		},
		Boards: []board.Config{
			{
				Name:  "board1",
				Model: "fake",
				Motors: []board.MotorConfig{
					{
						Name:             "g",
						Pins:             map[string]string{"a": "1", "b": "2", "pwm": "3"},
						Encoder:          "encoder",
						TicksPerRotation: 100,
					},
				},
				Servos: []board.ServoConfig{
					{
						Name: "servo1",
						Pin:  "12",
					},
				},
				Analogs: []board.AnalogConfig{
					{
						Name: "analog1",
						Pin:  "0",
					},
				}, DigitalInterrupts: []board.DigitalInterruptConfig{
					{
						Name: "encoder",
						Pin:  "14",
					},
				},
			},
		},
		Components: []config.Component{
			{
				Name:  "arm1",
				Type:  config.ComponentType("arm"),
				Model: "fake",
				Attributes: config.AttributeMap{
					"one": float64(1),
				},
			},
			{
				Name:  "base1",
				Type:  config.ComponentType("base"),
				Model: "fake",
				Attributes: config.AttributeMap{
					"two": float64(2),
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:      "1",
				Name:    "echo",
				Args:    []string{"hello", "world"},
				OneShot: true,
			},
			{
				ID:   "2",
				Name: "bash",
				Args: []string{"-c", "trap \"exit 0\" SIGINT; while true; do echo hey; sleep 2; done"},
				Log:  true,
			},
		},
		Functions: []functionvm.FunctionConfig{
			{
				Name: "func1",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
					Engine: "foo",
					Source: "1",
				},
			},
			{
				Name: "func2",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
					Engine: "bar",
					Source: "2",
				},
			},
		},
	}

	config2 := config.ModifiedConfigDiff{
		Remotes: []config.Remote{
			{
				Name:    "remote1",
				Address: "addr3",
				Prefix:  false,
			},
			{
				Name:    "remote2",
				Address: "addr4",
				Prefix:  false,
			},
		},
		Boards: map[string]board.ConfigDiff{
			"board1": {
				Added: &board.Config{
					Name:  "board1",
					Model: "fake",
				},
				Removed: &board.Config{
					Name:  "board1",
					Model: "fake",
				},
				Modified: &board.Config{
					Name:  "board1",
					Model: "fake",
					Motors: []board.MotorConfig{
						{
							Name:             "g",
							Pins:             map[string]string{"a": "2", "b": "3", "pwm": "4"},
							Encoder:          "encoder",
							TicksPerRotation: 101,
						},
					},
					Servos: []board.ServoConfig{
						{
							Name: "servo1",
							Pin:  "13",
						},
					},
					Analogs: []board.AnalogConfig{
						{
							Name: "analog1",
							Pin:  "1",
						},
					}, DigitalInterrupts: []board.DigitalInterruptConfig{
						{
							Name: "encoder",
							Pin:  "15",
						},
					},
				},
			},
		},
		Components: []config.Component{
			{
				Name:  "arm1",
				Type:  config.ComponentType("arm"),
				Model: "fake",
				Attributes: config.AttributeMap{
					"two": float64(2),
				},
			},
			{
				Name:  "base1",
				Type:  config.ComponentType("base"),
				Model: "fake",
				Attributes: config.AttributeMap{
					"three": float64(3),
				},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:      "1",
				Name:    "echo",
				Args:    []string{"hello", "world", "again"},
				OneShot: true,
			},
			{
				ID:   "2",
				Name: "bash",
				Args: []string{"-c", "trap \"exit 0\" SIGINT; while true; do echo hello; sleep 2; done"},
				Log:  true,
			},
		},
		Functions: []functionvm.FunctionConfig{
			{
				Name: "func1",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
					Engine: "foo",
					Source: "1+1",
				},
			},
			{
				Name: "func2",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
					Engine: "bar",
					Source: "2+2",
				},
			},
		},
	}

	for _, tc := range []struct {
		Name      string
		LeftFile  string
		RightFile string
		Expected  config.Diff
	}{
		{
			"empty",
			"data/diff_config_empty.json",
			"data/diff_config_empty.json",
			config.Diff{
				Added:    &config.Config{},
				Modified: &config.ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Removed:  &config.Config{},
				Equal:    true,
			},
		},
		{
			"equal",
			"data/diff_config_1.json",
			"data/diff_config_1.json",
			config.Diff{
				Added:    &config.Config{},
				Modified: &config.ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Removed:  &config.Config{},
				Equal:    true,
			},
		},
		{
			"only additions",
			"data/diff_config_empty.json",
			"data/diff_config_1.json",
			config.Diff{
				Added:    &config1,
				Modified: &config.ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Removed:  &config.Config{},
				Equal:    false,
			},
		},
		{
			"only removals",
			"data/diff_config_1.json",
			"data/diff_config_empty.json",
			config.Diff{
				Added:    &config.Config{},
				Removed:  &config1,
				Modified: &config.ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Equal:    false,
			},
		},
		{
			"only modifications",
			"data/diff_config_1.json",
			"data/diff_config_2.json",
			config.Diff{
				Added:    &config.Config{},
				Removed:  &config.Config{},
				Modified: &config2,
				Equal:    false,
			},
		},
		{
			"mixed",
			"data/diff_config_1.json",
			"data/diff_config_3.json",
			config.Diff{
				Added: &config.Config{
					Boards: []board.Config{
						{
							Name:   "board2",
							Servos: []board.ServoConfig{{Name: "servo2", Pin: "14"}},
						},
					},
					Components: []config.Component{
						{
							Name:  "base2",
							Type:  config.ComponentType("base"),
							Model: "fake",
						},
					},
					Processes: []pexec.ProcessConfig{
						{
							ID:   "3",
							Name: "bash",
							Args: []string{"-c", "trap \"exit 0\" SIGINT; while true; do echo world; sleep 2; done"},
							Log:  true,
						},
					},
					Functions: []functionvm.FunctionConfig{
						{
							Name: "func3",
							AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
								Engine: "baz",
								Source: "3",
							},
						},
					},
				},
				Modified: &config.ModifiedConfigDiff{
					Remotes: []config.Remote{
						{
							Name:    "remote1",
							Address: "addr3",
						},
						{
							Name:    "remote2",
							Address: "addr4",
						},
					},
					Boards: map[string]board.ConfigDiff{
						"board1": {
							Added: &board.Config{
								Name:   "board1",
								Model:  "fake",
								Motors: []board.MotorConfig{{Name: "h", Pins: map[string]string{"a": "2", "b": "3", "pwm": "4"}, Encoder: "encoder", EncoderB: "", TicksPerRotation: 101}},
							},
							Removed: &board.Config{
								Name:              "board1",
								Model:             "fake",
								Motors:            []board.MotorConfig{{Name: "g", Pins: map[string]string{"a": "1", "b": "2", "pwm": "3"}, Encoder: "encoder", EncoderB: "", TicksPerRotation: 100}},
								DigitalInterrupts: []board.DigitalInterruptConfig{{Name: "encoder", Pin: "14"}},
							},
							Modified: &board.Config{
								Name:    "board1",
								Model:   "fake",
								Servos:  []board.ServoConfig{{Name: "servo1", Pin: "13"}},
								Analogs: []board.AnalogConfig{{Name: "analog1", Pin: "1"}},
							},
						},
					},
					Components: []config.Component{
						{
							Name:  "arm1",
							Type:  config.ComponentType("arm"),
							Model: "fake",
							Attributes: config.AttributeMap{
								"two": float64(2),
							},
						},
					},
					Processes: []pexec.ProcessConfig{
						{
							ID:      "1",
							Name:    "echo",
							Args:    []string{"hello", "world", "again"},
							OneShot: true,
						},
					},
					Functions: []functionvm.FunctionConfig{
						{
							Name: "func1",
							AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
								Engine: "foo",
								Source: "1+1",
							},
						},
					},
				},
				Removed: &config.Config{
					Components: []config.Component{
						{
							Name:  "base1",
							Type:  config.ComponentType("base"),
							Model: "fake",
							Attributes: config.AttributeMap{
								"two": float64(2),
							},
						},
					},
					Processes: []pexec.ProcessConfig{
						{
							ID:   "2",
							Name: "bash",
							Args: []string{"-c", "trap \"exit 0\" SIGINT; while true; do echo hey; sleep 2; done"},
							Log:  true,
						},
					},
					Functions: []functionvm.FunctionConfig{
						{
							Name: "func2",
							AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
								Engine: "bar",
								Source: "2",
							},
						},
					},
				},
				Equal: false,
			},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			left, err := config.Read(tc.LeftFile)
			test.That(t, err, test.ShouldBeNil)
			right, err := config.Read(tc.RightFile)
			test.That(t, err, test.ShouldBeNil)

			diff, err := config.DiffConfigs(left, right)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, diff.Left, test.ShouldResemble, left)
			test.That(t, diff.Right, test.ShouldResemble, right)
			if tc.Expected.Equal {
				test.That(t, diff.PrettyDiff, test.ShouldBeEmpty)
			} else {
				test.That(t, diff.PrettyDiff, test.ShouldNotBeEmpty)
			}
			diff.PrettyDiff = ""
			tc.Expected.Left = diff.Left
			tc.Expected.Right = diff.Right
			for name, boardDiff := range tc.Expected.Modified.Boards {
				for _, b := range left.Boards {
					if b.Name == name {
						boardDiff.Left = &b
						break
					}
				}
				for _, b := range right.Boards {
					if b.Name == name {
						boardDiff.Right = &b
						break
					}
				}
				tc.Expected.Modified.Boards[name] = boardDiff
			}

			test.That(t, diff, test.ShouldResemble, &tc.Expected)
		})
	}
}

func TestDiffConfigHeterogenousTypes(t *testing.T) {
	for _, tc := range []struct {
		Name      string
		LeftFile  string
		RightFile string
		Expected  string
	}{
		{
			"board model",
			"data/diff_config_1.json",
			"data/diff_config_1_board_model.json",
			"",
		},
		{
			"digital interrupt type",
			"data/diff_config_1.json",
			"data/diff_config_1_digital_interrupt_type.json",
			"",
		},
		{
			"component type",
			"data/diff_config_1.json",
			"data/diff_config_1_component_type.json",
			"cannot modify type of existing component",
		},
		{
			"component subtype",
			"data/diff_config_1.json",
			"data/diff_config_1_component_subtype.json",
			"cannot modify type of existing component",
		},
		{
			"component model",
			"data/diff_config_1.json",
			"data/diff_config_1_component_model.json",
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			left, err := config.Read(tc.LeftFile)
			test.That(t, err, test.ShouldBeNil)
			right, err := config.Read(tc.RightFile)
			test.That(t, err, test.ShouldBeNil)

			_, err = config.DiffConfigs(left, right)
			if tc.Expected == "" {
				test.That(t, err, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.Expected)
		})
	}
}

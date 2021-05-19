package config

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/board"
	"go.viam.com/core/rexec"
)

func TestDiffConfigs(t *testing.T) {
	config1 := Config{
		Remotes: []Remote{
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
		Components: []Component{
			{
				Name:  "arm1",
				Type:  ComponentType("arm"),
				Model: "fake",
				Attributes: AttributeMap{
					"one": float64(1),
				},
			},
			{
				Name:  "base1",
				Type:  ComponentType("base"),
				Model: "fake",
				Attributes: AttributeMap{
					"two": float64(2),
				},
			},
		},
		Processes: []rexec.ProcessConfig{
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
	}

	config2 := ModifiedConfigDiff{
		Remotes: []Remote{
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
		Components: []Component{
			{
				Name:  "arm1",
				Type:  ComponentType("arm"),
				Model: "fake",
				Attributes: AttributeMap{
					"two": float64(2),
				},
			},
			{
				Name:  "base1",
				Type:  ComponentType("base"),
				Model: "fake",
				Attributes: AttributeMap{
					"three": float64(3),
				},
			},
		},
		Processes: []rexec.ProcessConfig{
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
	}

	for _, tc := range []struct {
		Name      string
		LeftFile  string
		RightFile string
		Expected  Diff
	}{
		{
			"empty",
			"data/diff_config_empty.json",
			"data/diff_config_empty.json",
			Diff{
				Added:    &Config{},
				Modified: &ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Removed:  &Config{},
				Equal:    true,
			},
		},
		{
			"equal",
			"data/diff_config_1.json",
			"data/diff_config_1.json",
			Diff{
				Added:    &Config{},
				Modified: &ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Removed:  &Config{},
				Equal:    true,
			},
		},
		{
			"only additions",
			"data/diff_config_empty.json",
			"data/diff_config_1.json",
			Diff{
				Added:    &config1,
				Modified: &ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Removed:  &Config{},
				Equal:    false,
			},
		},
		{
			"only removals",
			"data/diff_config_1.json",
			"data/diff_config_empty.json",
			Diff{
				Added:    &Config{},
				Removed:  &config1,
				Modified: &ModifiedConfigDiff{Boards: map[string]board.ConfigDiff{}},
				Equal:    false,
			},
		},
		{
			"only modifications",
			"data/diff_config_1.json",
			"data/diff_config_2.json",
			Diff{
				Added:    &Config{},
				Removed:  &Config{},
				Modified: &config2,
				Equal:    false,
			},
		},
		{
			"mixed",
			"data/diff_config_1.json",
			"data/diff_config_3.json",
			Diff{
				Added: &Config{
					Boards: []board.Config{
						{
							Name:   "board2",
							Servos: []board.ServoConfig{{Name: "servo2", Pin: "14"}},
						},
					},
					Components: []Component{
						{
							Name:  "base2",
							Type:  ComponentType("base"),
							Model: "fake",
						},
					},
					Processes: []rexec.ProcessConfig{
						{
							ID:   "3",
							Name: "bash",
							Args: []string{"-c", "trap \"exit 0\" SIGINT; while true; do echo world; sleep 2; done"},
							Log:  true,
						},
					},
				},
				Modified: &ModifiedConfigDiff{
					Remotes: []Remote{
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
					Components: []Component{
						{
							Name:  "arm1",
							Type:  ComponentType("arm"),
							Model: "fake",
							Attributes: AttributeMap{
								"two": float64(2),
							},
						},
					},
					Processes: []rexec.ProcessConfig{
						{
							ID:      "1",
							Name:    "echo",
							Args:    []string{"hello", "world", "again"},
							OneShot: true,
						},
					},
				},
				Removed: &Config{
					Components: []Component{
						{
							Name:  "base1",
							Type:  ComponentType("base"),
							Model: "fake",
							Attributes: AttributeMap{
								"two": float64(2),
							},
						},
					},
					Processes: []rexec.ProcessConfig{
						{
							ID:   "2",
							Name: "bash",
							Args: []string{"-c", "trap \"exit 0\" SIGINT; while true; do echo hey; sleep 2; done"},
							Log:  true,
						},
					},
				},
				Equal: false,
			},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			left, err := Read(tc.LeftFile)
			test.That(t, err, test.ShouldBeNil)
			right, err := Read(tc.RightFile)
			test.That(t, err, test.ShouldBeNil)

			diff, err := DiffConfigs(left, right)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, diff.Left, test.ShouldResemble, left)
			test.That(t, diff.Right, test.ShouldResemble, right)
			if tc.Expected.Equal {
				test.That(t, diff.prettyDiff, test.ShouldBeEmpty)
			} else {
				test.That(t, diff.prettyDiff, test.ShouldNotBeEmpty)
			}
			diff.prettyDiff = ""
			tc.Expected.Left = diff.Left
			tc.Expected.Right = diff.Right

			test.That(t, diff, test.ShouldResemble, &tc.Expected)
		})
	}
}

func TestDiffConfigErrors(t *testing.T) {
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
			"cannot modify model of existing board",
		},
		{
			"digital interrupt type",
			"data/diff_config_1.json",
			"data/diff_config_1_digital_interrupt_type.json",
			"cannot modify type of existing digital interrupt",
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
			"cannot modify model of existing component",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			left, err := Read(tc.LeftFile)
			test.That(t, err, test.ShouldBeNil)
			right, err := Read(tc.RightFile)
			test.That(t, err, test.ShouldBeNil)

			_, err = DiffConfigs(left, right)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.Expected)
		})
	}
}

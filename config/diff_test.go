package config_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/config"
)

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
		Components: []config.Component{
			{
				Name:  "arm1",
				Type:  arm.SubtypeName,
				Model: "fake",
				Attributes: config.AttributeMap{
					"one": float64(1),
				},
			},
			{
				Name:  "base1",
				Type:  base.SubtypeName,
				Model: "fake",
				Attributes: config.AttributeMap{
					"two": float64(2),
				},
			},
			{
				Name:  "board1",
				Model: "fake",
				Type:  board.SubtypeName,
				ConvertedAttributes: &board.Config{
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
		Components: []config.Component{
			{
				Name:  "arm1",
				Type:  arm.SubtypeName,
				Model: "fake",
				Attributes: config.AttributeMap{
					"two": float64(2),
				},
			},
			{
				Name:  "base1",
				Type:  base.SubtypeName,
				Model: "fake",
				Attributes: config.AttributeMap{
					"three": float64(3),
				},
			},
			{
				Name:  "board1",
				Model: "fake",
				Type:  board.SubtypeName,
				ConvertedAttributes: &board.Config{
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
				Modified: &config.ModifiedConfigDiff{},
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
				Modified: &config.ModifiedConfigDiff{},
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
				Modified: &config.ModifiedConfigDiff{},
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
				Modified: &config.ModifiedConfigDiff{},
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
					Components: []config.Component{
						{
							Name:  "base2",
							Type:  base.SubtypeName,
							Model: "fake",
						},
						{
							Name:  "board2",
							Type:  board.SubtypeName,
							Model: "fake",
							ConvertedAttributes: &board.Config{
								DigitalInterrupts: []board.DigitalInterruptConfig{{Name: "encoder2", Pin: "16"}},
							},
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
					Components: []config.Component{
						{
							Name:  "arm1",
							Type:  arm.SubtypeName,
							Model: "fake",
							Attributes: config.AttributeMap{
								"two": float64(2),
							},
						},
						{
							Name:  "board1",
							Type:  board.SubtypeName,
							Model: "fake",
							ConvertedAttributes: &board.Config{
								Analogs: []board.AnalogConfig{{Name: "analog1", Pin: "1"}},
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
				},
				Removed: &config.Config{
					Components: []config.Component{
						{
							Name:  "base1",
							Type:  base.SubtypeName,
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
				},
				Equal: false,
			},
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			logger := golog.NewTestLogger(t)
			left, err := config.Read(context.Background(), tc.LeftFile, logger)
			test.That(t, err, test.ShouldBeNil)
			right, err := config.Read(context.Background(), tc.RightFile, logger)
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
			logger := golog.NewTestLogger(t)
			left, err := config.Read(context.Background(), tc.LeftFile, logger)
			test.That(t, err, test.ShouldBeNil)
			right, err := config.Read(context.Background(), tc.RightFile, logger)
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

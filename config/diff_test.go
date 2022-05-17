package config_test

import (
	"context"
	"crypto/tls"
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
				Added:          &config.Config{},
				Modified:       &config.ModifiedConfigDiff{},
				Removed:        &config.Config{},
				ResourcesEqual: true,
				NetworkEqual:   true,
			},
		},
		{
			"equal",
			"data/diff_config_1.json",
			"data/diff_config_1.json",
			config.Diff{
				Added:          &config.Config{},
				Modified:       &config.ModifiedConfigDiff{},
				Removed:        &config.Config{},
				ResourcesEqual: true,
				NetworkEqual:   true,
			},
		},
		{
			"only additions",
			"data/diff_config_empty.json",
			"data/diff_config_1.json",
			config.Diff{
				Added:          &config1,
				Modified:       &config.ModifiedConfigDiff{},
				Removed:        &config.Config{},
				ResourcesEqual: false,
				NetworkEqual:   true,
			},
		},
		{
			"only removals",
			"data/diff_config_1.json",
			"data/diff_config_empty.json",
			config.Diff{
				Added:          &config.Config{},
				Removed:        &config1,
				Modified:       &config.ModifiedConfigDiff{},
				ResourcesEqual: false,
				NetworkEqual:   true,
			},
		},
		{
			"only modifications",
			"data/diff_config_1.json",
			"data/diff_config_2.json",
			config.Diff{
				Added:          &config.Config{},
				Removed:        &config.Config{},
				Modified:       &config2,
				ResourcesEqual: false,
				NetworkEqual:   true,
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
				ResourcesEqual: false,
				NetworkEqual:   true,
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
			if tc.Expected.ResourcesEqual {
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

func TestDiffNetwork(t *testing.T) {
	network1 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{FQDN: "abc"}}
	network2 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{FQDN: "xyz"}}

	tls1 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	tls2 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS10}}}

	cloud1 := &config.Cloud{ID: "1"}
	cloud2 := &config.Cloud{ID: "2"}

	auth1 := config.AuthConfig{
		Handlers: []config.AuthHandlerConfig{{Config: config.AttributeMap{"key": "value"}}},
	}
	auth2 := config.AuthConfig{
		Handlers: []config.AuthHandlerConfig{{Config: config.AttributeMap{"key2": "value2"}}},
	}
	for _, tc := range []struct {
		Name         string
		LeftCfg      config.Config
		RightCfg     config.Config
		NetworkEqual bool
	}{
		{
			"same",
			config.Config{Network: network1, Cloud: cloud1, Auth: auth1},
			config.Config{Network: network1, Cloud: cloud1, Auth: auth1},
			true,
		},
		{
			"diff network",
			config.Config{Network: network1},
			config.Config{Network: network2},
			false,
		},
		{
			"diff tls",
			config.Config{Network: tls1},
			config.Config{Network: tls2},
			true,
		},
		{
			"diff cloud",
			config.Config{Cloud: cloud1},
			config.Config{Cloud: cloud2},
			false,
		},
		{
			"diff auth",
			config.Config{Auth: auth1},
			config.Config{Auth: auth2},
			false,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			diff, err := config.DiffConfigs(&tc.LeftCfg, &tc.RightCfg)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, diff.NetworkEqual, test.ShouldEqual, tc.NetworkEqual)
		})
	}
}

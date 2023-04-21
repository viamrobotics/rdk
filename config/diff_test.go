package config_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

var (
	fakeModel = resource.NewDefaultModel("fake")
	extModel  = resource.NewModel("acme", "test", "model")
)

func TestDiffConfigs(t *testing.T) {
	config1 := config.Config{
		Modules: []config.Module{
			{
				Name:    "my-module",
				ExePath: "path/to/my-module",
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "remote1",
				Address: "addr1",
			},
			{
				Name:    "remote2",
				Address: "addr2",
			},
		},
		Components: []resource.Config{
			{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "arm1",
				DeprecatedSubtype:   arm.SubtypeName,
				API:                 arm.Subtype,
				Model:               fakeModel,
				Attributes: utils.AttributeMap{
					"one": float64(1),
				},
			},
			{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "base1",
				DeprecatedSubtype:   base.SubtypeName,
				API:                 base.Subtype,
				Model:               fakeModel,
				Attributes: utils.AttributeMap{
					"two": float64(2),
				},
			},
			{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "board1",
				Model:               fakeModel,
				DeprecatedSubtype:   board.SubtypeName,
				API:                 board.Subtype,
				ConvertedAttributes: &fakeboard.Config{
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
		Modules: []config.Module{
			{
				Name:    "my-module",
				ExePath: "new/path/to/my-module",
			},
		},
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
		Components: []resource.Config{
			{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "arm1",
				DeprecatedSubtype:   arm.SubtypeName,
				API:                 arm.Subtype,
				Model:               fakeModel,
				Attributes: utils.AttributeMap{
					"two": float64(2),
				},
			},
			{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "base1",
				DeprecatedSubtype:   base.SubtypeName,
				API:                 base.Subtype,
				Model:               extModel,
				Attributes: utils.AttributeMap{
					"three": float64(3),
				},
			},
			{
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "board1",
				Model:               fakeModel,
				DeprecatedSubtype:   board.SubtypeName,
				API:                 board.Subtype,
				ConvertedAttributes: &fakeboard.Config{
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
					Components: []resource.Config{
						{
							DeprecatedNamespace: resource.ResourceNamespaceRDK,
							Name:                "base2",
							DeprecatedSubtype:   base.SubtypeName,
							API:                 base.Subtype,
							Model:               fakeModel,
						},
						{
							DeprecatedNamespace: resource.ResourceNamespaceRDK,
							Name:                "board2",
							DeprecatedSubtype:   board.SubtypeName,
							API:                 board.Subtype,
							Model:               fakeModel,
							ConvertedAttributes: &fakeboard.Config{
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
					Components: []resource.Config{
						{
							DeprecatedNamespace: resource.ResourceNamespaceRDK,
							Name:                "arm1",
							DeprecatedSubtype:   arm.SubtypeName,
							API:                 arm.Subtype,
							Model:               extModel,
							Attributes: utils.AttributeMap{
								"two": float64(2),
							},
						},
						{
							DeprecatedNamespace: resource.ResourceNamespaceRDK,
							Name:                "board1",
							DeprecatedSubtype:   board.SubtypeName,
							API:                 board.Subtype,
							Model:               fakeModel,
							ConvertedAttributes: &fakeboard.Config{
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
					Modules: []config.Module{
						{
							Name:    "my-module",
							ExePath: "path/to/my-module",
						},
					},
					Components: []resource.Config{
						{
							DeprecatedNamespace: resource.ResourceNamespaceRDK,
							Name:                "base1",
							DeprecatedSubtype:   base.SubtypeName,
							API:                 base.Subtype,
							Model:               fakeModel,
							Attributes: utils.AttributeMap{
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
		// test with revealSensitiveConfigDiffs = true
		t.Run(tc.Name, func(t *testing.T) {
			logger := golog.NewTestLogger(t)
			// ensure parts are valid for components, services, modules, and remotes
			test.That(t, tc.Expected.Added.Ensure(false, logger), test.ShouldBeNil)
			test.That(t, tc.Expected.Removed.Ensure(false, logger), test.ShouldBeNil)
			test.That(t, modifiedConfigDiffValidate(tc.Expected.Modified), test.ShouldBeNil)
			tc.Expected.Added.Network = config.NetworkConfig{}
			tc.Expected.Removed.Network = config.NetworkConfig{}

			for _, revealSensitiveConfigDiffs := range []bool{true, false} {
				t.Run(fmt.Sprintf("revealSensitiveConfigDiffs=%t", revealSensitiveConfigDiffs), func(t *testing.T) {
					logger := golog.NewTestLogger(t)
					left, err := config.Read(context.Background(), tc.LeftFile, logger)
					test.That(t, err, test.ShouldBeNil)
					right, err := config.Read(context.Background(), tc.RightFile, logger)
					test.That(t, err, test.ShouldBeNil)

					diff, err := config.DiffConfigs(*left, *right, revealSensitiveConfigDiffs)
					test.That(t, err, test.ShouldBeNil)
					test.That(t, diff.Left, test.ShouldResemble, left)
					test.That(t, diff.Right, test.ShouldResemble, right)
					if tc.Expected.ResourcesEqual || !revealSensitiveConfigDiffs {
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

			_, err = config.DiffConfigs(*left, *right, true)
			if tc.Expected == "" {
				test.That(t, err, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.Expected)
		})
	}
}

func TestDiffNetworkingCfg(t *testing.T) {
	network1 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{FQDN: "abc"}}
	network2 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{FQDN: "xyz"}}

	tls1 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	tls2 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS10}}}

	tlsCfg3 := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("abc")}}, nil
		},
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("abc")}}, nil
		},
	}
	tls3 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: tlsCfg3}}
	tlsCfg4 := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("abc")}}, nil
		},
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("abc")}}, nil
		},
	}
	tls4 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: tlsCfg4}}
	tlsCfg5 := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("xyz")}}, nil
		},
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("abc")}}, nil
		},
	}
	tls5 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: tlsCfg5}}
	tlsCfg6 := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("abcd")}}, nil
		},
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &tls.Certificate{Certificate: [][]byte{[]byte("xyz")}}, nil
		},
	}
	tls6 := config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{TLSConfig: tlsCfg6}}

	cloud1 := &config.Cloud{ID: "1"}
	cloud2 := &config.Cloud{ID: "2"}

	auth1 := config.AuthConfig{
		Handlers: []config.AuthHandlerConfig{{Config: utils.AttributeMap{"key": "value"}}},
	}
	auth2 := config.AuthConfig{
		Handlers: []config.AuthHandlerConfig{{Config: utils.AttributeMap{"key2": "value2"}}},
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
			"same tls",
			config.Config{Network: tls3},
			config.Config{Network: tls4},
			true,
		},
		{
			"diff tls",
			config.Config{Network: tls1},
			config.Config{Network: tls2},
			false,
		},
		{
			"diff tls cert",
			config.Config{Network: tls3},
			config.Config{Network: tls5},
			false,
		},
		{
			"diff tls client cert",
			config.Config{Network: tls3},
			config.Config{Network: tls6},
			false,
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
			diff, err := config.DiffConfigs(tc.LeftCfg, tc.RightCfg, true)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, diff.NetworkEqual, test.ShouldEqual, tc.NetworkEqual)
		})
	}
}

func TestDiffSanitize(t *testing.T) {
	cloud1 := &config.Cloud{
		ID:             "1",
		Secret:         "hello",
		LocationSecret: "world",
		LocationSecrets: []config.LocationSecret{
			{ID: "id1", Secret: "sec1"},
			{ID: "id2", Secret: "sec2"},
		},
		TLSCertificate: "foo",
		TLSPrivateKey:  "bar",
	}

	// cloud
	// remotes.remote.auth.creds
	// remotes.remote.auth.signaling creds
	// remote.secret
	// .auth.handlers.config ***

	auth1 := config.AuthConfig{
		Handlers: []config.AuthHandlerConfig{
			{Config: utils.AttributeMap{"key1": "value1"}},
			{Config: utils.AttributeMap{"key2": "value2"}},
		},
	}
	remotes1 := []config.Remote{
		{
			Secret: "remsecret1",
			Auth: config.RemoteAuth{
				Credentials: &rpc.Credentials{
					Type:    "remauthtype1",
					Payload: "payload1",
				},
				SignalingCreds: &rpc.Credentials{
					Type:    "remauthtypesig1",
					Payload: "payloadsig1",
				},
			},
		},
		{
			Secret: "remsecret2",
			Auth: config.RemoteAuth{
				Credentials: &rpc.Credentials{
					Type:    "remauthtype2",
					Payload: "payload2",
				},
				SignalingCreds: &rpc.Credentials{
					Type:    "remauthtypesig2",
					Payload: "payloadsig2",
				},
			},
		},
	}

	left := config.Config{}
	leftOrig := config.Config{}
	right := config.Config{
		Cloud:   cloud1,
		Auth:    auth1,
		Remotes: remotes1,
	}
	rightOrig := config.Config{
		Cloud:   cloud1,
		Auth:    auth1,
		Remotes: remotes1,
	}

	diff, err := config.DiffConfigs(left, right, true)
	test.That(t, err, test.ShouldBeNil)

	// verify secrets did not change
	test.That(t, left, test.ShouldResemble, leftOrig)
	test.That(t, right, test.ShouldResemble, rightOrig)

	diffStr := diff.String()
	test.That(t, diffStr, test.ShouldContainSubstring, cloud1.ID)
	test.That(t, diffStr, test.ShouldNotContainSubstring, cloud1.Secret)
	test.That(t, diffStr, test.ShouldNotContainSubstring, cloud1.LocationSecret)
	test.That(t, diffStr, test.ShouldNotContainSubstring, cloud1.LocationSecrets[0].Secret)
	test.That(t, diffStr, test.ShouldNotContainSubstring, cloud1.LocationSecrets[1].Secret)
	test.That(t, diffStr, test.ShouldNotContainSubstring, cloud1.TLSCertificate)
	test.That(t, diffStr, test.ShouldNotContainSubstring, cloud1.TLSPrivateKey)
	for _, hdlr := range auth1.Handlers {
		for _, value := range hdlr.Config {
			test.That(t, diffStr, test.ShouldNotContainSubstring, value)
		}
	}
	for _, rem := range remotes1 {
		test.That(t, diffStr, test.ShouldContainSubstring, string(rem.Auth.Credentials.Type))
		test.That(t, diffStr, test.ShouldNotContainSubstring, rem.Secret)
		test.That(t, diffStr, test.ShouldNotContainSubstring, rem.Auth.Credentials.Payload)
		test.That(t, diffStr, test.ShouldNotContainSubstring, rem.Auth.SignalingCreds.Payload)
	}
}

func modifiedConfigDiffValidate(c *config.ModifiedConfigDiff) error {
	for idx := 0; idx < len(c.Remotes); idx++ {
		if _, err := c.Remotes[idx].Validate(fmt.Sprintf("%s.%d", "remotes", idx)); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Components); idx++ {
		dependsOn, err := c.Components[idx].Validate(fmt.Sprintf("%s.%d", "components", idx), resource.ResourceTypeComponent)
		if err != nil {
			return err
		}
		c.Components[idx].ImplicitDependsOn = dependsOn
	}

	for idx := 0; idx < len(c.Processes); idx++ {
		if err := c.Processes[idx].Validate(fmt.Sprintf("%s.%d", "processes", idx)); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Services); idx++ {
		dependsOn, err := c.Services[idx].Validate(fmt.Sprintf("%s.%d", "services", idx), resource.ResourceTypeService)
		if err != nil {
			return err
		}
		c.Services[idx].ImplicitDependsOn = dependsOn
	}

	for idx := 0; idx < len(c.Packages); idx++ {
		if err := c.Packages[idx].Validate(fmt.Sprintf("%s.%d", "packages", idx)); err != nil {
			return err
		}
	}

	return nil
}

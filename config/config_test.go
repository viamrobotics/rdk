package config_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/board"
	// board attribute converters.
	_ "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/motor"
	// motor attribute converters.
	_ "go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

func TestConfigRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/robot.json", logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Components, test.ShouldHaveLength, 4)
	test.That(t, len(cfg.Remotes), test.ShouldEqual, 2)
	test.That(t, cfg.Remotes[0], test.ShouldResemble, config.Remote{Name: "one", Address: "foo", Prefix: true})
	test.That(t, cfg.Remotes[1], test.ShouldResemble, config.Remote{Name: "two", Address: "bar"})
}

func TestConfig3(t *testing.T) {
	logger := golog.NewTestLogger(t)
	type temp struct {
		X int
		Y string
	}

	config.RegisterComponentAttributeConverter("foo", "eliot", "bar", func(sub interface{}) (interface{}, error) {
		t := &temp{}
		err := mapstructure.Decode(sub, t)
		return t, err
	},
	)

	test.That(t, os.Setenv("TEST_THING_FOO", "5"), test.ShouldBeNil)
	cfg, err := config.Read(context.Background(), "data/config3.json", logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(cfg.Components), test.ShouldEqual, 3)
	test.That(t, cfg.Components[0].Attributes.Int("foo", 0), test.ShouldEqual, 5)
	test.That(t, cfg.Components[0].Attributes.Bool("foo2", false), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Bool("foo3", false), test.ShouldEqual, false)
	test.That(t, cfg.Components[0].Attributes.Bool("xxxx", true), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Bool("xxxx", false), test.ShouldEqual, false)
	test.That(t, cfg.Components[0].Attributes.String("foo4"), test.ShouldEqual, "no")
	test.That(t, cfg.Components[0].Attributes.String("xxxx"), test.ShouldEqual, "")
	test.That(t, cfg.Components[0].Attributes.Has("foo"), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Has("xxxx"), test.ShouldEqual, false)

	bb := cfg.Components[0].Attributes["bar"]
	b := bb.(*temp)
	test.That(t, b.X, test.ShouldEqual, 6)
	test.That(t, b.Y, test.ShouldEqual, "eliot")

	test.That(t, cfg.Components[0].Attributes.Float64("bar5", 1.1), test.ShouldEqual, 5.17)
	test.That(t, cfg.Components[0].Attributes.Float64("bar5-no", 1.1), test.ShouldEqual, 1.1)

	test.That(t, cfg.Components[1].ConvertedAttributes, test.ShouldResemble, &board.Config{
		Analogs: []board.AnalogConfig{
			{Name: "analog1", Pin: "0"},
		},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "encoder", Pin: "14"},
		},
	})
	test.That(t, cfg.Components[2].ConvertedAttributes, test.ShouldResemble, &motor.Config{
		Pins: motor.PinConfig{
			Direction: "io17",
			PWM:       "io18",
		},
		EncoderA:         "encoder-steering-b",
		EncoderB:         "encoder-steering-a",
		TicksPerRotation: 10000,
		MaxPowerPct:      0.5,
	})
}

func TestCreateCloudRequest(t *testing.T) {
	cfg := config.Cloud{
		ID:     "a",
		Secret: "b",
		Path:   "c",
	}

	version := "test-version"
	gitRevision := "test-git-revision"
	config.Version = version
	config.GitRevision = gitRevision

	r, err := config.CreateCloudRequest(context.Background(), &cfg)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, r.Header.Get("Secret"), test.ShouldEqual, cfg.Secret)
	test.That(t, r.URL.String(), test.ShouldEqual, "c?id=a")

	userInfo := map[string]interface{}{}
	userInfoJSON := r.Header.Get("User-Info")
	json.Unmarshal([]byte(userInfoJSON), &userInfo)

	test.That(t, userInfo["version"], test.ShouldEqual, version)
	test.That(t, userInfo["gitRevision"], test.ShouldEqual, gitRevision)
}

func TestConfigEnsure(t *testing.T) {
	var emptyConfig config.Config
	test.That(t, emptyConfig.Ensure(false), test.ShouldBeNil)

	invalidCloud := config.Config{
		Cloud: &config.Cloud{},
	}
	err := invalidCloud.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `cloud`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)
	invalidCloud.Cloud.ID = "some_id"
	err = invalidCloud.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"secret" is required`)
	err = invalidCloud.Ensure(true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"fqdn" is required`)
	invalidCloud.Cloud.Secret = "my_secret"
	test.That(t, invalidCloud.Ensure(false), test.ShouldBeNil)
	test.That(t, invalidCloud.Ensure(true), test.ShouldNotBeNil)
	invalidCloud.Cloud.Secret = ""
	invalidCloud.Cloud.FQDN = "wooself"
	err = invalidCloud.Ensure(true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"local_fqdn" is required`)
	invalidCloud.Cloud.LocalFQDN = "yeeself"
	test.That(t, invalidCloud.Ensure(true), test.ShouldBeNil)

	invalidRemotes := config.Config{
		Remotes: []config.Remote{{}},
	}
	err = invalidRemotes.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `remotes.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidRemotes.Remotes[0].Name = "foo"
	err = invalidRemotes.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"address" is required`)
	invalidRemotes.Remotes[0].Address = "bar"
	test.That(t, invalidRemotes.Ensure(false), test.ShouldBeNil)

	invalidComponents := config.Config{
		Components: []config.Component{{}},
	}
	err = invalidComponents.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidComponents.Components[0].Name = "foo"
	test.That(t, invalidComponents.Ensure(false), test.ShouldBeNil)

	c1 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c1"}
	c2 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c2", DependsOn: []string{"c1"}}
	c3 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c3", DependsOn: []string{"c1", "c2"}}
	c4 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c4", DependsOn: []string{"c1", "c3"}}
	c5 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c5", DependsOn: []string{"c2", "c4"}}
	c6 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c6"}
	c7 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c7", DependsOn: []string{"c6", "c4"}}
	unsortedComponents := config.Config{
		Components: []config.Component{c7, c6, c5, c3, c4, c1, c2},
	}
	err = unsortedComponents.Ensure(false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, unsortedComponents.Components, test.ShouldResemble, []config.Component{c6, c1, c2, c3, c4, c7, c5})

	invalidProcesses := config.Config{
		Processes: []pexec.ProcessConfig{{}},
	}
	err = invalidProcesses.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `processes.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)
	invalidProcesses.Processes[0].ID = "bar"
	err = invalidProcesses.Ensure(false)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidProcesses.Processes[0].Name = "foo"
	test.That(t, invalidProcesses.Ensure(false), test.ShouldBeNil)

	invalidNetwork := config.Config{
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				TLSCertFile: "hey",
			},
		},
	}
	err = invalidNetwork.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `network`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `both tls`)

	invalidNetwork.Network.TLSCertFile = ""
	invalidNetwork.Network.TLSKeyFile = "hey"
	err = invalidNetwork.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `network`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `both tls`)

	invalidNetwork.Network.TLSCertFile = "dude"
	test.That(t, invalidNetwork.Ensure(false), test.ShouldBeNil)

	invalidNetwork.Network.TLSCertFile = ""
	invalidNetwork.Network.TLSKeyFile = ""
	test.That(t, invalidNetwork.Ensure(false), test.ShouldBeNil)

	invalidNetwork.Network.BindAddress = "woop"
	err = invalidNetwork.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `bind_address`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `missing port`)

	invalidAuthConfig := config.Config{
		Auth: config.AuthConfig{},
	}
	test.That(t, invalidAuthConfig.Ensure(false), test.ShouldBeNil)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		{Type: rpc.CredentialsTypeAPIKey},
	}
	err = invalidAuthConfig.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `required`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `key`)

	validAPIKeyHandler := config.AuthHandlerConfig{
		Type: rpc.CredentialsTypeAPIKey,
		Config: config.AttributeMap{
			"key": "foo",
		},
	}

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
		validAPIKeyHandler,
	}
	err = invalidAuthConfig.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.1`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `duplicate`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `api-key`)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
		{Type: "unknown"},
	}
	err = invalidAuthConfig.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.1`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `do not know how`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `unknown`)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
	}
	test.That(t, invalidAuthConfig.Ensure(false), test.ShouldBeNil)
}

func TestConfigSortComponents(t *testing.T) {
	c1 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c1"}
	c2 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c2", DependsOn: []string{"c1"}}
	c3 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c3", DependsOn: []string{"c1", "c2"}}
	c4 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c4", DependsOn: []string{"c1", "c3"}}
	c5 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c5", DependsOn: []string{"c2", "c4"}}
	c6 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c6"}
	c7 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c7", DependsOn: []string{"c6", "c4"}}
	c8 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c8", DependsOn: []string{"c6"}}

	circularC1 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c1", DependsOn: []string{"c2"}}
	circularC2 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c2", DependsOn: []string{"c3"}}
	circularC3 := config.Component{Namespace: resource.ResourceNamespaceRDK, Name: "c3", DependsOn: []string{"c1"}}
	for _, tc := range []struct {
		Name       string
		Components []config.Component
		Expected   []config.Component
		Err        string
	}{
		{
			"empty",
			[]config.Component{},
			[]config.Component{},
			"",
		},
		{
			"no change",
			[]config.Component{c1, c2},
			[]config.Component{c1, c2},
			"",
		},
		{
			"simple",
			[]config.Component{c2, c1},
			[]config.Component{c1, c2},
			"",
		},
		{
			"another simple",
			[]config.Component{c3, c2, c1},
			[]config.Component{c1, c2, c3},
			"",
		},
		{
			"complex",
			[]config.Component{c2, c3, c1},
			[]config.Component{c1, c2, c3},
			"",
		},
		{
			"more complex",
			[]config.Component{c7, c6, c5, c3, c4, c1, c2},
			[]config.Component{c6, c1, c2, c3, c4, c7, c5},
			"",
		},
		{
			"duplicate name",
			[]config.Component{c1, c1},
			nil,
			"not unique",
		},
		// TODO(RSDK-427): this check just raises a warning if a dependency is missing.
		// We cannot actually make the check fail since it will always fail for remote
		// dependencies.
		{
			"dependency not found",
			[]config.Component{c2},
			[]config.Component{c2},
			"",
		},
		{
			"circular dependency",
			[]config.Component{circularC1, c2},
			nil,
			"circular dependency detected in component list between c1, c2",
		},
		{
			"circular dependency 2",
			[]config.Component{circularC1, circularC2, circularC3},
			nil,
			"circular dependency detected in component list between c1, c2, c3",
		},
		{
			"circular dependency 3",
			[]config.Component{c6, c4, circularC1, c8, circularC2, circularC3},
			nil,
			"circular dependency detected in component list between c1, c2, c3",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			sorted, err := config.SortComponents(tc.Components)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sorted, test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, sorted, test.ShouldBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestNewTLSConfig(t *testing.T) {
	for _, tc := range []struct {
		TestName     string
		Config       *config.Config
		HasTLSConfig bool
	}{
		{TestName: "no cloud", Config: &config.Config{}, HasTLSConfig: false},
		{TestName: "cloud but no cert", Config: &config.Config{Cloud: &config.Cloud{TLSCertificate: ""}}, HasTLSConfig: false},
		{TestName: "cloud and cert", Config: &config.Config{Cloud: &config.Cloud{TLSCertificate: "abc"}}, HasTLSConfig: true},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := config.NewTLSConfig(tc.Config)
			if tc.HasTLSConfig {
				test.That(t, observed.MinVersion, test.ShouldEqual, tls.VersionTLS12)
			} else {
				test.That(t, observed, test.ShouldResemble, &config.TLSConfig{})
			}
		})
	}
}

func TestUpdateCert(t *testing.T) {
	t.Run("cert update", func(t *testing.T) {
		cfg := &config.Config{
			Cloud: &config.Cloud{
				TLSCertificate: `-----BEGIN CERTIFICATE-----
MIIBCzCBtgIJAIuXZJ6ZiHraMA0GCSqGSIb3DQEBCwUAMA0xCzAJBgNVBAYTAnVz
MB4XDTIyMDQwNTE5MTMzNVoXDTIzMDQwNTE5MTMzNVowDTELMAkGA1UEBhMCdXMw
XDANBgkqhkiG9w0BAQEFAANLADBIAkEAyiHLgbZFf5UNAue0HAdQfv1Z15n8ldkI
bi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYFSApxXwIDAQABMA0G
CSqGSIb3DQEBCwUAA0EAOYH+xj8NuneL6w5D/FlW0+qUwBaS+/J3nL+PW1MQqjs8
1AHgPDxOtY7dUXK2E8SYia75JjtK9/FnpaFVHdQ9jQ==
-----END CERTIFICATE-----`,
				TLSPrivateKey: `-----BEGIN PRIVATE KEY-----
MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEAyiHLgbZFf5UNAue0
HAdQfv1Z15n8ldkIbi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYF
SApxXwIDAQABAkAEY412qI2DwqnAqWVIwoPl7fxYaRiJ7Gd5dPiPEjP0OPglB7eJ
VuSJeiPi3XSFXE9tw//Lpe2oOITF6OBCZURBAiEA7oZslGO+24+leOffb8PpceNm
EgHnAdibedkHD7ZprX8CIQDY8NASxuaEMa6nH7b9kkx/KaOo0/dOkW+sWb5PeIbs
IQIgOUd6p5/UY3F5cTFtjK9lTf4nssdWLDFSFM6zTWimtA0CIHwhFj2YN2/uaYvQ
1siyfDjKn41Lc5cuGmLYms8oHLNhAiBxeGqLlEyHdk+Trp99+nK+pFi4cj5NZSFh
ph2C/7IgjA==
-----END PRIVATE KEY-----`,
			},
		}
		cert, err := tls.X509KeyPair([]byte(cfg.Cloud.TLSCertificate), []byte(cfg.Cloud.TLSPrivateKey))
		test.That(t, err, test.ShouldBeNil)

		tlsCfg := config.NewTLSConfig(cfg)
		err = tlsCfg.UpdateCert(cfg)
		test.That(t, err, test.ShouldBeNil)

		observed, err := tlsCfg.GetCertificate(&tls.ClientHelloInfo{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, observed, test.ShouldResemble, &cert)
	})
	t.Run("cert error", func(t *testing.T) {
		cfg := &config.Config{Cloud: &config.Cloud{TLSCertificate: "abcd", TLSPrivateKey: "abcd"}}
		tlsCfg := &config.TLSConfig{}
		err := tlsCfg.UpdateCert(cfg)
		test.That(t, err, test.ShouldBeError, errors.New("tls: failed to find any PEM data in certificate input"))
	})
}

func TestProcessConfig(t *testing.T) {
	cloud := &config.Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "def",
		Secret:           "ghi",
		TLSCertificate:   "",
	}
	cloudWTLS := &config.Cloud{
		ManagedBy:        "acme",
		SignalingAddress: "abc",
		ID:               "def",
		Secret:           "ghi",
		TLSCertificate: `-----BEGIN CERTIFICATE-----
MIIBCzCBtgIJAIuXZJ6ZiHraMA0GCSqGSIb3DQEBCwUAMA0xCzAJBgNVBAYTAnVz
MB4XDTIyMDQwNTE5MTMzNVoXDTIzMDQwNTE5MTMzNVowDTELMAkGA1UEBhMCdXMw
XDANBgkqhkiG9w0BAQEFAANLADBIAkEAyiHLgbZFf5UNAue0HAdQfv1Z15n8ldkI
bi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYFSApxXwIDAQABMA0G
CSqGSIb3DQEBCwUAA0EAOYH+xj8NuneL6w5D/FlW0+qUwBaS+/J3nL+PW1MQqjs8
1AHgPDxOtY7dUXK2E8SYia75JjtK9/FnpaFVHdQ9jQ==
-----END CERTIFICATE-----`,
		TLSPrivateKey: `-----BEGIN PRIVATE KEY-----
MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEAyiHLgbZFf5UNAue0
HAdQfv1Z15n8ldkIbi4Owm5Iwb9IGGdkQNniEgveue536vV/ugAdt8ZxLuM1vzYF
SApxXwIDAQABAkAEY412qI2DwqnAqWVIwoPl7fxYaRiJ7Gd5dPiPEjP0OPglB7eJ
VuSJeiPi3XSFXE9tw//Lpe2oOITF6OBCZURBAiEA7oZslGO+24+leOffb8PpceNm
EgHnAdibedkHD7ZprX8CIQDY8NASxuaEMa6nH7b9kkx/KaOo0/dOkW+sWb5PeIbs
IQIgOUd6p5/UY3F5cTFtjK9lTf4nssdWLDFSFM6zTWimtA0CIHwhFj2YN2/uaYvQ
1siyfDjKn41Lc5cuGmLYms8oHLNhAiBxeGqLlEyHdk+Trp99+nK+pFi4cj5NZSFh
ph2C/7IgjA==
-----END PRIVATE KEY-----`,
	}

	remoteAuth := config.RemoteAuth{
		Credentials:            &rpc.Credentials{rutils.CredentialsTypeRobotSecret, "xyz"},
		Managed:                false,
		SignalingServerAddress: "xyz",
		SignalingAuthEntity:    "xyz",
	}
	remote := config.Remote{
		ManagedBy: "acme",
		Auth:      remoteAuth,
	}
	remoteDiffManager := config.Remote{
		ManagedBy: "viam",
		Auth:      remoteAuth,
	}
	noCloudCfg := &config.Config{Remotes: []config.Remote{}}
	cloudCfg := &config.Config{Cloud: cloud, Remotes: []config.Remote{}}
	cloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{}}
	remotesNoCloudCfg := &config.Config{Remotes: []config.Remote{remote, remoteDiffManager}}
	remotesCloudCfg := &config.Config{Cloud: cloud, Remotes: []config.Remote{remote, remoteDiffManager}}
	remotesCloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{remote, remoteDiffManager}}

	expectedRemoteAuthNoCloud := remoteAuth
	expectedRemoteAuthNoCloud.SignalingCreds = expectedRemoteAuthNoCloud.Credentials

	expectedRemoteAuthCloud := remoteAuth
	expectedRemoteAuthCloud.Managed = true
	expectedRemoteAuthCloud.SignalingServerAddress = cloud.SignalingAddress
	expectedRemoteAuthCloud.SignalingAuthEntity = cloud.ID
	expectedRemoteAuthCloud.SignalingCreds = &rpc.Credentials{rutils.CredentialsTypeRobotSecret, cloud.Secret}

	expectedRemoteNoCloud := remote
	expectedRemoteNoCloud.Auth = expectedRemoteAuthNoCloud
	expectedRemoteCloud := remote
	expectedRemoteCloud.Auth = expectedRemoteAuthCloud

	expectedRemoteDiffManagerNoCloud := remoteDiffManager
	expectedRemoteDiffManagerNoCloud.Auth = expectedRemoteAuthNoCloud

	tlsCfg := &config.TLSConfig{}
	err := tlsCfg.UpdateCert(cloudWTLSCfg)
	test.That(t, err, test.ShouldBeNil)

	expectedCloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{}}
	expectedCloudWTLSCfg.Network.TLSConfig = tlsCfg.Config

	expectedRemotesCloudWTLSCfg := &config.Config{Cloud: cloudWTLS, Remotes: []config.Remote{expectedRemoteCloud, remoteDiffManager}}
	expectedRemotesCloudWTLSCfg.Network.TLSConfig = tlsCfg.Config

	for _, tc := range []struct {
		TestName string
		Config   *config.Config
		Expected *config.Config
	}{
		{TestName: "no cloud", Config: noCloudCfg, Expected: noCloudCfg},
		{TestName: "cloud but no cert", Config: cloudCfg, Expected: cloudCfg},
		{TestName: "cloud and cert", Config: cloudWTLSCfg, Expected: expectedCloudWTLSCfg},
		{
			TestName: "remotes no cloud",
			Config:   remotesNoCloudCfg,
			Expected: &config.Config{Remotes: []config.Remote{expectedRemoteNoCloud, expectedRemoteDiffManagerNoCloud}},
		},
		{
			TestName: "remotes cloud but no cert",
			Config:   remotesCloudCfg,
			Expected: &config.Config{Cloud: cloud, Remotes: []config.Remote{expectedRemoteCloud, remoteDiffManager}},
		},
		{TestName: "remotes cloud and cert", Config: remotesCloudWTLSCfg, Expected: expectedRemotesCloudWTLSCfg},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed, err := config.ProcessConfig(tc.Config, &config.TLSConfig{})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}

	t.Run("cert error", func(t *testing.T) {
		cfg := &config.Config{Cloud: &config.Cloud{TLSCertificate: "abcd", TLSPrivateKey: "abcd"}}
		_, err := config.ProcessConfig(cfg, &config.TLSConfig{})
		test.That(t, err, test.ShouldBeError, errors.New("tls: failed to find any PEM data in certificate input"))
	})
}

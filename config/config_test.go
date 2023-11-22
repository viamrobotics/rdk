package config_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/jwks"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/encoder/incremental"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

func TestConfigRobot(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/robot.json", logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Components, test.ShouldHaveLength, 4)
	test.That(t, len(cfg.Remotes), test.ShouldEqual, 2)
	test.That(t, cfg.Remotes[0].Name, test.ShouldEqual, "one")
	test.That(t, cfg.Remotes[0].Address, test.ShouldEqual, "foo")
	test.That(t, cfg.Remotes[1].Name, test.ShouldEqual, "two")
	test.That(t, cfg.Remotes[1].Address, test.ShouldEqual, "bar")

	var foundArm, foundCam bool
	for _, comp := range cfg.Components {
		if comp.API == arm.API && comp.Model == resource.DefaultModelFamily.WithModel("ur") {
			foundArm = true
		}
		if comp.API == camera.API && comp.Model == resource.DefaultModelFamily.WithModel("url") {
			foundCam = true
		}
	}
	test.That(t, foundArm, test.ShouldBeTrue)
	test.That(t, foundCam, test.ShouldBeTrue)

	// test that gripper geometry is being added correctly
	component := cfg.FindComponent("pieceGripper")
	bc, _ := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{4, 5, 6}), r3.Vector{1, 2, 3}, "")
	newBc, err := component.Frame.Geometry.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newBc, test.ShouldResemble, bc)
}

func TestConfig3(t *testing.T) {
	logger := logging.NewTestLogger(t)

	test.That(t, os.Setenv("TEST_THING_FOO", "5"), test.ShouldBeNil)
	cfg, err := config.Read(context.Background(), "data/config3.json", logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(cfg.Components), test.ShouldEqual, 4)
	test.That(t, cfg.Components[0].Attributes.Int("foo", 0), test.ShouldEqual, 5)
	test.That(t, cfg.Components[0].Attributes.Bool("foo2", false), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Bool("foo3", false), test.ShouldEqual, false)
	test.That(t, cfg.Components[0].Attributes.Bool("xxxx", true), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Bool("xxxx", false), test.ShouldEqual, false)
	test.That(t, cfg.Components[0].Attributes.String("foo4"), test.ShouldEqual, "no")
	test.That(t, cfg.Components[0].Attributes.String("xxxx"), test.ShouldEqual, "")
	test.That(t, cfg.Components[0].Attributes.Has("foo"), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Has("xxxx"), test.ShouldEqual, false)
	test.That(t, cfg.Components[0].Attributes.Float64("bar5", 1.1), test.ShouldEqual, 5.17)
	test.That(t, cfg.Components[0].Attributes.Float64("bar5-no", 1.1), test.ShouldEqual, 1.1)

	test.That(t, cfg.Components[1].ConvertedAttributes, test.ShouldResemble, &fakeboard.Config{
		AnalogReaders: []board.AnalogReaderConfig{
			{Name: "analog1", Pin: "0"},
		},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "encoder", Pin: "14"},
		},
	})

	test.That(t, cfg.Components[2].ConvertedAttributes, test.ShouldResemble, &fakemotor.Config{
		Pins: fakemotor.PinConfig{
			Direction: "io17",
			PWM:       "io18",
		},
		Encoder:          "encoder1",
		MaxPowerPct:      0.5,
		TicksPerRotation: 10000,
	})
	test.That(t, cfg.Components[2].AssociatedResourceConfigs, test.ShouldHaveLength, 1)
	test.That(t, cfg.Components[2].AssociatedResourceConfigs[0], test.ShouldResemble, resource.AssociatedResourceConfig{
		API: resource.APINamespaceRDK.WithServiceType("data_manager"),
		Attributes: rutils.AttributeMap{
			"hi":     1.1,
			"friend": 2.2,
		},
	})

	test.That(t, cfg.Components[3].ConvertedAttributes, test.ShouldResemble, &incremental.Config{
		Pins: incremental.Pins{
			A: "encoder-steering-b",
			B: "encoder-steering-a",
		},
		BoardName: "board1",
	})

	test.That(t, cfg.Network.Sessions.HeartbeatWindow, test.ShouldEqual, 5*time.Second)
	test.That(t, cfg.Remotes, test.ShouldHaveLength, 1)
	test.That(t, cfg.Remotes[0].ConnectionCheckInterval, test.ShouldEqual, 12*time.Second)
	test.That(t, cfg.Remotes[0].ReconnectInterval, test.ShouldEqual, 3*time.Second)
	test.That(t, cfg.Remotes[0].AssociatedResourceConfigs, test.ShouldHaveLength, 2)
	test.That(t, cfg.Remotes[0].AssociatedResourceConfigs[0], test.ShouldResemble, resource.AssociatedResourceConfig{
		API: resource.APINamespaceRDK.WithServiceType("data_manager"),
		Attributes: rutils.AttributeMap{
			"hi":     3.3,
			"friend": 4.4,
		},
		RemoteName: "rem1",
	})
	test.That(t, cfg.Remotes[0].AssociatedResourceConfigs[1], test.ShouldResemble, resource.AssociatedResourceConfig{
		API: resource.APINamespaceRDK.WithServiceType("some_type"),
		Attributes: rutils.AttributeMap{
			"hi":     5.5,
			"friend": 6.6,
		},
		RemoteName: "rem1",
	})
}

func TestConfigWithLogDeclarations(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/config_with_log.json", logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(cfg.Components), test.ShouldEqual, 4)
	// The board log level is explicitly configured as `Info`.
	test.That(t, cfg.Components[0].Name, test.ShouldEqual, "board1")
	test.That(t, cfg.Components[0].LogConfiguration.Level, test.ShouldEqual, logging.INFO)

	// The left motor is explicitly configured as `debug`. Note the lower case.
	test.That(t, cfg.Components[1].Name, test.ShouldEqual, "left_motor")
	test.That(t, cfg.Components[1].LogConfiguration.Level, test.ShouldEqual, logging.DEBUG)

	// The right motor is left unconfigured. The default log level is `Info`. However, the global
	// log configure for builtin fake motors would apply for a log level of `warn`. This "overlayed"
	// log level is not applied at config parsing time.
	test.That(t, cfg.Components[2].Name, test.ShouldEqual, "right_motor")
	test.That(t, cfg.Components[2].LogConfiguration.Level, test.ShouldEqual, logging.INFO)

	// The wheeled base is also left unconfigured. The global log configuration for things
	// implementing the `base` API is `error`. This "overlayed" log level is not applied at config
	// parsing time.
	test.That(t, cfg.Components[3].Name, test.ShouldEqual, "wheeley")
	test.That(t, cfg.Components[3].LogConfiguration.Level, test.ShouldEqual, logging.INFO)

	test.That(t, len(cfg.Services), test.ShouldEqual, 2)
	// The slam service has a log level of `WARN`. Note the upper case.
	test.That(t, cfg.Services[0].Name, test.ShouldEqual, "slam1")
	test.That(t, cfg.Services[0].LogConfiguration.Level, test.ShouldEqual, logging.WARN)

	// The data manager service is left unconfigured.
	test.That(t, cfg.Services[1].Name, test.ShouldEqual, "dm")
	test.That(t, cfg.Services[1].LogConfiguration.Level, test.ShouldEqual, logging.INFO)

	test.That(t, len(cfg.GlobalLogConfig), test.ShouldEqual, 2)
	// The first global configuration is to default `base`s to `error`.
	test.That(t, cfg.GlobalLogConfig[0].API.String(), test.ShouldEqual, "rdk:component:base")
	test.That(t, cfg.GlobalLogConfig[0].Level, test.ShouldEqual, logging.ERROR)

	// The second global configuration is to default `motor`s of the builtin fake variety to `warn`.
	test.That(t, cfg.GlobalLogConfig[1].API.String(), test.ShouldEqual, "rdk:component:motor")
	test.That(t, cfg.GlobalLogConfig[1].Level, test.ShouldEqual, logging.WARN)
}

func TestConfigEnsure(t *testing.T) {
	logger := logging.NewTestLogger(t)
	var emptyConfig config.Config
	test.That(t, emptyConfig.Ensure(false, logger), test.ShouldBeNil)

	invalidCloud := config.Config{
		Cloud: &config.Cloud{},
	}
	err := invalidCloud.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `cloud`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "id")
	invalidCloud.Cloud.ID = "some_id"
	err = invalidCloud.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "secret")
	err = invalidCloud.Ensure(true, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "fqdn")
	invalidCloud.Cloud.Secret = "my_secret"
	test.That(t, invalidCloud.Ensure(false, logger), test.ShouldBeNil)
	test.That(t, invalidCloud.Ensure(true, logger), test.ShouldNotBeNil)
	invalidCloud.Cloud.Secret = ""
	invalidCloud.Cloud.FQDN = "wooself"
	err = invalidCloud.Ensure(true, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "local_fqdn")
	invalidCloud.Cloud.LocalFQDN = "yeeself"
	test.That(t, invalidCloud.Ensure(true, logger), test.ShouldBeNil)

	invalidRemotes := config.Config{
		DisablePartialStart: true,
		Remotes:             []config.Remote{{}},
	}
	err = invalidRemotes.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `remotes.0`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "name")
	invalidRemotes.Remotes[0] = config.Remote{
		Name: "foo",
	}
	err = invalidRemotes.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "address")
	invalidRemotes.Remotes[0] = config.Remote{
		Name:    "foo",
		Address: "bar",
	}
	test.That(t, invalidRemotes.Ensure(false, logger), test.ShouldBeNil)

	invalidComponents := config.Config{
		DisablePartialStart: true,
		Components:          []resource.Config{{}},
	}
	err = invalidComponents.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "name")
	invalidComponents.Components[0] = resource.Config{
		Name:  "foo",
		API:   base.API,
		Model: fakeModel,
	}

	test.That(t, invalidComponents.Ensure(false, logger), test.ShouldBeNil)

	c1 := resource.Config{
		Name:  "c1",
		API:   base.API,
		Model: resource.DefaultModelFamily.WithModel("c1"),
	}
	c2 := resource.Config{
		Name:      "c2",
		API:       base.API,
		DependsOn: []string{"c1"},
		Model:     resource.DefaultModelFamily.WithModel("c2"),
	}
	c3 := resource.Config{
		Name:      "c3",
		API:       base.API,
		DependsOn: []string{"c1", "c2"},
		Model:     resource.DefaultModelFamily.WithModel("c3"),
	}
	c4 := resource.Config{
		Name:      "c4",
		API:       base.API,
		DependsOn: []string{"c1", "c3"},
		Model:     resource.DefaultModelFamily.WithModel("c4"),
	}
	c5 := resource.Config{
		Name:      "c5",
		API:       base.API,
		DependsOn: []string{"c2", "c4"},
		Model:     resource.DefaultModelFamily.WithModel("c5"),
	}
	c6 := resource.Config{
		Name:  "c6",
		API:   base.API,
		Model: resource.DefaultModelFamily.WithModel("c6"),
	}
	c7 := resource.Config{
		Name:      "c7",
		API:       base.API,
		DependsOn: []string{"c6", "c4"},
		Model:     resource.DefaultModelFamily.WithModel("c7"),
	}
	components := config.Config{
		DisablePartialStart: true,
		Components:          []resource.Config{c7, c6, c5, c3, c4, c1, c2},
	}
	err = components.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)

	invalidProcesses := config.Config{
		DisablePartialStart: true,
		Processes:           []pexec.ProcessConfig{{}},
	}
	err = invalidProcesses.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `processes.0`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "id")
	invalidProcesses = config.Config{
		DisablePartialStart: true,
		Processes:           []pexec.ProcessConfig{{ID: "bar"}},
	}
	err = invalidProcesses.Ensure(false, logger)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "name")
	invalidProcesses = config.Config{
		DisablePartialStart: true,
		Processes:           []pexec.ProcessConfig{{ID: "bar", Name: "foo"}},
	}
	test.That(t, invalidProcesses.Ensure(false, logger), test.ShouldBeNil)

	invalidNetwork := config.Config{
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				TLSCertFile: "hey",
			},
		},
	}
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `network`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `both tls`)

	invalidNetwork.Network.TLSCertFile = ""
	invalidNetwork.Network.TLSKeyFile = "hey"
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `network`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `both tls`)

	invalidNetwork.Network.TLSCertFile = "dude"
	test.That(t, invalidNetwork.Ensure(false, logger), test.ShouldBeNil)

	invalidNetwork.Network.TLSCertFile = ""
	invalidNetwork.Network.TLSKeyFile = ""
	test.That(t, invalidNetwork.Ensure(false, logger), test.ShouldBeNil)

	test.That(t, invalidNetwork.Network.Sessions.HeartbeatWindow, test.ShouldNotBeNil)
	test.That(t, invalidNetwork.Network.Sessions.HeartbeatWindow, test.ShouldEqual, config.DefaultSessionHeartbeatWindow)

	invalidNetwork.Network.Sessions.HeartbeatWindow = time.Millisecond
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `heartbeat_window`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `between`)

	invalidNetwork.Network.Sessions.HeartbeatWindow = 2 * time.Minute
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `heartbeat_window`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `between`)

	invalidNetwork.Network.Sessions.HeartbeatWindow = 30 * time.Millisecond
	test.That(t, invalidNetwork.Ensure(false, logger), test.ShouldBeNil)

	invalidNetwork.Network.BindAddress = "woop"
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `bind_address`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `missing port`)

	invalidNetwork.Network.BindAddress = "woop"
	invalidNetwork.Network.Listener = &net.TCPListener{}
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `only set one of`)

	invalidAuthConfig := config.Config{
		Auth: config.AuthConfig{},
	}
	test.That(t, invalidAuthConfig.Ensure(false, logger), test.ShouldBeNil)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		{Type: rpc.CredentialsTypeAPIKey},
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `required`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `key`)

	validAPIKeyHandler := config.AuthHandlerConfig{
		Type: rpc.CredentialsTypeAPIKey,
		Config: rutils.AttributeMap{
			"key": "foo",
		},
	}

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
		validAPIKeyHandler,
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.1`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `duplicate`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `api-key`)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
		{Type: "unknown"},
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.1`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `do not know how`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `unknown`)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
	}
	test.That(t, invalidAuthConfig.Ensure(false, logger), test.ShouldBeNil)

	validAPIKeyHandler.Config = rutils.AttributeMap{
		"keys": []string{},
	}
	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `required`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `key`)

	validAPIKeyHandler.Config = rutils.AttributeMap{
		"keys": []string{"one", "two"},
	}
	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
	}

	test.That(t, invalidAuthConfig.Ensure(false, logger), test.ShouldBeNil)
}

func TestConfigEnsurePartialStart(t *testing.T) {
	logger := logging.NewTestLogger(t)
	var emptyConfig config.Config
	test.That(t, emptyConfig.Ensure(false, logger), test.ShouldBeNil)

	invalidCloud := config.Config{
		Cloud: &config.Cloud{},
	}
	err := invalidCloud.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `cloud`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "id")
	invalidCloud.Cloud.ID = "some_id"
	err = invalidCloud.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "secret")
	err = invalidCloud.Ensure(true, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "fqdn")
	invalidCloud.Cloud.Secret = "my_secret"
	test.That(t, invalidCloud.Ensure(false, logger), test.ShouldBeNil)
	test.That(t, invalidCloud.Ensure(true, logger), test.ShouldNotBeNil)
	invalidCloud.Cloud.Secret = ""
	invalidCloud.Cloud.FQDN = "wooself"
	err = invalidCloud.Ensure(true, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "local_fqdn")
	invalidCloud.Cloud.LocalFQDN = "yeeself"
	test.That(t, invalidCloud.Ensure(true, logger), test.ShouldBeNil)

	invalidRemotes := config.Config{
		Remotes: []config.Remote{{}},
	}
	err = invalidRemotes.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)
	invalidRemotes.Remotes[0].Name = "foo"
	err = invalidRemotes.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)
	invalidRemotes.Remotes[0].Address = "bar"
	test.That(t, invalidRemotes.Ensure(false, logger), test.ShouldBeNil)

	invalidComponents := config.Config{
		Components: []resource.Config{{}},
	}
	err = invalidComponents.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)
	invalidComponents.Components[0].Name = "foo"

	c1 := resource.Config{Name: "c1"}
	c2 := resource.Config{Name: "c2", DependsOn: []string{"c1"}}
	c3 := resource.Config{Name: "c3", DependsOn: []string{"c1", "c2"}}
	c4 := resource.Config{Name: "c4", DependsOn: []string{"c1", "c3"}}
	c5 := resource.Config{Name: "c5", DependsOn: []string{"c2", "c4"}}
	c6 := resource.Config{Name: "c6"}
	c7 := resource.Config{Name: "c7", DependsOn: []string{"c6", "c4"}}
	components := config.Config{
		Components: []resource.Config{c7, c6, c5, c3, c4, c1, c2},
	}
	err = components.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)

	invalidProcesses := config.Config{
		Processes: []pexec.ProcessConfig{{}},
	}
	err = invalidProcesses.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)
	invalidProcesses.Processes[0].Name = "foo"
	test.That(t, invalidProcesses.Ensure(false, logger), test.ShouldBeNil)

	cloudErr := "bad cloud err doing validation"
	invalidModules := config.Config{
		Modules: []config.Module{{
			Name:        "testmodErr",
			ExePath:     ".",
			LogLevel:    "debug",
			Type:        config.ModuleTypeRegistry,
			ModuleID:    "mod:testmodErr",
			Environment: map[string]string{},
			Status: &config.AppValidationStatus{
				Error: cloudErr,
			},
		}},
	}
	invalidModules.DisablePartialStart = true
	err = invalidModules.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, cloudErr)

	invalidModules.DisablePartialStart = false
	err = invalidModules.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)

	invalidPackges := config.Config{
		Packages: []config.PackageConfig{{
			Name:    "testPackage",
			Type:    config.PackageTypeMlModel,
			Package: "hi/package/test",
			Status: &config.AppValidationStatus{
				Error: cloudErr,
			},
		}},
	}

	invalidModules.DisablePartialStart = true
	err = invalidModules.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, cloudErr)

	invalidModules.DisablePartialStart = false
	err = invalidPackges.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)

	invalidNetwork := config.Config{
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				TLSCertFile: "hey",
			},
		},
	}
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `network`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `both tls`)

	invalidNetwork.Network.TLSCertFile = ""
	invalidNetwork.Network.TLSKeyFile = "hey"
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `network`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `both tls`)

	invalidNetwork.Network.TLSCertFile = "dude"
	test.That(t, invalidNetwork.Ensure(false, logger), test.ShouldBeNil)

	invalidNetwork.Network.TLSCertFile = ""
	invalidNetwork.Network.TLSKeyFile = ""
	test.That(t, invalidNetwork.Ensure(false, logger), test.ShouldBeNil)

	invalidNetwork.Network.BindAddress = "woop"
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `bind_address`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `missing port`)

	invalidNetwork.Network.BindAddress = "woop"
	invalidNetwork.Network.Listener = &net.TCPListener{}
	err = invalidNetwork.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `only set one of`)

	invalidAuthConfig := config.Config{
		Auth: config.AuthConfig{},
	}
	test.That(t, invalidAuthConfig.Ensure(false, logger), test.ShouldBeNil)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		{Type: rpc.CredentialsTypeAPIKey},
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `required`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `key`)

	validAPIKeyHandler := config.AuthHandlerConfig{
		Type: rpc.CredentialsTypeAPIKey,
		Config: rutils.AttributeMap{
			"key": "foo",
		},
	}

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
		validAPIKeyHandler,
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.1`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `duplicate`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `api-key`)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
		{Type: "unknown"},
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.1`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `do not know how`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `unknown`)

	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
	}
	test.That(t, invalidAuthConfig.Ensure(false, logger), test.ShouldBeNil)

	validAPIKeyHandler.Config = rutils.AttributeMap{
		"keys": []string{},
	}
	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
	}
	err = invalidAuthConfig.Ensure(false, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `auth.handlers.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `required`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `key`)

	validAPIKeyHandler.Config = rutils.AttributeMap{
		"keys": []string{"one", "two"},
	}
	invalidAuthConfig.Auth.Handlers = []config.AuthHandlerConfig{
		validAPIKeyHandler,
	}

	test.That(t, invalidAuthConfig.Ensure(false, logger), test.ShouldBeNil)
}

func TestRemoteValidate(t *testing.T) {
	t.Run("remote invalid name", func(t *testing.T) {
		lc := &referenceframe.LinkConfig{
			Parent: "parent",
		}
		validRemote := config.Remote{
			Name:    "foo-_remote",
			Address: "address",
			Frame:   lc,
		}

		_, err := validRemote.Validate("path")
		test.That(t, err, test.ShouldBeNil)

		validRemote = config.Remote{
			Name:    "foo.remote",
			Address: "address",
			Frame:   lc,
		}
		_, err = validRemote.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldContainSubstring,
			"must start with a letter and must only contain letters, numbers, dashes, and underscores",
		)
	})
}

func TestCopyOnlyPublicFields(t *testing.T) {
	t.Run("copy sample config", func(t *testing.T) {
		content, err := os.ReadFile("data/robot.json")
		test.That(t, err, test.ShouldBeNil)
		var cfg config.Config
		json.Unmarshal(content, &cfg)

		cfgCopy, err := cfg.CopyOnlyPublicFields()
		test.That(t, err, test.ShouldBeNil)

		test.That(t, *cfgCopy, test.ShouldResemble, cfg)
	})

	t.Run("should not copy unexported json fields", func(t *testing.T) {
		cfg := &config.Config{
			Cloud: &config.Cloud{
				TLSCertificate: "abc",
			},
			Network: config.NetworkConfig{
				NetworkConfigData: config.NetworkConfigData{
					TLSConfig: &tls.Config{
						Time: time.Now().UTC,
					},
				},
			},
		}

		cfgCopy, err := cfg.CopyOnlyPublicFields()
		test.That(t, err, test.ShouldBeNil)

		test.That(t, cfgCopy.Cloud.TLSCertificate, test.ShouldEqual, cfg.Cloud.TLSCertificate)
		test.That(t, cfgCopy.Network.TLSConfig, test.ShouldBeNil)
	})
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

func TestAuthConfigEnsure(t *testing.T) {
	t.Run("unknown handler", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		config := config.Config{
			Auth: config.AuthConfig{
				Handlers: []config.AuthHandlerConfig{
					{
						Type:   "some-type",
						Config: rutils.AttributeMap{"key": "abc123"},
					},
				},
			},
		}

		err := config.Ensure(true, logger)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how to handle auth for \"some-type\"")
	})

	t.Run("api-key handler", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		config := config.Config{
			Auth: config.AuthConfig{
				Handlers: []config.AuthHandlerConfig{
					{
						Type:   rpc.CredentialsTypeAPIKey,
						Config: rutils.AttributeMap{"key": "abc123"},
					},
				},
			},
		}

		err := config.Ensure(true, logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("external auth with invalid keyset", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		config := config.Config{
			Auth: config.AuthConfig{
				ExternalAuthConfig: &config.ExternalAuthConfig{},
			},
		}

		err := config.Ensure(true, logger)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed to parse jwks")
	})

	t.Run("external auth valid config", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		algTypes := map[string]bool{
			"RS256": true,
			"RS384": true,
			"RS512": true,
		}

		for alg := range algTypes {
			keyset := jwk.NewSet()
			privKeyForWebAuth, err := rsa.GenerateKey(rand.Reader, 256)
			test.That(t, err, test.ShouldBeNil)
			publicKeyForWebAuth, err := jwk.New(privKeyForWebAuth.PublicKey)
			test.That(t, err, test.ShouldBeNil)
			publicKeyForWebAuth.Set("alg", alg)
			publicKeyForWebAuth.Set(jwk.KeyIDKey, "key-id-1")
			test.That(t, keyset.Add(publicKeyForWebAuth), test.ShouldBeTrue)

			config := config.Config{
				Auth: config.AuthConfig{
					ExternalAuthConfig: &config.ExternalAuthConfig{
						JSONKeySet: keysetToAttributeMap(t, keyset),
					},
				},
			}

			err = config.Ensure(true, logger)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, config.Auth.ExternalAuthConfig.ValidatedKeySet, test.ShouldNotBeNil)
			_, ok := config.Auth.ExternalAuthConfig.ValidatedKeySet.LookupKeyID("key-id-1")
			test.That(t, ok, test.ShouldBeTrue)
		}
	})

	t.Run("web-oauth invalid alg type", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		badTypes := []string{"invalid", "", "nil"} // nil is a special case and is not set.
		for _, badType := range badTypes {
			t.Run(fmt.Sprintf(" with %s", badType), func(t *testing.T) {
				keyset := jwk.NewSet()
				privKeyForWebAuth, err := rsa.GenerateKey(rand.Reader, 256)
				test.That(t, err, test.ShouldBeNil)
				publicKeyForWebAuth, err := jwk.New(privKeyForWebAuth.PublicKey)
				test.That(t, err, test.ShouldBeNil)

				if badType != "nil" {
					publicKeyForWebAuth.Set("alg", badType)
				}

				publicKeyForWebAuth.Set(jwk.KeyIDKey, "key-id-1")
				test.That(t, keyset.Add(publicKeyForWebAuth), test.ShouldBeTrue)

				config := config.Config{
					Auth: config.AuthConfig{
						ExternalAuthConfig: &config.ExternalAuthConfig{
							JSONKeySet: keysetToAttributeMap(t, keyset),
						},
					},
				}

				err = config.Ensure(true, logger)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid alg")
			})
		}
	})

	t.Run("external auth no keys", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		config := config.Config{
			Auth: config.AuthConfig{
				ExternalAuthConfig: &config.ExternalAuthConfig{
					JSONKeySet: keysetToAttributeMap(t, jwk.NewSet()),
				},
			},
		}

		err := config.Ensure(true, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "must contain at least 1 key")
	})
}

func TestValidateUniqueNames(t *testing.T) {
	logger := logging.NewTestLogger(t)
	component := resource.Config{
		Name:  "custom",
		Model: fakeModel,
		API:   arm.API,
	}
	service := resource.Config{
		Name:  "custom",
		Model: fakeModel,
		API:   shell.API,
	}
	package1 := config.PackageConfig{
		Package: "package1",
		Name:    "package1",
		Type:    config.PackageTypeMlModel,
	}
	module1 := config.Module{
		Name:     "m1",
		LogLevel: "info",
		ExePath:  ".",
	}

	process1 := pexec.ProcessConfig{
		ID: "process1", Name: "process1",
	}

	remote1 := config.Remote{
		Name:    "remote1",
		Address: "test",
	}
	config1 := config.Config{
		Components: []resource.Config{component, component},
	}
	config2 := config.Config{
		Services: []resource.Config{service, service},
	}

	config3 := config.Config{
		Packages: []config.PackageConfig{package1, package1},
	}
	config4 := config.Config{
		Modules: []config.Module{module1, module1},
	}
	config5 := config.Config{
		Processes: []pexec.ProcessConfig{process1, process1},
	}

	config6 := config.Config{
		Remotes: []config.Remote{remote1, remote1},
	}
	allConfigs := []config.Config{config1, config2, config3, config4, config5, config6}

	for _, config := range allConfigs {
		// returns an error instead of logging it
		config.DisablePartialStart = true
		// test that the logger returns an error after the ensure method is done
		err := config.Ensure(false, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "duplicate resource")

		observedLogger, logs := logging.NewObservedTestLogger(t)
		// now test it with logging enabled
		config.DisablePartialStart = false
		err = config.Ensure(false, observedLogger)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, logs.FilterMessageSnippet("duplicate resource").Len(), test.ShouldBeGreaterThan, 0)
	}

	// mix components and services with the same name -- no error as use triplets
	config7 := config.Config{
		Components: []resource.Config{component},
		Services:   []resource.Config{service},
		Modules:    []config.Module{module1},
		Remotes: []config.Remote{
			{
				Name:    module1.Name,
				Address: "test1",
			},
		},
	}
	config7.DisablePartialStart = true
	err := config7.Ensure(false, logger)
	test.That(t, err, test.ShouldBeNil)
}

func keysetToAttributeMap(t *testing.T, keyset jwks.KeySet) rutils.AttributeMap {
	t.Helper()

	// hack around marshaling the KeySet into pb.Struct. Passing interface directly
	// does not work.
	jwksAsJSON, err := json.Marshal(keyset)
	test.That(t, err, test.ShouldBeNil)

	jwksAsInterface := rutils.AttributeMap{}
	err = json.Unmarshal(jwksAsJSON, &jwksAsInterface)
	test.That(t, err, test.ShouldBeNil)

	return jwksAsInterface
}

func TestPackageConfig(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	viamDotDir := filepath.Join(homeDir, ".viam")

	packageTests := []struct {
		config               config.PackageConfig
		shouldFailValidation bool
		expectedRealFilePath string
	}{
		{
			config: config.PackageConfig{
				Name:    "my_package",
				Package: "my_org/my_package",
				Version: "0",
			},
			expectedRealFilePath: filepath.Join(viamDotDir, "packages", ".data", "ml_model", "my_org-my_package-0"),
		},
		{
			config: config.PackageConfig{
				Name:    "my_module",
				Type:    config.PackageTypeModule,
				Package: "my_org/my_module",
				Version: "1.2",
			},
			expectedRealFilePath: filepath.Join(viamDotDir, "packages", ".data", "module", "my_org-my_module-1_2"),
		},
		{
			config: config.PackageConfig{
				Name:    "my_ml_model",
				Type:    config.PackageTypeMlModel,
				Package: "my_org/my_ml_model",
				Version: "latest",
			},
			expectedRealFilePath: filepath.Join(viamDotDir, "packages", ".data", "ml_model", "my_org-my_ml_model-latest"),
		},
		{
			config: config.PackageConfig{
				Name:    "my_slam_map",
				Type:    config.PackageTypeSlamMap,
				Package: "my_org/my_slam_map",
				Version: "latest",
			},
			expectedRealFilePath: filepath.Join(viamDotDir, "packages", ".data", "slam_map", "my_org-my_slam_map-latest"),
		},
		{
			config: config.PackageConfig{
				Name:    "my_board_defs",
				Type:    config.PackageTypeBoardDefs,
				Package: "my_org/my_board_defs",
				Version: "latest",
			},
			expectedRealFilePath: filepath.Join(viamDotDir, "packages", ".data", "board_defs", "my_org-my_board_defs-latest"),
		},
		{
			config: config.PackageConfig{
				Name:    "::::",
				Type:    config.PackageTypeMlModel,
				Package: "my_org/my_ml_model",
				Version: "latest",
			},
			shouldFailValidation: true,
		},
		{
			config: config.PackageConfig{
				Name:    "my_ml_model",
				Type:    config.PackageType("willfail"),
				Package: "my_org/my_ml_model",
				Version: "latest",
			},
			shouldFailValidation: true,
		},
	}

	for _, pt := range packageTests {
		err := pt.config.Validate("")
		if pt.shouldFailValidation {
			test.That(t, err, test.ShouldBeError)
			continue
		}
		test.That(t, err, test.ShouldBeNil)
		actualFilepath := pt.config.LocalDataDirectory(filepath.Join(viamDotDir, "packages"))
		test.That(t, actualFilepath, test.ShouldEqual, pt.expectedRealFilePath)
	}
}

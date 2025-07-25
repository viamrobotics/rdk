package config_test

import (
	"context"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

func TestFromReaderValidate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	_, err := config.FromReader(context.Background(), "somepath", strings.NewReader(""), logger, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "json: EOF")

	_, err = config.FromReader(context.Background(), "somepath", strings.NewReader(`{"cloud": 1}`), logger, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unmarshal")

	conf, err := config.FromReader(context.Background(), "somepath", strings.NewReader(`{}`), logger, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, &config.Config{
		ConfigFilePath: "somepath",
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress:           "localhost:8080",
				BindAddressDefaultSet: true,
				Sessions: config.SessionsConfig{
					HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
				},
			},
		},
	})

	_, err = config.FromReader(context.Background(), "somepath", strings.NewReader(`{"cloud": {}}`), logger, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "id")

	conf, err = config.FromReader(context.Background(),
		"somepath",
		strings.NewReader(`{"components": [{"name": "foo", "type": "arm", "model": "foo"}]}`),
		logger, nil)
	test.That(t, err, test.ShouldBeNil)
	expected := &config.Config{
		ConfigFilePath: "somepath",
		Components: []resource.Config{
			{
				Name:  "foo",
				API:   arm.API,
				Model: resource.DefaultModelFamily.WithModel("foo"),
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress:           "localhost:8080",
			BindAddressDefaultSet: true,
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}
	test.That(t, expected.Ensure(false, logger), test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, expected)
}

func TestFromReaderEmptyModuleEnvironment(t *testing.T) {
	logger := logging.NewTestLogger(t)
	conf, err := config.FromReader(context.Background(),
		"somepath",
		strings.NewReader(`{"cloud":{"id":"123","secret":"abc"},"modules": [{"name": "foo"}]}`),
		logger, nil)
	test.That(t, err, test.ShouldBeNil)
	expected := &config.Config{
		ConfigFilePath: "somepath",
		Cloud:          &config.Cloud{ID: "123", Secret: "abc"},
		Modules: []config.Module{
			{
				Name: "foo",
				Environment: map[string]string{
					rutils.MachinePartIDEnvVar: "123",
					rutils.PrimaryOrgIDEnvVar:  "",
					rutils.LocationIDEnvVar:    "",
					rutils.MachineIDEnvVar:     "",
					rutils.MachineFQDNEnvVar:   "",
				},
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
			BindAddress:           "localhost:8080",
			BindAddressDefaultSet: true,
			Sessions: config.SessionsConfig{
				HeartbeatWindow: config.DefaultSessionHeartbeatWindow,
			},
		}},
	}
	test.That(t, expected.Ensure(false, logger), test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, expected)
}

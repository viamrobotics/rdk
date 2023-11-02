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
)

func TestFromReaderValidate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	_, err := config.FromReader(context.Background(), "somepath", strings.NewReader(""), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "json: EOF")

	_, err = config.FromReader(context.Background(), "somepath", strings.NewReader(`{"cloud": 1}`), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unmarshal")

	conf, err := config.FromReader(context.Background(), "somepath", strings.NewReader(`{}`), logger)
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

	_, err = config.FromReader(context.Background(), "somepath", strings.NewReader(`{"cloud": {}}`), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)

	_, err = config.FromReader(context.Background(),
		"somepath", strings.NewReader(`{"disable_partial_start":true,"components": [{}]}`), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	conf, err = config.FromReader(context.Background(),
		"somepath",
		strings.NewReader(`{"components": [{"name": "foo", "type": "arm", "model": "foo"}]}`),
		logger)
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

package config_test

import (
	"context"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func TestFromReaderValidate(t *testing.T) {
	logger := golog.NewTestLogger(t)
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
				DeprecatedNamespace: resource.ResourceNamespaceRDK,
				Name:                "foo",
				API:                 arm.Subtype,
				Model:               resource.NewDefaultModel("foo"),
				DeprecatedSubtype:   arm.SubtypeName,
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

func TestTransformAttributeMapToStruct(t *testing.T) {
	type myType struct {
		A          string            `json:"a"`
		B          string            `json:"b"`
		Attributes map[string]string `json:"attributes"`
	}

	var mt myType
	attrs := utils.AttributeMap{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": 5,
	}
	transformed, err := config.TransformAttributeMapToStruct(&mt, attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myType{
		A: "1",
		B: "2",
		Attributes: map[string]string{
			"c": "3",
			"d": "4",
		},
	})

	mt = myType{Attributes: map[string]string{}}
	transformed, err = config.TransformAttributeMapToStruct(&mt, attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myType{
		A: "1",
		B: "2",
		Attributes: map[string]string{
			"c": "3",
			"d": "4",
		},
	})

	type myExtendedType struct {
		A          string             `json:"a"`
		B          string             `json:"b"`
		Attributes utils.AttributeMap `json:"attributes"`
	}

	var met myExtendedType
	transformed, err = config.TransformAttributeMapToStruct(&met, attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: utils.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})

	met = myExtendedType{Attributes: utils.AttributeMap{}}
	transformed, err = config.TransformAttributeMapToStruct(&met, attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: utils.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})
}

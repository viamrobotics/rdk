package config_test

import (
	"context"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

func TestFromReaderValidate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	_, err := config.FromReader(context.Background(), "somepath", strings.NewReader(""), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected end")

	_, err = config.FromReader(context.Background(), "somepath", strings.NewReader(`{"cloud": 1}`), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unmarshal")

	conf, err := config.FromReader(context.Background(), "somepath", strings.NewReader(`{}`), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, &config.Config{
		ConfigFilePath: "somepath",
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{BindAddress: "localhost:8080", BindAddressDefaultSet: true},
		},
	})

	_, err = config.FromReader(context.Background(), "somepath", strings.NewReader(`{"cloud": {}}`), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)

	_, err = config.FromReader(context.Background(), "somepath", strings.NewReader(`{"components": [{}]}`), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	conf, err = config.FromReader(context.Background(),
		"somepath",
		strings.NewReader(`{"components": [{"name": "foo", "type": "arm"}]}`),
		logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, &config.Config{
		ConfigFilePath: "somepath",
		Components: []config.Component{
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      arm.SubtypeName,
			},
		},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{BindAddress: "localhost:8080", BindAddressDefaultSet: true}},
	})

	badComponentMapConverter := func() {
		config.RegisterComponentAttributeMapConverter(resource.SubtypeName("somecomponent"),
			"somemodel",
			func(attributes config.AttributeMap) (interface{}, error) {
				return &conf, nil
			}, nil)
	}
	test.That(t, badComponentMapConverter, test.ShouldPanic)

	badServiceMapConverter := func() {
		config.RegisterServiceAttributeMapConverter(config.ServiceType("someservice"), func(attributes config.AttributeMap) (interface{}, error) {
			return &conf, nil
		}, nil)
	}
	test.That(t, badServiceMapConverter, test.ShouldPanic)
}

func TestTransformAttributeMapToStruct(t *testing.T) {
	type myType struct {
		A          string            `json:"a"`
		B          string            `json:"b"`
		Attributes map[string]string `json:"attributes"`
	}

	var mt myType
	attrs := config.AttributeMap{
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
		A          string              `json:"a"`
		B          string              `json:"b"`
		Attributes config.AttributeMap `json:"attributes"`
	}

	var met myExtendedType
	transformed, err = config.TransformAttributeMapToStruct(&met, attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: config.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})

	met = myExtendedType{Attributes: config.AttributeMap{}}
	transformed, err = config.TransformAttributeMapToStruct(&met, attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: config.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})
}

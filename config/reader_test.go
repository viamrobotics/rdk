package config

import (
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestFromReaderValidate(t *testing.T) {
	_, err := FromReader("somepath", strings.NewReader(""))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "EOF")

	_, err = FromReader("somepath", strings.NewReader(`{"cloud": 1}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unmarshal")

	conf, err := FromReader("somepath", strings.NewReader(`{}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, &Config{
		ConfigFilePath: "somepath",
		Network:        NetworkConfig{BindAddress: "localhost:8080"},
	})

	_, err = FromReader("somepath", strings.NewReader(`{"cloud": {}}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)

	_, err = FromReader("somepath", strings.NewReader(`{"components": [{}]}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	conf, err = FromReader("somepath", strings.NewReader(`{"components": [{"name": "foo", "type": "arm"}]}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, &Config{
		ConfigFilePath: "somepath",
		Components: []Component{
			{
				Name: "foo",
				Type: ComponentTypeArm,
			},
		},
		Network: NetworkConfig{BindAddress: "localhost:8080"},
	})

	badComponentMapConverter := func() {
		RegisterComponentAttributeMapConverter(ComponentType("somecomponent"), "somemodel", func(attributes AttributeMap) (interface{}, error) {
			return &conf, nil
		}, nil)
	}
	test.That(t, badComponentMapConverter, test.ShouldPanic)

	badServiceMapConverter := func() {
		RegisterServiceAttributeMapConverter(ServiceType("someservice"), func(attributes AttributeMap) (interface{}, error) {
			return &conf, nil
		}, nil)
	}
	test.That(t, badServiceMapConverter, test.ShouldPanic)
}

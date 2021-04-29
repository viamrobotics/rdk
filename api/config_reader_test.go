package api

import (
	"strings"
	"testing"

	"github.com/edaniels/test"
)

func TestReadConfigFromReaderValidate(t *testing.T) {
	_, err := ReadConfigFromReader("somepath", strings.NewReader(""))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "EOF")

	_, err = ReadConfigFromReader("somepath", strings.NewReader(`{"cloud": 1}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unmarshal")

	conf, err := ReadConfigFromReader("somepath", strings.NewReader(`{}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, &Config{
		ConfigFilePath: "somepath",
	})

	_, err = ReadConfigFromReader("somepath", strings.NewReader(`{"cloud": {}}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)

	_, err = ReadConfigFromReader("somepath", strings.NewReader(`{"components": [{}]}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	conf, err = ReadConfigFromReader("somepath", strings.NewReader(`{"components": [{"name": "foo"}]}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf, test.ShouldResemble, &Config{
		ConfigFilePath: "somepath",
		Components: []ComponentConfig{
			{
				Name: "foo",
			},
		},
	})
}

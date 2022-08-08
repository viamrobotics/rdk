package camera

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestCommonCameraAttributes(t *testing.T) {
	testConfWrong := config.AttributeMap{
		"source": "TestSource",
		"stream": 5,
	}
	_, err := CommonCameraAttributes(testConfWrong)
	test.That(t, err.Error(), test.ShouldContainSubstring, "'stream' expected type 'string', got unconvertible type")
	testConf := config.AttributeMap{
		"stream": "color",
	}
	res, err := CommonCameraAttributes(testConf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Stream, test.ShouldEqual, "color")
}

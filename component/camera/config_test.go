package camera

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestCommonCameraAttributes(t *testing.T) {
	testConfWrong := config.AttributeMap{
		"source": "TestSource",
		"width":  5,
		"height": "7",
	}
	_, err := CommonCameraAttributes(testConfWrong)
	test.That(t, err.Error(), test.ShouldContainSubstring, "'height' expected type 'int', got unconvertible type")
	testConf := config.AttributeMap{
		"width":  5,
		"height": 7,
	}
	res, err := CommonCameraAttributes(testConf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Width, test.ShouldEqual, 5)
	test.That(t, res.Height, test.ShouldEqual, 7)
}

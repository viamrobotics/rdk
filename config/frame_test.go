package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"go.viam.com/test"

	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/utils"
)

func TestOrientation(t *testing.T) {
	file, err := os.Open("data/frames.json")
	test.That(t, err, test.ShouldBeNil)

	defer utils.UncheckedErrorFunc(file.Close)

	data, err := ioutil.ReadAll(file)
	test.That(t, err, test.ShouldBeNil)
	// Parse into config
	var frame FrameConfig
	err = json.Unmarshal(data, &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "a")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{1, 2, 3})
	test.That(t, frame.Orientation.Type, test.ShouldEqual, "ov_degrees")
	test.That(t, frame.Orientation.Value.OrientationVectorDegrees(), test.ShouldResemble, &spatial.OrientationVecDegrees{45, 0, 0, 1})

}

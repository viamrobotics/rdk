package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"go.viam.com/test"

	"github.com/go-errors/errors"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/num/quat"

	spatial "go.viam.com/core/spatialmath"
)

func TestOrientation(t *testing.T) {
	file, err := os.Open("data/frames.json")
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := ioutil.ReadAll(file)
	test.That(t, err, test.ShouldBeNil)
	// Parse into map of tests
	var testMap map[string]json.RawMessage
	err = json.Unmarshal(data, &testMap)
	test.That(t, err, test.ShouldBeNil)
	// go through each test case

	// Config with unknown orientation
	frame := Frame{}
	err = json.Unmarshal(testMap["wrong"], &frame)
	test.That(t, err, test.ShouldBeError, errors.New("orientation type oiler_angles not recognized"))

	// Empty Config
	frame = Frame{}
	err = json.Unmarshal(testMap["empty"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{0, 0, 0})
	test.That(t, frame.Orientation.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})

	// Mostly Empty Config
	frame = Frame{}
	err = json.Unmarshal(testMap["mostlyempty"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "a")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{0, 0, 0})
	test.That(t, frame.Orientation.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})

	// OrientationVectorDegrees Config
	frame = Frame{}
	err = json.Unmarshal(testMap["ovdegrees"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "a")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{1, 2, 3})
	test.That(t, frame.Orientation.OrientationVectorDegrees(), test.ShouldResemble, &spatial.OrientationVectorDegrees{45, 0, 0, 1})

	// OrientationVector Radians Config
	frame = Frame{}
	err = json.Unmarshal(testMap["ovradians"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "b")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{4, 5, 6})
	test.That(t, frame.Orientation.OrientationVectorRadians(), test.ShouldResemble, &spatial.OrientationVector{0.78539816, 0, 1, 0})

	// Euler Angles
	frame = Frame{}
	err = json.Unmarshal(testMap["euler"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "c")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{7, 8, 9})
	test.That(t, frame.Orientation.EulerAngles(), test.ShouldResemble, &spatial.EulerAngles{Roll: 0, Pitch: 0, Yaw: 45})

	// Axis angles Config
	frame = Frame{}
	err = json.Unmarshal(testMap["axisangle"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "d")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{0, 0, 0})
	test.That(t, frame.Orientation.AxisAngles(), test.ShouldResemble, &spatial.R4AA{0.78539816, 1, 0, 0})
}

package spatialmath

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/num/quat"
)

func TestOrientation(t *testing.T) {
	file, err := os.Open("data/orientations.json")
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
	ro := RawOrientation{}
	err = json.Unmarshal(testMap["wrong"], &ro)
	test.That(t, err, test.ShouldBeNil)
	_, err = ParseOrientation(ro)
	test.That(t, err, test.ShouldBeError, errors.New("orientation type oiler_angles not recognized"))

	// Config with good type, but bad value
	ro = RawOrientation{}
	err = json.Unmarshal(testMap["wrongvalue"], &ro)
	test.That(t, err, test.ShouldBeNil)
	_, err = ParseOrientation(ro)
	test.That(t, err, test.ShouldBeError,
		errors.New("json: cannot unmarshal string into Go struct field OrientationVectorDegrees.th of type float64"))

	// Empty Config
	ro = RawOrientation{}
	err = json.Unmarshal(testMap["empty"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err := ParseOrientation(ro)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})
	_, err = OrientationMap(o)
	test.That(t, err, test.ShouldBeError, errors.Errorf("do not know how to map Orientation type %T to json fields", o))

	// OrientationVectorDegrees Config
	ro = RawOrientation{}
	err = json.Unmarshal(testMap["ovdegrees"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ParseOrientation(ro)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.OrientationVectorDegrees(), test.ShouldResemble, &OrientationVectorDegrees{45, 0, 0, 1})
	om, err := OrientationMap(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, om["type"], test.ShouldEqual, string(OrientationVectorDegreesType))
	test.That(t, om["value"], test.ShouldResemble, o)

	// OrientationVector Radians Config
	ro = RawOrientation{}
	err = json.Unmarshal(testMap["ovradians"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ParseOrientation(ro)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.OrientationVectorRadians(), test.ShouldResemble, &OrientationVector{0.78539816, 0, 1, 0})
	om, err = OrientationMap(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, om["type"], test.ShouldEqual, string(OrientationVectorRadiansType))
	test.That(t, om["value"], test.ShouldResemble, o)

	// Euler Angles
	ro = RawOrientation{}
	err = json.Unmarshal(testMap["euler"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ParseOrientation(ro)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.EulerAngles(), test.ShouldResemble, &EulerAngles{Roll: 0, Pitch: 0, Yaw: 45})
	om, err = OrientationMap(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, om["type"], test.ShouldEqual, string(EulerAnglesType))
	test.That(t, om["value"], test.ShouldResemble, o)

	// Axis angles Config
	ro = RawOrientation{}
	err = json.Unmarshal(testMap["axisangle"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ParseOrientation(ro)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.AxisAngles(), test.ShouldResemble, &R4AA{0.78539816, 1, 0, 0})
	om, err = OrientationMap(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, om["type"], test.ShouldEqual, string(AxisAnglesType))
	test.That(t, om["value"], test.ShouldResemble, o)
}

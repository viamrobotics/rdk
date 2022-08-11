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
	testMap := loadOrientationTests(t)

	// Config with unknown orientation
	ro := OrientationConfig{}
	err := json.Unmarshal(testMap["wrong"], &ro)
	test.That(t, err, test.ShouldBeNil)
	_, err = ro.ParseConfig()
	test.That(t, err, test.ShouldBeError, newOrientationTypeUnsupportedError("oiler_angles"))

	// Config with good type, but bad value
	ro = OrientationConfig{}
	err = json.Unmarshal(testMap["wrongvalue"], &ro)
	test.That(t, err, test.ShouldBeNil)
	_, err = ro.ParseConfig()
	test.That(t, err, test.ShouldBeError,
		errors.New("json: cannot unmarshal string into Go struct field OrientationVectorDegrees.th of type float64"))

	// Empty Config
	ro = OrientationConfig{}
	err = json.Unmarshal(testMap["empty"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err := ro.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})

	// OrientationVectorDegrees Config
	ro = OrientationConfig{}
	err = json.Unmarshal(testMap["ovdegrees"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ro.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.OrientationVectorDegrees(), test.ShouldResemble, &OrientationVectorDegrees{45, 0, 0, 1})
	oc, err := NewOrientationConfig(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Type, test.ShouldEqual, string(OrientationVectorDegreesType))
	bytes, err := json.Marshal(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Value, test.ShouldResemble, json.RawMessage(bytes))

	// OrientationVector Radians Config
	ro = OrientationConfig{}
	err = json.Unmarshal(testMap["ovradians"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ro.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.OrientationVectorRadians(), test.ShouldResemble, &OrientationVector{0.78539816, 0, 1, 0})
	oc, err = NewOrientationConfig(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Type, test.ShouldEqual, string(OrientationVectorRadiansType))
	bytes, err = json.Marshal(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Value, test.ShouldResemble, json.RawMessage(bytes))

	// Euler Angles
	ro = OrientationConfig{}
	err = json.Unmarshal(testMap["euler"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ro.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.EulerAngles(), test.ShouldResemble, &EulerAngles{Roll: 0, Pitch: 0, Yaw: 45})
	oc, err = NewOrientationConfig(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Type, test.ShouldEqual, string(EulerAnglesType))
	bytes, err = json.Marshal(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Value, test.ShouldResemble, json.RawMessage(bytes))

	// Axis angles Config
	ro = OrientationConfig{}
	err = json.Unmarshal(testMap["axisangle"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ro.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, o.AxisAngles(), test.ShouldResemble, &R4AA{0.78539816, 1, 0, 0})
	oc, err = NewOrientationConfig(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Type, test.ShouldEqual, string(AxisAnglesType))
	bytes, err = json.Marshal(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Value, test.ShouldResemble, json.RawMessage(bytes))

	// Quaternion Config
	ro = OrientationConfig{}
	err = json.Unmarshal(testMap["quaternion"], &ro)
	test.That(t, err, test.ShouldBeNil)
	o, err = ro.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	aa := o.AxisAngles()
	test.That(t, aa.Theta, test.ShouldAlmostEqual, 1.5040802, .0001)
	test.That(t, aa.RX, test.ShouldAlmostEqual, 0.2672612, .0001)
	test.That(t, aa.RY, test.ShouldAlmostEqual, 0.5345225, .001)
	test.That(t, aa.RZ, test.ShouldAlmostEqual, 0.8017837, .001)
	oc, err = NewOrientationConfig(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Type, test.ShouldEqual, string(QuaternionType))
	bytes, err = json.Marshal(o)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.Value, test.ShouldResemble, json.RawMessage(bytes))
}

func loadOrientationTests(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	file, err := os.Open("data/orientations.json")
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := ioutil.ReadAll(file)
	test.That(t, err, test.ShouldBeNil)
	// Parse into map of tests
	var testMap map[string]json.RawMessage
	err = json.Unmarshal(data, &testMap)
	test.That(t, err, test.ShouldBeNil)
	return testMap
}

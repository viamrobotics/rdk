package lidar

import (
	"encoding/json"
	"math"
	"sort"
	"testing"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/utils"
	"gonum.org/v1/gonum/mat"
)

func TestMeasurement(t *testing.T) {
	m1 := NewMeasurement(0, 0)
	angRad := m1.AngleRad()
	test.That(t, angRad, test.ShouldEqual, 0)
	angDeg := m1.AngleDeg()
	test.That(t, angDeg, test.ShouldEqual, 0)
	d := m1.Distance()
	test.That(t, d, test.ShouldEqual, 0)
	x, y := m1.Coords()
	test.That(t, x, test.ShouldEqual, 0)
	test.That(t, y, test.ShouldEqual, 0)

	m2 := NewMeasurement(45, 10)
	angRad = m2.AngleRad()
	test.That(t, angRad, test.ShouldEqual, math.Pi/4)
	angDeg = m2.AngleDeg()
	test.That(t, angDeg, test.ShouldEqual, 45)
	d = m2.Distance()
	test.That(t, d, test.ShouldEqual, 10)
	x, y = m2.Coords()
	test.That(t, x, test.ShouldAlmostEqual, 7.071067811865475)
	test.That(t, y, test.ShouldAlmostEqual, 7.0710678118654755)

	m3 := NewMeasurement(135, 10)
	angRad = m3.AngleRad()
	test.That(t, angRad, test.ShouldEqual, 3*math.Pi/4)
	angDeg = m3.AngleDeg()
	test.That(t, angDeg, test.ShouldEqual, 135)
	d = m3.Distance()
	test.That(t, d, test.ShouldEqual, 10)
	x, y = m3.Coords()
	test.That(t, x, test.ShouldAlmostEqual, 7.071067811865475)
	test.That(t, y, test.ShouldAlmostEqual, -7.071067811865475)

	m4 := NewMeasurement(225, 10)
	angRad = m4.AngleRad()
	test.That(t, angRad, test.ShouldEqual, 5*math.Pi/4)
	angDeg = m4.AngleDeg()
	test.That(t, angDeg, test.ShouldEqual, 225)
	d = m4.Distance()
	test.That(t, d, test.ShouldEqual, 10)
	x, y = m4.Coords()
	test.That(t, x, test.ShouldAlmostEqual, -7.071067811865475)
	test.That(t, y, test.ShouldAlmostEqual, -7.071067811865475)

	m5 := NewMeasurement(315, 10)
	angRad = m5.AngleRad()
	test.That(t, angRad, test.ShouldEqual, 7*math.Pi/4)
	angDeg = m5.AngleDeg()
	test.That(t, angDeg, test.ShouldEqual, 315)
	d = m5.Distance()
	test.That(t, d, test.ShouldEqual, 10)
	x, y = m5.Coords()
	test.That(t, x, test.ShouldAlmostEqual, -7.071067811865475)
	test.That(t, y, test.ShouldAlmostEqual, 7.071067811865475)
}

func TestSortMeasurements(t *testing.T) {
	ms := Measurements{
		NewMeasurement(20, 3),
		NewMeasurement(20, 1),
		NewMeasurement(20, 2),
		NewMeasurement(0, 3),
		NewMeasurement(0, 1),
		NewMeasurement(0, 2),
		NewMeasurement(10, 3),
		NewMeasurement(10, 1),
		NewMeasurement(10, 2),
	}
	sort.Sort(ms)
	test.That(t, ms, test.ShouldResemble, Measurements{
		NewMeasurement(0, 1),
		NewMeasurement(0, 2),
		NewMeasurement(0, 3),
		NewMeasurement(10, 1),
		NewMeasurement(10, 2),
		NewMeasurement(10, 3),
		NewMeasurement(20, 1),
		NewMeasurement(20, 2),
		NewMeasurement(20, 3),
	})
}

func TestMeasurmentJSONRoundTrip(t *testing.T) {
	m1 := NewMeasurement(1, 2)
	md, err := json.Marshal(m1)
	test.That(t, err, test.ShouldBeNil)

	var m2 Measurement
	test.That(t, json.Unmarshal(md, &m2), test.ShouldBeNil)
	test.That(t, &m2, test.ShouldResemble, m1)

	err = json.Unmarshal([]byte(`{"angle":true}`), &m2)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot")
}

func TestMeasurementsFromVec2Matrix(t *testing.T) {
	m := mat.NewDense(3, 2, nil)
	m.Set(0, 0, 0)
	m.Set(1, 0, 0)
	m.Set(0, 1, -7.071067811865475)
	m.Set(1, 1, 7.071067811865475)

	ms := MeasurementsFromVec2Matrix((*utils.Vec2Matrix)(m))
	test.That(t, ms, test.ShouldResemble, Measurements{
		NewMeasurement(0, 0),
		NewMeasurement(315, 10),
	})
}

func TestMeasurementFromCoord(t *testing.T) {
	m1 := MeasurementFromCoord(0, 0)
	test.That(t, m1, test.ShouldResemble, NewMeasurement(0, 0))

	m2 := MeasurementFromCoord(-7.071067811865475, 7.071067811865475)
	test.That(t, m2, test.ShouldResemble, NewMeasurement(315, 10))
}

func TestMeasurementsClosestToDegree(t *testing.T) {
	ms := Measurements{
		NewMeasurement(50, 1),
		NewMeasurement(55, 1),
		NewMeasurement(60, 1),
	}

	m := ms.ClosestToDegree(51)
	test.That(t, 50.0, test.ShouldAlmostEqual, m.AngleDeg())

	m = ms.ClosestToDegree(57.2)
	test.That(t, 55.0, test.ShouldAlmostEqual, m.AngleDeg())

}

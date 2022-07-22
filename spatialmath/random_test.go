package spatialmath

import (
	"fmt"
	"math"
	"testing"

	"go.viam.com/test"
)

func TestAxisAngleRoundTrip(t *testing.T) {
	data := []R4AA{
		{1, 1, 1, 1},
		{1, 1, 0, 0},
		{1, 0, 1, 0},
		{1, 0, 0, 1},
	}

	// Quaternion [x, y, z, w]
	// from https://www.andre-gaschler.com/rotationconverter/
	qc := [][]float64{
		{0.2767965, 0.2767965, 0.2767965, 0.8775826},
		{0.4794255, 0, 0, 0.8775826},
		{0, 0.4794255, 0, 0.8775826},
		{0, 0, 0.4794255, 0.8775826},
	}

	for idx, d := range data {
		d.Normalize()
		q := Quaternion(d.Quaternion())

		d2 := q.AxisAngles()
		test.That(t, d2.Theta, test.ShouldAlmostEqual, d.Theta)
		test.That(t, d2.RX, test.ShouldAlmostEqual, d.RX)
		test.That(t, d2.RY, test.ShouldAlmostEqual, d.RY)
		test.That(t, d2.RZ, test.ShouldAlmostEqual, d.RZ)

		if idx < len(qc) {
			test.That(t, q.Real, test.ShouldAlmostEqual, qc[idx][3], .00001)
			test.That(t, q.Imag, test.ShouldAlmostEqual, qc[idx][0], .00001)
			test.That(t, q.Jmag, test.ShouldAlmostEqual, qc[idx][1], .00001)
			test.That(t, q.Kmag, test.ShouldAlmostEqual, qc[idx][2], .00001)
		} else {
			fmt.Printf("%#v\n%#v\n---\n", d, q)
		}

	}
}

func TestOrientationVectorRoundTrip(t *testing.T) {
	data := []OrientationVector{
		{1, 1, 1, 1},
		{1, 1, 0, 0},
		{1, 0, 1, 0},
		{1, 0, 0, 1},
	}

	for _, d := range data {
		d.Normalize()
		q := Quaternion(d.Quaternion())
		d2 := q.OrientationVectorRadians()
		test.That(t, d2.Theta, test.ShouldAlmostEqual, d.Theta)
		test.That(t, d2.OX, test.ShouldAlmostEqual, d.OX)
		test.That(t, d2.OY, test.ShouldAlmostEqual, d.OY)
		test.That(t, d2.OZ, test.ShouldAlmostEqual, d.OZ)
	}
}

func TestEulerRoundTrip(t *testing.T) {
	data := []EulerAngles{
		{1, 0, 0},
		{1, 1, 0},
		{1, 0, 1},
	}

	// Quaternion [x, y, z, w]
	// from https://www.andre-gaschler.com/rotationconverter/
	qc := [][]float64{
		{0.4794255, 0, 0, 0.8775826},
		{0.4207355, 0.4207355, 0.2298488, 0.7701512},
		{0.4207355, -0.2298488, 0.4207355, 0.7701512},
	}

	for idx, d := range data {
		q := Quaternion(d.Quaternion())
		d2 := q.EulerAngles()
		test.That(t, d2.Roll, test.ShouldAlmostEqual, d.Roll)
		test.That(t, d2.Pitch, test.ShouldAlmostEqual, d.Pitch)
		test.That(t, d2.Yaw, test.ShouldAlmostEqual, d.Yaw)

		if idx < len(qc) {
			test.That(t, q.Real, test.ShouldAlmostEqual, qc[idx][3], .00001)
			test.That(t, q.Imag, test.ShouldAlmostEqual, qc[idx][0], .00001)
			test.That(t, q.Jmag, test.ShouldAlmostEqual, qc[idx][1], .00001)
			test.That(t, q.Kmag, test.ShouldAlmostEqual, qc[idx][2], .00001)
		} else {
			fmt.Printf("%#v\n%#v\n---\n", d, q)
		}

	}

}

func TestOVToEuler(t *testing.T) {
	type p struct {
		ov OrientationVectorDegrees
		e  EulerAngles
	}

	data := []p{
		{OrientationVectorDegrees{90, 0, 1, 0}, EulerAngles{math.Pi / 2, 0, 0}},
	}

	for _, d := range data {
		e2 := d.ov.EulerAngles()
		fmt.Printf("%#v\n%#v\n%#v\n---\n", d.ov, d.e, e2)
		test.That(t, e2.Roll, test.ShouldAlmostEqual, d.e.Roll)
		test.That(t, e2.Pitch, test.ShouldAlmostEqual, d.e.Pitch)
		test.That(t, e2.Yaw, test.ShouldAlmostEqual, d.e.Yaw)

	}

}

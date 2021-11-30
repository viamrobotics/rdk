package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"
)

func TestQuaternionConversion(t *testing.T) {
	// Test that conversion to rotation matrix to quaternion is correct
	// http://www.euclideanspace.com/maths/geometry/rotations/conversions/matrixToQuaternion/examples/index.htm
	cos45 := 0.7071067811865476
	cases := []struct {
		input    [9]float64
		expected quat.Number
	}{
		{
			[9]float64{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
			NewZeroOrientation().Quaternion(),
		},
		{
			[9]float64{
				0, 0, -1,
				0, 1, 0,
				1, 0, 0,
			},
			quat.Number{cos45, 0, cos45, 0},
		},
		{
			[9]float64{
				-1, 0, 0,
				0, 1, 0,
				0, 0, -1,
			},
			quat.Number{0, 0, 1, 0},
		},
		{
			[9]float64{
				0, 1, 0,
				-1, 0, 0,
				0, 0, 1,
			},
			quat.Number{cos45, 0, 0, cos45},
		},
		{
			[9]float64{
				1, 0, 0,
				0, 0, 1,
				0, -1, 0,
			},
			quat.Number{cos45, cos45, 0, 0},
		},
		{
			[9]float64{
				0.43, -0.43, .78,
				0.75, -0.65, -0.04,
				-0.5, 0.61, 0.61,
			},
			quat.Number{cos45, cos45, 0, 0},
		},
	}

	for _, c := range cases {
		rm := &RotationMatrix{c.input}
		quatCompare(t, rm.Quaternion(), c.expected)
	}
}

func TestMatrixAtRowsCols(t *testing.T) {
	mat := [9]float64{
		1, 2, 3,
		4, 5, 6,
		7, 8, 9,
	}
	rm := &RotationMatrix{mat}

	// test At function
	test.That(t, rm.At(0, 0), test.ShouldEqual, 1)
	test.That(t, rm.At(0, 1), test.ShouldEqual, 2)
	test.That(t, rm.At(0, 2), test.ShouldEqual, 3)
	test.That(t, rm.At(1, 0), test.ShouldEqual, 4)
	test.That(t, rm.At(1, 1), test.ShouldEqual, 5)
	test.That(t, rm.At(1, 2), test.ShouldEqual, 6)
	test.That(t, rm.At(2, 0), test.ShouldEqual, 7)
	test.That(t, rm.At(2, 1), test.ShouldEqual, 8)
	test.That(t, rm.At(2, 2), test.ShouldEqual, 9)

	// test Rows function
	test.That(t, rm.Row(0).Cmp(r3.Vector{1, 2, 3}) == 0, test.ShouldBeTrue)
	test.That(t, rm.Row(1).Cmp(r3.Vector{4, 5, 6}) == 0, test.ShouldBeTrue)
	test.That(t, rm.Row(2).Cmp(r3.Vector{7, 8, 9}) == 0, test.ShouldBeTrue)

	// test Cols function
	test.That(t, rm.Col(0).Cmp(r3.Vector{1, 4, 7}) == 0, test.ShouldBeTrue)
	test.That(t, rm.Col(1).Cmp(r3.Vector{2, 5, 8}) == 0, test.ShouldBeTrue)
	test.That(t, rm.Col(2).Cmp(r3.Vector{3, 6, 9}) == 0, test.ShouldBeTrue)
}

package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
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
				-0.5003235, 0.1601237, 0.8509035,
				0.7536948, -0.4031713, 0.5190347,
				0.4261697, 0.9010068, 0.0810317,
			},
			quat.Number{-0.21067562973908407, 0.4532703843447015, 0.5040139879925649, 0.7043661157381153},
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

func TestMatrixMul(t *testing.T) {
	mat1 := []float64{
		1, 2, 3,
		4, 5, 6,
		7, 8, 9,
	}
	rm, err := NewRotationMatrix(mat1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, R3VectorAlmostEqual(rm.Mul(r3.Vector{1, 0, 0}), r3.Vector{1, 4, 7}, 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(rm.Mul(r3.Vector{1, 1, 0}), r3.Vector{3, 9, 15}, 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(rm.Mul(r3.Vector{1, 1, 1}), r3.Vector{6, 15, 24}, 1e-8), test.ShouldBeTrue)

	mat2 := []float64{
		9, 2, 3,
		6, 5, 4,
		3, 2, 1,
	}
	mm, err := NewRotationMatrix(mat2)
	test.That(t, err, test.ShouldBeNil)

	c, d, _ := multiplyAndconvertToFloats(mat1, mat2)

	mul := MatMul(*rm, *mm)
	lMul := rm.LeftMatMul(*mm).mat
	rMul := rm.RightMatMul(*mm).mat

	test.That(t, mul.mat, test.ShouldResemble, c)
	test.That(t, lMul, test.ShouldResemble, c)
	test.That(t, rMul, test.ShouldResemble, d)
}

func multiplyAndconvertToFloats(in1, in2 []float64) ([9]float64, [9]float64, error) {
	a := mat.NewDense(3, 3, in1)
	b := mat.NewDense(3, 3, in2)
	var c mat.Dense
	var d mat.Dense
	c.Mul(a, b) // c is left multiplication
	d.Mul(b, a) // d is right multiplication

	vecC := c.RawMatrix().Data
	vecD := d.RawMatrix().Data
	outC, err := NewRotationMatrix(vecC)
	if err != nil {
		return [9]float64{}, [9]float64{}, err
	}
	outD, err := NewRotationMatrix(vecD)
	if err != nil {
		return [9]float64{}, [9]float64{}, err
	}
	return outC.mat, outD.mat, err
}

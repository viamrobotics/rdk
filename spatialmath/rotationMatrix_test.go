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
		input    *rotationMatrix
		expected quat.Number
	}{
		{
			&rotationMatrix{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
			NewZeroOrientation().Quaternion(),
		},
		{
			&rotationMatrix{
				0, 0, -1,
				0, 1, 0,
				1, 0, 0,
			},
			quat.Number{cos45, 0, cos45, 0},
		},
		{
			&rotationMatrix{
				-1, 0, 0,
				0, 1, 0,
				0, 0, -1,
			},
			quat.Number{0, 0, 1, 0},
		},
		{
			&rotationMatrix{
				0, 1, 0,
				-1, 0, 0,
				0, 0, 1,
			},
			quat.Number{cos45, 0, 0, cos45},
		},
		{
			&rotationMatrix{
				1, 0, 0,
				0, 0, 1,
				0, -1, 0,
			},
			quat.Number{cos45, cos45, 0, 0},
		},
	}

	for _, c := range cases {
		quatCompare(t, c.input.Quaternion(), c.expected)
	}
}

func TestMatrixRows(t *testing.T) {
	rm := rotationMatrix{
		1, 2, 3,
		4, 5, 6,
		7, 8, 9,
	}
	test.That(t, rm.Row(0).Cmp(r3.Vector{1, 2, 3}) == 0, test.ShouldBeTrue)
	test.That(t, rm.Row(1).Cmp(r3.Vector{4, 5, 6}) == 0, test.ShouldBeTrue)
	test.That(t, rm.Row(2).Cmp(r3.Vector{7, 8, 9}) == 0, test.ShouldBeTrue)
}

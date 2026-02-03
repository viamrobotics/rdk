package transform

import (
	"testing"

	"go.viam.com/test"
)

func TestBrownConradyK6CheckValid(t *testing.T) {
	t.Run("nil &BrownConradyK6{} are invalid", func(t *testing.T) {
		var nilBrownConradyK6Ptr *BrownConradyK6
		err := nilBrownConradyK6Ptr.CheckValid()
		expected := "BrownConradyK6 shaped distortion_parameters not provided: invalid distortion_parameters"
		test.That(t, err.Error(), test.ShouldContainSubstring, expected)
	})

	t.Run("non nil &BrownConradyK6{} are valid", func(t *testing.T) {
		distortionsA := &BrownConradyK6{}
		test.That(t, distortionsA.CheckValid(), test.ShouldBeNil)
	})
}

func TestBrownConradyK6New(t *testing.T) {
	t.Run("valid parameters", func(t *testing.T) {
		params := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.01, 0.02}
		bc, err := NewBrownConradyK6(params)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bc.RadialK1, test.ShouldEqual, 0.1)
		test.That(t, bc.RadialK2, test.ShouldEqual, 0.2)
		test.That(t, bc.RadialK3, test.ShouldEqual, 0.3)
		test.That(t, bc.RadialK4, test.ShouldEqual, 0.4)
		test.That(t, bc.RadialK5, test.ShouldEqual, 0.5)
		test.That(t, bc.RadialK6, test.ShouldEqual, 0.6)
		test.That(t, bc.TangentialP1, test.ShouldEqual, 0.01)
		test.That(t, bc.TangentialP2, test.ShouldEqual, 0.02)
	})

	t.Run("empty parameters", func(t *testing.T) {
		bc, err := NewBrownConradyK6([]float64{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bc.Parameters(), test.ShouldResemble, []float64{0, 0, 0, 0, 0, 0, 0, 0})
	})

	t.Run("too many parameters", func(t *testing.T) {
		_, err := NewBrownConradyK6(make([]float64, 9))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "too long")
	})
}

func TestBrownConradyK6Parity(t *testing.T) {
	// When K4, K5, K6 are 0, BrownConradyK6 should behave exactly like BrownConrady
	params := []float64{0.1, 0.2, 0.3, 0.01, 0.02}
	bc, _ := NewBrownConrady(params)
	// Map params: K1, K2, K3, 0, 0, 0, P1, P2
	bcK6Params := []float64{params[0], params[1], params[2], 0, 0, 0, params[3], params[4]}
	bcK6, _ := NewBrownConradyK6(bcK6Params)

	x, y := 0.5, 0.5
	x1, y1 := bc.Transform(x, y)
	x2, y2 := bcK6.Transform(x, y)

	test.That(t, x2, test.ShouldAlmostEqual, x1)
	test.That(t, y2, test.ShouldAlmostEqual, y1)
}

func TestBrownConradyK6TransformAllZeroes(t *testing.T) {
	bc, _ := NewBrownConradyK6([]float64{0, 0, 0, 0, 0, 0, 0, 0})

	tx, ty := bc.Transform(0.5, 0.5)

	test.That(t, tx, test.ShouldAlmostEqual, 0.5, 1e-9)
	test.That(t, ty, test.ShouldAlmostEqual, 0.5, 1e-9)
}

func TestConstructorDoesNotModifyInput(t *testing.T) {
	// Create slice with extra capacity and fill the backing array with sentinel values
	backing := []float64{0.1, 0.01, 0.001, 0.0001, 0.00001, 0.000001, 99.0, 99.0}
	params := backing[:6] // Slice with length 6, capacity 8

	orig_full := make([]float64, len(backing))
	copy(orig_full, backing)

	NewBrownConradyK6(params)

	// This should FAIL because positions 6-7 changed from 99.0 to 0.0
	test.That(t, orig_full, test.ShouldResemble, backing)
}

func TestBrownConradyK6Transform(t *testing.T) {
	// Test with non-zero K4, K5, K6
	params := []float64{0.1, 0.01, 0.001, 0.0001, 0.00001, 0.000001, 0.01, 0.02}
	bc, _ := NewBrownConradyK6(params)

	x, y := 0.5, 0.5
	tx, ty := bc.Transform(x, y)

	expectedX := 0.55131578906250000571
	expectedY := 0.54631578906250000127

	test.That(t, tx, test.ShouldAlmostEqual, expectedX, 1e-9)
	test.That(t, ty, test.ShouldAlmostEqual, expectedY, 1e-9)
}

package transform

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestInverseBrownConradyCheckValid(t *testing.T) {
	t.Run("nil *InverseBrownConrady are invalid", func(t *testing.T) {
		var nilInverseBrownConradyPtr *InverseBrownConrady
		err := nilInverseBrownConradyPtr.CheckValid()
		expected := "InverseBrownConrady shaped distortion_parameters not provided: invalid distortion_parameters"
		test.That(t, err.Error(), test.ShouldContainSubstring, expected)
	})

	t.Run("non nil *InverseBrownConrady are valid", func(t *testing.T) {
		distortions := &InverseBrownConrady{}
		test.That(t, distortions.CheckValid(), test.ShouldBeNil)
	})
}

func TestNewInverseBrownConrady(t *testing.T) {
	t.Run("empty parameters", func(t *testing.T) {
		ibc, err := NewInverseBrownConrady([]float64{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ibc.RadialK1, test.ShouldEqual, 0.0)
		test.That(t, ibc.RadialK2, test.ShouldEqual, 0.0)
		test.That(t, ibc.RadialK3, test.ShouldEqual, 0.0)
		test.That(t, ibc.TangentialP1, test.ShouldEqual, 0.0)
		test.That(t, ibc.TangentialP2, test.ShouldEqual, 0.0)
	})

	t.Run("partial parameters", func(t *testing.T) {
		ibc, err := NewInverseBrownConrady([]float64{0.1, 0.2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ibc.RadialK1, test.ShouldEqual, 0.1)
		test.That(t, ibc.RadialK2, test.ShouldEqual, 0.2)
		test.That(t, ibc.RadialK3, test.ShouldEqual, 0.0)
		test.That(t, ibc.TangentialP1, test.ShouldEqual, 0.0)
		test.That(t, ibc.TangentialP2, test.ShouldEqual, 0.0)
	})

	t.Run("full parameters", func(t *testing.T) {
		ibc, err := NewInverseBrownConrady([]float64{0.1, 0.2, 0.3, 0.4, 0.5})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ibc.RadialK1, test.ShouldEqual, 0.1)
		test.That(t, ibc.RadialK2, test.ShouldEqual, 0.2)
		test.That(t, ibc.RadialK3, test.ShouldEqual, 0.3)
		test.That(t, ibc.TangentialP1, test.ShouldEqual, 0.4)
		test.That(t, ibc.TangentialP2, test.ShouldEqual, 0.5)
	})

	t.Run("too many parameters", func(t *testing.T) {
		_, err := NewInverseBrownConrady([]float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "list of parameters too long")
	})
}

func TestInverseBrownConradyModelType(t *testing.T) {
	ibc := &InverseBrownConrady{}
	test.That(t, ibc.ModelType(), test.ShouldEqual, InverseBrownConradyDistortionType)
}

func TestInverseBrownConradyParameters(t *testing.T) {
	t.Run("nil returns empty slice", func(t *testing.T) {
		var ibc *InverseBrownConrady
		params := ibc.Parameters()
		test.That(t, len(params), test.ShouldEqual, 0)
	})

	t.Run("returns correct parameters", func(t *testing.T) {
		ibc := &InverseBrownConrady{0.1, 0.2, 0.3, 0.4, 0.5}
		params := ibc.Parameters()
		test.That(t, params, test.ShouldResemble, []float64{0.1, 0.2, 0.3, 0.4, 0.5})
	})
}

func TestInverseBrownConradyTransform(t *testing.T) {
	t.Run("nil returns input unchanged", func(t *testing.T) {
		var ibc *InverseBrownConrady
		x, y := ibc.Transform(1.0, 2.0)
		test.That(t, x, test.ShouldEqual, 1.0)
		test.That(t, y, test.ShouldEqual, 2.0)
	})

	t.Run("zero distortion returns input unchanged", func(t *testing.T) {
		ibc := &InverseBrownConrady{}
		x, y := ibc.Transform(0.5, 0.3)
		test.That(t, math.Abs(x-0.5), test.ShouldBeLessThan, 1e-9)
		test.That(t, math.Abs(y-0.3), test.ShouldBeLessThan, 1e-9)
	})

	t.Run("inverse is inverse of forward Brown-Conrady", func(t *testing.T) {
		// Use typical camera distortion coefficients
		params := []float64{0.158701, -0.485405, 0.435342, -0.00143327, -0.000705919}
		bc, err := NewBrownConrady(params)
		test.That(t, err, test.ShouldBeNil)
		ibc, err := NewInverseBrownConrady(params)
		test.That(t, err, test.ShouldBeNil)

		// Test several undistorted points
		testPoints := []struct{ x, y float64 }{
			{0.0, 0.0},
			{0.1, 0.1},
			{-0.2, 0.3},
			{0.5, -0.5},
			{0.3, 0.4},
		}

		for _, pt := range testPoints {
			// Forward: undistorted -> distorted
			xd, yd := bc.Transform(pt.x, pt.y)
			// Inverse: distorted -> undistorted
			xu, yu := ibc.Transform(xd, yd)
			// Should recover original undistorted point
			test.That(t, math.Abs(xu-pt.x), test.ShouldBeLessThan, 1e-8)
			test.That(t, math.Abs(yu-pt.y), test.ShouldBeLessThan, 1e-8)
		}
	})

	t.Run("radial distortion only", func(t *testing.T) {
		params := []float64{0.1, -0.05, 0.01, 0.0, 0.0}
		bc, err := NewBrownConrady(params)
		test.That(t, err, test.ShouldBeNil)
		ibc, err := NewInverseBrownConrady(params)
		test.That(t, err, test.ShouldBeNil)

		xu, yu := 0.25, 0.35
		xd, yd := bc.Transform(xu, yu)
		xr, yr := ibc.Transform(xd, yd)

		test.That(t, math.Abs(xr-xu), test.ShouldBeLessThan, 1e-9)
		test.That(t, math.Abs(yr-yu), test.ShouldBeLessThan, 1e-9)
	})

	t.Run("tangential distortion only", func(t *testing.T) {
		params := []float64{0.0, 0.0, 0.0, 0.001, -0.002}
		bc, err := NewBrownConrady(params)
		test.That(t, err, test.ShouldBeNil)
		ibc, err := NewInverseBrownConrady(params)
		test.That(t, err, test.ShouldBeNil)

		xu, yu := 0.15, 0.25
		xd, yd := bc.Transform(xu, yu)
		xr, yr := ibc.Transform(xd, yd)

		test.That(t, math.Abs(xr-xu), test.ShouldBeLessThan, 1e-9)
		test.That(t, math.Abs(yr-yu), test.ShouldBeLessThan, 1e-9)
	})
}

func TestNewDistorterInverseBrownConrady(t *testing.T) {
	params := []float64{0.1, 0.2, 0.3, 0.04, 0.05}
	distorter, err := NewDistorter(InverseBrownConradyDistortionType, params)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distorter.ModelType(), test.ShouldEqual, InverseBrownConradyDistortionType)
	test.That(t, distorter.Parameters(), test.ShouldResemble, params)
}

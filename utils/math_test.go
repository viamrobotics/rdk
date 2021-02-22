package utils

import (
	"fmt"
	"math"
	"testing"

	"github.com/edaniels/test"
)

func TestAbs1(t *testing.T) {
	if 5 != AbsInt(5) {
		t.Errorf("wtf")
	}

	if 5 != AbsInt(-5) {
		t.Errorf("wtf")
	}

	if 0 != AbsInt(0) {
		t.Errorf("wtf")
	}

}

func TestSquare1(t *testing.T) {
	if 4.0 != Square(2.0) {
		t.Errorf("eliot can't do math")
	}

	if 4 != SquareInt(2) {
		t.Errorf("eliot can't do math")
	}

}

func TestDegToRad(t *testing.T) {
	test.That(t, DegToRad(0), test.ShouldEqual, 0)
	test.That(t, DegToRad(5.625), test.ShouldEqual, math.Pi/32)
	test.That(t, DegToRad(11.25), test.ShouldEqual, math.Pi/16)
	test.That(t, DegToRad(22.5), test.ShouldEqual, math.Pi/8)
	test.That(t, DegToRad(45), test.ShouldEqual, math.Pi/4)
	test.That(t, DegToRad(90), test.ShouldEqual, math.Pi/2)
	test.That(t, DegToRad(95.625), test.ShouldAlmostEqual, math.Pi/2+math.Pi/32)
	test.That(t, DegToRad(101.25), test.ShouldAlmostEqual, math.Pi/2+math.Pi/16)
	test.That(t, DegToRad(112.5), test.ShouldEqual, math.Pi/2+math.Pi/8)
	test.That(t, DegToRad(135), test.ShouldEqual, math.Pi/2+math.Pi/4)
	test.That(t, DegToRad(180), test.ShouldEqual, math.Pi)
	test.That(t, DegToRad(360), test.ShouldEqual, 2*math.Pi)
	test.That(t, DegToRad(720), test.ShouldEqual, 4*math.Pi)
}

func TestRadToDeg(t *testing.T) {
	test.That(t, 0, test.ShouldEqual, DegToRad(0))
	test.That(t, math.Pi/32, test.ShouldEqual, DegToRad(5.625))
	test.That(t, math.Pi/16, test.ShouldEqual, DegToRad(11.25))
	test.That(t, math.Pi/8, test.ShouldEqual, DegToRad(22.5))
	test.That(t, math.Pi/4, test.ShouldEqual, DegToRad(45))
	test.That(t, math.Pi/2, test.ShouldEqual, DegToRad(90))
	test.That(t, math.Pi/2+math.Pi/32, test.ShouldAlmostEqual, DegToRad(95.625))
	test.That(t, math.Pi/2+math.Pi/16, test.ShouldAlmostEqual, DegToRad(101.25))
	test.That(t, math.Pi/2+math.Pi/8, test.ShouldEqual, DegToRad(112.5))
	test.That(t, math.Pi/2+math.Pi/4, test.ShouldEqual, DegToRad(135))
	test.That(t, math.Pi, test.ShouldEqual, DegToRad(180))
	test.That(t, 2*math.Pi, test.ShouldEqual, DegToRad(360))
	test.That(t, 4*math.Pi, test.ShouldEqual, DegToRad(720))
}

func TestAngleDiffDeg(t *testing.T) {
	for _, tc := range []struct {
		a1, a2   float64
		expected float64
	}{
		{0, 0, 0},
		{0, 45, 45},
		{0, 90, 90},
		{0, 180, 180},
		{45, 0, 45},
		{90, 0, 90},
		{180, 0, 180},
		{0, 360, 0},
		{350, 20, 30},
		{20, 350, 30},
	} {
		t.Run(fmt.Sprintf("|%f-%f|=%f", tc.a1, tc.a2, tc.expected), func(t *testing.T) {
			test.That(t, AngleDiffDeg(tc.a1, tc.a2), test.ShouldEqual, tc.expected)
		})
	}
}

func TestAntiCWDeg(t *testing.T) {
	test.That(t, AntiCWDeg(0), test.ShouldEqual, 0)
	test.That(t, AntiCWDeg(360), test.ShouldEqual, 0)
	test.That(t, AntiCWDeg(180), test.ShouldEqual, 180)
	test.That(t, AntiCWDeg(1), test.ShouldEqual, 359)
	test.That(t, AntiCWDeg(90), test.ShouldEqual, 270)
	test.That(t, AntiCWDeg(270), test.ShouldEqual, 90)
	test.That(t, AntiCWDeg(45), test.ShouldEqual, 315)
}

func TestMedian(t *testing.T) {
	for i, tc := range []struct {
		values   []float64
		expected float64
	}{
		{
			[]float64{},
			math.NaN(),
		},
		{
			[]float64{1},
			1,
		},
		{
			[]float64{1, 2, 3},
			2,
		},
		{
			[]float64{3, 2, 1},
			2,
		},
		{
			[]float64{90, 90, 90},
			90,
		},
		{
			[]float64{90, 45, 80},
			80,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			if math.IsNaN(tc.expected) {
				test.That(t, math.IsNaN(Median(tc.values...)), test.ShouldBeTrue)
				return
			}
			test.That(t, Median(tc.values...), test.ShouldAlmostEqual, tc.expected)
		})
	}
}

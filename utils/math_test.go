package utils

import (
	"fmt"
	"math"
	"testing"

	"github.com/edaniels/test"
	"github.com/stretchr/testify/assert"
)

func TestAbs1(t *testing.T) {
	assert.Equal(t, 5, AbsInt(5))
	assert.Equal(t, 5, AbsInt(-5))
	assert.Equal(t, 0, AbsInt(0))

	assert.Equal(t, int64(5), AbsInt64(5))
	assert.Equal(t, int64(5), AbsInt64(-5))
	assert.Equal(t, int64(0), AbsInt64(0))
}

func TestSquare1(t *testing.T) {
	assert.Equal(t, 4.0, Square(2.0))
	assert.Equal(t, 4, SquareInt(2))
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

func TestDegrees(t *testing.T) {
	assert.Equal(t, 0.0, DegToRad(0))
	assert.Equal(t, 0.0, RadToDeg(DegToRad(0)))

	assert.Equal(t, math.Pi, DegToRad(180))
	assert.Equal(t, 180.0, RadToDeg(DegToRad(180)))
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

func TestModAngDeg(t *testing.T) {
	test.That(t, ModAngDeg(0-180), test.ShouldEqual, 180)
	test.That(t, ModAngDeg(360+40), test.ShouldEqual, 40)
	test.That(t, ModAngDeg(180+360), test.ShouldEqual, 180)
	test.That(t, ModAngDeg(1-209), test.ShouldEqual, 152)
	test.That(t, ModAngDeg(90-1), test.ShouldEqual, 89)
	test.That(t, ModAngDeg(270+90), test.ShouldEqual, 0)
	test.That(t, ModAngDeg(45), test.ShouldEqual, 45)
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

func TestMinMax(t *testing.T) {
	assert.Equal(t, 5, MaxInt(0, 5))
	assert.Equal(t, 5, MaxInt(-12, 5))
	assert.Equal(t, 5, MaxInt(5, 4))

	assert.Equal(t, 0, MinInt(0, 5))
	assert.Equal(t, -12, MinInt(-12, 5))
	assert.Equal(t, 4, MinInt(5, 4))

	assert.Equal(t, uint8(5), MaxUint8(0, 5))
	assert.Equal(t, uint8(5), MaxUint8(1, 5))
	assert.Equal(t, uint8(5), MaxUint8(5, 4))

	assert.Equal(t, uint8(0), MinUint8(0, 5))
	assert.Equal(t, uint8(1), MinUint8(1, 5))
	assert.Equal(t, uint8(4), MinUint8(5, 4))
}

func TestScaleByPct(t *testing.T) {
	assert.Equal(t, 0, ScaleByPct(0, 0))
	assert.Equal(t, 0, ScaleByPct(255, 0))
	assert.Equal(t, 255, ScaleByPct(255, 1))
	assert.Equal(t, 127, ScaleByPct(255, .5))
	assert.Equal(t, 0, ScaleByPct(255, -2))
}

func TestRayToUpwardCWCartesian(t *testing.T) {
	tt := func(angle, distance, X, Y float64) {
		x, y := RayToUpwardCWCartesian(angle, 1)
		assert.InDelta(t, X, x, 00001)
		assert.InDelta(t, Y, y, 00001)
	}

	tt(0, 1, 0, 1)
	tt(90, 1, 1, 0)
	tt(180, 1, 0, -1)
	tt(270, 1, -1, 0)

	tt(360, 1, 0, 1)
	tt(90+90, 1, 1, 0)
	tt(360+180, 1, 0, -1)
	tt(360+270, 1, -1, 0)

	tt(45, math.Sqrt(2), 1, 1)
	tt(135, math.Sqrt(2), 1, -1)
	tt(225, math.Sqrt(2), -1, -1)
	tt(315, math.Sqrt(2), -1, 1)
}

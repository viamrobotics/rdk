package utils

import (
	"fmt"
	"math"
	"testing"

	"go.viam.com/test"
)

func TestSwapCompasHeadingHandedness(t *testing.T) {
	type testCase struct {
		a float64
		b float64
	}

	testCases := []testCase{
		{
			a: 0,
			b: 0,
		},
		{
			a: 1,
			b: 359,
		},
		{
			a: 1.5,
			b: 358.5,
		},
		{
			a: 3,
			b: 357,
		},
		{
			a: 90,
			b: 270,
		},
		{
			a: 180,
			b: 180,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("expected SwapCompasHeadingHandedness(a) ~= b, where a=%f, b=%f", tc.a, tc.b), func(t *testing.T) {
			test.That(t, SwapCompasHeadingHandedness(tc.a), test.ShouldAlmostEqual, tc.b)
		})
		t.Run(fmt.Sprintf("expected SwapCompasHeadingHandedness(b) ~= a, where b=%f, a=%f", tc.b, tc.a), func(t *testing.T) {
			test.That(t, SwapCompasHeadingHandedness(tc.b), test.ShouldAlmostEqual, tc.a)
		})

	}
}

func TestAbs1(t *testing.T) {
	test.That(t, AbsInt(5), test.ShouldEqual, 5)
	test.That(t, AbsInt(-5), test.ShouldEqual, 5)
	test.That(t, AbsInt(0), test.ShouldEqual, 0)

	test.That(t, AbsInt64(5), test.ShouldEqual, int64(5))
	test.That(t, AbsInt64(-5), test.ShouldEqual, int64(5))
	test.That(t, AbsInt64(0), test.ShouldEqual, int64(0))
}

func TestCubeRoot(t *testing.T) {
	test.That(t, CubeRoot(1.0), test.ShouldAlmostEqual, 1.0)
	test.That(t, CubeRoot(8.0), test.ShouldAlmostEqual, 2.0)
}

func TestSquare1(t *testing.T) {
	test.That(t, Square(2.0), test.ShouldEqual, 4.0)
	test.That(t, SquareInt(2), test.ShouldEqual, 4)
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
	test.That(t, DegToRad(0), test.ShouldEqual, 0.0)
	test.That(t, RadToDeg(DegToRad(0)), test.ShouldEqual, 0.0)

	test.That(t, DegToRad(180), test.ShouldEqual, math.Pi)
	test.That(t, RadToDeg(DegToRad(180)), test.ShouldEqual, 180.0)
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
	test.That(t, MaxInt(0, 5), test.ShouldEqual, 5)
	test.That(t, MaxInt(-12, 5), test.ShouldEqual, 5)
	test.That(t, MaxInt(5, 4), test.ShouldEqual, 5)

	test.That(t, MinInt(0, 5), test.ShouldEqual, 0)
	test.That(t, MinInt(-12, 5), test.ShouldEqual, -12)
	test.That(t, MinInt(5, 4), test.ShouldEqual, 4)

	test.That(t, MaxUint8(0, 5), test.ShouldEqual, uint8(5))
	test.That(t, MaxUint8(1, 5), test.ShouldEqual, uint8(5))
	test.That(t, MaxUint8(5, 4), test.ShouldEqual, uint8(5))

	test.That(t, MinUint8(0, 5), test.ShouldEqual, uint8(0))
	test.That(t, MinUint8(1, 5), test.ShouldEqual, uint8(1))
	test.That(t, MinUint8(5, 4), test.ShouldEqual, uint8(4))
}

func TestScaleByPct(t *testing.T) {
	test.That(t, ScaleByPct(0, 0), test.ShouldEqual, 0)
	test.That(t, ScaleByPct(255, 0), test.ShouldEqual, 0)
	test.That(t, ScaleByPct(255, 1), test.ShouldEqual, 255)
	test.That(t, ScaleByPct(255, .5), test.ShouldEqual, 127)
	test.That(t, ScaleByPct(255, -2), test.ShouldEqual, 0)
	test.That(t, ScaleByPct(255, 2), test.ShouldEqual, 255)
}

func TestFloat64AlmostEqual(t *testing.T) {
	test.That(t, Float64AlmostEqual(1, 1.001, 1e-4), test.ShouldBeFalse)
	test.That(t, Float64AlmostEqual(1, 1.001, 1e-2), test.ShouldBeTrue)
}

func TestClamp(t *testing.T) {
	for i, tc := range []struct {
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{
			3,
			1,
			2,
			2,
		},
		{
			1.5,
			1,
			2,
			1.5,
		},
		{
			0.5,
			1,
			2,
			1,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			test.That(t, Clamp(tc.value, tc.min, tc.max), test.ShouldAlmostEqual, tc.expected)
		})
	}
}

func TestSampleNIntegersNormal(t *testing.T) {
	samples1 := SampleNIntegersNormal(256, -7, 8)
	test.That(t, len(samples1), test.ShouldEqual, 256)
	mean1 := 0.
	for _, sample := range samples1 {
		mean1 += float64(sample)
		test.That(t, sample, test.ShouldBeGreaterThanOrEqualTo, -7)
		test.That(t, sample, test.ShouldBeLessThanOrEqualTo, 8)
	}
	mean1 /= float64(len(samples1))
	// mean1 should be approximately 0.5
	test.That(t, mean1, test.ShouldBeLessThanOrEqualTo, 1.6)
	test.That(t, mean1, test.ShouldBeGreaterThanOrEqualTo, -0.6)

	nSample2 := 25000
	samples2 := SampleNIntegersNormal(nSample2, -16, 32)
	test.That(t, len(samples2), test.ShouldEqual, nSample2)
	mean2 := 0.
	// test that distribution is uniform
	counter := make(map[int]int)

	for _, sample := range samples2 {
		mean2 += float64(sample)
		test.That(t, sample, test.ShouldBeGreaterThanOrEqualTo, -16)
		test.That(t, sample, test.ShouldBeLessThanOrEqualTo, 32)
		if _, ok := counter[sample]; !ok {
			counter[sample] = 1
		} else {
			counter[sample]++
		}
	}
	mean2 /= float64(len(samples2))
	nMean := counter[6] + counter[7] + counter[8] + counter[9] + counter[10]
	nBelow := counter[-6] + counter[-5] + counter[-4] + counter[-3] + counter[-2]
	nAbove := counter[15] + counter[16] + counter[17] + counter[18] + counter[19]
	// mean2 should be approximately 8
	test.That(t, mean2, test.ShouldBeLessThanOrEqualTo, 10)
	test.That(t, mean2, test.ShouldBeGreaterThanOrEqualTo, 6)
	// test that bins around mean value have more points than at mean-X and mean+X
	test.That(t, nMean, test.ShouldBeGreaterThanOrEqualTo, nBelow)
	test.That(t, nMean, test.ShouldBeGreaterThanOrEqualTo, nAbove)
}

func TestCycleIntSlice(t *testing.T) {
	s := []int{1, 2, 3, 4, 5, 6, 7}
	res := CycleIntSliceByN(s, 0)
	test.That(t, res, test.ShouldResemble, []int{1, 2, 3, 4, 5, 6, 7})
	res = CycleIntSliceByN(s, 7)
	test.That(t, res, test.ShouldResemble, []int{1, 2, 3, 4, 5, 6, 7})
	res = CycleIntSliceByN(s, 14)
	test.That(t, res, test.ShouldResemble, []int{1, 2, 3, 4, 5, 6, 7})
	res = CycleIntSliceByN(s, 1)
	test.That(t, res, test.ShouldResemble, []int{2, 3, 4, 5, 6, 7, 1})
	res = CycleIntSliceByN(s, 15)
	test.That(t, res, test.ShouldResemble, []int{2, 3, 4, 5, 6, 7, 1})
}

func TestNRegularlySpaced(t *testing.T) {
	n := 13
	vMin, vMax := 0.0, 13.0
	res := SampleNRegularlySpaced(n, vMin, vMax)
	test.That(t, res, test.ShouldResemble, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})

	vMin, vMax = -5.0, 20.0
	res = SampleNRegularlySpaced(n, vMin, vMax)
	test.That(t, res, test.ShouldResemble, []int{-5, -3, -1, 1, 3, 5, 7, 8, 10, 12, 14, 16, 18})

	vMin, vMax = -5.0, 5.0
	res = SampleNRegularlySpaced(n, vMin, vMax)
	test.That(t, res, test.ShouldResemble, []int{-5, -4, -3, -3, -2, -1, 0, 0, 1, 2, 3, 3, 4})

	vMin, vMax = 5.0, -5.0
	test.That(t, func() { SampleNRegularlySpaced(n, vMin, vMax) }, test.ShouldPanic)
}

func TestSampleNIntegersUniform(t *testing.T) {
	samples1 := SampleNIntegersUniform(256, -7, 8)
	test.That(t, len(samples1), test.ShouldEqual, 256)
	for _, sample := range samples1 {
		test.That(t, sample, test.ShouldBeGreaterThanOrEqualTo, -7)
		test.That(t, sample, test.ShouldBeLessThanOrEqualTo, 8)
	}

	samples2 := SampleNIntegersUniform(16, -16, 32)
	test.That(t, len(samples2), test.ShouldEqual, 16)
	for _, sample := range samples2 {
		test.That(t, sample, test.ShouldBeGreaterThanOrEqualTo, -16)
		test.That(t, sample, test.ShouldBeLessThanOrEqualTo, 32)
	}

	samples3 := SampleNIntegersUniform(1000, -4, 16)
	// test number of samples
	test.That(t, len(samples3), test.ShouldEqual, 1000)
	// test that distribution is uniform
	counter := make(map[int]int)
	for _, sample := range samples3 {
		test.That(t, sample, test.ShouldBeGreaterThanOrEqualTo, -4)
		test.That(t, sample, test.ShouldBeLessThanOrEqualTo, 16)
		if _, ok := counter[sample]; !ok {
			counter[sample] = 1
		} else {
			counter[sample]++
		}
	}
	for _, value := range counter {
		// 1000 samples in a range of 20 values - counter should be on average 50
		test.That(t, value, test.ShouldBeGreaterThanOrEqualTo, 10)
		test.That(t, value, test.ShouldBeLessThanOrEqualTo, 68)
	}
}

func TestVarToBytes(t *testing.T) {
	var f32 float32
	var f64 float64
	var u32 uint32
	f32 = -6.2598534e18
	b := BytesFromFloat32LE(f32)
	test.That(t, b, test.ShouldResemble, []byte{0xEF, 0xBE, 0xAD, 0xDE})
	b = BytesFromFloat32BE(f32)
	test.That(t, b, test.ShouldResemble, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	f32 = 6.2598534e18
	b = BytesFromFloat32LE(f32)
	test.That(t, b, test.ShouldResemble, []byte{0xEF, 0xBE, 0xAD, 0x5E})
	b = BytesFromFloat32BE(f32)
	test.That(t, b, test.ShouldResemble, []byte{0x5E, 0xAD, 0xBE, 0xEF})

	f64 = -1.1885958550205170e+148
	b = BytesFromFloat64LE(f64)
	test.That(t, b, test.ShouldResemble,
		[]byte{0x01, 0xEE, 0xFF, 0xC0, 0xEF, 0xBE, 0xAD, 0xDE})
	b = BytesFromFloat64BE(f64)
	test.That(t, b, test.ShouldResemble,
		[]byte{0xDE, 0xAD, 0xBE, 0xEF, 0xC0, 0xFF, 0xEE, 0x01})
	f64 = 1.1885958550205170e+148
	b = BytesFromFloat64LE(f64)
	test.That(t, b, test.ShouldResemble,
		[]byte{0x01, 0xEE, 0xFF, 0xC0, 0xEF, 0xBE, 0xAD, 0x5E})
	b = BytesFromFloat64BE(f64)
	test.That(t, b, test.ShouldResemble,
		[]byte{0x5E, 0xAD, 0xBE, 0xEF, 0xC0, 0xFF, 0xEE, 0x01})

	u32 = 0x12345678

	b = BytesFromUint32BE(u32)
	test.That(t, b, test.ShouldResemble,
		[]byte{0x12, 0x34, 0x56, 0x78})
	b = BytesFromUint32LE(u32)
	test.That(t, b, test.ShouldResemble,
		[]byte{0x78, 0x56, 0x34, 0x12})
}

func TestBytesToVar(t *testing.T) {
	var f32 float32
	var f64 float64
	var u32 uint32
	var i16 int16
	f32 = -6.2598534e18
	v := Float32FromBytesLE([]byte{0xEF, 0xBE, 0xAD, 0xDE})
	test.That(t, v, test.ShouldEqual, f32)
	v = Float32FromBytesBE([]byte{0xDE, 0xAD, 0xBE, 0xEF})
	test.That(t, v, test.ShouldEqual, f32)

	f64 = -1.1885958550205170e+148
	v64 := Float64FromBytesBE([]byte{0xDE, 0xAD, 0xBE, 0xEF, 0xC0, 0xFF, 0xEE, 0x01})
	test.That(t, v64, test.ShouldEqual, f64)
	v64 = Float64FromBytesLE([]byte{0x01, 0xEE, 0xFF, 0xC0, 0xEF, 0xBE, 0xAD, 0xDE})
	test.That(t, v64, test.ShouldEqual, f64)

	u32 = 0x12345678
	vu32 := Uint32FromBytesLE([]byte{0x78, 0x56, 0x34, 0x12})
	test.That(t, vu32, test.ShouldEqual, u32)
	vu32 = Uint32FromBytesBE([]byte{0x12, 0x34, 0x56, 0x78})
	test.That(t, vu32, test.ShouldEqual, u32)

	i16 = 0x1234
	vi16 := Int16FromBytesLE([]byte{0x34, 0x12})
	test.That(t, vi16, test.ShouldEqual, i16)
	vi16 = Int16FromBytesBE([]byte{0x12, 0x34})
	test.That(t, vi16, test.ShouldEqual, i16)
}

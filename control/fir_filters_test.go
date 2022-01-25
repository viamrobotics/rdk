package control

import (
	"testing"

	"go.viam.com/test"
)

func TestFIRFilterMovingAverage(t *testing.T) {
	firFlt := movingAverageFilter{filterSize: 20}
	firFlt.Reset()
	test.That(t, len(firFlt.x), test.ShouldEqual, firFlt.filterSize)
}

func TestFIRFilterSinc(t *testing.T) {
	firFlt := firSinc{smpFreq: 1000, cutOffFreq: 6, order: 10}
	firFlt.Reset()
	test.That(t, len(firFlt.coeffs), test.ShouldEqual, firFlt.order)
	test.That(t, len(firFlt.x), test.ShouldEqual, firFlt.order)
	test.That(t, firFlt.coeffs[0], test.ShouldAlmostEqual, 0.01194252323789503)
	test.That(t, firFlt.coeffs[1], test.ShouldAlmostEqual, 0.011965210333859375)
	test.That(t, firFlt.coeffs[2], test.ShouldAlmostEqual, 0.011982242600545921)
	test.That(t, firFlt.coeffs[3], test.ShouldAlmostEqual, 0.011993605518831918)
	test.That(t, firFlt.coeffs[4], test.ShouldAlmostEqual, 0.011999289401107232)
	test.That(t, firFlt.coeffs[5], test.ShouldAlmostEqual, 0.011999289401107232)
	test.That(t, firFlt.coeffs[6], test.ShouldAlmostEqual, 0.011993605518831918)
	test.That(t, firFlt.coeffs[7], test.ShouldAlmostEqual, 0.011982242600545921)
	test.That(t, firFlt.coeffs[8], test.ShouldAlmostEqual, 0.011965210333859375)
	test.That(t, firFlt.coeffs[9], test.ShouldAlmostEqual, 0.01194252323789503)
}

func TestFIRFilterWindowedSinc(t *testing.T) {
	firFlt := firWindowedSinc{smpFreq: 100, cutOffFreq: 10, kernelSize: 6}
	firFlt.Reset()
	test.That(t, len(firFlt.kernel), test.ShouldEqual, firFlt.kernelSize)
	test.That(t, firFlt.kernel[0], test.ShouldAlmostEqual, 0.10319743280864632)
	test.That(t, firFlt.kernel[1], test.ShouldAlmostEqual, 0.1547961492129695)
	test.That(t, firFlt.kernel[2], test.ShouldAlmostEqual, 0.19133856308243086)
	test.That(t, firFlt.kernel[3], test.ShouldAlmostEqual, 0.20453314260055294)
	test.That(t, firFlt.kernel[4], test.ShouldAlmostEqual, 0.19133856308243086)
	test.That(t, firFlt.kernel[5], test.ShouldAlmostEqual, 0.1547961492129695)
}

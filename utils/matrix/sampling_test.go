package matrix

import (
	"testing"

	"go.viam.com/test"
)

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
}

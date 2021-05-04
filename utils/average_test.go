package utils

import (
	"testing"

	"github.com/edaniels/test"
)

func TestRolling1(t *testing.T) {
	ra := NewRollingAverage(2)
	ra.Add(5)
	ra.Add(9)
	test.That(t, ra.Average(), test.ShouldEqual, 7)

	ra.Add(11)
	test.That(t, ra.Average(), test.ShouldEqual, 10)

	test.That(t, ra.NumSamples(), test.ShouldEqual, 2)
}

package ams

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestConvertBytesToAngle(t *testing.T) {
	// 180 degrees
	msB := byte(math.Pow(2.0, 7.0))
	lsB := byte(0)
	deg := convertBytesToAngle(msB, lsB)
	test.That(t, deg, test.ShouldEqual, 180.0)

	// 270 degrees
	msB = byte(math.Pow(2.0, 6.0) + math.Pow(2.0, 7.0))
	lsB = byte(0)
	deg = convertBytesToAngle(msB, lsB)
	test.That(t, deg, test.ShouldEqual, 270.0)

	// 219.990234 degrees
	// 10011100011100 in binary, msB = 10011100, lsB = 00011100
	msB = byte(156)
	lsB = byte(28)
	deg = convertBytesToAngle(msB, lsB)
	test.That(t, deg, test.ShouldAlmostEqual, 219.990234, 1e-6)
}

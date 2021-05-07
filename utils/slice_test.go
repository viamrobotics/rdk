package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestRawBytesInt32Slice(t *testing.T) {

	ints := []uint32{0x55555555, 0x55555555}
	b := RawBytesFromSlice(ints)

	test.That(t, len(b), test.ShouldEqual, 8)
	test.That(t, b[0], test.ShouldEqual, 0x55)

	ints[0] = 0
	test.That(t, b[0], test.ShouldEqual, 0x0)

	ints[0] = 0xFFFFFFFF
	test.That(t, b[0], test.ShouldEqual, 0xFF)
	test.That(t, b[4], test.ShouldEqual, 0x55)
}

package utils

import (
	"testing"
	"unsafe"

	"go.viam.com/test"
)

func TestByteSliceFromInt32Slice(t *testing.T) {

	ints := []uint32{0x55555555, 0x55555555}

	b := ByteSliceFromPrimitivePointer(unsafe.Pointer(&ints[0]), 2, 4)

	test.That(t, len(b), test.ShouldEqual, 8)
	test.That(t, b[0], test.ShouldEqual, 0x55)

	ints[0] = 0
	test.That(t, b[0], test.ShouldEqual, 0x0)

	ints[0] = 0xFFFFFFFF
	test.That(t, b[0], test.ShouldEqual, 0xFF)
	test.That(t, b[4], test.ShouldEqual, 0x55)

	ints2 := *(*[]uint32)(ByteSliceToPrimitivePointer(b, 4))
	test.That(t, len(ints2), test.ShouldEqual, len(ints))
	test.That(t, ints2[0], test.ShouldEqual, ints[0])
	test.That(t, ints2[1], test.ShouldEqual, ints[1])
}

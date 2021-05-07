package utils

import (
	"reflect"
	"unsafe"
)

// usage:
// x := []int{ 5, 5}
// foo := ByteSliceFromPrimitivePointer(unsafe.Pointer(&x[0]), 2, 4)
func ByteSliceFromPrimitivePointer(p unsafe.Pointer, lengthOfSlice, sizeOfPrimitive int) []byte {
	size := lengthOfSlice * sizeOfPrimitive
	header := unsafe.Pointer(&reflect.SliceHeader{
		Len:  size,
		Cap:  size,
		Data: uintptr(p),
	})

	return *(*[]byte)(header)
}

// usage:
// b := []byte{ ... }
// foo := *(*[]uint32)(ByteSliceToPrimitivePointer(b, 4))
func ByteSliceToPrimitivePointer(b []byte, sizeOfPrimitive int) unsafe.Pointer {
	header := &reflect.SliceHeader{
		Len:  len(b) / sizeOfPrimitive,
		Cap:  len(b) / sizeOfPrimitive,
		Data: (uintptr)(unsafe.Pointer(&b[0])),
	}
	return unsafe.Pointer(header)
}

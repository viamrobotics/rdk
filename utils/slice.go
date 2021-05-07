package utils

import (
	"fmt"
	"reflect"
	"unsafe"
)

// RawBytesFromSlice returns a view of the given slice value. It is valid
// as long as the given value stays within GC.
func RawBytesFromSlice(val interface{}) []byte {
	valV := reflect.ValueOf(val)
	if valV.Kind() != reflect.Slice {
		panic(fmt.Errorf("expected slice but got %T", val))
	}
	if valV.Len() == 0 {
		return nil
	}

	size := valV.Len() * int(valV.Type().Elem().Size())
	firstElem := valV.Index(0).UnsafeAddr()
	//nolint
	header := &reflect.SliceHeader{
		Len:  size,
		Cap:  size,
		Data: firstElem,
	}
	return *(*[]byte)(unsafe.Pointer(header))
}

package bread

import (
	"bytes"
	"unsafe"
)

// BoolA reads ...
func BoolA(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + BoolFA(ob, b, offset+4, arrayLength)
}

// Int8A reads ...
func Int8A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + Int8FA(ob, b, offset+4, arrayLength)
}

// UInt8A reads ...
func UInt8A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + UInt8FA(ob, b, offset+4, arrayLength)
}

// Int16A reads ...
func Int16A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + Int16FA(ob, b, offset+4, arrayLength)
}

// UInt16A reads ...
func UInt16A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + UInt16FA(ob, b, offset+4, arrayLength)
}

// Int32A reads ...
func Int32A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + Int32FA(ob, b, offset+4, arrayLength)
}

// UInt32A reads ...
func UInt32A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + UInt32FA(ob, b, offset+4, arrayLength)
}

// Int64A reads ...
func Int64A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + Int64FA(ob, b, offset+4, arrayLength)
}

// UInt64A reads ...
func UInt64A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + UInt64FA(ob, b, offset+4, arrayLength)
}

// Float32A reads ...
func Float32A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + Float32FA(ob, b, offset+4, arrayLength)
}

// Float64A reads ...
func Float64A(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + Float64FA(ob, b, offset+4, arrayLength)
}

// TimeA reads ...
func TimeA(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + TimeFA(ob, b, offset+4, arrayLength)
}

// DurationA reads ...
func DurationA(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + DurationFA(ob, b, offset+4, arrayLength)
}

// StringA reads ...
func StringA(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		arrayLength int32
	)
	arrayLength = *(*int32)(unsafe.Pointer(&b[offset]))
	return 4 + StringFA(ob, b, offset+4, arrayLength)
}

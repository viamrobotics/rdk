package bread

import (
	"bytes"
	"math"
	"strconv"
	"unsafe"
)

// Bool reads ...
func Bool(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value uint8
	value = *(*uint8)(unsafe.Pointer(&b[offset]))
	if value == 0 {
		ob.WriteString("false")
	} else {
		ob.WriteString("true")
	}
	return 1
}

// Int8 reads ...
func Int8(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value int8
	value = *(*int8)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatInt(int64(value), 10))
	return 1
}

// UInt8 reads ...
func UInt8(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value uint8
	value = *(*uint8)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatUint(uint64(value), 10))
	return 1
}

// Int16 reads ...
func Int16(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value int16
	value = *(*int16)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatInt(int64(value), 10))
	return 2
}

// UInt16 reads ...
func UInt16(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value uint16
	value = *(*uint16)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatUint(uint64(value), 10))
	return 2
}

// Int32 reads ...
func Int32(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value int32
	value = *(*int32)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatInt(int64(value), 10))
	return 4
}

// UInt32 reads ...
func UInt32(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value uint32
	value = *(*uint32)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatUint(uint64(value), 10))
	return 4
}

// Int64 reads ...
func Int64(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value int64
	value = *(*int64)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatInt(int64(value), 10))
	return 8
}

// UInt64 reads ...
func UInt64(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value uint64
	value = *(*uint64)(unsafe.Pointer(&b[offset]))
	ob.WriteString(strconv.FormatUint(value, 10))
	return 8
}

// Float32 reads ...
func Float32(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value uint32
	value = *(*uint32)(unsafe.Pointer(&b[offset]))
	floatValue := math.Float32frombits(value)
	if math.IsInf(float64(floatValue), 0) || math.IsNaN(float64(floatValue)) {
		floatValue = 0.0
	}
	ob.WriteString(strconv.FormatFloat(float64(floatValue), 'g', -1, 64))
	return 4
}

// Float64 reads ...
func Float64(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var value uint64
	value = *(*uint64)(unsafe.Pointer(&b[offset]))
	floatValue := math.Float64frombits(value)
	if math.IsInf(floatValue, 0) || math.IsNaN(floatValue) {
		floatValue = 0.0
	}
	ob.WriteString(strconv.FormatFloat(floatValue, 'g', -1, 64))
	return 8
}

// Time reads ...
func Time(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	ob.WriteByte('{')
	var (
		size  int32
		secs  int32
		nsecs int32
	)
	secs = *(*int32)(unsafe.Pointer(&b[offset+size]))
	size += 4
	ob.WriteString("\"secs\":")
	ob.WriteString(strconv.FormatInt(int64(secs), 10))
	nsecs = *(*int32)(unsafe.Pointer(&b[offset+size]))
	size += 4
	ob.WriteString(",\"nsecs\":")
	ob.WriteString(strconv.FormatInt(int64(nsecs), 10))
	ob.WriteByte('}')
	return size
}

// Duration reads ...
func Duration(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	ob.WriteByte('{')
	var (
		size  int32
		secs  int32
		nsecs int32
	)
	secs = *(*int32)(unsafe.Pointer(&b[offset+size]))
	size += 4
	ob.WriteString("\"secs\":")
	ob.WriteString(strconv.FormatInt(int64(secs), 10))
	nsecs = *(*int32)(unsafe.Pointer(&b[offset+size]))
	size += 4
	ob.WriteString(",\"nsecs\":")
	ob.WriteString(strconv.FormatInt(int64(nsecs), 10))
	ob.WriteByte('}')
	return size
}

func stripCtlAndExtFromBytes(b []byte) []byte {
	var bl int
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c >= 32 && c < 127 {
			b[bl] = c
			bl++
		}
	}
	return b[:bl]
}

// String reads ...
func String(ob *bytes.Buffer, b []byte, offset int32, _ int32) int32 {
	var (
		stringLength uint32
		size         int32
	)
	stringLength = *(*uint32)(unsafe.Pointer(&b[offset+size]))
	size = size + 4
	// log.Debugf("String length: %v", stringLength)
	stringValue := string(stripCtlAndExtFromBytes(b[offset+size : offset+size+int32(stringLength)]))
	// ob.WriteByte('"')
	ob.WriteString(strconv.Quote(stringValue))
	// ob.WriteByte('"')
	return size + int32(stringLength)
}

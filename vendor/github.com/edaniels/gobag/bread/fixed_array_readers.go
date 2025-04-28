package bread

import (
	"bytes"
)

// BoolFA reads ...
func BoolFA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Bool(ob, b, offset+i, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al
}

// Int8FA reads ...
func Int8FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Int8(ob, b, offset+i, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al
}

// UInt8FA reads ...
func UInt8FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		UInt8(ob, b, offset+i, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al
}

// Int16FA reads ...
func Int16FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Int16(ob, b, offset+i*2, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 2
}

// UInt16FA reads ...
func UInt16FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		UInt16(ob, b, offset+i*2, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 2
}

// Int32FA reads ...
func Int32FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Int32(ob, b, offset+i*4, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 4
}

// UInt32FA reads ...
func UInt32FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		UInt32(ob, b, offset+i*4, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 4
}

// Int64FA reads ...
func Int64FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Int64(ob, b, offset+i*8, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 8
}

// UInt64FA reads ...
func UInt64FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		UInt64(ob, b, offset+i*8, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 8
}

// Float32FA reads ...
func Float32FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Float32(ob, b, offset+i*4, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 4
}

// Float64FA reads ...
func Float64FA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Float64(ob, b, offset+i*8, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 8
}

// TimeFA reads ...
func TimeFA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Time(ob, b, offset+i*8, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 8
}

// DurationFA reads ...
func DurationFA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		Duration(ob, b, offset+i*8, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return al * 8
}

// StringFA reads ...
func StringFA(ob *bytes.Buffer, b []byte, offset int32, al int32) int32 {
	var (
		size int32
	)
	ob.WriteByte('[')
	for i := int32(0); i < al; i++ {
		size = size + String(ob, b, offset+size, 0)
		if i < al-1 {
			ob.WriteByte(',')
		}
	}
	ob.WriteByte(']')
	return size
}

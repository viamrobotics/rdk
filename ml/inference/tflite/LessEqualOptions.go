// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type LessEqualOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsLessEqualOptions(buf []byte, offset flatbuffers.UOffsetT) *LessEqualOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &LessEqualOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsLessEqualOptions(buf []byte, offset flatbuffers.UOffsetT) *LessEqualOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &LessEqualOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *LessEqualOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *LessEqualOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func LessEqualOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(0)
}
func LessEqualOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

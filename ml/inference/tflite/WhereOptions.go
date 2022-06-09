// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type WhereOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsWhereOptions(buf []byte, offset flatbuffers.UOffsetT) *WhereOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &WhereOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsWhereOptions(buf []byte, offset flatbuffers.UOffsetT) *WhereOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &WhereOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *WhereOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *WhereOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func WhereOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(0)
}
func WhereOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

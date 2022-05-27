// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite_metadata

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type ValueRange struct {
	_tab flatbuffers.Table
}

func GetRootAsValueRange(buf []byte, offset flatbuffers.UOffsetT) *ValueRange {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &ValueRange{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsValueRange(buf []byte, offset flatbuffers.UOffsetT) *ValueRange {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &ValueRange{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *ValueRange) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *ValueRange) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *ValueRange) Min() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *ValueRange) MutateMin(n int32) bool {
	return rcv._tab.MutateInt32Slot(4, n)
}

func (rcv *ValueRange) Max() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *ValueRange) MutateMax(n int32) bool {
	return rcv._tab.MutateInt32Slot(6, n)
}

func ValueRangeStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func ValueRangeAddMin(builder *flatbuffers.Builder, min int32) {
	builder.PrependInt32Slot(0, min, 0)
}
func ValueRangeAddMax(builder *flatbuffers.Builder, max int32) {
	builder.PrependInt32Slot(1, max, 0)
}
func ValueRangeEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

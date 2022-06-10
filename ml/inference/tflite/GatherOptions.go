// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type GatherOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsGatherOptions(buf []byte, offset flatbuffers.UOffsetT) *GatherOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &GatherOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsGatherOptions(buf []byte, offset flatbuffers.UOffsetT) *GatherOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &GatherOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *GatherOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *GatherOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *GatherOptions) Axis() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *GatherOptions) MutateAxis(n int32) bool {
	return rcv._tab.MutateInt32Slot(4, n)
}

func (rcv *GatherOptions) BatchDims() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *GatherOptions) MutateBatchDims(n int32) bool {
	return rcv._tab.MutateInt32Slot(6, n)
}

func GatherOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func GatherOptionsAddAxis(builder *flatbuffers.Builder, axis int32) {
	builder.PrependInt32Slot(0, axis, 0)
}
func GatherOptionsAddBatchDims(builder *flatbuffers.Builder, batchDims int32) {
	builder.PrependInt32Slot(1, batchDims, 0)
}
func GatherOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

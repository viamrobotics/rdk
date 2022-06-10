// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type IfOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsIfOptions(buf []byte, offset flatbuffers.UOffsetT) *IfOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &IfOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsIfOptions(buf []byte, offset flatbuffers.UOffsetT) *IfOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &IfOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *IfOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *IfOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *IfOptions) ThenSubgraphIndex() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *IfOptions) MutateThenSubgraphIndex(n int32) bool {
	return rcv._tab.MutateInt32Slot(4, n)
}

func (rcv *IfOptions) ElseSubgraphIndex() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *IfOptions) MutateElseSubgraphIndex(n int32) bool {
	return rcv._tab.MutateInt32Slot(6, n)
}

func IfOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func IfOptionsAddThenSubgraphIndex(builder *flatbuffers.Builder, thenSubgraphIndex int32) {
	builder.PrependInt32Slot(0, thenSubgraphIndex, 0)
}
func IfOptionsAddElseSubgraphIndex(builder *flatbuffers.Builder, elseSubgraphIndex int32) {
	builder.PrependInt32Slot(1, elseSubgraphIndex, 0)
}
func IfOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

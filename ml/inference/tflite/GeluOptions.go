// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type GeluOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsGeluOptions(buf []byte, offset flatbuffers.UOffsetT) *GeluOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &GeluOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsGeluOptions(buf []byte, offset flatbuffers.UOffsetT) *GeluOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &GeluOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *GeluOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *GeluOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *GeluOptions) Approximate() bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetBool(o + rcv._tab.Pos)
	}
	return false
}

func (rcv *GeluOptions) MutateApproximate(n bool) bool {
	return rcv._tab.MutateBoolSlot(4, n)
}

func GeluOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}
func GeluOptionsAddApproximate(builder *flatbuffers.Builder, approximate bool) {
	builder.PrependBoolSlot(0, approximate, false)
}
func GeluOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

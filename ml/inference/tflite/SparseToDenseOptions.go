// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type SparseToDenseOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsSparseToDenseOptions(buf []byte, offset flatbuffers.UOffsetT) *SparseToDenseOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &SparseToDenseOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsSparseToDenseOptions(buf []byte, offset flatbuffers.UOffsetT) *SparseToDenseOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &SparseToDenseOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *SparseToDenseOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *SparseToDenseOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *SparseToDenseOptions) ValidateIndices() bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetBool(o + rcv._tab.Pos)
	}
	return false
}

func (rcv *SparseToDenseOptions) MutateValidateIndices(n bool) bool {
	return rcv._tab.MutateBoolSlot(4, n)
}

func SparseToDenseOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}
func SparseToDenseOptionsAddValidateIndices(builder *flatbuffers.Builder, validateIndices bool) {
	builder.PrependBoolSlot(0, validateIndices, false)
}
func SparseToDenseOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

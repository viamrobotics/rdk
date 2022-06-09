// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type MulOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsMulOptions(buf []byte, offset flatbuffers.UOffsetT) *MulOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &MulOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsMulOptions(buf []byte, offset flatbuffers.UOffsetT) *MulOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &MulOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *MulOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *MulOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *MulOptions) FusedActivationFunction() ActivationFunctionType {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return ActivationFunctionType(rcv._tab.GetInt8(o + rcv._tab.Pos))
	}
	return 0
}

func (rcv *MulOptions) MutateFusedActivationFunction(n ActivationFunctionType) bool {
	return rcv._tab.MutateInt8Slot(4, int8(n))
}

func MulOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}
func MulOptionsAddFusedActivationFunction(builder *flatbuffers.Builder, fusedActivationFunction ActivationFunctionType) {
	builder.PrependInt8Slot(0, int8(fusedActivationFunction), 0)
}
func MulOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type OperatorCode struct {
	_tab flatbuffers.Table
}

func GetRootAsOperatorCode(buf []byte, offset flatbuffers.UOffsetT) *OperatorCode {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &OperatorCode{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsOperatorCode(buf []byte, offset flatbuffers.UOffsetT) *OperatorCode {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &OperatorCode{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *OperatorCode) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *OperatorCode) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *OperatorCode) DeprecatedBuiltinCode() int8 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetInt8(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *OperatorCode) MutateDeprecatedBuiltinCode(n int8) bool {
	return rcv._tab.MutateInt8Slot(4, n)
}

func (rcv *OperatorCode) CustomCode() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *OperatorCode) Version() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 1
}

func (rcv *OperatorCode) MutateVersion(n int32) bool {
	return rcv._tab.MutateInt32Slot(8, n)
}

func (rcv *OperatorCode) BuiltinCode() BuiltinOperator {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return BuiltinOperator(rcv._tab.GetInt32(o + rcv._tab.Pos))
	}
	return 0
}

func (rcv *OperatorCode) MutateBuiltinCode(n BuiltinOperator) bool {
	return rcv._tab.MutateInt32Slot(10, int32(n))
}

func OperatorCodeStart(builder *flatbuffers.Builder) {
	builder.StartObject(4)
}
func OperatorCodeAddDeprecatedBuiltinCode(builder *flatbuffers.Builder, deprecatedBuiltinCode int8) {
	builder.PrependInt8Slot(0, deprecatedBuiltinCode, 0)
}
func OperatorCodeAddCustomCode(builder *flatbuffers.Builder, customCode flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(customCode), 0)
}
func OperatorCodeAddVersion(builder *flatbuffers.Builder, version int32) {
	builder.PrependInt32Slot(2, version, 1)
}
func OperatorCodeAddBuiltinCode(builder *flatbuffers.Builder, builtinCode BuiltinOperator) {
	builder.PrependInt32Slot(3, int32(builtinCode), 0)
}
func OperatorCodeEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type QuantizationParameters struct {
	_tab flatbuffers.Table
}

func GetRootAsQuantizationParameters(buf []byte, offset flatbuffers.UOffsetT) *QuantizationParameters {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &QuantizationParameters{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsQuantizationParameters(buf []byte, offset flatbuffers.UOffsetT) *QuantizationParameters {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &QuantizationParameters{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *QuantizationParameters) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *QuantizationParameters) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *QuantizationParameters) Min(j int) float32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetFloat32(a + flatbuffers.UOffsetT(j*4))
	}
	return 0
}

func (rcv *QuantizationParameters) MinLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *QuantizationParameters) MutateMin(j int, n float32) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateFloat32(a+flatbuffers.UOffsetT(j*4), n)
	}
	return false
}

func (rcv *QuantizationParameters) Max(j int) float32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetFloat32(a + flatbuffers.UOffsetT(j*4))
	}
	return 0
}

func (rcv *QuantizationParameters) MaxLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *QuantizationParameters) MutateMax(j int, n float32) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateFloat32(a+flatbuffers.UOffsetT(j*4), n)
	}
	return false
}

func (rcv *QuantizationParameters) Scale(j int) float32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetFloat32(a + flatbuffers.UOffsetT(j*4))
	}
	return 0
}

func (rcv *QuantizationParameters) ScaleLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *QuantizationParameters) MutateScale(j int, n float32) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateFloat32(a+flatbuffers.UOffsetT(j*4), n)
	}
	return false
}

func (rcv *QuantizationParameters) ZeroPoint(j int) int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetInt64(a + flatbuffers.UOffsetT(j*8))
	}
	return 0
}

func (rcv *QuantizationParameters) ZeroPointLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *QuantizationParameters) MutateZeroPoint(j int, n int64) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateInt64(a+flatbuffers.UOffsetT(j*8), n)
	}
	return false
}

func (rcv *QuantizationParameters) DetailsType() QuantizationDetails {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return QuantizationDetails(rcv._tab.GetByte(o + rcv._tab.Pos))
	}
	return 0
}

func (rcv *QuantizationParameters) MutateDetailsType(n QuantizationDetails) bool {
	return rcv._tab.MutateByteSlot(12, byte(n))
}

func (rcv *QuantizationParameters) Details(obj *flatbuffers.Table) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		rcv._tab.Union(obj, o)
		return true
	}
	return false
}

func (rcv *QuantizationParameters) QuantizedDimension() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *QuantizationParameters) MutateQuantizedDimension(n int32) bool {
	return rcv._tab.MutateInt32Slot(16, n)
}

func QuantizationParametersStart(builder *flatbuffers.Builder) {
	builder.StartObject(7)
}
func QuantizationParametersAddMin(builder *flatbuffers.Builder, min flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(min), 0)
}
func QuantizationParametersStartMinVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func QuantizationParametersAddMax(builder *flatbuffers.Builder, max flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(max), 0)
}
func QuantizationParametersStartMaxVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func QuantizationParametersAddScale(builder *flatbuffers.Builder, scale flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(2, flatbuffers.UOffsetT(scale), 0)
}
func QuantizationParametersStartScaleVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func QuantizationParametersAddZeroPoint(builder *flatbuffers.Builder, zeroPoint flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(3, flatbuffers.UOffsetT(zeroPoint), 0)
}
func QuantizationParametersStartZeroPointVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(8, numElems, 8)
}
func QuantizationParametersAddDetailsType(builder *flatbuffers.Builder, detailsType QuantizationDetails) {
	builder.PrependByteSlot(4, byte(detailsType), 0)
}
func QuantizationParametersAddDetails(builder *flatbuffers.Builder, details flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(5, flatbuffers.UOffsetT(details), 0)
}
func QuantizationParametersAddQuantizedDimension(builder *flatbuffers.Builder, quantizedDimension int32) {
	builder.PrependInt32Slot(6, quantizedDimension, 0)
}
func QuantizationParametersEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type Uint16Vector struct {
	_tab flatbuffers.Table
}

func GetRootAsUint16Vector(buf []byte, offset flatbuffers.UOffsetT) *Uint16Vector {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &Uint16Vector{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsUint16Vector(buf []byte, offset flatbuffers.UOffsetT) *Uint16Vector {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &Uint16Vector{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *Uint16Vector) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *Uint16Vector) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *Uint16Vector) Values(j int) uint16 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetUint16(a + flatbuffers.UOffsetT(j*2))
	}
	return 0
}

func (rcv *Uint16Vector) ValuesLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *Uint16Vector) MutateValues(j int, n uint16) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateUint16(a+flatbuffers.UOffsetT(j*2), n)
	}
	return false
}

func Uint16VectorStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}
func Uint16VectorAddValues(builder *flatbuffers.Builder, values flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(values), 0)
}
func Uint16VectorStartValuesVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(2, numElems, 2)
}
func Uint16VectorEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

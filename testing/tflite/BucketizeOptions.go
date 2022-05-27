// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type BucketizeOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsBucketizeOptions(buf []byte, offset flatbuffers.UOffsetT) *BucketizeOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &BucketizeOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsBucketizeOptions(buf []byte, offset flatbuffers.UOffsetT) *BucketizeOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &BucketizeOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *BucketizeOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *BucketizeOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *BucketizeOptions) Boundaries(j int) float32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetFloat32(a + flatbuffers.UOffsetT(j*4))
	}
	return 0
}

func (rcv *BucketizeOptions) BoundariesLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *BucketizeOptions) MutateBoundaries(j int, n float32) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateFloat32(a+flatbuffers.UOffsetT(j*4), n)
	}
	return false
}

func BucketizeOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}
func BucketizeOptionsAddBoundaries(builder *flatbuffers.Builder, boundaries flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(boundaries), 0)
}
func BucketizeOptionsStartBoundariesVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func BucketizeOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type SpaceToDepthOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsSpaceToDepthOptions(buf []byte, offset flatbuffers.UOffsetT) *SpaceToDepthOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &SpaceToDepthOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsSpaceToDepthOptions(buf []byte, offset flatbuffers.UOffsetT) *SpaceToDepthOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &SpaceToDepthOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *SpaceToDepthOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *SpaceToDepthOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *SpaceToDepthOptions) BlockSize() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *SpaceToDepthOptions) MutateBlockSize(n int32) bool {
	return rcv._tab.MutateInt32Slot(4, n)
}

func SpaceToDepthOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}
func SpaceToDepthOptionsAddBlockSize(builder *flatbuffers.Builder, blockSize int32) {
	builder.PrependInt32Slot(0, blockSize, 0)
}
func SpaceToDepthOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type HashtableImportOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsHashtableImportOptions(buf []byte, offset flatbuffers.UOffsetT) *HashtableImportOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &HashtableImportOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsHashtableImportOptions(buf []byte, offset flatbuffers.UOffsetT) *HashtableImportOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &HashtableImportOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *HashtableImportOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *HashtableImportOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func HashtableImportOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(0)
}
func HashtableImportOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite_metadata

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type TensorMetadataT struct {
	Name            string
	Description     string
	DimensionNames  []string
	Content         *ContentT
	ProcessUnits    []*ProcessUnitT
	Stats           *StatsT
	AssociatedFiles []*AssociatedFileT
}

func (t *TensorMetadataT) Pack(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	if t == nil {
		return 0
	}
	nameOffset := builder.CreateString(t.Name)
	descriptionOffset := builder.CreateString(t.Description)
	dimensionNamesOffset := flatbuffers.UOffsetT(0)
	if t.DimensionNames != nil {
		dimensionNamesLength := len(t.DimensionNames)
		dimensionNamesOffsets := make([]flatbuffers.UOffsetT, dimensionNamesLength)
		for j := 0; j < dimensionNamesLength; j++ {
			dimensionNamesOffsets[j] = builder.CreateString(t.DimensionNames[j])
		}
		TensorMetadataStartDimensionNamesVector(builder, dimensionNamesLength)
		for j := dimensionNamesLength - 1; j >= 0; j-- {
			builder.PrependUOffsetT(dimensionNamesOffsets[j])
		}
		dimensionNamesOffset = builder.EndVector(dimensionNamesLength)
	}
	contentOffset := t.Content.Pack(builder)
	processUnitsOffset := flatbuffers.UOffsetT(0)
	if t.ProcessUnits != nil {
		processUnitsLength := len(t.ProcessUnits)
		processUnitsOffsets := make([]flatbuffers.UOffsetT, processUnitsLength)
		for j := 0; j < processUnitsLength; j++ {
			processUnitsOffsets[j] = t.ProcessUnits[j].Pack(builder)
		}
		TensorMetadataStartProcessUnitsVector(builder, processUnitsLength)
		for j := processUnitsLength - 1; j >= 0; j-- {
			builder.PrependUOffsetT(processUnitsOffsets[j])
		}
		processUnitsOffset = builder.EndVector(processUnitsLength)
	}
	statsOffset := t.Stats.Pack(builder)
	associatedFilesOffset := flatbuffers.UOffsetT(0)
	if t.AssociatedFiles != nil {
		associatedFilesLength := len(t.AssociatedFiles)
		associatedFilesOffsets := make([]flatbuffers.UOffsetT, associatedFilesLength)
		for j := 0; j < associatedFilesLength; j++ {
			associatedFilesOffsets[j] = t.AssociatedFiles[j].Pack(builder)
		}
		TensorMetadataStartAssociatedFilesVector(builder, associatedFilesLength)
		for j := associatedFilesLength - 1; j >= 0; j-- {
			builder.PrependUOffsetT(associatedFilesOffsets[j])
		}
		associatedFilesOffset = builder.EndVector(associatedFilesLength)
	}
	TensorMetadataStart(builder)
	TensorMetadataAddName(builder, nameOffset)
	TensorMetadataAddDescription(builder, descriptionOffset)
	TensorMetadataAddDimensionNames(builder, dimensionNamesOffset)
	TensorMetadataAddContent(builder, contentOffset)
	TensorMetadataAddProcessUnits(builder, processUnitsOffset)
	TensorMetadataAddStats(builder, statsOffset)
	TensorMetadataAddAssociatedFiles(builder, associatedFilesOffset)
	return TensorMetadataEnd(builder)
}

func (rcv *TensorMetadata) UnPackTo(t *TensorMetadataT) {
	t.Name = string(rcv.Name())
	t.Description = string(rcv.Description())
	dimensionNamesLength := rcv.DimensionNamesLength()
	t.DimensionNames = make([]string, dimensionNamesLength)
	for j := 0; j < dimensionNamesLength; j++ {
		t.DimensionNames[j] = string(rcv.DimensionNames(j))
	}
	t.Content = rcv.Content(nil).UnPack()
	processUnitsLength := rcv.ProcessUnitsLength()
	t.ProcessUnits = make([]*ProcessUnitT, processUnitsLength)
	for j := 0; j < processUnitsLength; j++ {
		x := ProcessUnit{}
		rcv.ProcessUnits(&x, j)
		t.ProcessUnits[j] = x.UnPack()
	}
	t.Stats = rcv.Stats(nil).UnPack()
	associatedFilesLength := rcv.AssociatedFilesLength()
	t.AssociatedFiles = make([]*AssociatedFileT, associatedFilesLength)
	for j := 0; j < associatedFilesLength; j++ {
		x := AssociatedFile{}
		rcv.AssociatedFiles(&x, j)
		t.AssociatedFiles[j] = x.UnPack()
	}
}

func (rcv *TensorMetadata) UnPack() *TensorMetadataT {
	if rcv == nil {
		return nil
	}
	t := &TensorMetadataT{}
	rcv.UnPackTo(t)
	return t
}

type TensorMetadata struct {
	_tab flatbuffers.Table
}

func GetRootAsTensorMetadata(buf []byte, offset flatbuffers.UOffsetT) *TensorMetadata {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &TensorMetadata{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsTensorMetadata(buf []byte, offset flatbuffers.UOffsetT) *TensorMetadata {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &TensorMetadata{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *TensorMetadata) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *TensorMetadata) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *TensorMetadata) Name() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *TensorMetadata) Description() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *TensorMetadata) DimensionNames(j int) []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.ByteVector(a + flatbuffers.UOffsetT(j*4))
	}
	return nil
}

func (rcv *TensorMetadata) DimensionNamesLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *TensorMetadata) Content(obj *Content) *Content {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(Content)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

func (rcv *TensorMetadata) ProcessUnits(obj *ProcessUnit, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *TensorMetadata) ProcessUnitsLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *TensorMetadata) Stats(obj *Stats) *Stats {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(Stats)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

func (rcv *TensorMetadata) AssociatedFiles(obj *AssociatedFile, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *TensorMetadata) AssociatedFilesLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func TensorMetadataStart(builder *flatbuffers.Builder) {
	builder.StartObject(7)
}
func TensorMetadataAddName(builder *flatbuffers.Builder, name flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(name), 0)
}
func TensorMetadataAddDescription(builder *flatbuffers.Builder, description flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(description), 0)
}
func TensorMetadataAddDimensionNames(builder *flatbuffers.Builder, dimensionNames flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(2, flatbuffers.UOffsetT(dimensionNames), 0)
}
func TensorMetadataStartDimensionNamesVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func TensorMetadataAddContent(builder *flatbuffers.Builder, content flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(3, flatbuffers.UOffsetT(content), 0)
}
func TensorMetadataAddProcessUnits(builder *flatbuffers.Builder, processUnits flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(4, flatbuffers.UOffsetT(processUnits), 0)
}
func TensorMetadataStartProcessUnitsVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func TensorMetadataAddStats(builder *flatbuffers.Builder, stats flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(5, flatbuffers.UOffsetT(stats), 0)
}
func TensorMetadataAddAssociatedFiles(builder *flatbuffers.Builder, associatedFiles flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(6, flatbuffers.UOffsetT(associatedFiles), 0)
}
func TensorMetadataStartAssociatedFilesVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func TensorMetadataEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
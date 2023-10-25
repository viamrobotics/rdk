// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite_metadata

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type RegexTokenizerOptionsT struct {
	DelimRegexPattern string
	VocabFile         []*AssociatedFileT
}

func (t *RegexTokenizerOptionsT) Pack(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	if t == nil {
		return 0
	}
	delimRegexPatternOffset := builder.CreateString(t.DelimRegexPattern)
	vocabFileOffset := flatbuffers.UOffsetT(0)
	if t.VocabFile != nil {
		vocabFileLength := len(t.VocabFile)
		vocabFileOffsets := make([]flatbuffers.UOffsetT, vocabFileLength)
		for j := 0; j < vocabFileLength; j++ {
			vocabFileOffsets[j] = t.VocabFile[j].Pack(builder)
		}
		RegexTokenizerOptionsStartVocabFileVector(builder, vocabFileLength)
		for j := vocabFileLength - 1; j >= 0; j-- {
			builder.PrependUOffsetT(vocabFileOffsets[j])
		}
		vocabFileOffset = builder.EndVector(vocabFileLength)
	}
	RegexTokenizerOptionsStart(builder)
	RegexTokenizerOptionsAddDelimRegexPattern(builder, delimRegexPatternOffset)
	RegexTokenizerOptionsAddVocabFile(builder, vocabFileOffset)
	return RegexTokenizerOptionsEnd(builder)
}

func (rcv *RegexTokenizerOptions) UnPackTo(t *RegexTokenizerOptionsT) {
	t.DelimRegexPattern = string(rcv.DelimRegexPattern())
	vocabFileLength := rcv.VocabFileLength()
	t.VocabFile = make([]*AssociatedFileT, vocabFileLength)
	for j := 0; j < vocabFileLength; j++ {
		x := AssociatedFile{}
		rcv.VocabFile(&x, j)
		t.VocabFile[j] = x.UnPack()
	}
}

func (rcv *RegexTokenizerOptions) UnPack() *RegexTokenizerOptionsT {
	if rcv == nil {
		return nil
	}
	t := &RegexTokenizerOptionsT{}
	rcv.UnPackTo(t)
	return t
}

type RegexTokenizerOptions struct {
	_tab flatbuffers.Table
}

func GetRootAsRegexTokenizerOptions(buf []byte, offset flatbuffers.UOffsetT) *RegexTokenizerOptions {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &RegexTokenizerOptions{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsRegexTokenizerOptions(buf []byte, offset flatbuffers.UOffsetT) *RegexTokenizerOptions {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &RegexTokenizerOptions{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *RegexTokenizerOptions) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *RegexTokenizerOptions) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *RegexTokenizerOptions) DelimRegexPattern() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *RegexTokenizerOptions) VocabFile(obj *AssociatedFile, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *RegexTokenizerOptions) VocabFileLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func RegexTokenizerOptionsStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func RegexTokenizerOptionsAddDelimRegexPattern(builder *flatbuffers.Builder, delimRegexPattern flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(delimRegexPattern), 0)
}
func RegexTokenizerOptionsAddVocabFile(builder *flatbuffers.Builder, vocabFile flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(vocabFile), 0)
}
func RegexTokenizerOptionsStartVocabFileVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func RegexTokenizerOptionsEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}

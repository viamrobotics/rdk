// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package tflite_metadata

import (
	"strconv"

	flatbuffers "github.com/google/flatbuffers/go"
)

type ContentProperties byte

const (
	ContentPropertiesNONE                  ContentProperties = 0
	ContentPropertiesFeatureProperties     ContentProperties = 1
	ContentPropertiesImageProperties       ContentProperties = 2
	ContentPropertiesBoundingBoxProperties ContentProperties = 3
	ContentPropertiesAudioProperties       ContentProperties = 4
)

var EnumNamesContentProperties = map[ContentProperties]string{
	ContentPropertiesNONE:                  "NONE",
	ContentPropertiesFeatureProperties:     "FeatureProperties",
	ContentPropertiesImageProperties:       "ImageProperties",
	ContentPropertiesBoundingBoxProperties: "BoundingBoxProperties",
	ContentPropertiesAudioProperties:       "AudioProperties",
}

var EnumValuesContentProperties = map[string]ContentProperties{
	"NONE":                  ContentPropertiesNONE,
	"FeatureProperties":     ContentPropertiesFeatureProperties,
	"ImageProperties":       ContentPropertiesImageProperties,
	"BoundingBoxProperties": ContentPropertiesBoundingBoxProperties,
	"AudioProperties":       ContentPropertiesAudioProperties,
}

func (v ContentProperties) String() string {
	if s, ok := EnumNamesContentProperties[v]; ok {
		return s
	}
	return "ContentProperties(" + strconv.FormatInt(int64(v), 10) + ")"
}

type ContentPropertiesT struct {
	Type  ContentProperties
	Value interface{}
}

func (t *ContentPropertiesT) Pack(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	if t == nil {
		return 0
	}
	switch t.Type {
	case ContentPropertiesFeatureProperties:
		return t.Value.(*FeaturePropertiesT).Pack(builder)
	case ContentPropertiesImageProperties:
		return t.Value.(*ImagePropertiesT).Pack(builder)
	case ContentPropertiesBoundingBoxProperties:
		return t.Value.(*BoundingBoxPropertiesT).Pack(builder)
	case ContentPropertiesAudioProperties:
		return t.Value.(*AudioPropertiesT).Pack(builder)
	}
	return 0
}

func (rcv ContentProperties) UnPack(table flatbuffers.Table) *ContentPropertiesT {
	switch rcv {
	case ContentPropertiesFeatureProperties:
		x := FeatureProperties{_tab: table}
		return &ContentPropertiesT{Type: ContentPropertiesFeatureProperties, Value: x.UnPack()}
	case ContentPropertiesImageProperties:
		x := ImageProperties{_tab: table}
		return &ContentPropertiesT{Type: ContentPropertiesImageProperties, Value: x.UnPack()}
	case ContentPropertiesBoundingBoxProperties:
		x := BoundingBoxProperties{_tab: table}
		return &ContentPropertiesT{Type: ContentPropertiesBoundingBoxProperties, Value: x.UnPack()}
	case ContentPropertiesAudioProperties:
		x := AudioProperties{_tab: table}
		return &ContentPropertiesT{Type: ContentPropertiesAudioProperties, Value: x.UnPack()}
	}
	return nil
}

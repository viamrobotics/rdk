package resource

import (
	"testing"
	"time"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestResponseToProto(t *testing.T) {
	ts := time.UnixMilli(12345)
	metadata := ResponseMetadata{CapturedAt: ts, Attributes: map[string]interface{}{"key": "value"}}
	proto := metadata.AsProto()
	test.That(t, proto.CapturedAt.AsTime(), test.ShouldEqual, ts)
	test.That(t, proto.Attributes.Fields["key"].GetStringValue(), test.ShouldEqual, "value")
}

func TestResponseFromProto(t *testing.T) {
	ts := &timestamppb.Timestamp{Seconds: 12, Nanos: 345000000}
	proto := &commonpb.ResponseMetadata{CapturedAt: ts, Attributes: &structpb.Struct{Fields: map[string]*structpb.Value{"key": {Kind: &structpb.Value_StringValue{StringValue: "value"}}}}}
	metadata := ResponseMetadataFromProto(proto)
	test.That(t, metadata.CapturedAt, test.ShouldEqual, time.UnixMilli(12345))
	test.That(t, metadata.Attributes["key"], test.ShouldEqual, "value")
}

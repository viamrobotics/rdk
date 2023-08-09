package resource

import (
	"testing"
	"time"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestResponseToProto(t *testing.T) {
	ts := time.UnixMilli(12345)
	metadata := ResponseMetadata{CapturedAt: ts}
	proto := metadata.AsProto()
	test.That(t, proto.CapturedAt.AsTime(), test.ShouldEqual, ts)
}

func TestResponseFromProto(t *testing.T) {
	ts := &timestamppb.Timestamp{Seconds: 12, Nanos: 345000000}
	proto := &commonpb.ResponseMetadata{CapturedAt: ts}
	metadata := ResponseMetadataFromProto(proto)
	test.That(t, metadata.CapturedAt, test.ShouldEqual, time.UnixMilli(12345))
}

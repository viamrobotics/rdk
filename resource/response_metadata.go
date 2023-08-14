package resource

import (
	"time"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ResponseMetadata contains extra info associated with a Resource's standard response.
type ResponseMetadata struct {
	CapturedAt time.Time
}

// AsProto turns the ResponseMetadata struct into a protobuf message.
func (rm ResponseMetadata) AsProto() *commonpb.ResponseMetadata {
	metadata := &commonpb.ResponseMetadata{}
	metadata.CapturedAt = timestamppb.New(rm.CapturedAt)
	return metadata
}

// ResponseMetadataFromProto turns the protobuf message into a ResponseMetadata struct.
func ResponseMetadataFromProto(proto *commonpb.ResponseMetadata) ResponseMetadata {
	metadata := ResponseMetadata{}
	metadata.CapturedAt = proto.CapturedAt.AsTime()
	return metadata
}

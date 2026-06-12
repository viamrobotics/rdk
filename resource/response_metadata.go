package resource

import (
	"time"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ResponseMetadata contains extra info associated with a Resource's standard response.
type ResponseMetadata struct {
	CapturedAt time.Time
	Attributes map[string]interface{}
}

// AsProto turns the ResponseMetadata struct into a protobuf message.
func (rm ResponseMetadata) AsProto() *commonpb.ResponseMetadata {
	metadata := &commonpb.ResponseMetadata{}
	metadata.CapturedAt = timestamppb.New(rm.CapturedAt)
	attributes := make(map[string]*structpb.Value)
	for k, v := range rm.Attributes {
		value, err := structpb.NewValue(v)
		if err != nil {
			continue
		}
		attributes[k] = value
	}
	metadata.Attributes = &structpb.Struct{Fields: attributes}
	return metadata
}

// ResponseMetadataFromProto turns the protobuf message into a ResponseMetadata struct.
func ResponseMetadataFromProto(proto *commonpb.ResponseMetadata) ResponseMetadata {
	metadata := ResponseMetadata{}
	metadata.CapturedAt = proto.CapturedAt.AsTime()
	metadata.Attributes = proto.Attributes.AsMap()
	return metadata
}

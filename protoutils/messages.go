// Package protoutils are a collection of util methods for using proto in rdk
package protoutils

import (
	"context"
	"strconv"

	"github.com/golang/geo/r3"
	protov1 "github.com/golang/protobuf/proto" //nolint:staticcheck
	commonpb "go.viam.com/api/common/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// MetadataFromProto converts a proto GetCloudMetadataResponse to Metadata.
func MetadataFromProto(pbMetadata *robotpb.GetCloudMetadataResponse) cloud.Metadata {
	if pbMetadata == nil {
		return cloud.Metadata{}
	}
	return cloud.Metadata{
		MachinePartID: pbMetadata.MachinePartId,
		MachineID:     pbMetadata.MachineId,
		PrimaryOrgID:  pbMetadata.PrimaryOrgId,
		LocationID:    pbMetadata.LocationId,
	}
}

// MetadataToProto converts a Metadata its proto counterpart.
func MetadataToProto(metadata cloud.Metadata) *robotpb.GetCloudMetadataResponse {
	return &robotpb.GetCloudMetadataResponse{
		// TODO: RSDK-7181 remove RobotPartId
		RobotPartId:   metadata.MachinePartID, // Deprecated: Duplicates MachinePartId,
		MachinePartId: metadata.MachinePartID,
		MachineId:     metadata.MachineID,
		PrimaryOrgId:  metadata.PrimaryOrgID,
		LocationId:    metadata.LocationID,
	}
}

// ResourceNameToProto converts a resource.Name to its proto counterpart.
func ResourceNameToProto(name resource.Name) *commonpb.ResourceName {
	return &commonpb.ResourceName{
		Namespace: string(name.API.Type.Namespace),
		Type:      name.API.Type.Name,
		Subtype:   name.API.SubtypeName,
		Name:      name.ShortName(),
	}
}

// ResourceNameFromProto converts a proto ResourceName to its rdk counterpart.
func ResourceNameFromProto(name *commonpb.ResourceName) resource.Name {
	return resource.NewName(
		resource.APINamespace(name.Namespace).WithType(name.Type).WithSubtype(name.Subtype),
		name.Name,
	)
}

// ConvertVectorProtoToR3 TODO.
func ConvertVectorProtoToR3(v *commonpb.Vector3) r3.Vector {
	if v == nil {
		return r3.Vector{}
	}
	return r3.Vector{X: v.X, Y: v.Y, Z: v.Z}
}

// ConvertVectorR3ToProto TODO.
func ConvertVectorR3ToProto(v r3.Vector) *commonpb.Vector3 {
	return &commonpb.Vector3{X: v.X, Y: v.Y, Z: v.Z}
}

// ConvertOrientationToProto TODO.
func ConvertOrientationToProto(o spatialmath.Orientation) *commonpb.Orientation {
	oo := &commonpb.Orientation{}
	// the orientation structure returned in the response can be nil as well,
	// so we check again before populating the message.
	if o != nil {
		ov := o.OrientationVectorDegrees()
		if ov != nil {
			oo.OX = ov.OX
			oo.OY = ov.OY
			oo.OZ = ov.OZ
			oo.Theta = ov.Theta
		}
	}
	return oo
}

// ConvertProtoToOrientation TODO.
func ConvertProtoToOrientation(o *commonpb.Orientation) spatialmath.Orientation {
	return &spatialmath.OrientationVectorDegrees{
		OX:    o.OX,
		OY:    o.OY,
		OZ:    o.OZ,
		Theta: o.Theta,
	}
}

// ConvertStringToAnyPB takes a string and parses it to an Any pb type.
func ConvertStringToAnyPB(str string) (*anypb.Any, error) {
	var wrappedVal protoreflect.ProtoMessage
	if boolVal, err := strconv.ParseBool(str); err == nil {
		wrappedVal = wrapperspb.Bool(boolVal)
	} else if int64Val, err := strconv.ParseInt(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.Int64(int64Val)
	} else if uint64Val, err := strconv.ParseUint(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.UInt64(uint64Val)
	} else if float64Val, err := strconv.ParseFloat(str, 64); err == nil {
		wrappedVal = wrapperspb.Double(float64Val)
	} else {
		wrappedVal = wrapperspb.String(str)
	}
	anyVal, err := anypb.New(wrappedVal)
	if err != nil {
		return nil, err
	}
	return anyVal, nil
}

// ConvertStringMapToAnyPBMap takes a string map and parses each value to an Any proto type.
func ConvertStringMapToAnyPBMap(params map[string]string) (map[string]*anypb.Any, error) {
	methodParams := map[string]*anypb.Any{}
	for key, paramVal := range params {
		anyVal, err := ConvertStringToAnyPB(paramVal)
		if err != nil {
			return nil, err
		}
		methodParams[key] = anyVal
	}
	return methodParams, nil
}

// ConvertMapToProtoAny converts a map of string keys and interface{} values to a map of string keys and
// *anypb.Any values, where each value in the input map is converted to a protobuf Any type.
func ConvertMapToProtoAny(input map[string]interface{}) (map[string]*anypb.Any, error) {
	protoMap := make(map[string]*anypb.Any)
	for key, value := range input {
		structValue, err := structpb.NewValue(value)
		if err != nil {
			return nil, err
		}
		anyValue, err := anypb.New(structValue)
		if err != nil {
			return nil, err
		}
		protoMap[key] = anyValue
	}
	return protoMap, nil
}

// MessageToProtoV1 converts a message to a protov1.Message. It is
// assumed it is either a proto.Message or a protov1.Message.
func MessageToProtoV1(msg interface{}) protov1.Message {
	switch v := msg.(type) {
	case proto.Message:
		return protov1.MessageV1(v)
	case protov1.Message:
		return v
	}
	return nil
}

// ClientDoCommander is a gRPC client that allows the execution of DoCommand.
type ClientDoCommander interface {
	// DoCommand sends/receives arbitrary commands
	DoCommand(ctx context.Context, in *commonpb.DoCommandRequest,
		opts ...grpc.CallOption) (*commonpb.DoCommandResponse, error)
}

// DoFromResourceClient is a helper to allow DoCommand() calls from any client.
func DoFromResourceClient(ctx context.Context, svc ClientDoCommander, name string,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}
	resp, err := svc.DoCommand(ctx, &commonpb.DoCommandRequest{
		Name:    name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}

// DoFromResourceServer is a helper to allow DoCommand() calls from any server.
func DoFromResourceServer(
	ctx context.Context,
	res resource.Resource,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	resp, err := res.DoCommand(ctx, req.Command.AsMap())
	if err != nil {
		return nil, err
	}
	pbRes, err := protoutils.StructToStructPb(resp)
	if err != nil {
		return nil, err
	}
	return &commonpb.DoCommandResponse{Result: pbRes}, nil
}

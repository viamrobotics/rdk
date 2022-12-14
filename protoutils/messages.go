// Package protoutils are a collection of util methods for using proto in rdk
package protoutils

import (
	"strconv"

	"github.com/golang/geo/r3"
	//nolint:staticcheck
	protov1 "github.com/golang/protobuf/proto"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// ResourceNameToProto converts a resource.Name to its proto counterpart.
func ResourceNameToProto(name resource.Name) *commonpb.ResourceName {
	return &commonpb.ResourceName{
		Namespace: string(name.Namespace),
		Type:      string(name.ResourceType),
		Subtype:   string(name.ResourceSubtype),
		Name:      name.ShortName(),
	}
}

// ResourceNameFromProto converts a proto ResourceName to its rdk counterpart.
func ResourceNameFromProto(name *commonpb.ResourceName) resource.Name {
	return resource.NewName(
		resource.Namespace(name.Namespace),
		resource.TypeName(name.Type),
		resource.SubtypeName(name.Subtype),
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
	if o != nil {
		ov := o.OrientationVectorDegrees()
		oo.OX = ov.OX
		oo.OY = ov.OY
		oo.OZ = ov.OZ
		oo.Theta = ov.Theta
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

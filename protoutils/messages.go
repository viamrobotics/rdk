// protoutils are a collection of util methods for using proto in rdk
package protoutils

import (
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
)

// ResourceNameToProto converts a resource.Name to its proto counterpart.
func ResourceNameToProto(name resource.Name) *commonpb.ResourceName {
	return &commonpb.ResourceName{
		Uuid:      name.UUID,
		Namespace: string(name.Namespace),
		Type:      string(name.ResourceType),
		Subtype:   string(name.ResourceSubtype),
		Name:      name.Name,
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

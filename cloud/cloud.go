// Package cloud contains app-related functionality.
package cloud

import pb "go.viam.com/api/robot/v1"

// Metadata contains app-related information about the robot.
type Metadata struct {
	PrimaryOrgID  string
	LocationID    string
	MachineID     string
	MachinePartID string
}

// ResourceNameToProto converts a resource.Name to its proto counterpart.
func CloudMetadataToProto(metadata Metadata) *pb.GetCloudMetadataResponse {
	return &pb.GetCloudMetadataResponse{
		MachinePartId: metadata.MachinePartID,
			MachineId: metadata.MachineID,
			PrimaryOrgId: metadata.PrimaryOrgID,
			LocationId: metadata.LocationID,
	}
}

// ResourceNameFromProto converts a proto ResourceName to its rdk counterpart.
func CloudMetadataFromProto(pbMetadata *pb.GetCloudMetadataResponse) Metadata {
	return Metadata{
		MachinePartID: pbMetadata.MachinePartId,
			MachineID: pbMetadata.MachineId,
			PrimaryOrgID: pbMetadata.PrimaryOrgId,
			LocationID: pbMetadata.LocationId,
	}
}

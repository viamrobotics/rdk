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

// MetadataToProto converts a Metadata its proto counterpart.
func MetadataToProto(metadata Metadata) *pb.GetCloudMetadataResponse {
	return &pb.GetCloudMetadataResponse{
		MachinePartId: metadata.MachinePartID,
		MachineId:     metadata.MachineID,
		PrimaryOrgId:  metadata.PrimaryOrgID,
		LocationId:    metadata.LocationID,
	}
}

// MetadataFromProto converts a proto GetCloudMetadataResponse to Metadata.
func MetadataFromProto(pbMetadata *pb.GetCloudMetadataResponse) Metadata {
	return Metadata{
		MachinePartID: pbMetadata.MachinePartId,
		MachineID:     pbMetadata.MachineId,
		PrimaryOrgID:  pbMetadata.PrimaryOrgId,
		LocationID:    pbMetadata.LocationId,
	}
}

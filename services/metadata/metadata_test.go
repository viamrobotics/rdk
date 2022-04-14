package metadata_test

import (
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/metadata"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(injectMetadata *inject.Metadata) (pb.MetadataServiceServer, error) {
	subtypeSvcMap := map[resource.Name]interface{}{
		metadata.Name: injectMetadata,
	}

	return metadata.NewServerFromMap(subtypeSvcMap)
}

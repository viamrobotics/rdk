package server_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/metadata"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

var emptyResources = &pb.ResourcesResponse{
	Resources: []*commonpb.ResourceName{},
}

var serverNewResource = resource.NewName(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	arm.SubtypeName,
	"",
)

var serverOneResourceResponse = []*commonpb.ResourceName{
	{
		Namespace: string(serverNewResource.Namespace),
		Type:      string(serverNewResource.ResourceType),
		Subtype:   string(serverNewResource.ResourceSubtype),
		Name:      serverNewResource.Name,
	},
}

// CR erodkin: see if we can still have this only defined in one place
func newServer(injectMetadata *inject.Metadata) (pb.MetadataServiceServer, error) {
	subtypeSvcMap := map[resource.Name]interface{}{
		metadata.Name: injectMetadata,
	}

	subtypeSvc, err := subtype.New(subtypeSvcMap)
	if err != nil {
		return nil, err
	}
	return server.NewMetadataServer(subtypeSvc), nil
}

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		injectMetadata := &inject.Metadata{}
		server, err := newServer(injectMetadata)
		test.That(t, err, test.ShouldBeNil)
		injectMetadata.ResourcesFunc = func() ([]resource.Name, error) {
			return []resource.Name{}, nil
		}
		resourceResp, err := server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectMetadata.ResourcesFunc = func() ([]resource.Name, error) {
			return []resource.Name{serverNewResource}, nil
		}
		resourceResp, err = server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, serverOneResourceResponse)
	})
}

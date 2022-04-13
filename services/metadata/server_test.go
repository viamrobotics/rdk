package metadata_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/resource"
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
		Uuid:      serverNewResource.UUID,
		Namespace: string(serverNewResource.Namespace),
		Type:      string(serverNewResource.ResourceType),
		Subtype:   string(serverNewResource.ResourceSubtype),
		Name:      serverNewResource.Name,
	},
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

package server_test

import (
	"context"
	"testing"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/grpc/metadata/server"
	pb "go.viam.com/core/proto/api/service/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/testutils/inject"

	"go.viam.com/test"
)

func newServer() (pb.MetadataServiceServer, *inject.Metadata) {
	injectMetadata := &inject.Metadata{}
	return server.New(injectMetadata), injectMetadata
}

var emptyResources = &pb.ResourcesResponse{
	Resources: []*pb.ResourceName{},
}

var newResource = resource.NewName(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	arm.SubtypeName,
	"",
)

var oneResourceResponse = []*pb.ResourceName{
	{
		Uuid:      newResource.UUID,
		Namespace: string(newResource.Namespace),
		Type:      string(newResource.ResourceType),
		Subtype:   string(newResource.ResourceSubtype),
		Name:      newResource.Name,
	},
}

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		server, injectMetadata := newServer()
		injectMetadata.AllFunc = func() []resource.Name {
			return []resource.Name{}
		}
		resourceResp, err := server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectMetadata.AllFunc = func() []resource.Name {
			return []resource.Name{newResource}
		}
		resourceResp, err = server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, oneResourceResponse)
	})
}

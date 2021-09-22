package server_test

import (
	"context"
	"testing"

	"go.viam.com/core/grpc/metadata/server"
	"go.viam.com/core/metadata/service"
	pb "go.viam.com/core/proto/api/service/v1"
	"go.viam.com/core/resource"

	"go.viam.com/test"
)

func newServer() (pb.MetadataServiceServer, *service.Service) {
	injectMetadata := service.Service{}
	return server.New(&injectMetadata), &injectMetadata
}

var emptyResources = &pb.ResourcesResponse{
	Resources: []*pb.ResourceName{},
}

var newResource, _ = resource.New(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	resource.ResourceSubtypeArm,
	"",
)

var oneResourceResponse = []*pb.ResourceName{
	{
		Uuid:      newResource.UUID,
		Namespace: newResource.Namespace,
		Type:      newResource.Type,
		Subtype:   newResource.Subtype,
		Name:      newResource.Name,
	},
}

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		server, injectMetadata := newServer()
		resourceResp, err := server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		err = injectMetadata.Add(newResource)
		test.That(t, err, test.ShouldBeNil)
		resourceResp, err = server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, oneResourceResponse)
	})
}

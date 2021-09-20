package server_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	grpcserver "go.viam.com/core/grpc/metadata/server"
	pb "go.viam.com/core/proto/api/service/v1"
	"go.viam.com/core/resources"

	"go.viam.com/test"
)

func newServer() (pb.MetadataServiceServer, *resources.Resources) {
	injectRes := resources.Resources{}
	return grpcserver.New(&injectRes), &injectRes
}

var emptyResources = &pb.ResourcesResponse{
	Resources: []*pb.ResourceName{},
}

var newResource = resources.Resource{
	Uuid:      uuid.NewString(),
	Namespace: resources.ResourceNamespaceCore,
	Type:      resources.ResourceTypeComponent,
	Subtype:   resources.ResourceSubtypeArm,
	Name:      "",
}

var oneResourceResponse = []*pb.ResourceName{
	{
		Uuid:      newResource.Uuid,
		Namespace: newResource.Namespace,
		Type:      newResource.Type,
		Subtype:   newResource.Subtype,
		Name:      newResource.Name,
	},
}

func TestServer(t *testing.T) {
	t.Run("Resources", func(t *testing.T) {
		server, injectRes := newServer()
		resourceResp, err := server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		err = injectRes.AddResource(newResource)
		test.That(t, err, test.ShouldBeNil)
		resourceResp, err = server.Resources(context.Background(), &pb.ResourcesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, oneResourceResponse)
	})
}

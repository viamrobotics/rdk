package server_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/grpc/metadata/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.MetadataServiceServer, *inject.Metadata) {
	injectMetadata := &inject.Metadata{}
	return server.New(injectMetadata), injectMetadata
}

var emptyResources = &pb.ResourcesResponse{
	Resources: []*commonpb.ResourceName{},
}

var newResource = resource.NewName(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	arm.SubtypeName,
	"",
)

var oneResourceResponse = []*commonpb.ResourceName{
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

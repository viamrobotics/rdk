package server_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var emptyResources = &pb.ResourceNamesResponse{
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

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)

		resourceResp, err := server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{serverNewResource} }

		resourceResp, err = server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, serverOneResourceResponse)
	})

	t.Run("Discovery", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)
		key := discovery.Key{arm.Named("arm").ResourceSubtype, "some arm"}
		disc := discovery.Discovery{Key: key, Discovered: struct{}{}}
		discoveries := []discovery.Discovery{disc}
		injectRobot.DiscoverFunc = func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
			return discoveries, nil
		}
		req := &pb.DiscoverRequest{
			Keys: []*pb.Key{
				{Subtype: string(key.SubtypeName), Model: key.Model},
			},
		}

		resp, err := server.Discover(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

		observed := resp.Discovery[0].Discovered.AsMap()
		expected := map[string]interface{}{}
		test.That(t, observed, test.ShouldResemble, expected)
	})
}

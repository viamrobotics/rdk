package client_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/grpc/metadata/client"
	"go.viam.com/rdk/grpc/metadata/server"
	pb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
)

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

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	injectMetadata := &inject.Metadata{}
	pb.RegisterMetadataServiceServer(gServer1, server.New(injectMetadata))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// failing
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.New(cancelCtx, listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	// working
	client, err := client.New(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	injectMetadata.AllFunc = func() []resource.Name {
		return []resource.Name{newResource}
	}
	resource, err := client.Resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resource, test.ShouldResemble, oneResourceResponse)

	err = client.Close()
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectMetadata := &inject.Metadata{}
	pb.RegisterMetadataServiceServer(gServer, server.New(injectMetadata))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := client.New(ctx, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := client.New(ctx, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = client1.Close()
	test.That(t, err, test.ShouldBeNil)
	err = client2.Close()
	test.That(t, err, test.ShouldBeNil)
}

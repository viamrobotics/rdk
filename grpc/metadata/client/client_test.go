package client_test

import (
	"context"
	"net"
	"testing"

	rpcclient "go.viam.com/utils/rpc/client"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/grpc/metadata/client"
	"go.viam.com/core/grpc/metadata/server"
	pb "go.viam.com/core/proto/api/service/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
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
	_, err = client.NewClient(cancelCtx, listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	// working
	client, err := client.NewClient(context.Background(), listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
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

	td := &trackingDialer{Dialer: dialer.NewCachedDialer()}
	ctx := dialer.ContextWithDialer(context.Background(), td)
	client1, err := client.NewClient(ctx, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	client2, err := client.NewClient(ctx, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.dialCalled, test.ShouldEqual, 2)

	err = client1.Close()
	test.That(t, err, test.ShouldBeNil)
	err = client2.Close()
	test.That(t, err, test.ShouldBeNil)
}

type trackingDialer struct {
	dialer.Dialer
	dialCalled int
}

func (td *trackingDialer) Dial(ctx context.Context, target string, opts ...grpc.DialOption) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.Dial(ctx, target, opts...)
}

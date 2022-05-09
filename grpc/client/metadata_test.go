package client_test

// CR erodkin: probably we should delete this module entirely and move relevant testing
// into client_test.go

//import (
//"context"
//"net"
//"testing"

//"github.com/edaniels/golog"
//"go.viam.com/test"
//"go.viam.com/utils"
//"go.viam.com/utils/rpc"
//"google.golang.org/grpc"

//"go.viam.com/rdk/component/arm"
//"go.viam.com/rdk/grpc/client"
//"go.viam.com/rdk/grpc/server"
//commonpb "go.viam.com/rdk/proto/api/common/v1"
//pb "go.viam.com/rdk/proto/api/robot/v1"
//"go.viam.com/rdk/protoutils"
//"go.viam.com/rdk/resource"
//"go.viam.com/rdk/robot/metadata"
//"go.viam.com/rdk/subtype"
//"go.viam.com/rdk/testutils"
//"go.viam.com/rdk/testutils/inject"
//)

//var (
//clientNewResource         = arm.Named("")
//clientOneResourceResponse = []resource.Name{protoutils.ResourceNameFromProto(
//&commonpb.ResourceName{
//Namespace: string(clientNewResource.Namespace),
//Type:      string(clientNewResource.ResourceType),
//Subtype:   string(clientNewResource.ResourceSubtype),
//Name:      clientNewResource.Name,
//},
//)}
//)

//// CR erodkin: see if we can still have this only defined in one place.
//func newServer(injectMetadata *inject.Metadata) (pb.MetadataServiceServer, error) {
//subtypeSvcMap := map[resource.Name]interface{}{
//metadata.Name: injectMetadata,
//}

//subtypeSvc, err := subtype.New(subtypeSvcMap)
//if err != nil {
//return nil, err
//}
//return server.NewMetadataServer(subtypeSvc), nil
//}

//func TestClient(t *testing.T) {
//logger := golog.NewTestLogger(t)
//listener1, err := net.Listen("tcp", "localhost:0")
//test.That(t, err, test.ShouldBeNil)
//test.That(t, err, test.ShouldBeNil)
//gServer1 := grpc.NewServer()

//injectMetadata := &inject.Metadata{}

//metadataServer, err := newServer(injectMetadata)
//test.That(t, err, test.ShouldBeNil)
//pb.RegisterMetadataServiceServer(gServer1, metadataServer)

//go gServer1.Serve(listener1)
//defer gServer1.Stop()

//// failing
//cancelCtx, cancel := context.WithCancel(context.Background())
//cancel()
//_, err = client.NewMetadataClient(cancelCtx, listener1.Addr().String(), logger)
//test.That(t, err, test.ShouldNotBeNil)
//test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

//// working
//client, err := client.NewMetadataClient(context.Background(), listener1.Addr().String(), logger)
//test.That(t, err, test.ShouldBeNil)

//injectMetadata.ResourcesFunc = func() ([]resource.Name, error) {
//return []resource.Name{clientNewResource}, nil
//}
//resource, err := client.Resources(context.Background())
//test.That(t, err, test.ShouldBeNil)
//test.That(t, resource, test.ShouldResemble, clientOneResourceResponse)

//err = utils.TryClose(context.Background(), client)
//test.That(t, err, test.ShouldBeNil)
//}

//func TestClientDialerOption(t *testing.T) {
//logger := golog.NewTestLogger(t)
//listener, err := net.Listen("tcp", "localhost:0")
//test.That(t, err, test.ShouldBeNil)
//gServer := grpc.NewServer()
//injectMetadata := &inject.Metadata{}
//metadataServer, err := newServer(injectMetadata)
//test.That(t, err, test.ShouldBeNil)
//pb.RegisterMetadataServiceServer(gServer, metadataServer)

//go gServer.Serve(listener)
//defer gServer.Stop()

//td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
//ctx := rpc.ContextWithDialer(context.Background(), td)
//client1, err := client.NewMetadataClient(ctx, listener.Addr().String(), logger)
//test.That(t, err, test.ShouldBeNil)
//test.That(t, td.NewConnections, test.ShouldEqual, 3)
//client2, err := client.NewMetadataClient(ctx, listener.Addr().String(), logger)
//test.That(t, err, test.ShouldBeNil)
//test.That(t, td.NewConnections, test.ShouldEqual, 3)

//err = utils.TryClose(context.Background(), client1)
//test.That(t, err, test.ShouldBeNil)
//err = utils.TryClose(context.Background(), client2)
//test.That(t, err, test.ShouldBeNil)
//}

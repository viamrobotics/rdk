package rpc

import (
	"context"
	"net"
	"testing"

	pb "go.viam.com/robotcore/proto/rpc/examples/echo/v1"

	"github.com/edaniels/test"
	"google.golang.org/grpc"
)

func TestCallClientMethodLineJSON(t *testing.T) {
	rpcServer, err := NewServer()
	test.That(t, err, test.ShouldBeNil)

	es := echoServer{}
	err = rpcServer.RegisterServiceServer(
		context.Background(),
		&pb.EchoService_ServiceDesc,
		&es,
		pb.RegisterEchoServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)

	httpListener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	errChan := make(chan error)
	go func() {
		errChan <- rpcServer.Serve(httpListener)
	}()

	conn, err := grpc.DialContext(context.Background(), httpListener.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	test.That(t, err, test.ShouldBeNil)
	client := pb.NewEchoServiceClient(conn)

	resp, err := CallClientMethodLineJSON(context.Background(), client, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, []byte(nil))

	resp, err = CallClientMethodLineJSON(context.Background(), client, []byte(`Echo`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, []byte(`{"message":""}`))

	_, err = CallClientMethodLineJSON(context.Background(), client, []byte(`Echo foo`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error unmarshaling")

	resp, err = CallClientMethodLineJSON(context.Background(), client, []byte(`Echo {}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, []byte(`{"message":""}`))

	resp, err = CallClientMethodLineJSON(context.Background(), client, []byte(`Echo {"message": "hey"}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, []byte(`{"message":"hey"}`))

	es.fail = true
	_, err = CallClientMethodLineJSON(context.Background(), client, []byte(`Echo {}`))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	test.That(t, rpcServer.Stop(), test.ShouldBeNil)
	err = <-errChan
	test.That(t, err, test.ShouldBeNil)
}

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	pb "go.viam.com/robotcore/proto/rpc/examples/echo/v1"

	"github.com/edaniels/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func TestServer(t *testing.T) {
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

	// TODO(erd): replace with signal or waiting on request
	time.Sleep(time.Second)

	// standard grpc
	conn, err := grpc.DialContext(context.Background(), httpListener.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	test.That(t, err, test.ShouldBeNil)
	client := pb.NewEchoServiceClient(conn)

	echoResp, err := client.Echo(context.Background(), &pb.EchoRequest{Message: "hello"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, echoResp.GetMessage(), test.ShouldEqual, "hello")

	es.fail = true
	_, err = client.Echo(context.Background(), &pb.EchoRequest{Message: "hello"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Convert(err).Message(), test.ShouldEqual, "whoops")
	es.fail = false

	// grpc-web
	httpURL := fmt.Sprintf("http://%s/proto.rpc.examples.echo.v1.EchoService/Echo", httpListener.Addr().String())
	grpcWebReq := `AAAAAAYKBGhleSE=`
	httpResp1, err := http.Post(httpURL, "application/grpc-web-text", strings.NewReader(grpcWebReq))
	test.That(t, err, test.ShouldBeNil)
	defer httpResp1.Body.Close()
	test.That(t, httpResp1.StatusCode, test.ShouldEqual, 200)
	rd, err := ioutil.ReadAll(httpResp1.Body)
	test.That(t, err, test.ShouldBeNil)
	// it says hey!
	test.That(t, string(rd), test.ShouldResemble, "AAAAAAYKBGhleSE=gAAAABBncnBjLXN0YXR1czogMA0K")

	es.fail = true
	httpResp1, err = http.Post(httpURL, "application/grpc-web-text", strings.NewReader(grpcWebReq))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, httpResp1.StatusCode, test.ShouldEqual, 200)
	es.fail = false
	rd, err = ioutil.ReadAll(httpResp1.Body)
	test.That(t, err, test.ShouldBeNil)
	// it says hey!
	test.That(t, httpResp1.Header["Grpc-Message"], test.ShouldResemble, []string{"whoops"})
	test.That(t, string(rd), test.ShouldResemble, "")

	// JSON
	httpURL = fmt.Sprintf("http://%s/v1/echo", httpListener.Addr().String())
	httpResp2, err := http.Post(httpURL, "application/json", strings.NewReader(`{"message": "world"}`))
	test.That(t, err, test.ShouldBeNil)
	defer httpResp2.Body.Close()
	test.That(t, httpResp2.StatusCode, test.ShouldEqual, 200)
	dec := json.NewDecoder(httpResp2.Body)
	var echoM map[string]interface{}
	test.That(t, dec.Decode(&echoM), test.ShouldBeNil)
	test.That(t, echoM, test.ShouldResemble, map[string]interface{}{"message": "world"})

	es.fail = true
	httpResp2, err = http.Post(httpURL, "application/json", strings.NewReader(`{"message": "world"}`))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, httpResp2.StatusCode, test.ShouldEqual, 500)
	es.fail = false

	test.That(t, rpcServer.Stop(), test.ShouldBeNil)
	err = <-errChan
	test.That(t, err, test.ShouldBeNil)
}

type echoServer struct {
	pb.UnimplementedEchoServiceServer
	fail bool
}

func (es *echoServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	if es.fail {
		return nil, errors.New("whoops")
	}
	return &pb.EchoResponse{Message: req.Message}, nil
}

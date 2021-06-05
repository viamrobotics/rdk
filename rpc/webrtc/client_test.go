package rpcwebrtc_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"google.golang.org/grpc"

	echopb "go.viam.com/core/proto/rpc/examples/echo/v1"
	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
	echoserver "go.viam.com/core/rpc/examples/echo/server"
	rpcwebrtc "go.viam.com/core/rpc/webrtc"
	"go.viam.com/core/testutils"
)

func TestClientServer(t *testing.T) {
	testutils.SkipUnlessInternet(t)
	logger := golog.NewTestLogger(t)

	signalingServer := rpcwebrtc.NewSignalingServer()

	grpcListener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	grpcServer := grpc.NewServer()
	grpcServer.RegisterService(&webrtcpb.SignalingService_ServiceDesc, signalingServer)

	serveDone := make(chan error)
	go func() {
		serveDone <- grpcServer.Serve(grpcListener)
	}()

	answerer := rpcwebrtc.NewSignalingAnswerer("foo", nil, logger)
	go func() {
		time.Sleep(time.Second)
		answerer.Stop()
	}()
	test.That(t, answerer.Start(), test.ShouldEqual, context.Canceled)

	webrtcServer := rpcwebrtc.NewServer(logger)
	webrtcServer.RegisterService(&echopb.EchoService_ServiceDesc, &echoserver.Server{})

	answerer = rpcwebrtc.NewSignalingAnswerer(grpcListener.Addr().String(), webrtcServer, logger)
	test.That(t, answerer.Start(), test.ShouldBeNil)

	cc, err := rpcwebrtc.Dial(context.Background(), grpcListener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, cc.Close(), test.ShouldBeNil)
	}()

	echoClient := echopb.NewEchoServiceClient(cc)
	resp, err := echoClient.Echo(context.Background(), &echopb.EchoRequest{Message: "hello"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, "hello")

	// big message
	bigZ := strings.Repeat("z", 1<<23)
	resp, err = echoClient.Echo(context.Background(), &echopb.EchoRequest{Message: bigZ})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Message, test.ShouldEqual, bigZ)

	webrtcServer.Stop()
	answerer.Stop()
	grpcServer.Stop()
	test.That(t, <-serveDone, test.ShouldBeNil)
}

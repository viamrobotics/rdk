package rpc

import (
	"context"
	"net"
	"testing"

	pb "go.viam.com/robotcore/proto/rpc/examples/echo/v1"
	"google.golang.org/grpc"

	"github.com/edaniels/test"
)

func TestCachedDialer(t *testing.T) {
	rpcServer1, err := NewServer()
	test.That(t, err, test.ShouldBeNil)
	rpcServer2, err := NewServer()
	test.That(t, err, test.ShouldBeNil)

	es := echoServer{}
	err = rpcServer1.RegisterServiceServer(
		context.Background(),
		&pb.EchoService_ServiceDesc,
		&es,
		pb.RegisterEchoServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)
	err = rpcServer2.RegisterServiceServer(
		context.Background(),
		&pb.EchoService_ServiceDesc,
		&es,
		pb.RegisterEchoServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)

	httpListener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	httpListener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	errChan1 := make(chan error)
	go func() {
		errChan1 <- rpcServer1.Serve(httpListener1)
	}()
	errChan2 := make(chan error)
	go func() {
		errChan2 <- rpcServer2.Serve(httpListener2)
	}()

	dialer := NewCachedDialer()
	conn1, err := dialer.Dial(context.Background(), httpListener1.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	test.That(t, err, test.ShouldBeNil)
	conn2, err := dialer.Dial(context.Background(), httpListener1.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	test.That(t, err, test.ShouldBeNil)
	conn3, err := dialer.Dial(context.Background(), httpListener2.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.(*reffedConn).ClientConn, test.ShouldEqual, conn2.(*reffedConn).ClientConn)
	test.That(t, conn2.(*reffedConn).ClientConn, test.ShouldNotEqual, conn3.(*reffedConn).ClientConn)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
	test.That(t, conn3.Close(), test.ShouldBeNil)

	test.That(t, dialer.Close(), test.ShouldBeNil)

	test.That(t, rpcServer1.Stop(), test.ShouldBeNil)
	test.That(t, rpcServer2.Stop(), test.ShouldBeNil)
	err = <-errChan1
	test.That(t, err, test.ShouldBeNil)
	err = <-errChan2
	test.That(t, err, test.ShouldBeNil)
}

func TestReffedConn(t *testing.T) {
	tracking := &closeReffedConn{}
	wrapper := newRefCountedConnWrapper(tracking)
	conn1 := wrapper.Ref()
	conn2 := wrapper.Ref()
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, tracking.closeCalled, test.ShouldEqual, 0)
	test.That(t, conn2.Close(), test.ShouldBeNil)
	test.That(t, tracking.closeCalled, test.ShouldEqual, 1)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, tracking.closeCalled, test.ShouldEqual, 1)
	test.That(t, conn2.Close(), test.ShouldBeNil)
	test.That(t, tracking.closeCalled, test.ShouldEqual, 1)
}

func TestRefCountedValue(t *testing.T) {
	rcv := NewRefCountedValue(nil)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, rcv.Ref(), test.ShouldBeNil)
	test.That(t, rcv.Ref(), test.ShouldBeNil)
	test.That(t, rcv.Deref(), test.ShouldBeFalse)
	test.That(t, rcv.Deref(), test.ShouldBeTrue)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, func() { rcv.Ref() }, test.ShouldPanic)

	someIntPtr := 5
	rcv = NewRefCountedValue(&someIntPtr)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, rcv.Ref(), test.ShouldEqual, &someIntPtr)
	test.That(t, rcv.Ref(), test.ShouldEqual, &someIntPtr)
	test.That(t, rcv.Deref(), test.ShouldBeFalse)
	test.That(t, rcv.Deref(), test.ShouldBeTrue)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, func() { rcv.Ref() }, test.ShouldPanic)
}

type closeReffedConn struct {
	ClientConn
	closeCalled int
}

func (crc *closeReffedConn) Close() error {
	crc.closeCalled++
	return nil
}

func TestContextDialer(t *testing.T) {
	ctx := context.Background()
	dialer := NewCachedDialer()
	ctx = ContextWithDialer(ctx, dialer)
	dialer2 := ContextDialer(context.Background())
	test.That(t, dialer2, test.ShouldBeNil)
	dialer2 = ContextDialer(ctx)
	test.That(t, dialer2, test.ShouldEqual, dialer)
}

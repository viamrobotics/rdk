package cli

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	http2Preface       = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	http2FrameHeader   = 9
	http2PingFrameType = 0x6
	http2AckFlag       = 0x1
)

// pingCountingServer is a gRPC server that counts the HTTP/2 PING frames its clients send.
type pingCountingServer struct {
	addr  string
	pings atomic.Int64
}

func newPingCountingServer(t *testing.T) *pingCountingServer {
	t.Helper()

	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	server := &pingCountingServer{addr: listener.Addr().String()}
	grpcServer := grpc.NewServer()
	go func() {
		utils.UncheckedError(grpcServer.Serve(&pingCountingListener{Listener: listener, server: server}))
	}()
	t.Cleanup(grpcServer.Stop)

	return server
}

type pingCountingListener struct {
	net.Listener
	server *pingCountingServer
}

func (l *pingCountingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &pingCountingConn{Conn: conn, server: l.server}, nil
}

// pingCountingConn counts PING frames as the server reads them. Reads happen on a single
// goroutine, so only the count itself needs to be safe for the test to read.
type pingCountingConn struct {
	net.Conn
	server       *pingCountingServer
	buf          []byte
	prefaceEaten bool
}

func (c *pingCountingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.countPings(b[:n])
	}
	return n, err
}

func (c *pingCountingConn) countPings(read []byte) {
	c.buf = append(c.buf, read...)
	if !c.prefaceEaten {
		if len(c.buf) < len(http2Preface) {
			return
		}
		c.buf = c.buf[len(http2Preface):]
		c.prefaceEaten = true
	}
	for len(c.buf) >= http2FrameHeader {
		payload := int(c.buf[0])<<16 | int(c.buf[1])<<8 | int(c.buf[2])
		if len(c.buf) < http2FrameHeader+payload {
			return
		}
		// acks answer the server's own pings, bandwidth estimation for example, and say nothing
		// about the client's keepalive.
		if c.buf[3] == http2PingFrameType && c.buf[4]&http2AckFlag == 0 {
			c.server.pings.Add(1)
		}
		c.buf = c.buf[http2FrameHeader+payload:]
	}
}

// TestAppDialerDoesNotPingWhileIdle asserts that a connection to app made through the app dialer
// stops pinging once it has no RPCs in flight, while other connections keep goutils' pings.
func TestAppDialerDoesNotPingWhileIdle(t *testing.T) {
	app := newPingCountingServer(t)
	machine := newPingCountingServer(t)

	dialer := newAppDialer(app.addr)
	defer func() {
		test.That(t, dialer.Close(), test.ShouldBeNil)
	}()
	ctx := rpc.ContextWithDialer(context.Background(), dialer)

	appConn, err := rpc.DialDirectGRPC(ctx, app.addr, nil, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, appConn.Close(), test.ShouldBeNil)
	}()

	err = appConn.Invoke(context.Background(), "/cli.test.v1.TestService/Test", &emptypb.Empty{}, &emptypb.Empty{})
	test.That(t, status.Code(err), test.ShouldEqual, codes.Unimplemented)

	// dialed through the same dialer but not to app, so it keeps pinging while idle.
	machineConn, err := rpc.DialDirectGRPC(ctx, machine.addr, nil, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, machineConn.Close(), test.ShouldBeNil)
	}()

	// wait out goutils' ping interval on the machine connection. The app connection read last
	// before the machine connection was dialed, so its ping would have been sent first had it
	// been permitted.
	testutils.WaitForAssertionWithSleep(t, time.Second, 60, func(tb testing.TB) {
		test.That(tb, machine.pings.Load(), test.ShouldBeGreaterThan, int64(0))
	})
	test.That(t, app.pings.Load(), test.ShouldEqual, int64(0))
}

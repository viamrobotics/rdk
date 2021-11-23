package gantry_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/utils"
	rpcclient "go.viam.com/utils/rpc/client"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/component/gantry"
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/core/grpc"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()

	var (
		gantryPos []float64
	)

	gantry1 := "gantry1"
	pos1 := []float64{1.0, 2.0, 3.0}
	len1 := []float64{2.0, 3.0, 4.0}
	injectGantry := &inject.Gantry{}
	injectGantry.CurrentPositionFunc = func(ctx context.Context) ([]float64, error) {
		return pos1, nil
	}
	injectGantry.MoveToPositionFunc = func(ctx context.Context, pos []float64) error {
		gantryPos = pos
		return nil
	}
	injectGantry.LengthsFunc = func(ctx context.Context) ([]float64, error) {
		return len1, nil
	}

	gantry2 := "gantry2"
	pos2 := []float64{4.0, 5.0, 6.0}
	len2 := []float64{5.0, 6.0, 7.0}
	injectGantry2 := &inject.Gantry{}
	injectGantry2.CurrentPositionFunc = func(ctx context.Context) ([]float64, error) {
		return pos2, nil
	}
	injectGantry2.MoveToPositionFunc = func(ctx context.Context, pos []float64) error {
		gantryPos = pos
		return nil
	}
	injectGantry2.LengthsFunc = func(ctx context.Context) ([]float64, error) {
		return len2, nil
	}

	gantrySvc, err := subtype.New((map[resource.Name]interface{}{gantry.Named(gantry1): injectGantry, gantry.Named(gantry2): injectGantry2}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGantryServiceServer(gServer1, gantry.NewServer(gantrySvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = gantry.NewClient(cancelCtx, gantry1, listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	gantry1Client, err := gantry.NewClient(context.Background(), gantry2, listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("gantry client 1", func(t *testing.T) {
		pos, err := gantry1Client.CurrentPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldResemble, pos2)

		err = gantry1Client.MoveToPosition(context.Background(), pos1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gantryPos, test.ShouldResemble, pos1)

		len, err := gantry1Client.Lengths(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len, test.ShouldResemble, len2)
	})

	t.Run("gantry client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
		test.That(t, err, test.ShouldBeNil)
		gantry1Client2 := gantry.NewClientFromConn(conn, gantry1, logger)
		test.That(t, err, test.ShouldBeNil)
		pos, err := gantry1Client2.CurrentPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldResemble, pos1)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, utils.TryClose(gantry1Client), test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectGantry := &inject.Gantry{}
	gantry1 := "gantry1"

	gantrySvc, err := subtype.New((map[resource.Name]interface{}{gantry.Named(gantry1): injectGantry}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGantryServiceServer(gServer, gantry.NewServer(gantrySvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &trackingDialer{Dialer: dialer.NewCachedDialer()}
	ctx := dialer.ContextWithDialer(context.Background(), td)
	client1, err := gantry.NewClient(ctx, gantry1, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	client2, err := gantry.NewClient(ctx, gantry1, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.dialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(client2)
	test.That(t, err, test.ShouldBeNil)
}

type trackingDialer struct {
	dialer.Dialer
	dialCalled int
}

func (td *trackingDialer) DialDirect(ctx context.Context, target string, opts ...grpc.DialOption) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.DialDirect(ctx, target, opts...)
}

func (td *trackingDialer) DialFunc(proto string, target string, f func() (dialer.ClientConn, error)) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.DialFunc(proto, target, f)
}

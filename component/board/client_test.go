package board_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/board"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	board1 := "board1"

	var (
		mockGPIOSetErr error = nil
	)

	injectBoard := &inject.Board{}
	injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
		return mockGPIOSetErr
	}

	boardSvc, err := subtype.New((map[resource.Name]interface{}{board.Named(board1): injectBoard}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterBoardServiceServer(gServer, board.NewServer(boardSvc))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = board.NewClient(cancelCtx, board1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("Board client 1", func(t *testing.T) {
		// working
		board1Client, err := board.NewClient(context.Background(), board1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)

		err = board1Client.GPIOSet(context.Background(), "0", true)
		test.That(t, err, test.ShouldBeNil)

		// TODO(maximpertsov): add remaining client methods
	})

	t.Run("Board client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		board1Client2 := board.NewClientFromConn(conn, board1, logger)
		test.That(t, err, test.ShouldBeNil)

		err = board1Client2.GPIOSet(context.Background(), "0", true)
		test.That(t, err, test.ShouldBeNil)

		// TODO(maximpertsov): add remaining client methods
	})
}

func TestClientZeroValues(t *testing.T) {
	// TODO
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectBoard := &inject.Board{}
	board1 := "board1"

	boardSvc, err := subtype.New((map[resource.Name]interface{}{board.Named(board1): injectBoard}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterBoardServiceServer(gServer, board.NewServer(boardSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := board.NewClient(ctx, board1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := board.NewClient(ctx, board1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

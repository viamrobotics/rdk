package board_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/board"
	viamgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/board/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func newBoardWithStatus(injectStatus *commonpb.BoardStatus) *inject.Board {
	injectBoard := &inject.Board{}

	injectBoard.StatusFunc = func(ctx context.Context) (*commonpb.BoardStatus, error) {
		return injectStatus, nil
	}

	return injectBoard
}

func setupService(t *testing.T, injectBoard *inject.Board) (net.Listener, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	boardSvc, err := subtype.New(map[resource.Name]interface{}{board.Named(testBoardName): injectBoard})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterBoardServiceServer(gServer, board.NewServer(boardSvc))

	go gServer.Serve(listener)
	return listener, func() { gServer.Stop() }
}

func TestFailingClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectBoard := newBoardWithStatus(&commonpb.BoardStatus{})

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := board.NewClient(cancelCtx, testBoardName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
}

func TestWorkingClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectBoard := newBoardWithStatus(&commonpb.BoardStatus{})

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	testWorkingClient := func(t *testing.T, client board.Board) {
		t.Helper()

		// Status
		injectStatus := &commonpb.BoardStatus{}
		injectBoard.StatusFunc = func(ctx context.Context) (*commonpb.BoardStatus, error) {
			return injectStatus, nil
		}
		respStatus, err := client.Status(context.Background())
		test.That(t, respStatus, test.ShouldResemble, injectStatus)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectBoard.StatusCap()[1:], test.ShouldResemble, []interface{}{})

		injectGPIOPin := &inject.GPIOPin{}
		injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		}

		// Set
		injectGPIOPin.SetFunc = func(ctx context.Context, high bool) error {
			return nil
		}
		onePin, err := client.GPIOPinByName("one")
		test.That(t, err, test.ShouldBeNil)
		err = onePin.Set(context.Background(), true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectGPIOPin.SetCap()[1:], test.ShouldResemble, []interface{}{true})

		// Get
		injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		isHigh, err := onePin.Get(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isHigh, test.ShouldBeTrue)
		test.That(t, injectGPIOPin.GetCap()[1:], test.ShouldResemble, []interface{}{})

		// SetPWM
		injectGPIOPin.SetPWMFunc = func(ctx context.Context, dutyCyclePct float64) error {
			return nil
		}
		err = onePin.SetPWM(context.Background(), 0.03)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectGPIOPin.SetPWMCap()[1:], test.ShouldResemble, []interface{}{0.03})

		// SetPWMFreq
		injectGPIOPin.SetPWMFreqFunc = func(ctx context.Context, freqHz uint) error {
			return nil
		}
		err = onePin.SetPWMFreq(context.Background(), 11233)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectGPIOPin.SetPWMFreqCap()[1:], test.ShouldResemble, []interface{}{uint(11233)})

		// Analog
		injectAnalogReader := &inject.AnalogReader{}
		injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return injectAnalogReader, true
		}
		analog1, ok := injectBoard.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, injectBoard.AnalogReaderByNameCap(), test.ShouldResemble, []interface{}{"analog1"})

		// Analog:Read
		injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
			return 6, nil
		}
		readVal, err := analog1.Read(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readVal, test.ShouldEqual, 6)

		// Digital Interrupt
		injectDigitalInterrupt := &inject.DigitalInterrupt{}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			return injectDigitalInterrupt, true
		}
		digital1, ok := injectBoard.DigitalInterruptByName("digital1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, injectBoard.DigitalInterruptByNameCap(), test.ShouldResemble, []interface{}{"digital1"})

		// Digital Interrupt:Value
		injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
			return 287, nil
		}
		digital1Val, err := digital1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Val, test.ShouldEqual, 287)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	}

	t.Run("New client", func(t *testing.T) {
		client, err := board.NewClient(context.Background(), testBoardName, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
	})

	t.Run("New client from connection", func(t *testing.T) {
		ctx := context.Background()
		conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := board.NewClientFromConn(ctx, conn, testBoardName, logger)
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
	})
}

func TestClientWithStatus(t *testing.T) {
	logger := golog.NewTestLogger(t)

	injectStatus := &commonpb.BoardStatus{
		Analogs: map[string]*commonpb.AnalogStatus{
			"analog1": {},
		},
		DigitalInterrupts: map[string]*commonpb.DigitalInterruptStatus{
			"digital1": {},
		},
	}
	injectBoard := newBoardWithStatus(injectStatus)

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	client, err := board.NewClient(context.Background(), testBoardName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, injectBoard.StatusCap()[1:], test.ShouldResemble, []interface{}{})

	respAnalogReaders := client.AnalogReaderNames()
	test.That(t, respAnalogReaders, test.ShouldResemble, []string{"analog1"})

	respDigitalInterrupts := client.DigitalInterruptNames()
	test.That(t, respDigitalInterrupts, test.ShouldResemble, []string{"digital1"})

	respSPIs := client.SPINames()
	test.That(t, respSPIs, test.ShouldResemble, []string{})

	respI2Cs := client.I2CNames()
	test.That(t, respI2Cs, test.ShouldResemble, []string{})

	err = utils.TryClose(context.Background(), client)
	test.That(t, err, test.ShouldBeNil)
}

func TestClientWithoutStatus(t *testing.T) {
	logger := golog.NewTestLogger(t)

	injectBoard := &inject.Board{}
	injectBoard.StatusFunc = func(ctx context.Context) (*commonpb.BoardStatus, error) {
		return nil, errors.New("no status")
	}

	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	boardSvc, err := subtype.New(map[resource.Name]interface{}{board.Named(testBoardName): injectBoard})
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(board.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, boardSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	rClient := resourceSubtype.RPCClient(context.Background(), conn, testBoardName, logger)
	client, ok := rClient.(board.Board)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, injectBoard.StatusCap()[1:], test.ShouldResemble, []interface{}{})

	test.That(t, func() { client.AnalogReaderNames() }, test.ShouldPanic)
	test.That(t, func() { client.DigitalInterruptNames() }, test.ShouldPanic)
	test.That(t, func() { client.SPINames() }, test.ShouldPanic)
	test.That(t, func() { client.I2CNames() }, test.ShouldPanic)

	err = utils.TryClose(context.Background(), client)
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectBoard := newBoardWithStatus(&commonpb.BoardStatus{})

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := board.NewClient(ctx, testBoardName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := board.NewClient(ctx, testBoardName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

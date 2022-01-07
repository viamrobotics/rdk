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
	pb "go.viam.com/rdk/proto/api/component/v1"
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

func setupService(t *testing.T, name string, injectBoard *inject.Board) (net.Listener, func()) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	boardSvc, err := subtype.New((map[resource.Name]interface{}{board.Named(name): injectBoard}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterBoardServiceServer(gServer, board.NewServer(boardSvc))

	go gServer.Serve(listener)
	return listener, func() { gServer.Stop() }
}

func TestFailingClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	boardName := "board1"
	injectBoard := newBoardWithStatus(&commonpb.BoardStatus{})

	listener, cleanup := setupService(t, boardName, injectBoard)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := board.NewClient(cancelCtx, boardName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
}

func TestWorkingClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	boardName := "board1"
	injectBoard := newBoardWithStatus(&commonpb.BoardStatus{})

	listener, cleanup := setupService(t, boardName, injectBoard)
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
		test.That(t, injectBoard.StatusCap[1:], test.ShouldResemble, []interface{}{})
		injectBoard.StatusCap = []interface{}(nil)

		// GPIOSet
		injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
			return nil
		}
		err = client.GPIOSet(context.Background(), "one", true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectBoard.GPIOSetCap[1:], test.ShouldResemble, []interface{}{"one", true})
		injectBoard.GPIOSetCap = []interface{}(nil)

		// GPIOGet
		injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
			return true, nil
		}
		isHigh, err := client.GPIOGet(context.Background(), "one")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isHigh, test.ShouldBeTrue)
		test.That(t, injectBoard.GPIOGetCap[1:], test.ShouldResemble, []interface{}{"one"})
		injectBoard.GPIOGetCap = []interface{}(nil)

		// PWMSet
		injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
			return nil
		}
		err = client.PWMSet(context.Background(), "one", 7)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectBoard.PWMSetCap[1:], test.ShouldResemble, []interface{}{"one", byte(7)})
		injectBoard.PWMSetCap = []interface{}(nil)

		// PWMSetFreq
		injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
			return nil
		}
		err = client.PWMSetFreq(context.Background(), "one", 11233)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectBoard.PWMSetFreqCap[1:], test.ShouldResemble, []interface{}{"one", uint(11233)})
		injectBoard.PWMSetFreqCap = []interface{}(nil)

		// Analog
		injectAnalogReader := &inject.AnalogReader{}
		injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return injectAnalogReader, true
		}
		analog1, ok := injectBoard.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, injectBoard.AnalogReaderByNameCap, test.ShouldResemble, []interface{}{"analog1"})
		injectBoard.AnalogReaderByNameCap = []interface{}(nil)

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
		test.That(t, injectBoard.DigitalInterruptByNameCap, test.ShouldResemble, []interface{}{"digital1"})
		injectBoard.DigitalInterruptByNameCap = []interface{}(nil)

		// Digital Interrupt:Config
		digitalIntConfig := board.DigitalInterruptConfig{
			Name:    "foo",
			Pin:     "bar",
			Type:    "baz",
			Formula: "baf",
		}
		injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
			return digitalIntConfig, nil
		}
		digital1Config, err := digital1.Config(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Config, test.ShouldResemble, digitalIntConfig)

		// Digital Interrupt:Value
		injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
			return 287, nil
		}
		digital1Val, err := digital1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Val, test.ShouldEqual, 287)

		// Digital Interrupt:Tick
		injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
			return nil
		}
		err = digital1.Tick(context.Background(), true, 44)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectDigitalInterrupt.TickCap()[1:], test.ShouldResemble, []interface{}{true, uint64(44)})

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	}

	t.Run("New client", func(t *testing.T) {
		client, err := board.NewClient(context.Background(), boardName, listener.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
	})

	t.Run("New client from connection", func(t *testing.T) {
		ctx := context.Background()
		conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		client := board.NewClientFromConn(ctx, conn, boardName, logger)
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
	})
}

func TestClientWithStatus(t *testing.T) {
	logger := golog.NewTestLogger(t)

	boardName := "board1"
	injectStatus := &commonpb.BoardStatus{
		Analogs: map[string]*commonpb.AnalogStatus{
			"analog1": {},
		},
		DigitalInterrupts: map[string]*commonpb.DigitalInterruptStatus{
			"digital1": {},
		},
	}
	injectBoard := newBoardWithStatus(injectStatus)

	listener, cleanup := setupService(t, boardName, injectBoard)
	defer cleanup()

	client, err := board.NewClient(context.Background(), boardName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	test.That(t, injectBoard.StatusCap[1:], test.ShouldResemble, []interface{}{})
	injectBoard.StatusCap = []interface{}(nil)

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

	boardName := "board1"
	injectBoard := &inject.Board{}
	injectBoard.StatusFunc = func(ctx context.Context) (*commonpb.BoardStatus, error) {
		return nil, errors.New("no status")
	}

	listener, cleanup := setupService(t, boardName, injectBoard)
	defer cleanup()

	client, err := board.NewClient(context.Background(), boardName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	test.That(t, injectBoard.StatusCap[1:], test.ShouldResemble, []interface{}{})
	injectBoard.StatusCap = []interface{}(nil)

	test.That(t, func() { client.AnalogReaderNames() }, test.ShouldPanic)
	test.That(t, func() { client.DigitalInterruptNames() }, test.ShouldPanic)
	test.That(t, func() { client.SPINames() }, test.ShouldPanic)
	test.That(t, func() { client.I2CNames() }, test.ShouldPanic)

	err = utils.TryClose(context.Background(), client)
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	boardName := "board1"
	injectBoard := newBoardWithStatus(&commonpb.BoardStatus{})

	listener, cleanup := setupService(t, boardName, injectBoard)
	defer cleanup()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := board.NewClient(ctx, boardName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := board.NewClient(ctx, boardName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

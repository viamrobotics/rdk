package board_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func setupService(t *testing.T, injectBoard *inject.Board) (net.Listener, func()) {
	t.Helper()
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	boardSvc, err := subtype.New(map[resource.Name]interface{}{board.Named(testBoardName): injectBoard})
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(board.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, boardSvc)

	generic.RegisterService(rpcServer, boardSvc)

	go rpcServer.Serve(listener)
	return listener, func() { rpcServer.Stop() }
}

func TestFailingClient(t *testing.T) {
	logger := golog.NewTestLogger(t)

	injectBoard := &inject.Board{}

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
}

func TestWorkingClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectBoard := &inject.Board{}

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	testWorkingClient := func(t *testing.T, client board.Board) {
		t.Helper()

		expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
		var actualExtra map[string]interface{}

		// Do
		injectBoard.DoFunc = generic.EchoFunc
		resp, err := client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		// Status
		injectStatus := &commonpb.BoardStatus{}
		injectBoard.StatusFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
			fmt.Println("STATUS FUNC")
			actualExtra = extra
			return injectStatus, nil
		}
		respStatus, err := client.Status(context.Background(), expectedExtra)
		test.That(t, respStatus, test.ShouldResemble, injectStatus)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectBoard.StatusCap()[1:], test.ShouldResemble, []interface{}{})
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		injectGPIOPin := &inject.GPIOPin{}
		injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		}

		// Set
		injectGPIOPin.SetFunc = func(ctx context.Context, high bool, extra map[string]interface{}) error {
			actualExtra = extra
			return nil
		}
		onePin, err := client.GPIOPinByName("one")
		test.That(t, err, test.ShouldBeNil)
		err = onePin.Set(context.Background(), true, expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectGPIOPin.SetCap()[1:], test.ShouldResemble, []interface{}{true})
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// Get
		injectGPIOPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
			actualExtra = extra
			return true, nil
		}
		isHigh, err := onePin.Get(context.Background(), expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isHigh, test.ShouldBeTrue)
		test.That(t, injectGPIOPin.GetCap()[1:], test.ShouldResemble, []interface{}{})
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// SetPWM
		injectGPIOPin.SetPWMFunc = func(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
			actualExtra = extra
			return nil
		}
		err = onePin.SetPWM(context.Background(), 0.03, expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectGPIOPin.SetPWMCap()[1:], test.ShouldResemble, []interface{}{0.03})
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// SetPWMFreq
		injectGPIOPin.SetPWMFreqFunc = func(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
			actualExtra = extra
			return nil
		}
		err = onePin.SetPWMFreq(context.Background(), 11233, expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectGPIOPin.SetPWMFreqCap()[1:], test.ShouldResemble, []interface{}{uint(11233)})
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// Analog
		injectAnalogReader := &inject.AnalogReader{}
		injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return injectAnalogReader, true
		}
		analog1, ok := injectBoard.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, injectBoard.AnalogReaderByNameCap(), test.ShouldResemble, []interface{}{"analog1"})

		// Analog:Read
		injectAnalogReader.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
			actualExtra = extra
			return 6, nil
		}
		readVal, err := analog1.Read(context.Background(), expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readVal, test.ShouldEqual, 6)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// Digital Interrupt
		injectDigitalInterrupt := &inject.DigitalInterrupt{}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			return injectDigitalInterrupt, true
		}
		digital1, ok := injectBoard.DigitalInterruptByName("digital1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, injectBoard.DigitalInterruptByNameCap(), test.ShouldResemble, []interface{}{"digital1"})

		// Digital Interrupt:Value
		injectDigitalInterrupt.ValueFunc = func(ctx context.Context, extra map[string]interface{}) (int64, error) {
			actualExtra = extra
			return 287, nil
		}
		digital1Val, err := digital1.Value(context.Background(), expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Val, test.ShouldEqual, 287)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	}

	t.Run("New client from connection", func(t *testing.T) {
		ctx := context.Background()
		conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := board.NewClientFromConn(ctx, conn, testBoardName, logger)
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
		test.That(t, conn.Close(), test.ShouldBeNil)
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

	injectBoard := &inject.Board{}
	injectBoard.StatusFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
		return injectStatus, nil
	}

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client := board.NewClientFromConn(context.Background(), conn, testBoardName, logger)

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
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestClientWithoutStatus(t *testing.T) {
	logger := golog.NewTestLogger(t)

	injectBoard := &inject.Board{}
	injectBoard.StatusFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
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
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectBoard := &inject.Board{}

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := board.NewClientFromConn(ctx, conn1, testBoardName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := board.NewClientFromConn(ctx, conn2, testBoardName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

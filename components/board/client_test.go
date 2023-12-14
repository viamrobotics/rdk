package board_test

import (
	"context"
	"net"
	"testing"
	"time"

	commonpb "go.viam.com/api/common/v1"
	boardpb "go.viam.com/api/component/board/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/board"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testBoardName    = "board1"
	missingBoardName = "board2"
)

func setupService(t *testing.T, injectBoard *inject.Board) (net.Listener, func()) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	boardSvc, err := resource.NewAPIResourceCollection(board.API, map[resource.Name]board.Board{board.Named(testBoardName): injectBoard})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[board.Board](board.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, boardSvc), test.ShouldBeNil)

	go rpcServer.Serve(listener)
	return listener, func() { rpcServer.Stop() }
}

func TestFailingClient(t *testing.T) {
	logger := logging.NewTestLogger(t)

	injectBoard := &inject.Board{}

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, context.Canceled)
}

func TestWorkingClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectBoard := &inject.Board{}

	injectBoard.StatusFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
		return nil, viamgrpc.UnimplementedError
	}

	listener, cleanup := setupService(t, injectBoard)
	defer cleanup()

	testWorkingClient := func(t *testing.T, client board.Board) {
		t.Helper()

		expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
		var actualExtra map[string]interface{}

		// DoCommand
		injectBoard.DoFunc = testutils.EchoFunc
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		// Status
		injectStatus := &commonpb.BoardStatus{}
		injectBoard.StatusFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
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

		// Analog Reader
		injectAnalogReader := &inject.AnalogReader{}
		injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return injectAnalogReader, true
		}
		analog1, ok := injectBoard.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, injectBoard.AnalogReaderByNameCap(), test.ShouldResemble, []interface{}{"analog1"})

		// Analog Reader:Read
		injectAnalogReader.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
			actualExtra = extra
			return 6, nil
		}
		readVal, err := analog1.Read(context.Background(), expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readVal, test.ShouldEqual, 6)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// write analog
		injectBoard.WriteAnalogFunc = func(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
			actualExtra = extra
			return nil
		}
		err = injectBoard.WriteAnalog(context.Background(), "pin1", 6, expectedExtra)
		test.That(t, err, test.ShouldBeNil)
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

		// SetPowerMode (currently unimplemented in RDK)
		injectBoard.SetPowerModeFunc = func(ctx context.Context, mode boardpb.PowerMode, duration *time.Duration) error {
			return viamgrpc.UnimplementedError
		}
		err = client.SetPowerMode(context.Background(), boardpb.PowerMode_POWER_MODE_OFFLINE_DEEP, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldHaveSameTypeAs, viamgrpc.UnimplementedError)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}

	t.Run("New client from connection", func(t *testing.T) {
		ctx := context.Background()
		conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := board.NewClientFromConn(ctx, conn, "", board.Named(testBoardName), logger)
		test.That(t, err, test.ShouldBeNil)

		testWorkingClient(t, client)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientWithStatus(t *testing.T) {
	logger := logging.NewTestLogger(t)

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
	client, err := board.NewClientFromConn(context.Background(), conn, "", board.Named(testBoardName), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, injectBoard.StatusCap()[1:], test.ShouldResemble, []interface{}{})

	respAnalogReaders := client.AnalogReaderNames()
	test.That(t, respAnalogReaders, test.ShouldResemble, []string{"analog1"})

	respDigitalInterrupts := client.DigitalInterruptNames()
	test.That(t, respDigitalInterrupts, test.ShouldResemble, []string{"digital1"})

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestClientWithoutStatus(t *testing.T) {
	logger := logging.NewTestLogger(t)

	injectBoard := &inject.Board{}

	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	boardSvc, err := resource.NewAPIResourceCollection(board.API, map[resource.Name]board.Board{board.Named(testBoardName): injectBoard})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[board.Board](board.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, boardSvc), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	rClient, err := resourceAPI.RPCClient(context.Background(), conn, "", board.Named(testBoardName), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, injectBoard.StatusCap()[1:], test.ShouldResemble, []interface{}{})

	test.That(t, rClient.AnalogReaderNames(), test.ShouldResemble, []string{})
	test.That(t, rClient.DigitalInterruptNames(), test.ShouldResemble, []string{})

	err = rClient.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

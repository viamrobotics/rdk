package board_test

import (
	"context"
	"net"
	"testing"
	"time"

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
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
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
		injectAnalog := &inject.Analog{}
		injectBoard.AnalogByNameFunc = func(name string) (board.Analog, error) {
			return injectAnalog, nil
		}
		analog1, err := injectBoard.AnalogByName("analog1")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectBoard.AnalogByNameCap(), test.ShouldResemble, []interface{}{"analog1"})

		// Analog: Read
		injectAnalog.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (board.AnalogValue, error) {
			actualExtra = extra
			return board.AnalogValue{Value: 6, Min: 0, Max: 10, StepSize: 0.1}, nil
		}
		analogVal, err := analog1.Read(context.Background(), expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, analogVal.Value, test.ShouldEqual, 6)
		test.That(t, analogVal.Min, test.ShouldEqual, 0)
		test.That(t, analogVal.Max, test.ShouldEqual, 10)
		test.That(t, analogVal.StepSize, test.ShouldEqual, 0.1)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// write analog
		injectAnalog.WriteFunc = func(ctx context.Context, value int, extra map[string]interface{}) error {
			actualExtra = extra
			return nil
		}
		analog2, err := injectBoard.AnalogByName("pin1")
		test.That(t, err, test.ShouldBeNil)
		err = analog2.Write(context.Background(), 6, expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil

		// Digital Interrupt
		injectDigitalInterrupt := &inject.DigitalInterrupt{}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, error) {
			return injectDigitalInterrupt, nil
		}
		digital1, err := injectBoard.DigitalInterruptByName("digital1")
		test.That(t, err, test.ShouldBeNil)
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

		// StreamTicks
		injectBoard.StreamTicksFunc = func(ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick,
			extra map[string]interface{},
		) error {
			actualExtra = extra
			return nil
		}
		err = injectBoard.StreamTicks(context.Background(), []board.DigitalInterrupt{digital1}, make(chan board.Tick), expectedExtra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
		actualExtra = nil
		injectDigitalInterrupt.NameFunc = func() string {
			return "digital1"
		}
		name := digital1.Name()
		test.That(t, name, test.ShouldEqual, "digital1")

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

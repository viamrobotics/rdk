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

	injectAnalogReader := &inject.AnalogReader{}
	injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
		return 6, nil
	}

	injectDigitalInterrupt := &inject.DigitalInterrupt{}
	digitalIntConfig := board.DigitalInterruptConfig{
		Name:    "foo",
		Pin:     "bar",
		Type:    "baz",
		Formula: "baf",
	}
	injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
		return digitalIntConfig, nil
	}
	injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
		return 287, nil
	}
	var capDigitalInterruptHigh bool
	var capDigitalInterruptNanos uint64
	injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
		capDigitalInterruptHigh = high
		capDigitalInterruptNanos = nanos
		return nil
	}

	board1 := "board1"
	injectBoard := &inject.Board{}
	var (
		capBoardName string

		capGPIOSetPin      string
		capGPIOSetHigh     bool
		capGPIOGetPin      string
		capPWMSetPin       string
		capPWMSetDutyCycle byte
		capPWMSetFreqPin   string
		capPWMSetFreqFreq  uint

		capAnalogReaderName     string
		capDigitalInterruptName string
	)

	injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
		capGPIOSetPin = pin
		capGPIOSetHigh = high
		return nil
	}
	injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
		capGPIOGetPin = pin
		return true, nil
	}
	injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
		capPWMSetPin = pin
		capPWMSetDutyCycle = dutyCycle
		return nil
	}
	injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
		capPWMSetFreqPin = pin
		capPWMSetFreqFreq = freq
		return nil
	}
	injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		capAnalogReaderName = name
		return injectAnalogReader, true
	}
	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		capDigitalInterruptName = name
		return injectDigitalInterrupt, true
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

		err = board1Client.GPIOSet(context.Background(), "one", true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capGPIOSetPin, test.ShouldEqual, "one")
		test.That(t, capGPIOSetHigh, test.ShouldBeTrue)

		isHigh, err := board1Client.GPIOGet(context.Background(), "one")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isHigh, test.ShouldBeTrue)
		test.That(t, capGPIOGetPin, test.ShouldEqual, "one")

		err = board1Client.PWMSet(context.Background(), "one", 7)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capPWMSetPin, test.ShouldEqual, "one")
		test.That(t, capPWMSetDutyCycle, test.ShouldEqual, byte(7))

		err = board1Client.PWMSetFreq(context.Background(), "one", 11233)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capPWMSetFreqPin, test.ShouldEqual, "one")
		test.That(t, capPWMSetFreqFreq, test.ShouldEqual, uint(11233))

        // Analogs + Digital Interrupts

		// board3, ok := client.BoardByName("board3")
		// test.That(t, ok, test.ShouldBeTrue)
		// analog1, ok := board3.AnalogReaderByName("analog1")
		analog1, ok := injectBoard.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		readVal, err := analog1.Read(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readVal, test.ShouldEqual, 6)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capAnalogReaderName, test.ShouldEqual, "analog1")

		// digital1, ok := board3.DigitalInterruptByName("digital1")
		digital1, ok := injectBoard.DigitalInterruptByName("digital1")
		test.That(t, ok, test.ShouldBeTrue)
		digital1Config, err := digital1.Config(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Config, test.ShouldResemble, digitalIntConfig)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

		digital1Val, err := digital1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Val, test.ShouldEqual, 287)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

		err = digital1.Tick(context.Background(), true, 44)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capDigitalInterruptHigh, test.ShouldBeTrue)
		test.That(t, capDigitalInterruptNanos, test.ShouldEqual, 44)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

		// TODO(maximpertsov): add remaining client methods
	})

	t.Run("Board client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		board1Client2 := board.NewClientFromConn(conn, board1, logger)
		test.That(t, err, test.ShouldBeNil)

		err = board1Client2.GPIOSet(context.Background(), "one", true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capGPIOSetPin, test.ShouldEqual, "one")
		test.That(t, capGPIOSetHigh, test.ShouldBeTrue)

		isHigh, err := board1Client2.GPIOGet(context.Background(), "one")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isHigh, test.ShouldBeTrue)
		test.That(t, capGPIOGetPin, test.ShouldEqual, "one")

		err = board1Client2.PWMSet(context.Background(), "one", 7)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capPWMSetPin, test.ShouldEqual, "one")
		test.That(t, capPWMSetDutyCycle, test.ShouldEqual, byte(7))

		err = board1Client2.PWMSetFreq(context.Background(), "one", 11233)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capPWMSetFreqPin, test.ShouldEqual, "one")
		test.That(t, capPWMSetFreqFreq, test.ShouldEqual, uint(11233))

        // Analogs + Digital Interrupts
		analog1, ok := injectBoard.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		readVal, err := analog1.Read(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readVal, test.ShouldEqual, 6)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capAnalogReaderName, test.ShouldEqual, "analog1")

		digital1, ok := injectBoard.DigitalInterruptByName("digital1")
		test.That(t, ok, test.ShouldBeTrue)
		digital1Config, err := digital1.Config(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Config, test.ShouldResemble, digitalIntConfig)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

		digital1Val, err := digital1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, digital1Val, test.ShouldEqual, 287)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

		err = digital1.Tick(context.Background(), true, 44)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capDigitalInterruptHigh, test.ShouldBeTrue)
		test.That(t, capDigitalInterruptNanos, test.ShouldEqual, 44)
		test.That(t, capBoardName, test.ShouldEqual, "board3")
		test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

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

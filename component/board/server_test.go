package board_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

const (
	boardName        = "board1"
	invalidBoardName = "board2"
	missingBoardName = "board3"
)

func newServer() (pb.BoardServiceServer, *inject.Board, error) {
	injectBoard := &inject.Board{}
	boards := map[resource.Name]interface{}{
		board.Named(boardName):        injectBoard,
		board.Named(invalidBoardName): "notBoard",
	}
	boardSvc, err := subtype.New(boards)
	if err != nil {
		return nil, nil, err
	}
	return board.NewServer(boardSvc), injectBoard, nil
}

func TestServer(t *testing.T) {
	ctx := context.Background()

	// t.Run("BoardStatus", func(t *testing.T) {
	// 	server, injectRobot, err := newServer()
	// 	test.That(t, err, test.ShouldBeNil)
	//
	// 	var capName string
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
	// 		Name: "board1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, "board1")
	//
	// 	injectBoard := &inject.Board{}
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	//
	// 	err1 := errors.New("whoops")
	// 	status := &pb.BoardStatus{
	// 		Analogs: map[string]*pb.AnalogStatus{
	// 			"analog1": {},
	// 		},
	// 		DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
	// 			"encoder": {},
	// 		},
	// 	}
	// 	injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
	// 		return nil, err1
	// 	}
	// 	_, err = server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
	// 		Name: "board1",
	// 	})
	// 	test.That(t, err, test.ShouldEqual, err1)
	//
	// 	injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
	// 		return status, nil
	// 	}
	// 	resp, err := server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
	// 		Name: "board1",
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, resp.Status, test.ShouldResemble, status)
	// })

	t.Run("BoardGPIOSet", func(t *testing.T) {
		server, injectBoard, err := newServer()
		test.That(t, err, test.ShouldBeNil)

		testWithBadBoards(t, func(name string) (interface{}, error) {
			return server.GPIOSet(ctx, &pb.BoardServiceGPIOSetRequest{Name: name})
		})

		var capArgs []interface{}

		err1 := errors.New("whoops")
		injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
			capArgs = []interface{}{ctx, pin, high}
			return err1
		}
		_, err = server.GPIOSet(ctx, &pb.BoardServiceGPIOSetRequest{
			Name: boardName,
			Pin:  "one",
			High: true,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", true})

		injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
			capArgs = []interface{}{ctx, pin, high}
			return nil
		}
		_, err = server.GPIOSet(ctx, &pb.BoardServiceGPIOSetRequest{
			Name: boardName,
			Pin:  "one",
			High: true,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", true})
	})

	t.Run("BoardGPIOGet", func(t *testing.T) {
		server, injectBoard, err := newServer()
		test.That(t, err, test.ShouldBeNil)

		testWithBadBoards(t, func(name string) (interface{}, error) {
			return server.GPIOGet(ctx, &pb.BoardServiceGPIOGetRequest{Name: name})
		})

		var capArgs []interface{}

		err1 := errors.New("whoops")
		injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
			capArgs = []interface{}{ctx, pin}
			return false, err1
		}
		_, err = server.GPIOGet(ctx, &pb.BoardServiceGPIOGetRequest{
			Name: boardName,
			Pin:  "one",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one"})

		injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
			capArgs = []interface{}{ctx, pin}
			return true, nil
		}
		getResp, err := server.GPIOGet(ctx, &pb.BoardServiceGPIOGetRequest{
			Name: boardName,
			Pin:  "one",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one"})
		test.That(t, getResp.High, test.ShouldBeTrue)
	})

	t.Run("BoardPWMSet", func(t *testing.T) {
		server, injectBoard, err := newServer()
		test.That(t, err, test.ShouldBeNil)

		testWithBadBoards(t, func(name string) (interface{}, error) {
			return server.PWMSet(ctx, &pb.BoardServicePWMSetRequest{Name: name})
		})

		var capArgs []interface{}

		err1 := errors.New("whoops")
		injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
			capArgs = []interface{}{ctx, pin, dutyCycle}
			return err1
		}
		_, err = server.PWMSet(ctx, &pb.BoardServicePWMSetRequest{
			Name:      "board1",
			Pin:       "one",
			DutyCycle: 7,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", byte(7)})

		injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
			capArgs = []interface{}{ctx, pin, dutyCycle}
			return nil
		}
		_, err = server.PWMSet(ctx, &pb.BoardServicePWMSetRequest{
			Name:      "board1",
			Pin:       "one",
			DutyCycle: 7,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", byte(7)})
	})

	t.Run("BoardPWMSetFrequency", func(t *testing.T) {
		server, injectBoard, err := newServer()
		test.That(t, err, test.ShouldBeNil)

		testWithBadBoards(t, func(name string) (interface{}, error) {
			return server.PWMSetFrequency(ctx, &pb.BoardServicePWMSetFrequencyRequest{Name: name})
		})

		var capArgs []interface{}

		err1 := errors.New("whoops")
		injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
			capArgs = []interface{}{ctx, pin, freq}
			return err1
		}
		_, err = server.PWMSetFrequency(ctx, &pb.BoardServicePWMSetFrequencyRequest{
			Name:      "board1",
			Pin:       "one",
			Frequency: 123123,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", uint(123123)})

		injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
			capArgs = []interface{}{ctx, pin, freq}
			return nil
		}
		_, err = server.PWMSetFrequency(ctx, &pb.BoardServicePWMSetFrequencyRequest{
			Name:      "board1",
			Pin:       "one",
			Frequency: 123123,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", uint(123123)})
	})

	// //nolint:dupl
	// t.Run("BoardAnalogReaderRead", func(t *testing.T) {
	// 	server, injectRobot := newServer()
	// 	var capName string
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
	// 		BoardName: "board1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, "board1")
	//
	// 	injectBoard := &inject.Board{}
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
	// 		BoardName:        "board1",
	// 		AnalogReaderName: "analog1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown analog reader")
	// 	test.That(t, capName, test.ShouldEqual, "analog1")
	//
	// 	injectAnalogReader := &inject.AnalogReader{}
	// 	injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
	// 		return injectAnalogReader, true
	// 	}
	//
	// 	var capCtx context.Context
	// 	err1 := errors.New("whoops")
	// 	injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
	// 		capCtx = ctx
	// 		return 0, err1
	// 	}
	// 	ctx := context.Background()
	// 	_, err = server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
	// 		BoardName:        "board1",
	// 		AnalogReaderName: "analog1",
	// 	})
	// 	test.That(t, err, test.ShouldEqual, err1)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	//
	// 	injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
	// 		capCtx = ctx
	// 		return 8, nil
	// 	}
	// 	readResp, err := server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
	// 		BoardName:        "board1",
	// 		AnalogReaderName: "analog1",
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	// 	test.That(t, readResp.Value, test.ShouldEqual, 8)
	// })
	//
	// t.Run("BoardDigitalInterruptConfig", func(t *testing.T) {
	// 	server, injectRobot := newServer()
	// 	var capName string
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
	// 		BoardName: "board1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, "board1")
	//
	// 	injectBoard := &inject.Board{}
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown digital interrupt")
	// 	test.That(t, capName, test.ShouldEqual, "digital1")
	//
	// 	injectDigitalInterrupt := &inject.DigitalInterrupt{}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		return injectDigitalInterrupt, true
	// 	}
	//
	// 	var capCtx context.Context
	// 	err1 := errors.New("whoops")
	// 	injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
	// 		capCtx = ctx
	// 		return board.DigitalInterruptConfig{}, err1
	// 	}
	// 	ctx := context.Background()
	// 	_, err = server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldEqual, err1)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	//
	// 	theConfig := board.DigitalInterruptConfig{
	// 		Name:    "foo",
	// 		Pin:     "bar",
	// 		Type:    "baz",
	// 		Formula: "baf",
	// 	}
	// 	injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
	// 		capCtx = ctx
	// 		return theConfig, nil
	// 	}
	// 	configResp, err := server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	// 	test.That(t, client.DigitalInterruptConfigFromProto(configResp.Config), test.ShouldResemble, theConfig)
	// })
	//
	// //nolint:dupl
	// t.Run("BoardDigitalInterruptValue", func(t *testing.T) {
	// 	server, injectRobot := newServer()
	// 	var capName string
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
	// 		BoardName: "board1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, "board1")
	//
	// 	injectBoard := &inject.Board{}
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown digital interrupt")
	// 	test.That(t, capName, test.ShouldEqual, "digital1")
	//
	// 	injectDigitalInterrupt := &inject.DigitalInterrupt{}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		return injectDigitalInterrupt, true
	// 	}
	//
	// 	var capCtx context.Context
	// 	err1 := errors.New("whoops")
	// 	injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
	// 		capCtx = ctx
	// 		return 0, err1
	// 	}
	// 	ctx := context.Background()
	// 	_, err = server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldEqual, err1)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	//
	// 	injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
	// 		capCtx = ctx
	// 		return 42, nil
	// 	}
	// 	valueResp, err := server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	// 	test.That(t, valueResp.Value, test.ShouldEqual, 42)
	// })
	//
	// t.Run("BoardDigitalInterruptTick", func(t *testing.T) {
	// 	server, injectRobot := newServer()
	// 	var capName string
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
	// 		BoardName: "board1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, "board1")
	//
	// 	injectBoard := &inject.Board{}
	// 	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown digital interrupt")
	// 	test.That(t, capName, test.ShouldEqual, "digital1")
	//
	// 	injectDigitalInterrupt := &inject.DigitalInterrupt{}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		return injectDigitalInterrupt, true
	// 	}
	//
	// 	var capArgs []interface{}
	// 	err1 := errors.New("whoops")
	// 	injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
	// 		capArgs = []interface{}{ctx, high, nanos}
	// 		return err1
	// 	}
	// 	ctx := context.Background()
	// 	_, err = server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 		High:                 true,
	// 		Nanos:                1028,
	// 	})
	// 	test.That(t, err, test.ShouldEqual, err1)
	// 	test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, true, uint64(1028)})
	//
	// 	injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
	// 		capArgs = []interface{}{ctx, high, nanos}
	// 		return nil
	// 	}
	// 	_, err = server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
	// 		BoardName:            "board1",
	// 		DigitalInterruptName: "digital1",
	// 		High:                 true,
	// 		Nanos:                1028,
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, true, uint64(1028)})
	// })
}

func testWithBadBoards(t *testing.T, f func(name string) (interface{}, error)) {
	_, err := f(missingBoardName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Board")

	_, err = f(invalidBoardName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not a Board")
}

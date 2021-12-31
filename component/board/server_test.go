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

var genericError = errors.New("whoops")

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
	t.Run("GPIOSet", func(t *testing.T) {
		server, injectBoard, err := newServer()
		test.That(t, err, test.ShouldBeNil)

		serverMethod := server.GPIOSet
		type request = pb.BoardServiceGPIOSetRequest
		ctx := context.Background()

		_, err = serverMethod(ctx, &request{Name: invalidBoardName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a Board")

		_, err = serverMethod(ctx, &request{Name: missingBoardName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no Board")

		var capArgs []interface{}

		err1 := errors.New("whoops")
		injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
			capArgs = []interface{}{ctx, pin, high}
			return err1
		}
		_, err = serverMethod(ctx, &request{
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
		_, err = serverMethod(ctx, &request{
			Name: boardName,
			Pin:  "one",
			High: true,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", true})
	})

	t.Run("GPIOGet", func(t *testing.T) {
		type request = pb.BoardServiceGPIOGetRequest
		type response = pb.BoardServiceGPIOGetResponse
		ctx := context.Background()

		tests := []struct {
			injectResult bool
			injectErr    error
			req          *request
			expCapArgs   []interface{}
			expResp      *response
			expRespErr   error
		}{
			{
				injectResult: false,
				injectErr:    nil,
				req:          &request{Name: missingBoardName},
				expCapArgs:   []interface{}(nil),
				expResp:      nil,
				expRespErr:   errors.Errorf("no Board with name (%s)", missingBoardName),
			},
			{
				injectResult: false,
				injectErr:    nil,
				req:          &request{Name: invalidBoardName},
				expCapArgs:   []interface{}(nil),
				expResp:      nil,
				expRespErr:   errors.Errorf("resource with name (%s) is not a Board", invalidBoardName),
			},
			{
				injectResult: false,
				injectErr:    genericError,
				req:          &request{Name: boardName, Pin: "one"},
				expCapArgs:   []interface{}{ctx, "one"},
				expResp:      nil,
				expRespErr:   genericError,
			},
			{
				injectResult: true,
				injectErr:    nil,
				req:          &request{Name: boardName, Pin: "one"},
				expCapArgs:   []interface{}{ctx, "one"},
				expResp:      &response{High: true},
				expRespErr:   nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capArgs []interface{}
				injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
					capArgs = []interface{}{ctx, pin}
					return tc.injectResult, tc.injectErr
				}

				resp, err := server.GPIOGet(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
					test.That(t, resp.High, test.ShouldBeTrue)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
		// server, injectBoard, err := newServer()
		// test.That(t, err, test.ShouldBeNil)
		//
		// serverMethod := server.GPIOGet
		// type request = pb.BoardServiceGPIOGetRequest
		// ctx := context.Background()
		//
		// _, err = serverMethod(ctx, &request{Name: invalidBoardName})
		// test.That(t, err, test.ShouldNotBeNil)
		// test.That(t, err.Error(), test.ShouldContainSubstring, "not a Board")
		//
		// _, err = serverMethod(ctx, &request{Name: missingBoardName})
		// test.That(t, err, test.ShouldNotBeNil)
		// test.That(t, err.Error(), test.ShouldContainSubstring, "no Board")
		//
		// var capArgs []interface{}
		//
		// err1 := errors.New("whoops")
		// injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
		// 	capArgs = []interface{}{ctx, pin}
		// 	return false, err1
		// }
		// _, err = serverMethod(ctx, &request{Name: boardName, Pin: "one"})
		// test.That(t, err, test.ShouldEqual, err1)
		// test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one"})
		//
		// injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
		// 	capArgs = []interface{}{ctx, pin}
		// 	return true, nil
		// }
		// getResp, err := serverMethod(ctx, &request{Name: boardName, Pin: "one"})
		// test.That(t, err, test.ShouldBeNil)
		// test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one"})
		// test.That(t, getResp.High, test.ShouldBeTrue)
	})

	t.Run("PWMSet", func(t *testing.T) {
		type request = pb.BoardServicePWMSetRequest
		ctx := context.Background()

		tests := []struct {
			injectErr  error
			req        *request
			expCapArgs []interface{}
			expRespErr error
		}{
			{
				injectErr:  nil,
				req:        &request{Name: missingBoardName},
				expCapArgs: []interface{}(nil),
				expRespErr: errors.Errorf("no Board with name (%s)", missingBoardName),
			},
			{
				injectErr:  nil,
				req:        &request{Name: invalidBoardName},
				expCapArgs: []interface{}(nil),
				expRespErr: errors.Errorf("resource with name (%s) is not a Board", invalidBoardName),
			},
			{
				injectErr:  genericError,
				req:        &request{Name: boardName, Pin: "one", DutyCycle: 7},
				expCapArgs: []interface{}{ctx, "one", byte(7)},
				expRespErr: genericError,
			},
			{
				injectErr:  nil,
				req:        &request{Name: boardName, Pin: "one", DutyCycle: 7},
				expCapArgs: []interface{}{ctx, "one", byte(7)},
				expRespErr: nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capArgs []interface{}
				injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
					capArgs = []interface{}{ctx, pin, dutyCycle}
					return tc.injectErr
				}

				_, err = server.PWMSet(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
	})

	t.Run("PWMSetFrequency", func(t *testing.T) {
		type request = pb.BoardServicePWMSetFrequencyRequest
		ctx := context.Background()

		tests := []struct {
			injectErr  error
			req        *request
			expCapArgs []interface{}
			expRespErr error
		}{
			{
				injectErr:  nil,
				req:        &request{Name: missingBoardName},
				expCapArgs: []interface{}(nil),
				expRespErr: errors.Errorf("no Board with name (%s)", missingBoardName),
			},
			{
				injectErr:  nil,
				req:        &request{Name: invalidBoardName},
				expCapArgs: []interface{}(nil),
				expRespErr: errors.Errorf("resource with name (%s) is not a Board", invalidBoardName),
			},
			{
				injectErr:  genericError,
				req:        &request{Name: boardName, Pin: "one", Frequency: 123123},
				expCapArgs: []interface{}{ctx, "one", uint(123123)},
				expRespErr: genericError,
			},
			{
				injectErr:  nil,
				req:        &request{Name: boardName, Pin: "one", Frequency: 123123},
				expCapArgs: []interface{}{ctx, "one", uint(123123)},
				expRespErr: nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capArgs []interface{}
				injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
					capArgs = []interface{}{ctx, pin, freq}
					return tc.injectErr
				}

				_, err = server.PWMSetFrequency(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
	})

	// //nolint:dupl
	// t.Run("BoardAnalogReaderRead", func(t *testing.T) {
	// 	server, injectBoard, err := newServer()
	// 	test.That(t, err, test.ShouldBeNil)
	//
	// 	var capName string
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
	// 		BoardName: boardName,
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, boardName)
	//
	// 	injectBoard := &inject.Board{}
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
	// 		BoardName:        boardName,
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
	// 		BoardName:        boardName,
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
	// 		BoardName:        boardName,
	// 		AnalogReaderName: "analog1",
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	// 	test.That(t, readResp.Value, test.ShouldEqual, 8)
	// })
	//
	// t.Run("BoardDigitalInterruptConfig", func(t *testing.T) {
	// 	server, injectBoard, err := newServer()
	// 	test.That(t, err, test.ShouldBeNil)
	//
	// 	var capName string
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
	// 		BoardName: boardName,
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, boardName)
	//
	// 	injectBoard := &inject.Board{}
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
	// 		BoardName:            boardName,
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
	// 		BoardName:            boardName,
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
	// 		BoardName:            boardName,
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	// 	test.That(t, client.DigitalInterruptConfigFromProto(configResp.Config), test.ShouldResemble, theConfig)
	// })
	//
	// //nolint:dupl
	// t.Run("BoardDigitalInterruptValue", func(t *testing.T) {
	// 	server, injectBoard, err := newServer()
	// 	test.That(t, err, test.ShouldBeNil)
	//
	// 	var capName string
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
	// 		BoardName: boardName,
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, boardName)
	//
	// 	injectBoard := &inject.Board{}
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
	// 		BoardName:            boardName,
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
	// 		BoardName:            boardName,
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
	// 		BoardName:            boardName,
	// 		DigitalInterruptName: "digital1",
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capCtx, test.ShouldEqual, ctx)
	// 	test.That(t, valueResp.Value, test.ShouldEqual, 42)
	// })
	//
	// t.Run("BoardDigitalInterruptTick", func(t *testing.T) {
	// 	server, injectBoard, err := newServer()
	// 	test.That(t, err, test.ShouldBeNil)
	//
	// 	var capName string
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err := server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
	// 		BoardName: boardName,
	// 	})
	// 	test.That(t, err, test.ShouldNotBeNil)
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	// 	test.That(t, capName, test.ShouldEqual, boardName)
	//
	// 	injectBoard := &inject.Board{}
	// 	injectBoard.BoardByNameFunc = func(name string) (board.Board, bool) {
	// 		return injectBoard, true
	// 	}
	// 	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
	// 		capName = name
	// 		return nil, false
	// 	}
	//
	// 	_, err = server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
	// 		BoardName:            boardName,
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
	// 		BoardName:            boardName,
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
	// 		BoardName:            boardName,
	// 		DigitalInterruptName: "digital1",
	// 		High:                 true,
	// 		Nanos:                1028,
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, true, uint64(1028)})
	// })
	//
}

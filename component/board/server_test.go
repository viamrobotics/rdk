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

// TODO(maximpertsov): add board with analogs, with interrupts, etc

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
		type request = pb.BoardServiceGPIOSetRequest
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
				req:        &request{Name: boardName, Pin: "one", High: true},
				expCapArgs: []interface{}{ctx, "one", true},
				expRespErr: genericError,
			},
			{
				injectErr:  nil,
				req:        &request{Name: boardName, Pin: "one", High: true},
				expCapArgs: []interface{}{ctx, "one", true},
				expRespErr: nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capArgs []interface{}
				injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
					capArgs = []interface{}{ctx, pin, high}
					return tc.injectErr
				}

				_, err = server.GPIOSet(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
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
					test.That(t, resp, test.ShouldResemble, tc.expResp)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
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

	t.Run("AnalogReaderRead", func(t *testing.T) {
		type request = pb.BoardServiceAnalogReaderReadRequest
		type response = pb.BoardServiceAnalogReaderReadResponse
		ctx := context.Background()

		tests := []struct {
			injectAnalogReader     *inject.AnalogReader
			injectAnalogReaderOk   bool
			injectResult           int
			injectErr              error
			req                    *request
			expCapAnalogReaderArgs []interface{}
			expCapArgs             []interface{}
			expResp                *response
			expRespErr             error
		}{
			{
				injectAnalogReader:     nil,
				injectAnalogReaderOk:   false,
				injectResult:           0,
				injectErr:              nil,
				req:                    &request{BoardName: missingBoardName},
				expCapAnalogReaderArgs: []interface{}(nil),
				expCapArgs:             []interface{}(nil),
				expResp:                nil,
				expRespErr:             errors.Errorf("no Board with name (%s)", missingBoardName),
			},
			{
				injectAnalogReader:     nil,
				injectAnalogReaderOk:   false,
				injectResult:           0,
				injectErr:              nil,
				req:                    &request{BoardName: invalidBoardName},
				expCapAnalogReaderArgs: []interface{}(nil),
				expCapArgs:             []interface{}(nil),
				expResp:                nil,
				expRespErr:             errors.Errorf("resource with name (%s) is not a Board", invalidBoardName),
			},
			{
				injectAnalogReader:     nil,
				injectAnalogReaderOk:   false,
				injectResult:           0,
				injectErr:              nil,
				req:                    &request{BoardName: boardName, AnalogReaderName: "analog1"},
				expCapAnalogReaderArgs: []interface{}{"analog1"},
				expCapArgs:             []interface{}(nil),
				expResp:                nil,
				expRespErr:             errors.Errorf("unknown analog reader: analog1"),
			},
			{
				injectAnalogReader:     &inject.AnalogReader{},
				injectAnalogReaderOk:   true,
				injectResult:           0,
				injectErr:              genericError,
				req:                    &request{BoardName: boardName, AnalogReaderName: "analog1"},
				expCapAnalogReaderArgs: []interface{}{"analog1"},
				expCapArgs:             []interface{}{ctx},
				expResp:                nil,
				expRespErr:             genericError,
			},
			{
				injectAnalogReader:     &inject.AnalogReader{},
				injectAnalogReaderOk:   true,
				injectResult:           8,
				injectErr:              nil,
				req:                    &request{BoardName: boardName, AnalogReaderName: "analog1"},
				expCapAnalogReaderArgs: []interface{}{"analog1"},
				expCapArgs:             []interface{}{ctx},
				expResp:                &response{Value: 8},
				expRespErr:             nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capAnalogReaderArgs []interface{}
				injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
					capAnalogReaderArgs = []interface{}{name}
					return tc.injectAnalogReader, tc.injectAnalogReaderOk
				}

				var capArgs []interface{}
				if tc.injectAnalogReader != nil {
					tc.injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
						capArgs = []interface{}{ctx}
						return tc.injectResult, tc.injectErr
					}
				}

				resp, err := server.AnalogReaderRead(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
					test.That(t, resp, test.ShouldResemble, tc.expResp)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}

				test.That(t, capAnalogReaderArgs, test.ShouldResemble, tc.expCapAnalogReaderArgs)
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
	})

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

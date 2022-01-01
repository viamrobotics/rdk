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

	t.Run("DigitalInterruptConfig", func(t *testing.T) {
		type request = pb.BoardServiceDigitalInterruptConfigRequest
		type response = pb.BoardServiceDigitalInterruptConfigResponse
		ctx := context.Background()

		theConfig := board.DigitalInterruptConfig{
			Name:    "foo",
			Pin:     "bar",
			Type:    "baz",
			Formula: "baf",
		}

		tests := []struct {
			injectDigitalInterrupt     *inject.DigitalInterrupt
			injectDigitalInterruptOk   bool
			injectResult               board.DigitalInterruptConfig
			injectErr                  error
			req                        *request
			expCapDigitalInterruptArgs []interface{}
			expCapArgs                 []interface{}
			expResp                    *response
			expRespErr                 error
		}{
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectResult:               theConfig,
				injectErr:                  nil,
				req:                        &request{BoardName: missingBoardName},
				expCapDigitalInterruptArgs: []interface{}(nil),
				expCapArgs:                 []interface{}(nil),
				expResp:                    nil,
				expRespErr:                 errors.Errorf("no Board with name (%s)", missingBoardName),
			},
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectResult:               theConfig,
				injectErr:                  nil,
				req:                        &request{BoardName: invalidBoardName},
				expCapDigitalInterruptArgs: []interface{}(nil),
				expCapArgs:                 []interface{}(nil),
				expResp:                    nil,
				expRespErr:                 errors.Errorf("resource with name (%s) is not a Board", invalidBoardName),
			},
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectResult:               theConfig,
				injectErr:                  nil,
				req:                        &request{BoardName: boardName, DigitalInterruptName: "digital1"},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}(nil),
				expResp:                    nil,
				expRespErr:                 errors.Errorf("unknown digital interrupt: digital1"),
			},
			{
				injectDigitalInterrupt:     &inject.DigitalInterrupt{},
				injectDigitalInterruptOk:   true,
				injectResult:               theConfig,
				injectErr:                  genericError,
				req:                        &request{BoardName: boardName, DigitalInterruptName: "digital1"},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}{ctx},
				expResp:                    nil,
				expRespErr:                 genericError,
			},
			{
				injectDigitalInterrupt:     &inject.DigitalInterrupt{},
				injectDigitalInterruptOk:   true,
				injectResult:               theConfig,
				injectErr:                  nil,
				req:                        &request{BoardName: boardName, DigitalInterruptName: "digital1"},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}{ctx},
				expResp: &response{Config: &pb.DigitalInterruptConfig{
					Name:    "foo",
					Pin:     "bar",
					Type:    "baz",
					Formula: "baf",
				}},
				expRespErr: nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capDigitalInterruptArgs []interface{}
				injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
					capDigitalInterruptArgs = []interface{}{name}
					return tc.injectDigitalInterrupt, tc.injectDigitalInterruptOk
				}

				var capArgs []interface{}
				if tc.injectDigitalInterrupt != nil {
					tc.injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
						capArgs = []interface{}{ctx}
						return tc.injectResult, tc.injectErr
					}
				}

				resp, err := server.DigitalInterruptConfig(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
					test.That(t, resp, test.ShouldResemble, tc.expResp)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}

				test.That(t, capDigitalInterruptArgs, test.ShouldResemble, tc.expCapDigitalInterruptArgs)
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
	})

	t.Run("DigitalInterruptValue", func(t *testing.T) {
		type request = pb.BoardServiceDigitalInterruptValueRequest
		type response = pb.BoardServiceDigitalInterruptValueResponse
		ctx := context.Background()

		tests := []struct {
			injectDigitalInterrupt     *inject.DigitalInterrupt
			injectDigitalInterruptOk   bool
			injectResult               int64
			injectErr                  error
			req                        *request
			expCapDigitalInterruptArgs []interface{}
			expCapArgs                 []interface{}
			expResp                    *response
			expRespErr                 error
		}{
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectResult:               0,
				injectErr:                  nil,
				req:                        &request{BoardName: missingBoardName},
				expCapDigitalInterruptArgs: []interface{}(nil),
				expCapArgs:                 []interface{}(nil),
				expResp:                    nil,
				expRespErr:                 errors.Errorf("no Board with name (%s)", missingBoardName),
			},
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectResult:               0,
				injectErr:                  nil,
				req:                        &request{BoardName: invalidBoardName},
				expCapDigitalInterruptArgs: []interface{}(nil),
				expCapArgs:                 []interface{}(nil),
				expResp:                    nil,
				expRespErr:                 errors.Errorf("resource with name (%s) is not a Board", invalidBoardName),
			},
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectResult:               0,
				injectErr:                  nil,
				req:                        &request{BoardName: boardName, DigitalInterruptName: "digital1"},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}(nil),
				expResp:                    nil,
				expRespErr:                 errors.Errorf("unknown digital interrupt: digital1"),
			},
			{
				injectDigitalInterrupt:     &inject.DigitalInterrupt{},
				injectDigitalInterruptOk:   true,
				injectResult:               0,
				injectErr:                  genericError,
				req:                        &request{BoardName: boardName, DigitalInterruptName: "digital1"},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}{ctx},
				expResp:                    nil,
				expRespErr:                 genericError,
			},
			{
				injectDigitalInterrupt:     &inject.DigitalInterrupt{},
				injectDigitalInterruptOk:   true,
				injectResult:               42,
				injectErr:                  nil,
				req:                        &request{BoardName: boardName, DigitalInterruptName: "digital1"},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}{ctx},
				expResp:                    &response{Value: 42},
				expRespErr:                 nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capDigitalInterruptArgs []interface{}
				injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
					capDigitalInterruptArgs = []interface{}{name}
					return tc.injectDigitalInterrupt, tc.injectDigitalInterruptOk
				}

				var capArgs []interface{}
				if tc.injectDigitalInterrupt != nil {
					tc.injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
						capArgs = []interface{}{ctx}
						return tc.injectResult, tc.injectErr
					}
				}

				resp, err := server.DigitalInterruptValue(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
					test.That(t, resp, test.ShouldResemble, tc.expResp)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}

				test.That(t, capDigitalInterruptArgs, test.ShouldResemble, tc.expCapDigitalInterruptArgs)
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
	})

	t.Run("DigitalInterruptTick", func(t *testing.T) {
		type request = pb.BoardServiceDigitalInterruptTickRequest
		type response = pb.BoardServiceDigitalInterruptTickResponse
		ctx := context.Background()

		tests := []struct {
			injectDigitalInterrupt     *inject.DigitalInterrupt
			injectDigitalInterruptOk   bool
			injectErr                  error
			req                        *request
			expCapDigitalInterruptArgs []interface{}
			expCapArgs                 []interface{}
			expRespErr                 error
		}{
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectErr:                  nil,
				req:                        &request{BoardName: missingBoardName},
				expCapDigitalInterruptArgs: []interface{}(nil),
				expCapArgs:                 []interface{}(nil),
				expRespErr:                 errors.Errorf("no Board with name (%s)", missingBoardName),
			},
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectErr:                  nil,
				req:                        &request{BoardName: invalidBoardName},
				expCapDigitalInterruptArgs: []interface{}(nil),
				expCapArgs:                 []interface{}(nil),
				expRespErr:                 errors.Errorf("resource with name (%s) is not a Board", invalidBoardName),
			},
			{
				injectDigitalInterrupt:     nil,
				injectDigitalInterruptOk:   false,
				injectErr:                  nil,
				req:                        &request{BoardName: boardName, DigitalInterruptName: "digital1"},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}(nil),
				expRespErr:                 errors.Errorf("unknown digital interrupt: digital1"),
			},
			{
				injectDigitalInterrupt:   &inject.DigitalInterrupt{},
				injectDigitalInterruptOk: true,
				injectErr:                genericError,
				req: &request{
					BoardName:            boardName,
					DigitalInterruptName: "digital1",
					High:                 true,
					Nanos:                1028,
				},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}{ctx, true, uint64(1028)},
				expRespErr:                 genericError,
			},
			{
				injectDigitalInterrupt:   &inject.DigitalInterrupt{},
				injectDigitalInterruptOk: true,
				injectErr:                nil,
				req: &request{
					BoardName:            boardName,
					DigitalInterruptName: "digital1",
					High:                 true,
					Nanos:                1028,
				},
				expCapDigitalInterruptArgs: []interface{}{"digital1"},
				expCapArgs:                 []interface{}{ctx, true, uint64(1028)},
				expRespErr:                 nil,
			},
		}

		for _, tc := range tests {
			t.Run("", func(t *testing.T) {
				server, injectBoard, err := newServer()
				test.That(t, err, test.ShouldBeNil)

				var capDigitalInterruptArgs []interface{}
				injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
					capDigitalInterruptArgs = []interface{}{name}
					return tc.injectDigitalInterrupt, tc.injectDigitalInterruptOk
				}

				var capArgs []interface{}
				if tc.injectDigitalInterrupt != nil {
					tc.injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
						capArgs = []interface{}{ctx, high, nanos}
						return tc.injectErr
					}
				}

				_, err = server.DigitalInterruptTick(ctx, tc.req)
				if tc.expRespErr == nil {
					test.That(t, err, test.ShouldBeNil)
				} else {
					test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
				}

				test.That(t, capDigitalInterruptArgs, test.ShouldResemble, tc.expCapDigitalInterruptArgs)
				test.That(t, capArgs, test.ShouldResemble, tc.expCapArgs)
			})
		}
	})
}

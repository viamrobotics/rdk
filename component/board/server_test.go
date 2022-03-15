package board_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/board/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

var errFoo = errors.New("whoops")

func newServer() (pb.BoardServiceServer, *inject.Board, error) {
	injectBoard := &inject.Board{}
	boards := map[resource.Name]interface{}{
		board.Named(testBoardName): injectBoard,
		board.Named(fakeBoardName): "notBoard",
	}
	boardSvc, err := subtype.New(boards)
	if err != nil {
		return nil, nil, err
	}
	return board.NewServer(boardSvc), injectBoard, nil
}

func TestServerStatus(t *testing.T) {
	type request = pb.StatusRequest
	type response = pb.StatusResponse
	ctx := context.Background()

	status := &commonpb.BoardStatus{
		Analogs: map[string]*commonpb.AnalogStatus{
			"analog1": {},
		},
		DigitalInterrupts: map[string]*commonpb.DigitalInterruptStatus{
			"encoder": {},
		},
	}

	tests := []struct {
		injectResult *commonpb.BoardStatus
		injectErr    error
		req          *request
		expCapArgs   []interface{}
		expResp      *response
		expRespErr   error
	}{
		{
			injectResult: status,
			injectErr:    nil,
			req:          &request{Name: missingBoardName},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   errors.Errorf("no board with name (%s)", missingBoardName),
		},
		{
			injectResult: status,
			injectErr:    nil,
			req:          &request{Name: fakeBoardName},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   errors.Errorf("resource with name (%s) is not a board", fakeBoardName),
		},
		{
			injectResult: status,
			injectErr:    errFoo,
			req:          &request{Name: testBoardName},
			expCapArgs:   []interface{}{ctx},
			expResp:      nil,
			expRespErr:   errFoo,
		},
		{
			injectResult: status,
			injectErr:    nil,
			req:          &request{Name: testBoardName},
			expCapArgs:   []interface{}{ctx},
			expResp:      &response{Status: status},
			expRespErr:   nil,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)

			injectBoard.StatusFunc = func(ctx context.Context) (*commonpb.BoardStatus, error) {
				return tc.injectResult, tc.injectErr
			}

			resp, err := server.Status(ctx, tc.req)
			if tc.expRespErr == nil {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
			}
			test.That(t, injectBoard.StatusCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerSetGPIO(t *testing.T) {
	type request = pb.SetGPIORequest
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
			expRespErr: errors.Errorf("no board with name (%s)", missingBoardName),
		},
		{
			injectErr:  nil,
			req:        &request{Name: fakeBoardName},
			expCapArgs: []interface{}(nil),
			expRespErr: errors.Errorf("resource with name (%s) is not a board", fakeBoardName),
		},
		{
			injectErr:  errFoo,
			req:        &request{Name: testBoardName, Pin: "one", High: true},
			expCapArgs: []interface{}{ctx, true},
			expRespErr: errFoo,
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one", High: true},
			expCapArgs: []interface{}{ctx, true},
			expRespErr: nil,
		},
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				return injectGPIOPin, nil
			}

			injectGPIOPin.SetFunc = func(ctx context.Context, high bool) error {
				return tc.injectErr
			}

			_, err = server.SetGPIO(ctx, tc.req)
			if tc.expRespErr == nil {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
			}
			test.That(t, injectGPIOPin.SetCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerGetGPIO(t *testing.T) {
	type request = pb.GetGPIORequest
	type response = pb.GetGPIOResponse
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
			expRespErr:   errors.Errorf("no board with name (%s)", missingBoardName),
		},
		{
			injectResult: false,
			injectErr:    nil,
			req:          &request{Name: fakeBoardName},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   errors.Errorf("resource with name (%s) is not a board", fakeBoardName),
		},
		{
			injectResult: false,
			injectErr:    errFoo,
			req:          &request{Name: testBoardName, Pin: "one"},
			expCapArgs:   []interface{}{ctx},
			expResp:      nil,
			expRespErr:   errFoo,
		},
		{
			injectResult: true,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Pin: "one"},
			expCapArgs:   []interface{}{ctx},
			expResp:      &response{High: true},
			expRespErr:   nil,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				return injectGPIOPin, nil
			}

			injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
				return tc.injectResult, tc.injectErr
			}

			resp, err := server.GetGPIO(ctx, tc.req)
			if tc.expRespErr == nil {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
			}
			test.That(t, injectGPIOPin.GetCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerSetPWM(t *testing.T) {
	type request = pb.SetPWMRequest
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
			expRespErr: errors.Errorf("no board with name (%s)", missingBoardName),
		},
		{
			injectErr:  nil,
			req:        &request{Name: fakeBoardName},
			expCapArgs: []interface{}(nil),
			expRespErr: errors.Errorf("resource with name (%s) is not a board", fakeBoardName),
		},
		{
			injectErr:  errFoo,
			req:        &request{Name: testBoardName, Pin: "one", DutyCyclePct: 0.03},
			expCapArgs: []interface{}{ctx, 0.03},
			expRespErr: errFoo,
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one", DutyCyclePct: 0.03},
			expCapArgs: []interface{}{ctx, 0.03},
			expRespErr: nil,
		},
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				return injectGPIOPin, nil
			}

			injectGPIOPin.SetPWMFunc = func(ctx context.Context, dutyCyclePct float64) error {
				return tc.injectErr
			}

			_, err = server.SetPWM(ctx, tc.req)
			if tc.expRespErr == nil {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
			}
			test.That(t, injectGPIOPin.SetPWMCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerSetPWMFrequency(t *testing.T) {
	type request = pb.SetPWMFrequencyRequest
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
			expRespErr: errors.Errorf("no board with name (%s)", missingBoardName),
		},
		{
			injectErr:  nil,
			req:        &request{Name: fakeBoardName},
			expCapArgs: []interface{}(nil),
			expRespErr: errors.Errorf("resource with name (%s) is not a board", fakeBoardName),
		},
		{
			injectErr:  errFoo,
			req:        &request{Name: testBoardName, Pin: "one", FrequencyHz: 123123},
			expCapArgs: []interface{}{ctx, uint(123123)},
			expRespErr: errFoo,
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one", FrequencyHz: 123123},
			expCapArgs: []interface{}{ctx, uint(123123)},
			expRespErr: nil,
		},
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				return injectGPIOPin, nil
			}

			injectGPIOPin.SetPWMFreqFunc = func(ctx context.Context, freqHz uint) error {
				return tc.injectErr
			}

			_, err = server.SetPWMFrequency(ctx, tc.req)
			if tc.expRespErr == nil {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
			}
			test.That(t, injectGPIOPin.SetPWMFreqCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

//nolint:dupl
func TestServerReadAnalogReader(t *testing.T) {
	type request = pb.ReadAnalogReaderRequest
	type response = pb.ReadAnalogReaderResponse
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
			expRespErr:             errors.Errorf("no board with name (%s)", missingBoardName),
		},
		{
			injectAnalogReader:     nil,
			injectAnalogReaderOk:   false,
			injectResult:           0,
			injectErr:              nil,
			req:                    &request{BoardName: fakeBoardName},
			expCapAnalogReaderArgs: []interface{}(nil),
			expCapArgs:             []interface{}(nil),
			expResp:                nil,
			expRespErr:             errors.Errorf("resource with name (%s) is not a board", fakeBoardName),
		},
		{
			injectAnalogReader:     nil,
			injectAnalogReaderOk:   false,
			injectResult:           0,
			injectErr:              nil,
			req:                    &request{BoardName: testBoardName, AnalogReaderName: "analog1"},
			expCapAnalogReaderArgs: []interface{}{"analog1"},
			expCapArgs:             []interface{}(nil),
			expResp:                nil,
			expRespErr:             errors.New("unknown analog reader: analog1"),
		},
		{
			injectAnalogReader:     &inject.AnalogReader{},
			injectAnalogReaderOk:   true,
			injectResult:           0,
			injectErr:              errFoo,
			req:                    &request{BoardName: testBoardName, AnalogReaderName: "analog1"},
			expCapAnalogReaderArgs: []interface{}{"analog1"},
			expCapArgs:             []interface{}{ctx},
			expResp:                nil,
			expRespErr:             errFoo,
		},
		{
			injectAnalogReader:     &inject.AnalogReader{},
			injectAnalogReaderOk:   true,
			injectResult:           8,
			injectErr:              nil,
			req:                    &request{BoardName: testBoardName, AnalogReaderName: "analog1"},
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

			injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return tc.injectAnalogReader, tc.injectAnalogReaderOk
			}

			if tc.injectAnalogReader != nil {
				tc.injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
					return tc.injectResult, tc.injectErr
				}
			}

			resp, err := server.ReadAnalogReader(ctx, tc.req)
			if tc.expRespErr == nil {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
			}
			test.That(t, injectBoard.AnalogReaderByNameCap(), test.ShouldResemble, tc.expCapAnalogReaderArgs)
			test.That(t, tc.injectAnalogReader.ReadCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

//nolint:dupl
func TestServerGetDigitalInterruptValue(t *testing.T) {
	type request = pb.GetDigitalInterruptValueRequest
	type response = pb.GetDigitalInterruptValueResponse
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
			expRespErr:                 errors.Errorf("no board with name (%s)", missingBoardName),
		},
		{
			injectDigitalInterrupt:     nil,
			injectDigitalInterruptOk:   false,
			injectResult:               0,
			injectErr:                  nil,
			req:                        &request{BoardName: fakeBoardName},
			expCapDigitalInterruptArgs: []interface{}(nil),
			expCapArgs:                 []interface{}(nil),
			expResp:                    nil,
			expRespErr:                 errors.Errorf("resource with name (%s) is not a board", fakeBoardName),
		},
		{
			injectDigitalInterrupt:     nil,
			injectDigitalInterruptOk:   false,
			injectResult:               0,
			injectErr:                  nil,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1"},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}(nil),
			expResp:                    nil,
			expRespErr:                 errors.New("unknown digital interrupt: digital1"),
		},
		{
			injectDigitalInterrupt:     &inject.DigitalInterrupt{},
			injectDigitalInterruptOk:   true,
			injectResult:               0,
			injectErr:                  errFoo,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1"},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}{ctx},
			expResp:                    nil,
			expRespErr:                 errFoo,
		},
		{
			injectDigitalInterrupt:     &inject.DigitalInterrupt{},
			injectDigitalInterruptOk:   true,
			injectResult:               42,
			injectErr:                  nil,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1"},
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

			injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
				return tc.injectDigitalInterrupt, tc.injectDigitalInterruptOk
			}

			if tc.injectDigitalInterrupt != nil {
				tc.injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
					return tc.injectResult, tc.injectErr
				}
			}

			resp, err := server.GetDigitalInterruptValue(ctx, tc.req)
			if tc.expRespErr == nil {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.expRespErr.Error())
			}

			test.That(t, injectBoard.DigitalInterruptByNameCap(), test.ShouldResemble, tc.expCapDigitalInterruptArgs)
			test.That(t, tc.injectDigitalInterrupt.ValueCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

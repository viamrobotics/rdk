package board_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errFoo        = errors.New("whoops")
	errNotFound   = errors.New("not found")
	errSendFailed = errors.New("send fail")
	errAnalog     = errors.New("unknown analog error")
	errDigital    = errors.New("unknown digital interrupt error")
)

func newServer() (pb.BoardServiceServer, *inject.Board, error) {
	injectBoard := &inject.Board{}
	boards := map[resource.Name]board.Board{
		board.Named(testBoardName): injectBoard,
	}
	boardSvc, err := resource.NewAPIResourceCollection(board.API, boards)
	if err != nil {
		return nil, nil, err
	}
	return board.NewRPCServiceServer(boardSvc).(pb.BoardServiceServer), injectBoard, nil
}

func TestServerSetGPIO(t *testing.T) {
	type request = pb.SetGPIORequest
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectErr  error
		req        *request
		expCapArgs []interface{}
		expRespErr string
	}{
		{
			injectErr:  nil,
			req:        &request{Name: missingBoardName},
			expCapArgs: []interface{}(nil),
			expRespErr: errNotFound.Error(),
		},
		{
			injectErr:  errFoo,
			req:        &request{Name: testBoardName, Pin: "one", High: true},
			expCapArgs: []interface{}{ctx, true},
			expRespErr: errFoo.Error(),
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one", High: true, Extra: pbExpectedExtra},
			expCapArgs: []interface{}{ctx, true},
			expRespErr: "",
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one", High: true},
			expCapArgs: []interface{}(nil),
			expRespErr: board.ErrGPIOPinByNameReturnNil(testBoardName).Error(),
		},
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				if tc.expRespErr == board.ErrGPIOPinByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return injectGPIOPin, nil
			}

			injectGPIOPin.SetFunc = func(ctx context.Context, high bool, extra map[string]interface{}) error {
				actualExtra = extra
				return tc.injectErr
			}

			_, err = server.SetGPIO(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectGPIOPin.SetCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerGetGPIO(t *testing.T) {
	type request = pb.GetGPIORequest
	type response = pb.GetGPIOResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectResult bool
		injectErr    error
		req          *request
		expCapArgs   []interface{}
		expResp      *response
		expRespErr   string
	}{
		{
			injectResult: false,
			injectErr:    nil,
			req:          &request{Name: missingBoardName},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   errNotFound.Error(),
		},
		{
			injectResult: false,
			injectErr:    errFoo,
			req:          &request{Name: testBoardName, Pin: "one"},
			expCapArgs:   []interface{}{ctx},
			expResp:      nil,
			expRespErr:   errFoo.Error(),
		},
		{
			injectResult: true,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Pin: "one", Extra: pbExpectedExtra},
			expCapArgs:   []interface{}{ctx},
			expResp:      &response{High: true},
			expRespErr:   "",
		},
		{
			injectResult: false,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Pin: "one"},
			expResp:      nil,
			expRespErr:   board.ErrGPIOPinByNameReturnNil(testBoardName).Error(),
		},
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				if tc.expRespErr == board.ErrGPIOPinByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return injectGPIOPin, nil
			}

			injectGPIOPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
				actualExtra = extra
				return tc.injectResult, tc.injectErr
			}

			resp, err := server.GetGPIO(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectGPIOPin.GetCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

//nolint:dupl
func TestServerPWM(t *testing.T) {
	type request = pb.PWMRequest
	type response = pb.PWMResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectResult float64
		injectErr    error
		req          *request
		expCapArgs   []interface{}
		expResp      *response
		expRespErr   string
	}{
		{
			injectResult: 0,
			injectErr:    nil,
			req:          &request{Name: missingBoardName},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   errNotFound.Error(),
		},
		{
			injectResult: 0,
			injectErr:    errFoo,
			req:          &request{Name: testBoardName, Pin: "one"},
			expCapArgs:   []interface{}{ctx},
			expResp:      nil,
			expRespErr:   errFoo.Error(),
		},
		{
			injectResult: 0.1,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Pin: "one", Extra: pbExpectedExtra},
			expCapArgs:   []interface{}{ctx},
			expResp:      &response{DutyCyclePct: 0.1},
			expRespErr:   "",
		},
		{
			injectResult: 0,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Pin: "one"},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   board.ErrGPIOPinByNameReturnNil(testBoardName).Error(),
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				if tc.expRespErr == board.ErrGPIOPinByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return injectGPIOPin, nil
			}

			injectGPIOPin.PWMFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
				actualExtra = extra
				return tc.injectResult, tc.injectErr
			}

			resp, err := server.PWM(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectGPIOPin.PWMCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerSetPWM(t *testing.T) {
	type request = pb.SetPWMRequest
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectErr  error
		req        *request
		expCapArgs []interface{}
		expRespErr string
	}{
		{
			injectErr:  nil,
			req:        &request{Name: missingBoardName},
			expCapArgs: []interface{}(nil),
			expRespErr: errNotFound.Error(),
		},
		{
			injectErr:  errFoo,
			req:        &request{Name: testBoardName, Pin: "one", DutyCyclePct: 0.03},
			expCapArgs: []interface{}{ctx, 0.03},
			expRespErr: errFoo.Error(),
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one", DutyCyclePct: 0.03, Extra: pbExpectedExtra},
			expCapArgs: []interface{}{ctx, 0.03},
			expRespErr: "",
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one"},
			expCapArgs: []interface{}(nil),
			expRespErr: board.ErrGPIOPinByNameReturnNil(testBoardName).Error(),
		},
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				if tc.expRespErr == board.ErrGPIOPinByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return injectGPIOPin, nil
			}

			injectGPIOPin.SetPWMFunc = func(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
				actualExtra = extra
				return tc.injectErr
			}

			_, err = server.SetPWM(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectGPIOPin.SetPWMCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

//nolint:dupl
func TestServerPWMFrequency(t *testing.T) {
	type request = pb.PWMFrequencyRequest
	type response = pb.PWMFrequencyResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectResult uint
		injectErr    error
		req          *request
		expCapArgs   []interface{}
		expResp      *response
		expRespErr   string
	}{
		{
			injectResult: 0,
			injectErr:    nil,
			req:          &request{Name: missingBoardName},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   errNotFound.Error(),
		},
		{
			injectResult: 0,
			injectErr:    errFoo,
			req:          &request{Name: testBoardName, Pin: "one"},
			expCapArgs:   []interface{}{ctx},
			expResp:      nil,
			expRespErr:   errFoo.Error(),
		},
		{
			injectResult: 1,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Pin: "one", Extra: pbExpectedExtra},
			expCapArgs:   []interface{}{ctx},
			expResp:      &response{FrequencyHz: 1},
			expRespErr:   "",
		},
		{
			injectResult: 0,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Pin: "one"},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   board.ErrGPIOPinByNameReturnNil(testBoardName).Error(),
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				if tc.expRespErr == board.ErrGPIOPinByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return injectGPIOPin, nil
			}

			injectGPIOPin.PWMFreqFunc = func(ctx context.Context, extra map[string]interface{}) (uint, error) {
				actualExtra = extra
				return tc.injectResult, tc.injectErr
			}

			resp, err := server.PWMFrequency(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectGPIOPin.PWMFreqCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerSetPWMFrequency(t *testing.T) {
	type request = pb.SetPWMFrequencyRequest
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectErr  error
		req        *request
		expCapArgs []interface{}
		expRespErr string
	}{
		{
			injectErr:  nil,
			req:        &request{Name: missingBoardName},
			expCapArgs: []interface{}(nil),
			expRespErr: errNotFound.Error(),
		},
		{
			injectErr:  errFoo,
			req:        &request{Name: testBoardName, Pin: "one", FrequencyHz: 123123},
			expCapArgs: []interface{}{ctx, uint(123123)},
			expRespErr: errFoo.Error(),
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one", FrequencyHz: 123123, Extra: pbExpectedExtra},
			expCapArgs: []interface{}{ctx, uint(123123)},
			expRespErr: "",
		},
		{
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "one"},
			expCapArgs: []interface{}(nil),
			expRespErr: board.ErrGPIOPinByNameReturnNil(testBoardName).Error(),
		},
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
				if tc.expRespErr == board.ErrGPIOPinByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return injectGPIOPin, nil
			}

			injectGPIOPin.SetPWMFreqFunc = func(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
				actualExtra = extra
				return tc.injectErr
			}

			_, err = server.SetPWMFrequency(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectGPIOPin.SetPWMFreqCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerReadAnalogReader(t *testing.T) {
	type request = pb.ReadAnalogReaderRequest
	type response = pb.ReadAnalogReaderResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectAnalog     *inject.Analog
		injectAnalogErr  error
		injectVal        board.AnalogValue
		injectErr        error
		req              *request
		expCapAnalogArgs []interface{}
		expCapArgs       []interface{}
		expResp          *response
		expRespErr       string
	}{
		{
			injectAnalog:     nil,
			injectAnalogErr:  errAnalog,
			injectVal:        board.AnalogValue{Value: 0},
			injectErr:        nil,
			req:              &request{BoardName: missingBoardName},
			expCapAnalogArgs: []interface{}(nil),
			expCapArgs:       []interface{}(nil),
			expResp:          nil,
			expRespErr:       errNotFound.Error(),
		},
		{
			injectAnalog:     nil,
			injectAnalogErr:  errAnalog,
			injectVal:        board.AnalogValue{Value: 0},
			injectErr:        nil,
			req:              &request{BoardName: testBoardName, AnalogReaderName: "analog1"},
			expCapAnalogArgs: []interface{}{"analog1"},
			expCapArgs:       []interface{}(nil),
			expResp:          nil,
			expRespErr:       "unknown analog error",
		},
		{
			injectAnalog:     &inject.Analog{},
			injectAnalogErr:  nil,
			injectVal:        board.AnalogValue{Value: 0},
			injectErr:        errFoo,
			req:              &request{BoardName: testBoardName, AnalogReaderName: "analog1"},
			expCapAnalogArgs: []interface{}{"analog1"},
			expCapArgs:       []interface{}{ctx},
			expResp:          nil,
			expRespErr:       errFoo.Error(),
		},
		{
			injectAnalog:     &inject.Analog{},
			injectAnalogErr:  nil,
			injectVal:        board.AnalogValue{Value: 8, Min: 0, Max: 10, StepSize: 0.1},
			injectErr:        nil,
			req:              &request{BoardName: testBoardName, AnalogReaderName: "analog1", Extra: pbExpectedExtra},
			expCapAnalogArgs: []interface{}{"analog1"},
			expCapArgs:       []interface{}{ctx},
			expResp:          &response{Value: 8, MinRange: 0, MaxRange: 10, StepSize: 0.1},
			expRespErr:       "",
		},
		{
			injectAnalog:     &inject.Analog{},
			injectAnalogErr:  nil,
			injectVal:        board.AnalogValue{Value: 8, Min: 0, Max: 10, StepSize: 0.1},
			injectErr:        nil,
			req:              &request{BoardName: testBoardName, AnalogReaderName: "analog1"},
			expCapAnalogArgs: []interface{}{"analog1"},
			expCapArgs:       []interface{}(nil),
			expResp:          nil,
			expRespErr:       board.ErrAnalogByNameReturnNil(testBoardName).Error(),
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectBoard.AnalogByNameFunc = func(name string) (board.Analog, error) {
				if tc.expRespErr == board.ErrAnalogByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return tc.injectAnalog, tc.injectAnalogErr
			}

			if tc.injectAnalog != nil {
				tc.injectAnalog.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (board.AnalogValue, error) {
					actualExtra = extra
					return tc.injectVal, tc.injectErr
				}
			}

			resp, err := server.ReadAnalogReader(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectBoard.AnalogByNameCap(), test.ShouldResemble, tc.expCapAnalogArgs)
			test.That(t, tc.injectAnalog.ReadCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

func TestServerWriteAnalog(t *testing.T) {
	type request = pb.WriteAnalogRequest
	type response = pb.WriteAnalogResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		name           string
		injectErr      error
		req            *request
		expCaptureArgs []interface{}
		expResp        *response
		expRespErr     string
	}{
		{
			name:       "Successful analog write",
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "analogwriter1", Value: 1, Extra: pbExpectedExtra},
			expResp:    &response{},
			expRespErr: "",
		},
		{
			name:       "Analog write called on a board that does not exist should return not found error",
			injectErr:  nil,
			req:        &request{Name: missingBoardName},
			expResp:    nil,
			expRespErr: errNotFound.Error(),
		},
		{
			name:      "An error on the analog writer write should be returned",
			injectErr: errFoo,
			req:       &request{Name: testBoardName, Pin: "analogwriter1", Value: 3},

			expResp:    nil,
			expRespErr: errFoo.Error(),
		},
		{
			name:       "Analog should be nil",
			injectErr:  nil,
			req:        &request{Name: testBoardName, Pin: "analog1", Extra: pbExpectedExtra},
			expResp:    nil,
			expRespErr: board.ErrAnalogByNameReturnNil(testBoardName).Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectAnalog := inject.Analog{}
			injectAnalog.WriteFunc = func(ctx context.Context, value int, extra map[string]interface{}) error {
				actualExtra = extra
				return tc.injectErr
			}
			injectBoard.AnalogByNameFunc = func(pin string) (board.Analog, error) {
				if tc.expRespErr == board.ErrAnalogByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return &injectAnalog, nil
			}

			resp, err := server.WriteAnalog(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
		})
	}
}

func TestServerGetDigitalInterruptValue(t *testing.T) {
	type request = pb.GetDigitalInterruptValueRequest
	type response = pb.GetDigitalInterruptValueResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectDigitalInterrupt     *inject.DigitalInterrupt
		injectDigitalInterruptErr  error
		injectResult               int64
		injectErr                  error
		req                        *request
		expCapDigitalInterruptArgs []interface{}
		expCapArgs                 []interface{}
		expResp                    *response
		expRespErr                 string
	}{
		{
			injectDigitalInterrupt:     nil,
			injectDigitalInterruptErr:  errDigital,
			injectResult:               0,
			injectErr:                  nil,
			req:                        &request{BoardName: missingBoardName},
			expCapDigitalInterruptArgs: []interface{}(nil),
			expCapArgs:                 []interface{}(nil),
			expResp:                    nil,
			expRespErr:                 errNotFound.Error(),
		},
		{
			injectDigitalInterrupt:     nil,
			injectDigitalInterruptErr:  errDigital,
			injectResult:               0,
			injectErr:                  nil,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1"},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}(nil),
			expResp:                    nil,
			expRespErr:                 "unknown digital interrupt error",
		},
		{
			injectDigitalInterrupt:     &inject.DigitalInterrupt{},
			injectDigitalInterruptErr:  nil,
			injectResult:               0,
			injectErr:                  errFoo,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1"},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}{ctx},
			expResp:                    nil,
			expRespErr:                 errFoo.Error(),
		},
		{
			injectDigitalInterrupt:     &inject.DigitalInterrupt{},
			injectDigitalInterruptErr:  nil,
			injectResult:               42,
			injectErr:                  nil,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1", Extra: pbExpectedExtra},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}{ctx},
			expResp:                    &response{Value: 42},
			expRespErr:                 "",
		},
		{
			injectDigitalInterrupt:     &inject.DigitalInterrupt{},
			injectDigitalInterruptErr:  nil,
			injectResult:               0,
			injectErr:                  nil,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1"},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}(nil),
			expResp:                    nil,
			expRespErr:                 board.ErrDigitalInterruptByNameReturnNil(testBoardName).Error(),
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, error) {
				if tc.expRespErr == board.ErrDigitalInterruptByNameReturnNil(testBoardName).Error() {
					return nil, nil
				}
				return tc.injectDigitalInterrupt, tc.injectDigitalInterruptErr
			}

			if tc.injectDigitalInterrupt != nil {
				tc.injectDigitalInterrupt.ValueFunc = func(ctx context.Context, extra map[string]interface{}) (int64, error) {
					actualExtra = extra
					return tc.injectResult, tc.injectErr
				}
			}

			resp, err := server.GetDigitalInterruptValue(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}

			test.That(t, injectBoard.DigitalInterruptByNameCap(), test.ShouldResemble, tc.expCapDigitalInterruptArgs)
			test.That(t, tc.injectDigitalInterrupt.ValueCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
}

type streamTicksServer struct {
	grpc.ServerStream
	ctx       context.Context
	ticksChan chan *pb.StreamTicksResponse
	fail      bool
}

func (x *streamTicksServer) Context() context.Context {
	return x.ctx
}

func (x *streamTicksServer) Send(m *pb.StreamTicksResponse) error {
	if x.fail {
		return errSendFailed
	}
	if x.ticksChan == nil {
		return nil
	}
	x.ticksChan <- m
	return nil
}

func TestStreamTicks(t *testing.T) {
	type request = pb.StreamTicksRequest
	type response = pb.StreamTicksResponse

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		name                      string
		injectDigitalInterrupts   []*inject.DigitalInterrupt
		injectDigitalInterruptErr error
		streamTicksErr            error
		req                       *request
		expResp                   *response
		expRespErr                string
		sendFail                  bool
	}{
		{
			name:                      "successful stream with multiple interrupts",
			injectDigitalInterrupts:   []*inject.DigitalInterrupt{{}, {}},
			injectDigitalInterruptErr: nil,
			streamTicksErr:            nil,
			req:                       &request{Name: testBoardName, PinNames: []string{"digital1", "digital2"}, Extra: pbExpectedExtra},
			expResp:                   &response{PinName: "digital1", Time: uint64(time.Nanosecond), High: true},
			sendFail:                  false,
		},
		{
			name:                      "successful stream with one interrupt",
			injectDigitalInterrupts:   []*inject.DigitalInterrupt{{}},
			injectDigitalInterruptErr: nil,
			streamTicksErr:            nil,
			req:                       &request{Name: testBoardName, PinNames: []string{"digital1"}, Extra: pbExpectedExtra},
			expResp:                   &response{PinName: "digital1", Time: uint64(time.Nanosecond), High: true},
			sendFail:                  false,
		},
		{
			name:           "missing board name should return error",
			streamTicksErr: nil,
			req:            &request{Name: missingBoardName, PinNames: []string{"pin1"}},
			expResp:        nil,
			expRespErr:     errNotFound.Error(),
		},
		{
			name:                      "unknown digital interrupt should return error",
			injectDigitalInterrupts:   []*inject.DigitalInterrupt{{}, {}},
			injectDigitalInterruptErr: errDigital,
			streamTicksErr:            errors.New("unknown digital interrupt: digital3"),
			req:                       &request{Name: testBoardName, PinNames: []string{"digital3"}},
			expResp:                   nil,
			expRespErr:                "unknown digital interrupt: digital3",
			sendFail:                  false,
		},
		{
			name:                      "failing to send tick should return error",
			injectDigitalInterrupts:   []*inject.DigitalInterrupt{{}},
			injectDigitalInterruptErr: errSendFailed,
			streamTicksErr:            nil,
			req:                       &request{Name: testBoardName, PinNames: []string{"digital1"}, Extra: pbExpectedExtra},
			expResp:                   &response{PinName: "digital1", Time: uint64(time.Nanosecond), High: true},
			expRespErr:                "send fail",
			sendFail:                  true,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}
			callbacks := []chan board.Tick{}

			injectBoard.StreamTicksFunc = func(
				ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick,
				extra map[string]interface{},
			) error {
				actualExtra = extra
				callbacks = append(callbacks, ch)
				return tc.streamTicksErr
			}

			injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, error) {
				if name == "digital1" {
					return tc.injectDigitalInterrupts[0], tc.injectDigitalInterruptErr
				} else if name == "digital2" {
					return tc.injectDigitalInterrupts[1], tc.injectDigitalInterruptErr
				}
				return nil, nil
			}

			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ch := make(chan *pb.StreamTicksResponse)
			s := &streamTicksServer{
				ctx:       cancelCtx,
				ticksChan: ch,
				fail:      tc.sendFail,
			}

			sendTick := func() {
				for _, ch := range callbacks {
					ch <- board.Tick{Name: "digital1", High: true, TimestampNanosec: uint64(time.Nanosecond)}
				}
			}
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = server.StreamTicks(tc.req, s)
			}()

			if tc.expRespErr == "" {
				// First resp will be blank
				<-s.ticksChan

				sendTick()
				resp := <-s.ticksChan

				test.That(t, err, test.ShouldBeNil)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
				test.That(t, resp.High, test.ShouldEqual, true)
				test.That(t, resp.PinName, test.ShouldEqual, "digital1")
				test.That(t, resp.Time, test.ShouldEqual, uint64(time.Nanosecond))

				cancel()
				wg.Wait()
			} else {
				// Canceling the stream before checking the error to avoid a data race.
				cancel()
				wg.Wait()
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
		})
	}
}

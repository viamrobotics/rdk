package board_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
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

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectResult *commonpb.BoardStatus
		injectErr    error
		req          *request
		expCapArgs   []interface{}
		expResp      *response
		expRespErr   string
	}{
		{
			injectResult: status,
			injectErr:    nil,
			req:          &request{Name: missingBoardName},
			expCapArgs:   []interface{}(nil),
			expResp:      nil,
			expRespErr:   errNotFound.Error(),
		},
		{
			injectResult: status,
			injectErr:    errFoo,
			req:          &request{Name: testBoardName},
			expCapArgs:   []interface{}{ctx},
			expResp:      nil,
			expRespErr:   errFoo.Error(),
		},
		{
			injectResult: status,
			injectErr:    nil,
			req:          &request{Name: testBoardName, Extra: pbExpectedExtra},
			expCapArgs:   []interface{}{ctx},
			expResp:      &response{Status: status},
			expRespErr:   "",
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)

			var actualExtra map[string]interface{}

			injectBoard.StatusFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
				actualExtra = extra
				return tc.injectResult, tc.injectErr
			}

			resp, err := server.Status(ctx, tc.req)
			if tc.expRespErr == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp, test.ShouldResemble, tc.expResp)
				test.That(t, actualExtra, test.ShouldResemble, expectedExtra)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expRespErr)
			}
			test.That(t, injectBoard.StatusCap(), test.ShouldResemble, tc.expCapArgs)
		})
	}
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
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
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
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
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
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
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
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
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
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
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
	}

	//nolint:dupl
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectGPIOPin := &inject.GPIOPin{}
			injectBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
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

//nolint:dupl
func TestServerReadAnalogReader(t *testing.T) {
	type request = pb.ReadAnalogReaderRequest
	type response = pb.ReadAnalogReaderResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectAnalogReader     *inject.AnalogReader
		injectAnalogReaderOk   bool
		injectResult           int
		injectErr              error
		req                    *request
		expCapAnalogReaderArgs []interface{}
		expCapArgs             []interface{}
		expResp                *response
		expRespErr             string
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
			expRespErr:             errNotFound.Error(),
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
			expRespErr:             "unknown analog reader: analog1",
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
			expRespErr:             errFoo.Error(),
		},
		{
			injectAnalogReader:     &inject.AnalogReader{},
			injectAnalogReaderOk:   true,
			injectResult:           8,
			injectErr:              nil,
			req:                    &request{BoardName: testBoardName, AnalogReaderName: "analog1", Extra: pbExpectedExtra},
			expCapAnalogReaderArgs: []interface{}{"analog1"},
			expCapArgs:             []interface{}{ctx},
			expResp:                &response{Value: 8},
			expRespErr:             "",
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return tc.injectAnalogReader, tc.injectAnalogReaderOk
			}

			if tc.injectAnalogReader != nil {
				tc.injectAnalogReader.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
					actualExtra = extra
					return tc.injectResult, tc.injectErr
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
			test.That(t, injectBoard.AnalogReaderByNameCap(), test.ShouldResemble, tc.expCapAnalogReaderArgs)
			test.That(t, tc.injectAnalogReader.ReadCap(), test.ShouldResemble, tc.expCapArgs)
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectBoard.WriteAnalogFunc = func(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
				actualExtra = extra
				return tc.injectErr
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

//nolint:dupl
func TestServerGetDigitalInterruptValue(t *testing.T) {
	type request = pb.GetDigitalInterruptValueRequest
	type response = pb.GetDigitalInterruptValueResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		injectDigitalInterrupt     *inject.DigitalInterrupt
		injectDigitalInterruptOk   bool
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
			injectDigitalInterruptOk:   false,
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
			injectDigitalInterruptOk:   false,
			injectResult:               0,
			injectErr:                  nil,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1"},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}(nil),
			expResp:                    nil,
			expRespErr:                 "unknown digital interrupt: digital1",
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
			expRespErr:                 errFoo.Error(),
		},
		{
			injectDigitalInterrupt:     &inject.DigitalInterrupt{},
			injectDigitalInterruptOk:   true,
			injectResult:               42,
			injectErr:                  nil,
			req:                        &request{BoardName: testBoardName, DigitalInterruptName: "digital1", Extra: pbExpectedExtra},
			expCapDigitalInterruptArgs: []interface{}{"digital1"},
			expCapArgs:                 []interface{}{ctx},
			expResp:                    &response{Value: 42},
			expRespErr:                 "",
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
				return tc.injectDigitalInterrupt, tc.injectDigitalInterruptOk
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
		name                     string
		injectDigitalInterrupts  []*inject.DigitalInterrupt
		injectDigitalInterruptOk bool
		streamTicksErr           error
		req                      *request
		expResp                  *response
		expRespErr               string
		sendFail                 bool
	}{
		{
			name:                     "successful stream with multiple interrupts",
			injectDigitalInterrupts:  []*inject.DigitalInterrupt{{}, {}},
			injectDigitalInterruptOk: true,
			streamTicksErr:           nil,
			req:                      &request{Name: testBoardName, PinNames: []string{"digital1", "digital2"}, Extra: pbExpectedExtra},
			expResp:                  &response{PinName: "digital1", Time: uint64(time.Nanosecond), High: true},
			sendFail:                 false,
		},
		{
			name:                     "successful stream with one interrupt",
			injectDigitalInterrupts:  []*inject.DigitalInterrupt{{}},
			injectDigitalInterruptOk: true,
			streamTicksErr:           nil,
			req:                      &request{Name: testBoardName, PinNames: []string{"digital1"}, Extra: pbExpectedExtra},
			expResp:                  &response{PinName: "digital1", Time: uint64(time.Nanosecond), High: true},
			sendFail:                 false,
		},
		{
			name:           "missing board name should return error",
			streamTicksErr: nil,
			req:            &request{Name: missingBoardName, PinNames: []string{"pin1"}},
			expResp:        nil,
			expRespErr:     errNotFound.Error(),
		},
		{
			name:                     "unknown digital interrupt should return error",
			injectDigitalInterrupts:  nil,
			injectDigitalInterruptOk: false,
			streamTicksErr:           errors.New("unknown digital interrupt: digital1"),
			req:                      &request{Name: testBoardName, PinNames: []string{"digital1"}},
			expResp:                  nil,
			expRespErr:               "unknown digital interrupt: digital1",
			sendFail:                 false,
		},
		{
			name:                     "failing to send tick should return error",
			injectDigitalInterrupts:  []*inject.DigitalInterrupt{{}},
			injectDigitalInterruptOk: true,
			streamTicksErr:           nil,
			req:                      &request{Name: testBoardName, PinNames: []string{"digital1"}, Extra: pbExpectedExtra},
			expResp:                  &response{PinName: "digital1", Time: uint64(time.Nanosecond), High: true},
			expRespErr:               "send fail",
			sendFail:                 true,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}
			callbacks := []chan board.Tick{}

			injectBoard.StreamTicksFunc = func(ctx context.Context, interrupts []string, ch chan board.Tick, extra map[string]interface{}) error {
				actualExtra = extra
				callbacks = append(callbacks, ch)
				return tc.streamTicksErr
			}

			injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
				if name == "digital1" {
					return tc.injectDigitalInterrupts[0], tc.injectDigitalInterruptOk
				}
				return tc.injectDigitalInterrupts[1], tc.injectDigitalInterruptOk
			}
			if tc.injectDigitalInterrupts != nil {
				for _, i := range tc.injectDigitalInterrupts {
					i.RemoveCallbackFunc = func(c chan board.Tick) {
						for id := range callbacks {
							if callbacks[id] == c {
								// To remove this item, we replace it with the last item in the list, then truncate the
								// list by 1.
								callbacks[id] = callbacks[len(callbacks)-1]
								callbacks = callbacks[:len(callbacks)-1]
								break
							}
						}
					}
				}
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

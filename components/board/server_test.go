package board_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errFoo      = errors.New("whoops")
	errNotFound = errors.New("not found")
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

//nolint:dupl
func TestServerWriteAnalog(t *testing.T) {
	type request = pb.WriteAnalogRequest
	type response = pb.WriteAnalogResponse
	ctx := context.Background()

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}
	pbExpectedExtra, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		name                   string
		injectAnalogWriter     *inject.AnalogWriter
		injectAnalogWriterOk   bool
		injectErr              error
		req                    *request
		expCapAnalogWriterArgs []interface{}
		expCapArgs             []interface{}
		expResp                *response
		expRespErr             string
	}{
		{
			name:                   "Successful analog write",
			injectAnalogWriter:     &inject.AnalogWriter{},
			injectAnalogWriterOk:   true,
			injectErr:              nil,
			req:                    &request{Name: testBoardName, Pin: "analogwriter1", Extra: pbExpectedExtra},
			expCapAnalogWriterArgs: []interface{}{"analogwriter1"},
			expCapArgs:             []interface{}{ctx},
			expResp:                &response{},
			expRespErr:             "",
		},
		{
			name:                   "Analog write called on a board that does not exist should return not found error",
			injectAnalogWriter:     nil,
			injectAnalogWriterOk:   false,
			injectErr:              nil,
			req:                    &request{Name: missingBoardName},
			expCapAnalogWriterArgs: []interface{}(nil),
			expCapArgs:             []interface{}(nil),
			expResp:                nil,
			expRespErr:             errNotFound.Error(),
		},
		{
			name:                   "Analog write called on a nonexistent analog writer should return unknown analog error",
			injectAnalogWriter:     nil,
			injectAnalogWriterOk:   false,
			injectErr:              nil,
			req:                    &request{Name: testBoardName, Pin: "analogwriter1"},
			expCapAnalogWriterArgs: []interface{}{"analogwriter1"},
			expCapArgs:             []interface{}(nil),
			expResp:                nil,
			expRespErr:             "unknown analog writer: analogwriter1",
		},
		{
			name:                   "An error on write should be returned",
			injectAnalogWriter:     &inject.AnalogWriter{},
			injectAnalogWriterOk:   true,
			injectErr:              errFoo,
			req:                    &request{Name: testBoardName, Pin: "analogwriter1"},
			expCapAnalogWriterArgs: []interface{}{"analogwriter1"},
			expCapArgs:             []interface{}{ctx},
			expResp:                nil,
			expRespErr:             errFoo.Error(),
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			server, injectBoard, err := newServer()
			test.That(t, err, test.ShouldBeNil)
			var actualExtra map[string]interface{}

			injectBoard.AnalogWriterByNameFunc = func(name string) (board.AnalogWriter, bool) {
				return tc.injectAnalogWriter, tc.injectAnalogWriterOk
			}

			if tc.injectAnalogWriter != nil {
				tc.injectAnalogWriter.WriteFunc = func(ctx context.Context, value int32, extra map[string]interface{}) error {
					actualExtra = extra
					return tc.injectErr
				}
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
			test.That(t, injectBoard.AnalogWriterByNameCap(), test.ShouldResemble, tc.expCapAnalogWriterArgs)
			test.That(t, tc.injectAnalogWriter.WriteCap(), test.ShouldResemble, tc.expCapArgs)
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

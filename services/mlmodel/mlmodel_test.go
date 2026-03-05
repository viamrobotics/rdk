package mlmodel

import (
	"testing"

	servicepb "go.viam.com/api/service/mlmodel/v1"
	"go.viam.com/test"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/ml"
)

func TestTensorRoundTrip(t *testing.T) {
	testCases := []struct {
		name   string
		tensor *tensor.Dense
	}{
		{"int8", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]int8{-1, 2, 3, 4, -5, 6}))},
		{"uint8", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]uint8{1, 2, 3, 4, 5, 6}))},
		{"int16", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]int16{-1, 2, 3, 4, -5, 6}))},
		{"uint16", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]uint16{1, 2, 3, 4, 5, 6}))},
		{"int32", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]int32{-1, 2, 3, 4, -5, 6}))},
		{"uint32", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]uint32{1, 2, 3, 4, 5, 6}))},
		{"int64", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]int64{-1, 2, 3, 4, -5, 6}))},
		{"uint64", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]uint64{1, 2, 3, 4, 5, 6}))},
		{"float32", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]float32{1.1, 2.0, 3.0, 4.0, 5.0, -6.0}))},
		{"float64", tensor.New(tensor.WithShape(2, 3), tensor.WithBacking([]float64{-1.1, 2.0, -3.0, 4.5, 5.0, 6.0}))},
	}

	for _, tensor := range testCases {
		t.Run(tensor.name, func(t *testing.T) {
			resp, err := tensorToProto(tensor.tensor)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp.Shape, test.ShouldHaveLength, 2)
			test.That(t, resp.Shape[0], test.ShouldEqual, 2)
			test.That(t, resp.Shape[1], test.ShouldEqual, 3)
			back, err := ml.CreateNewTensor(resp)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, back.Shape(), test.ShouldResemble, tensor.tensor.Shape())
			test.That(t, back.Data(), test.ShouldResemble, tensor.tensor.Data())
		})
	}
}

// TestEmptyTensorToProto tests that tensors with 0 dimensions (e.g., from models
// that return 0 detections) are handled correctly without panicking.
func TestEmptyTensorToProto(t *testing.T) {
	testCases := []struct {
		name          string
		tensor        *tensor.Dense
		expectedShape []uint64
		expectedType  interface{}
	}{
		{
			name:          "empty_float32_1d",
			tensor:        tensor.New(tensor.WithShape(0), tensor.Of(tensor.Float32)),
			expectedShape: []uint64{0},
			expectedType:  &servicepb.FlatTensor_FloatTensor{},
		},
		{
			name:          "empty_float32_2d",
			tensor:        tensor.New(tensor.WithShape(0, 4), tensor.Of(tensor.Float32)),
			expectedShape: []uint64{0, 4},
			expectedType:  &servicepb.FlatTensor_FloatTensor{},
		},
		{
			name:          "empty_uint8_1d",
			tensor:        tensor.New(tensor.WithShape(0), tensor.Of(tensor.Uint8)),
			expectedShape: []uint64{0},
			expectedType:  &servicepb.FlatTensor_Uint8Tensor{},
		},
		{
			name:          "empty_int32_2d",
			tensor:        tensor.New(tensor.WithShape(0, 10), tensor.Of(tensor.Int32)),
			expectedShape: []uint64{0, 10},
			expectedType:  &servicepb.FlatTensor_Int32Tensor{},
		},
		{
			name:          "empty_float64_3d",
			tensor:        tensor.New(tensor.WithShape(0, 3, 4), tensor.Of(tensor.Float64)),
			expectedShape: []uint64{0, 3, 4},
			expectedType:  &servicepb.FlatTensor_DoubleTensor{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This should not panic
			resp, err := tensorToProto(tc.tensor)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp, test.ShouldNotBeNil)
			test.That(t, resp.Shape, test.ShouldResemble, tc.expectedShape)
			// Verify the tensor type is correct
			test.That(t, resp.Tensor, test.ShouldHaveSameTypeAs, tc.expectedType)
		})
	}
}

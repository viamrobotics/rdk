package mlmodel

import (
	"testing"

	"go.viam.com/test"
	"gorgonia.org/tensor"
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
			back, err := createNewTensor(resp)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, back.Shape(), test.ShouldResemble, tensor.tensor.Shape())
			test.That(t, back.Data(), test.ShouldResemble, tensor.tensor.Data())
		})
	}
}

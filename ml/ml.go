// Package ml provides some fundamental machine learning primitives.
package ml

import (
	"math"
	"unsafe"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/mlmodel/v1"
	"golang.org/x/exp/constraints"
	"gorgonia.org/tensor"
)

// ProtoToTensors takes pb.FlatTensors and turns it into a Tensors map.
func ProtoToTensors(pbft *pb.FlatTensors) (Tensors, error) {
	if pbft == nil {
		return nil, errors.New("protobuf FlatTensors is nil")
	}
	tensors := Tensors{}
	for name, ftproto := range pbft.Tensors {
		t, err := CreateNewTensor(ftproto)
		if err != nil {
			return nil, err
		}
		tensors[name] = t
	}
	return tensors, nil
}

// CreateNewTensor turns a proto FlatTensor into a *tensor.Dense.
func CreateNewTensor(pft *pb.FlatTensor) (*tensor.Dense, error) {
	shape := make([]int, 0, len(pft.Shape))
	for _, s := range pft.Shape {
		shape = append(shape, int(s))
	}
	pt := pft.Tensor
	switch t := pt.(type) {
	case *pb.FlatTensor_Int8Tensor:
		data := t.Int8Tensor
		if data == nil {
			return nil, errors.New("tensor of type Int8Tensor is nil")
		}
		dataSlice := data.GetData()
		unsafeInt8Slice := *(*[]int8)(unsafe.Pointer(&dataSlice)) //nolint:gosec
		int8Slice := make([]int8, 0, len(dataSlice))
		int8Slice = append(int8Slice, unsafeInt8Slice...)
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(int8Slice)), nil
	case *pb.FlatTensor_Uint8Tensor:
		data := t.Uint8Tensor
		if data == nil {
			return nil, errors.New("tensor of type Uint8Tensor is nil")
		}
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(data.GetData())), nil
	case *pb.FlatTensor_Int16Tensor:
		data := t.Int16Tensor
		if data == nil {
			return nil, errors.New("tensor of type Int16Tensor is nil")
		}
		int16Data := uint32ToInt16(data.GetData())
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(int16Data)), nil
	case *pb.FlatTensor_Uint16Tensor:
		data := t.Uint16Tensor
		if data == nil {
			return nil, errors.New("tensor of type Uint16Tensor is nil")
		}
		uint16Data := uint32ToUint16(data.GetData())
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(uint16Data)), nil
	case *pb.FlatTensor_Int32Tensor:
		data := t.Int32Tensor
		if data == nil {
			return nil, errors.New("tensor of type Int32Tensor is nil")
		}
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(data.GetData())), nil
	case *pb.FlatTensor_Uint32Tensor:
		data := t.Uint32Tensor
		if data == nil {
			return nil, errors.New("tensor of type Uint32Tensor is nil")
		}
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(data.GetData())), nil
	case *pb.FlatTensor_Int64Tensor:
		data := t.Int64Tensor
		if data == nil {
			return nil, errors.New("tensor of type Int64Tensor is nil")
		}
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(data.GetData())), nil
	case *pb.FlatTensor_Uint64Tensor:
		data := t.Uint64Tensor
		if data == nil {
			return nil, errors.New("tensor of type Uint64Tensor is nil")
		}
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(data.GetData())), nil
	case *pb.FlatTensor_FloatTensor:
		data := t.FloatTensor
		if data == nil {
			return nil, errors.New("tensor of type FloatTensor is nil")
		}
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(data.GetData())), nil
	case *pb.FlatTensor_DoubleTensor:
		data := t.DoubleTensor
		if data == nil {
			return nil, errors.New("tensor of type DoubleTensor is nil")
		}
		return tensor.New(tensor.WithShape(shape...), tensor.WithBacking(data.GetData())), nil
	default:
		return nil, errors.Errorf("don't know how to create tensor.Dense from proto type %T", pt)
	}
}

func uint32ToInt16(uint32Slice []uint32) []int16 {
	int16Slice := make([]int16, len(uint32Slice))

	for i, value := range uint32Slice {
		int16Slice[i] = int16(value)
	}
	return int16Slice
}

func uint32ToUint16(uint32Slice []uint32) []uint16 {
	uint16Slice := make([]uint16, len(uint32Slice))

	for i, value := range uint32Slice {
		uint16Slice[i] = uint16(value)
	}
	return uint16Slice
}

// number interface for converting between numbers.
type number interface {
	constraints.Integer | constraints.Float
}

// convertNumberSlice converts any number slice into another number slice.
func convertNumberSlice[T1, T2 number](t1 []T1) []T2 {
	t2 := make([]T2, len(t1))
	for i := range t1 {
		t2[i] = T2(t1[i])
	}
	return t2
}

// ConvertToFloat64Slice converts any numbers or slice of numbers into a float64 slice.
func ConvertToFloat64Slice(slice interface{}) ([]float64, error) {
	switch v := slice.(type) {
	case []float64:
		return v, nil
	case float64:
		return []float64{v}, nil
	case []float32:
		return convertNumberSlice[float32, float64](v), nil
	case float32:
		return convertNumberSlice[float32, float64]([]float32{v}), nil
	case []int:
		return convertNumberSlice[int, float64](v), nil
	case int:
		return convertNumberSlice[int, float64]([]int{v}), nil
	case []uint:
		return convertNumberSlice[uint, float64](v), nil
	case uint:
		return convertNumberSlice[uint, float64]([]uint{v}), nil
	case []int8:
		return convertNumberSlice[int8, float64](v), nil
	case int8:
		return convertNumberSlice[int8, float64]([]int8{v}), nil
	case []int16:
		return convertNumberSlice[int16, float64](v), nil
	case int16:
		return convertNumberSlice[int16, float64]([]int16{v}), nil
	case []int32:
		return convertNumberSlice[int32, float64](v), nil
	case int32:
		return convertNumberSlice[int32, float64]([]int32{v}), nil
	case []int64:
		return convertNumberSlice[int64, float64](v), nil
	case int64:
		return convertNumberSlice[int64, float64]([]int64{v}), nil
	case []uint8:
		return convertNumberSlice[uint8, float64](v), nil
	case uint8:
		return convertNumberSlice[uint8, float64]([]uint8{v}), nil
	case []uint16:
		return convertNumberSlice[uint16, float64](v), nil
	case uint16:
		return convertNumberSlice[uint16, float64]([]uint16{v}), nil
	case []uint32:
		return convertNumberSlice[uint32, float64](v), nil
	case uint32:
		return convertNumberSlice[uint32, float64]([]uint32{v}), nil
	case []uint64:
		return convertNumberSlice[uint64, float64](v), nil
	case uint64:
		return convertNumberSlice[uint64, float64]([]uint64{v}), nil
	default:
		return nil, errors.Errorf("dont know how to convert slice of %T into a []float64", slice)
	}
}

// softmax takes the input slice and applies the softmax function.
func softmax(in []float64) []float64 {
	out := make([]float64, 0, len(in))
	bigSum := 0.0
	for _, x := range in {
		bigSum += math.Exp(x)
	}
	for _, x := range in {
		out = append(out, math.Exp(x)/bigSum)
	}
	return out
}

// TensorNames returns all the names of the tensors.
func TensorNames(t Tensors) []string {
	names := []string{}
	for name := range t {
		names = append(names, name)
	}
	return names
}

// GetIndex returns the index of an int in an array of ints
// Will return -1 if it's not there.
func GetIndex(s []int, num int) int {
	for i, v := range s {
		if v == num {
			return i
		}
	}
	return -1
}

package mlmodel

import (
	"context"
	"unsafe"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/mlmodel/v1"
	"go.viam.com/utils/rpc"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/resource"
)

// client implements MLModelServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	conn   rpc.ClientConn
	client pb.MLModelServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.ZapCompatibleLogger,
) (Service, error) {
	grpcClient := pb.NewMLModelServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		conn:   conn,
		client: grpcClient,
		logger: logging.FromZapCompatible(logger),
	}
	return c, nil
}

func (c *client) Infer(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error) {
	tensorProto, err := TensorsToProto(tensors)
	if err != nil {
		return nil, err
	}
	if tensors == nil {
		tensorProto = nil
	}
	resp, err := c.client.Infer(ctx, &pb.InferRequest{
		Name:         c.name,
		InputTensors: tensorProto,
	})
	if err != nil {
		return nil, err
	}
	tensorResp, err := ProtoToTensors(resp.OutputTensors)
	if err != nil {
		return nil, err
	}
	return tensorResp, nil
}

// ProtoToTensors takes pb.FlatTensors and turns it into a Tensors map.
func ProtoToTensors(pbft *pb.FlatTensors) (ml.Tensors, error) {
	if pbft == nil {
		return nil, errors.New("protobuf FlatTensors is nil")
	}
	tensors := ml.Tensors{}
	for name, ftproto := range pbft.Tensors {
		t, err := createNewTensor(ftproto)
		if err != nil {
			return nil, err
		}
		tensors[name] = t
	}
	return tensors, nil
}

// createNewTensor turns a proto FlatTensor into a *tensor.Dense.
func createNewTensor(pft *pb.FlatTensor) (*tensor.Dense, error) {
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

func (c *client) Metadata(ctx context.Context) (MLMetadata, error) {
	resp, err := c.client.Metadata(ctx, &pb.MetadataRequest{
		Name: c.name,
	})
	if err != nil {
		return MLMetadata{}, err
	}
	return protoToMetadata(resp.Metadata), nil
}

// protoToMetadata takes a pb.Metadata protobuf message and turns it into an MLMetadata struct.
func protoToMetadata(pbmd *pb.Metadata) MLMetadata {
	metadata := MLMetadata{
		ModelName:        pbmd.Name,
		ModelType:        pbmd.Type,
		ModelDescription: pbmd.Description,
	}
	inputData := make([]TensorInfo, 0, len(pbmd.InputInfo))
	for _, idproto := range pbmd.InputInfo {
		inputData = append(inputData, protoToTensorInfo(idproto))
	}
	metadata.Inputs = inputData
	outputData := make([]TensorInfo, 0, len(pbmd.OutputInfo))
	for _, odproto := range pbmd.OutputInfo {
		outputData = append(outputData, protoToTensorInfo(odproto))
	}
	metadata.Outputs = outputData
	return metadata
}

// protoToTensorInfo takes a pb.TensorInfo protobuf message and turns it into an TensorInfo struct.
func protoToTensorInfo(pbti *pb.TensorInfo) TensorInfo {
	ti := TensorInfo{
		Name:        pbti.Name,
		Description: pbti.Description,
		DataType:    pbti.DataType,
		Extra:       pbti.Extra.AsMap(),
	}
	associatedFiles := make([]File, 0, len(pbti.AssociatedFiles))
	for _, afproto := range pbti.AssociatedFiles {
		associatedFiles = append(associatedFiles, protoToFile(afproto))
	}
	shape := make([]int, 0, len(pbti.Shape))
	for _, s := range pbti.Shape {
		shape = append(shape, int(s))
	}
	ti.Shape = shape
	ti.AssociatedFiles = associatedFiles
	return ti
}

// protoToFile takes a pb.File protobuf message and turns it into an File struct.
func protoToFile(pbf *pb.File) File {
	f := File{
		Name:        pbf.Name,
		Description: pbf.Description,
	}
	switch pbf.LabelType {
	case pb.LabelType_LABEL_TYPE_UNSPECIFIED:
		f.LabelType = LabelTypeUnspecified
	case pb.LabelType_LABEL_TYPE_TENSOR_VALUE:
		f.LabelType = LabelTypeTensorValue
	case pb.LabelType_LABEL_TYPE_TENSOR_AXIS:
		f.LabelType = LabelTypeTensorAxis
	default:
		// this should never happen as long as all possible enums are included in the switch
		f.LabelType = LabelTypeUnspecified
	}
	return f
}

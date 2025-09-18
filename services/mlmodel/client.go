package mlmodel

import (
	"context"

	pb "go.viam.com/api/service/mlmodel/v1"
	"go.viam.com/utils/rpc"

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
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewMLModelServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
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
	tensorResp, err := ml.ProtoToTensors(resp.OutputTensors)
	if err != nil {
		return nil, err
	}
	return tensorResp, nil
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

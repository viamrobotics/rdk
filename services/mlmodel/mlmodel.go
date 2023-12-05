// Package mlmodel defines the client and server for a service that can take in a map of
// input tensors/arrays, pass them through an inference engine, and then return a map output tensors/arrays.
package mlmodel

import (
	"context"
	"unsafe"

	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/mlmodel/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterMLModelServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.MLModelService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// Service defines the ML Model interface, which takes a map of inputs, runs it through
// an inference engine, and creates a map of outputs. Metadata is necessary in order to build
// the struct that will decode that map[string]interface{} correctly.
type Service interface {
	resource.Resource
	Infer(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error)
	Metadata(ctx context.Context) (MLMetadata, error)
}

// TensorsToProto turns the ml.Tensors map into a protobuf message of FlatTensors.
func TensorsToProto(ts ml.Tensors) (*servicepb.FlatTensors, error) {
	pbts := &servicepb.FlatTensors{
		Tensors: make(map[string]*servicepb.FlatTensor),
	}
	for name, t := range ts {
		tp, err := tensorToProto(t)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert tensor to proto message")
		}
		pbts.Tensors[name] = tp
	}
	return pbts, nil
}

func tensorToProto(t *tensor.Dense) (*servicepb.FlatTensor, error) {
	ftpb := &servicepb.FlatTensor{}
	shape := t.Shape()
	for _, s := range shape {
		ftpb.Shape = append(ftpb.Shape, uint64(s))
	}
	// switch on data type of the underlying array
	data := t.Data()
	switch dataSlice := data.(type) {
	case []int8:
		unsafeByteSlice := *(*[]byte)(unsafe.Pointer(&dataSlice)) //nolint:gosec
		data := &servicepb.FlatTensorDataInt8{}
		data.Data = append(data.Data, unsafeByteSlice...)
		ftpb.Tensor = &servicepb.FlatTensor_Int8Tensor{Int8Tensor: data}
	case []uint8:
		ftpb.Tensor = &servicepb.FlatTensor_Uint8Tensor{
			Uint8Tensor: &servicepb.FlatTensorDataUInt8{
				Data: dataSlice,
			},
		}
	case []int16:
		ftpb.Tensor = &servicepb.FlatTensor_Int16Tensor{
			Int16Tensor: &servicepb.FlatTensorDataInt16{
				Data: int16ToUint32(dataSlice),
			},
		}
	case []uint16:
		ftpb.Tensor = &servicepb.FlatTensor_Uint16Tensor{
			Uint16Tensor: &servicepb.FlatTensorDataUInt16{
				Data: uint16ToUint32(dataSlice),
			},
		}
	case []int32:
		ftpb.Tensor = &servicepb.FlatTensor_Int32Tensor{
			Int32Tensor: &servicepb.FlatTensorDataInt32{
				Data: dataSlice,
			},
		}
	case []uint32:
		ftpb.Tensor = &servicepb.FlatTensor_Uint32Tensor{
			Uint32Tensor: &servicepb.FlatTensorDataUInt32{
				Data: dataSlice,
			},
		}
	case []int64:
		ftpb.Tensor = &servicepb.FlatTensor_Int64Tensor{
			Int64Tensor: &servicepb.FlatTensorDataInt64{
				Data: dataSlice,
			},
		}
	case []uint64:
		ftpb.Tensor = &servicepb.FlatTensor_Uint64Tensor{
			Uint64Tensor: &servicepb.FlatTensorDataUInt64{
				Data: dataSlice,
			},
		}
	case []int:
		unsafeInt64Slice := *(*[]int64)(unsafe.Pointer(&dataSlice)) //nolint:gosec
		data := &servicepb.FlatTensorDataInt64{}
		data.Data = append(data.Data, unsafeInt64Slice...)
		ftpb.Tensor = &servicepb.FlatTensor_Int64Tensor{Int64Tensor: data}
	case []uint:
		unsafeUint64Slice := *(*[]uint64)(unsafe.Pointer(&dataSlice)) //nolint:gosec
		data := &servicepb.FlatTensorDataUInt64{}
		data.Data = append(data.Data, unsafeUint64Slice...)
		ftpb.Tensor = &servicepb.FlatTensor_Uint64Tensor{Uint64Tensor: data}
	case []float32:
		ftpb.Tensor = &servicepb.FlatTensor_FloatTensor{
			FloatTensor: &servicepb.FlatTensorDataFloat{
				Data: dataSlice,
			},
		}
	case []float64:
		ftpb.Tensor = &servicepb.FlatTensor_DoubleTensor{
			DoubleTensor: &servicepb.FlatTensorDataDouble{
				Data: dataSlice,
			},
		}
	default:
		return nil, errors.Errorf("cannot turn underlying tensor data of type %T into proto message", dataSlice)
	}
	return ftpb, nil
}

func int16ToUint32(int16Slice []int16) []uint32 {
	uint32Slice := make([]uint32, len(int16Slice))

	for i, value := range int16Slice {
		uint32Slice[i] = uint32(value)
	}
	return uint32Slice
}

func uint16ToUint32(uint16Slice []uint16) []uint32 {
	uint32Slice := make([]uint32, len(uint16Slice))

	for i, value := range uint16Slice {
		uint32Slice[i] = uint32(value)
	}
	return uint32Slice
}

// MLMetadata contains the metadata of the model file, such as the name of the model, what
// kind of model it is, and the expected tensor/array shape and types of the inputs and outputs of the model.
type MLMetadata struct {
	ModelName        string
	ModelType        string // e.g. object_detector, text_classifier
	ModelDescription string
	Inputs           []TensorInfo
	Outputs          []TensorInfo
}

// toProto turns the MLMetadata struct into a protobuf message.
func (mm MLMetadata) toProto() (*servicepb.Metadata, error) {
	pbmm := &servicepb.Metadata{
		Name:        mm.ModelName,
		Type:        mm.ModelType,
		Description: mm.ModelDescription,
	}
	inputInfo := make([]*servicepb.TensorInfo, 0, len(mm.Inputs))
	for _, inp := range mm.Inputs {
		inproto, err := inp.toProto()
		if err != nil {
			return nil, err
		}
		inputInfo = append(inputInfo, inproto)
	}
	pbmm.InputInfo = inputInfo
	outputInfo := make([]*servicepb.TensorInfo, 0, len(mm.Outputs))
	for _, outp := range mm.Outputs {
		outproto, err := outp.toProto()
		if err != nil {
			return nil, err
		}
		outputInfo = append(outputInfo, outproto)
	}
	pbmm.OutputInfo = outputInfo
	return pbmm, nil
}

// TensorInfo contains all the information necessary to build a struct from the input and output maps.
// it describes the name of the output field, what data type it has, and how many dimensions the
// array/tensor will have. AssociatedFiles points to where more information is located, e.g. in case the ints
// within the array/tensor need to be converted into a string.
type TensorInfo struct {
	Name            string // e.g. bounding_boxes
	Description     string
	DataType        string // e.g. uint8, float32, int
	Shape           []int  // number of dimensions in the array
	AssociatedFiles []File
	Extra           map[string]interface{}
}

// toProto turns the TensorInfo struct into a protobuf message.
func (tf TensorInfo) toProto() (*servicepb.TensorInfo, error) {
	pbtf := &servicepb.TensorInfo{
		Name:        tf.Name,
		Description: tf.Description,
		DataType:    tf.DataType,
	}
	shape := make([]int32, 0, len(tf.Shape))
	for _, s := range tf.Shape {
		shape = append(shape, int32(s))
	}
	pbtf.Shape = shape
	associatedFiles := make([]*servicepb.File, 0, len(tf.AssociatedFiles))
	for _, af := range tf.AssociatedFiles {
		associatedFiles = append(associatedFiles, af.toProto())
	}
	pbtf.AssociatedFiles = associatedFiles
	extra, err := vprotoutils.StructToStructPb(tf.Extra)
	if err != nil {
		return nil, err
	}
	pbtf.Extra = extra
	return pbtf, nil
}

// File contains information about how to interpret the numbers within the tensor/array. The label type
// describes how to read the tensor in order to successfully label the numbers.
type File struct {
	Name        string // e.g. category_labels.txt
	Description string
	LabelType   LabelType // TENSOR_VALUE, or TENSOR_AXIS
}

// toProto turns the File struct into a protobuf message.
func (f File) toProto() *servicepb.File {
	pbf := &servicepb.File{
		Name:        f.Name,
		Description: f.Description,
	}
	switch f.LabelType {
	case LabelTypeUnspecified, "":
		pbf.LabelType = servicepb.LabelType_LABEL_TYPE_UNSPECIFIED
	case LabelTypeTensorValue:
		pbf.LabelType = servicepb.LabelType_LABEL_TYPE_TENSOR_VALUE
	case LabelTypeTensorAxis:
		pbf.LabelType = servicepb.LabelType_LABEL_TYPE_TENSOR_AXIS
	default:
		// if we don't know the label type, then just assign unspecified
		pbf.LabelType = servicepb.LabelType_LABEL_TYPE_UNSPECIFIED
	}
	return pbf
}

// LabelType describes how labels from the file are assigned to the tensors. TENSOR_VALUE means that
// labels are the actual value in the tensor. TENSOR_AXIS means that labels are positional within the
// tensor axis.
type LabelType string

const (
	// LabelTypeUnspecified means the label type is not known.
	LabelTypeUnspecified = LabelType("UNSPECIFIED")
	// LabelTypeTensorValue means the labels are assigned by the actual value in the tensor
	// e.g. for 4 results and 3 categories : [0, 1, 2, 1].
	LabelTypeTensorValue = LabelType("TENSOR_VALUE")
	// LabelTypeTensorAxis means labels are assigned by the position within the tensor axis
	// e.g. for 4 results and 3 categories : [[.8, .1, .1], [.2, .7, .1], [.1, .1, .8],[.05, .9, .05]].
	LabelTypeTensorAxis = LabelType("TENSOR_AXIS")
)

// SubtypeName is the name of the type of service.
const SubtypeName = "mlmodel"

// API is a variable that identifies the ML model service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named ML model service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named ML model service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

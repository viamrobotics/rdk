// Package mlmodel defines the client and server for a service that can take in a map of
// input tensors/arrays, pass them through an inference engine, and then return a map output tensors/arrays.
package mlmodel

import (
	"context"

	"github.com/edaniels/golog"
	servicepb "go.viam.com/api/service/mlmodel/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.MLModelService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterMLModelServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.MLModelService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		MaxInstance: resource.DefaultMaxInstance,
	})
}

// Service defines the ML Model interface, which takes a map of inputs, runs it through
// an inference engine, and creates a map of outputs. Metadata is necessary in order to build
// the struct that will decode that map[string]interface{} correctly.
type Service interface {
	Infer(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	Metadata(ctx context.Context) (MLMetadata, error)
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
	associatedFiles := make([]*servicepb.File, 0, len(tf.AssociatedFiles))
	for _, af := range tf.AssociatedFiles {
		associatedFiles = append(associatedFiles, af.toProto())
	}
	pbtf.AssociatedFiles = associatedFiles
	shape := make([]int32, 0, len(tf.Shape))
	for _, s := range tf.Shape {
		shape = append(shape, int32(s))
	}
	pbtf.Shape = shape
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
const SubtypeName = resource.SubtypeName("mlmodel")

// Subtype is a constant that identifies the ML model service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named ML model service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named ML model service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Service)(nil), actual)
}

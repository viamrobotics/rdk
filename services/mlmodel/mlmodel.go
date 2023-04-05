package mlmodel

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/mlmodel/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
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
		Reconfigurable: WrapWithReconfigurable,
		MaxInstance:    resource.DefaultMaxInstance,
	})
}

type Service interface {
	Infer(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	Metadata(ctx context.Context) (ModelMetadata, error)
}

type ModelMetadata struct {
	modelName        string
	modelType        string // e.g. object_detector, text_classifier
	modelDescription string
	inputs           []TensorInfo
	outputs          []TensorInfo
}

func (mm ModelMetadata) ToProto() (*servicepb.Metadata, error) {
	pbmm := &servicepb.Metadata{
		Name:        mm.modelName,
		Type:        mm.modelType,
		Description: mm.modelDescription,
	}
	inputInfo := make([]*servicepb.TensorInfo, 0, len(mm.inputs))
	for _, inp := range mm.inputs {
		inproto, err := inp.ToProto()
		if err != nil {
			return nil, err
		}
		inputInfo = append(inputInfo, inproto)
	}
	pbmm.InputInfo = inputInfo
	outputInfo := make([]*servicepb.TensorInfo, 0, len(mm.output))
	for _, outp := range mm.outputs {
		outproto, err := outp.ToProto()
		if err != nil {
			return nil, err
		}
		outputInfo = append(outputInfo, outproto)
	}
	pbmm.OutputInfo = outputInfo
	return pbmm, nil
}

type TensorInfo struct {
	name            string // e.g. bounding_boxes
	description     string
	dataType        string // e.g. uint8, float32, int
	nDim            int    // number of dimensions in the array
	associatedFiles []File
	extra           map[string]interface{}
}

func (tf TensorInfo) ToProto() (*servicepb.TensorInfo, error) {
	pbtf := &servicepb.TensorInfo{
		Name:        tf.name,
		Description: tf.description,
		DataType:    tf.dataType,
		NDim:        tf.nDim,
	}
	associatedFiles := make([]*servicepb.File, 0, len(tf.associatedFiles))
	for _, af := range mm.associatedFiles {
		afproto, err := af.ToProto()
		if err != nil {
			return nil, err
		}
		associatedFiles = append(associatedFiles, afproto)
	}
	pbtf.AssociatedFiles = associatedFiles
	extra, err := structpb.NewStruct(af.extra)
	if err != nil {
		return nil, err
	}
	pbtf.Extra = extra
	return pbtf, nil
}

type File struct {
	name        string // e.g. category_labels.txt
	description string
	labelType   LabelType // TENSOR_VALUE, or TENSOR_AXIS
}

func (f File) ToProto() (*servicepb.File, error) {
	pbf := &servicepb.File{
		Name:        f.name,
		Description: f.description,
	}
	switch f.labelType {
	case LabelTypeUnspecified:
		pbf.LabelType = servicepb.LABEL_TYPE_UNSPECIFIED
	case LabelTypeTensorValue:
		pbf.LabelType = servicepb.LABEL_TYPE_TENSOR_VALUE
	case LabelTypeTensorAxis:
		pbf.LabelType = servicepb.LABEL_TYPE_TENSOR_AXIS
	default:
		return nil, errors.Errorf("do not know about label type %q in ML Model protobuf", f.labelType)
	}
	return pbf, nil
}

// LabelType describes how labels from the file are assigned to the sensors. TENSOR_VALUE means that
// labels are the actual value in the tensor. TENSOR_AXIS means that labels are positional within the
// tensor axis.
type LabelType string

const (
	LabelTypeUnspecified = LabelType("UNSPECIFIED")
	LabelTypeTensorValue = LabelType("TENSOR_VALUE")
	LabelTypeTensorAxis  = LabelType("TENSOR_AXIS")
)

var (
	_ = Service(&reconfigurableMLModel{})
	_ = resource.Reconfigurable(&reconfigurableMLModel{})
	_ = goutils.ContextCloser(&reconfigurableMLModel{})
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

type reconfigurableMLModelService struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Service
}

func (svc *reconfigurableMLModelService) Name() resource.Name {
	return svc.name
}

func (svc *reconfigurableMLModelService) Infer(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Infer(ctx, input)
}

func (svc *reconfigurableMLModelService) Metadata(ctx context.Context) (ModelMetadata, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Metadata(ctx)
}

// Reconfigure replaces the old ML Model Service with a new ML Model Service.
func (svc *reconfigurableMLModelService) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableMLModelService)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a ML Model Service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}, name resource.Name) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableMLModelService); ok {
		return reconfigurable, nil
	}

	return &reconfigurableMLModelService{name: name, actual: svc}, nil
}

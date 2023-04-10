// Package vision is the service that allows you to access various computer vision algorithms
// (like detection, segmentation, tracking, etc) that usually only require a camera or image input.
package vision

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	servicepb "go.viam.com/api/service/vision/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	goutils "go.viam.com/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.VisionService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterVisionServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.VisionService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// A Service that implements various computer vision algorithms like detection and segmentation.
type Service interface {
	DetectionsFromCamera(ctx context.Context, cameraName string, extra map[string]interface{}) ([]objectdetection.Detection, error)
	Detections(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error)
	// classifier methods
	ClassificationsFromCamera(
		ctx context.Context,
		cameraName string,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)
	Classifications(
		ctx context.Context,
		img image.Image,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)
	// segmenter methods
	GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error)
	resource.Generic
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Service)(nil), actual)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("vision")

// Subtype is a constant that identifies the vision service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named vision's typed resource name.
// RSDK-347 Implements vision's Named.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named vision service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// Detector defines the Detect method as it is used in the objectdetection package
type Detector interface {
	Detect(ctx context.Context, img image.Image) ([]objectdetection.Detection, error)
}

// Segmenter3D defines the Segment method as it is used in the segmentation package
type Segmenter3D interface {
	Segment(ctx context.Context, c camera.Camera) ([]*vision.Object, error)
}

// Classifier defines the Classify method as it is used in the classification package
type Classifier interface {
	Classify(context.Context, image.Image) (classification.Classifications, error)
}

// vizModel wraps the vision model with all the service interface methods.
type vizModel struct {
	generic.Unimplemented
	name          string
	model         interface{} // can be any combo of detector, classifier, 3D_segmenter
	r             robot.Robot // in order to get access to all cameras
	isDetector    bool
	isClassifier  bool
	is3DSegmenter bool
}

// NewService wraps the vision model in the struct that fulfills the vision service interface.
func NewService(name string, model interface{}, r robot.Robot) (Service, error) {
	_, isDetector := model.(Detector)
	_, isClassifier := model.(Classifier)
	_, is3DSegmenter := model.(Segmenter3D)
	if !isDetector && !isClassifier && !is3DSegmenter {
		return nil, errors.New("model does not fulfill any method of the vision service. It is neither a detector, nor classifier, nor 3D segmenter.")
	}
	return &vizModel{
		name:          name,
		model:         model,
		r:             r,
		isDetector:    isDetector,
		isClassifier:  isClassifier,
		is3DSegmenter: is3DSegmenter,
	}, nil
}

// Detections returns the detections of given image if the model implements objectdetector.Detector.
func (vm *vizModel) Detections(
	ctx context.Context,
	img image.Image,
	extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Detections::"+vm.name)
	defer span.End()
	if detector, ok := vm.model.(Detector); ok {
		return detector.Detect(ctx, img)
	}
	return nil, errors.Errorf("vision model %q does not implement a Detector", vm.name)
}

// DetectionsFromCamera returns the detections of the next image from the given camera.
func (vm *vizModel) DetectionsFromCamera(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::DetectionsFromCamera::"+vm.name)
	defer span.End()
	if !vm.isDetector {
		return nil, errors.Errorf("vision model %q does not implement a Detector", vm.name)
	}
	cam, err := camera.FromRobot(vm.r, cameraName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find camera named %s", cameraName)
	}
	img, release, err := camera.ReadImage(ctx, cam)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get image from %s", cameraName)
	}
	defer release()
	if detector, ok := vm.model.(Detector); ok {
		return detector.Detect(ctx, img)
	}
	return nil, errors.Errorf("vision model %q does not implement a Detector", vm.name)
}

// Classifications returns the classifications of given image if the model implements classifications.Classifier
func (vm *vizModel) Classifications(
	ctx context.Context,
	img image.Image,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Classifications::"+vm.name)
	defer span.End()
	if classifier, ok := vm.model.(Classifier); ok {
		fullClassifications, err := classifier.Classify(ctx, img)
		if err != nil {
			return nil, errors.Wrap(err, "could not get classifications from image")
		}
		return fullClassifications.TopN(n)
	}
	return nil, errors.Errorf("vision model %q does not implement a Classifier", vm.name)
}

// ClassificationsFromCamera returns the classifications of the next image from the given camera.
func (vm *vizModel) ClassificationsFromCamera(
	ctx context.Context,
	cameraName string,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::ClassificationsFromCamera::"+vm.name)
	defer span.End()
	if !vm.isClassifier {
		return nil, errors.Errorf("vision model %q does not implement a Classifier", vm.name)
	}
	cam, err := camera.FromRobot(vm.r, cameraName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find camera named %s", cameraName)
	}
	img, release, err := camera.ReadImage(ctx, cam)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get image from %s", cameraName)
	}
	defer release()
	if classifier, ok := vm.model.(Classifier); ok {
		fullClassifications, err := classifier.Classify(ctx, img)
		if err != nil {
			return nil, errors.Wrap(err, "could not get classifications from image")
		}
		return fullClassifications.TopN(n)
	}
	return nil, errors.Errorf("vision model %q does not implement a Classifier", vm.name)
}

// GetObjectPointClouds returns all the found objects in a 3D image if the model implements Segmenter3D
func (vm *vizModel) GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetObjectPointClouds::"+vm.name)
	defer span.End()
	cam, err := camera.FromRobot(vm.r, cameraName)
	if err != nil {
		return nil, err
	}
	if segmenter, ok := vm.model.(Segmenter3D); ok {
		return segmenter.Segment(ctx, cam)
	}
	return nil, errors.Errorf("vision model %q does not implement a 3D segmenter", vm.name)
}

func (vm *vizModel) Close(ctx context.Context) error {
	return goutils.TryClose(ctx, vm.model)
}

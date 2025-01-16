// Package vision is the service that allows you to access various computer vision algorithms
// (like detection, segmentation, tracking, etc) that usually only require a camera or image input.
// For more information, see the [vision service docs].
//
// [vision service docs]: https://docs.viam.com/services/vision/
package vision

import (
	"context"
	"image"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	servicepb "go.viam.com/api/service/vision/v1"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
	"go.viam.com/rdk/vision/viscapture"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterVisionServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.VisionService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: captureAllFromCamera.String(),
	}, newCaptureAllFromCameraCollector)
}

// A Service implements various computer vision algorithms like detection and segmentation.
// For more information, see the [vision service docs].
//
// DetectionsFromCamera example:
//
//	myDetectorService, err := vision.FromRobot(machine, "my_detector")
//	if err != nil {
//		logger.Error(err)
//		return
//	}
//
//	// Get detections from the camera output
//	detections, err := myDetectorService.DetectionsFromCamera(context.Background(), "my_camera", nil)
//	if err != nil {
//		logger.Fatalf("Could not get detections: %v", err)
//	}
//	if len(detections) > 0 {
//		logger.Info(detections[0])
//	}
//
// For more information, see the [DetectionsFromCamera method docs].
//
// Detections example:
//
//	 // add "go.viam.com/rdk/utils" to imports to use this code snippet
//
//		myCam, err := camera.FromRobot(machine, "my_camera")
//		if err != nil {
//			logger.Error(err)
//			return
//		}
//		// Get an image from the camera decoded as an image.Image
//		img, err = camera.DecodeImageFromCamera(context.Background(), utils.MimeTypeJPEG, nil, myCam)
//
//		myDetectorService, err := vision.FromRobot(machine, "my_detector")
//		if err != nil {
//			logger.Error(err)
//			return
//		}
//		// Get the detections from the image
//		detections, err := myDetectorService.Detections(context.Background(), img, nil)
//		if err != nil {
//			logger.Fatalf("Could not get detections: %v", err)
//		}
//		if len(detections) > 0 {
//			logger.Info(detections[0])
//		}
//
// For more information, see the [Detections method docs].
//
// ClassificationsFromCamera example:
//
//	myClassifierService, err := vision.FromRobot(machine, "my_classifier")
//	if err != nil {
//		logger.Error(err)
//		return
//	}
//	// Get the 2 classifications with the highest confidence scores from the camera output
//	classifications, err := myClassifierService.ClassificationsFromCamera(context.Background(), "my_camera", 2, nil)
//	if err != nil {
//		logger.Fatalf("Could not get classifications: %v", err)
//	}
//	if len(classifications) > 0 {
//		logger.Info(classifications[0])
//	}
//
// For more information, see the [ClassificationsFromCamera method docs].
//
// Classifications example:
//
//	 // add "go.viam.com/rdk/utils" to imports to use this code snippet
//
//		myCam, err := camera.FromRobot(machine, "my_camera")
//		if err != nil {
//			logger.Error(err)
//			return
//		}
//		// Get an image from the camera decoded as an image.Image
//		img, err = camera.DecodeImageFromCamera(context.Background(), utils.MimeTypeJPEG, nil, myCam)
//
//		myClassifierService, err := vision.FromRobot(machine, "my_classifier")
//		if err != nil {
//			logger.Error(err)
//			return
//		}
//		// Get the 2 classifications with the highest confidence scores from the image
//		classifications, err := myClassifierService.Classifications(context.Background(), img, 2, nil)
//		if err != nil {
//			logger.Fatalf("Could not get classifications: %v", err)
//		}
//		if len(classifications) > 0 {
//			logger.Info(classifications[0])
//		}
//
// For more information, see the [Classifications method docs].
//
// GetObjectPointClouds example:
//
//	mySegmenterService, err := vision.FromRobot(machine, "my_segmenter")
//	if err != nil {
//		logger.Error(err)
//		return
//	}
//	// Get the objects from the camera output
//	objects, err := mySegmenterService.GetObjectPointClouds(context.Background(), "my_camera", nil)
//	if err != nil {
//		logger.Fatalf("Could not get point clouds: %v", err)
//	}
//	if len(objects) > 0 {
//		logger.Info(objects[0])
//	}
//
// For more information, see the [GetObjectPointClouds method docs].
//
// CaptureAllFromCamera example:
//
//	// The data to capture and return from the camera
//	captOpts := viscapture.CaptureOptions{
//		ReturnImage: true,
//		ReturnDetections: true,
//	}
//	// Get the captured data for a camera
//	capture, err := visService.CaptureAllFromCamera(context.Background(), "my_camera", captOpts, nil)
//	if err != nil {
//		logger.Fatalf("Could not get capture data from vision service: %v", err)
//	}
//	image := capture.Image
//	detections := capture.Detections
//	classifications := capture.Classifications
//	objects := capture.Objects
//
// For more information, see the [CaptureAllFromCamera method docs].
//
// [vision service docs]: https://docs.viam.com/dev/reference/apis/services/vision/
// [DetectionsFromCamera method docs]: https://docs.viam.com/dev/reference/apis/services/vision/#getdetectionsfromcamera
// [Detections method docs]: https://docs.viam.com/dev/reference/apis/services/vision/#getdetections
// [ClassificationsFromCamera method docs]: https://docs.viam.com/dev/reference/apis/services/vision/#getclassificationsfromcamera
// [Classifications method docs]: https://docs.viam.com/dev/reference/apis/services/vision/#getclassifications
// [GetObjectPointClouds method docs]: https://docs.viam.com/dev/reference/apis/services/vision/#getobjectpointclouds
// [CaptureAllFromCamera method docs]: https://docs.viam.com/dev/reference/apis/services/vision/#captureallfromcamera
type Service interface {
	resource.Resource
	// DetectionsFromCamera returns a list of detections from the next image from a specified camera using a configured detector.
	DetectionsFromCamera(ctx context.Context, cameraName string, extra map[string]interface{}) ([]objectdetection.Detection, error)

	// Detections returns a list of detections from a given image using a configured detector.
	Detections(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error)

	// ClassificationsFromCamera returns a list of classifications from the next image from a specified camera using a configured classifier.
	ClassificationsFromCamera(
		ctx context.Context,
		cameraName string,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)

	// Classifications returns a list of classifications from a given image using a configured classifier.
	Classifications(
		ctx context.Context,
		img image.Image,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)

	// GetObjectPointClouds returns a list of 3D point cloud objects and metadata from the latest 3D camera image using a specified segmenter.
	GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error)
	// properties
	GetProperties(ctx context.Context, extra map[string]interface{}) (*Properties, error)
	// CaptureAllFromCamera returns the next image, detections, classifications, and objects all together, given a camera name. Used for
	// visualization.
	CaptureAllFromCamera(ctx context.Context,
		cameraName string,
		opts viscapture.CaptureOptions,
		extra map[string]interface{},
	) (viscapture.VisCapture, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = "vision"

// API is a variable that identifies the vision service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named vision's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named vision service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FromDependencies is a helper for getting the named vision service from a collection of dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// vizModel wraps the vision model with all the service interface methods.
type vizModel struct {
	resource.Named
	resource.AlwaysRebuild
	r               robot.Robot // in order to get access to all cameras
	properties      Properties
	closerFunc      func(ctx context.Context) error // close the underlying model
	classifierFunc  classification.Classifier
	detectorFunc    objectdetection.Detector
	segmenter3DFunc segmentation.Segmenter
}

// Properties returns various information regarding the current vision service,
// specifically, which vision tasks are supported by the resource.
type Properties struct {
	ClassificationSupported bool
	DetectionSupported      bool
	ObjectPCDsSupported     bool
}

// NewService wraps the vision model in the struct that fulfills the vision service interface.
func NewService(
	name resource.Name,
	r robot.Robot,
	c func(ctx context.Context) error,
	cf classification.Classifier,
	df objectdetection.Detector,
	s3f segmentation.Segmenter,
) (Service, error) {
	if cf == nil && df == nil && s3f == nil {
		return nil, errors.Errorf(
			"model %q does not fulfill any method of the vision service. It is neither a detector, nor classifier, nor 3D segmenter", name)
	}

	p := Properties{false, false, false}
	if cf != nil {
		p.ClassificationSupported = true
	}
	if df != nil {
		p.DetectionSupported = true
	}
	if s3f != nil {
		p.ObjectPCDsSupported = true
	}

	return &vizModel{
		Named:           name.AsNamed(),
		r:               r,
		properties:      p,
		closerFunc:      c,
		classifierFunc:  cf,
		detectorFunc:    df,
		segmenter3DFunc: s3f,
	}, nil
}

// Detections returns the detections of given image if the model implements objectdetector.Detector.
func (vm *vizModel) Detections(
	ctx context.Context,
	img image.Image,
	extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Detections::"+vm.Named.Name().String())
	defer span.End()
	if vm.detectorFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Detector", vm.Named.Name())
	}
	return vm.detectorFunc(ctx, img)
}

// DetectionsFromCamera returns the detections of the next image from the given camera.
func (vm *vizModel) DetectionsFromCamera(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::DetectionsFromCamera::"+vm.Named.Name().String())
	defer span.End()
	if vm.detectorFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Detector", vm.Named.Name())
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
	return vm.detectorFunc(ctx, img)
}

// Classifications returns the classifications of given image if the model implements classifications.Classifier.
func (vm *vizModel) Classifications(
	ctx context.Context,
	img image.Image,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Classifications::"+vm.Named.Name().String())
	defer span.End()
	if vm.classifierFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Classifier", vm.Named.Name())
	}
	fullClassifications, err := vm.classifierFunc(ctx, img)
	if err != nil {
		return nil, errors.Wrap(err, "could not get classifications from image")
	}
	return fullClassifications.TopN(n)
}

// ClassificationsFromCamera returns the classifications of the next image from the given camera.
func (vm *vizModel) ClassificationsFromCamera(
	ctx context.Context,
	cameraName string,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::ClassificationsFromCamera::"+vm.Named.Name().String())
	defer span.End()
	if vm.classifierFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Classifier", vm.Named.Name())
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
	fullClassifications, err := vm.classifierFunc(ctx, img)
	if err != nil {
		return nil, errors.Wrap(err, "could not get classifications from image")
	}
	return fullClassifications.TopN(n)
}

// GetObjectPointClouds returns all the found objects in a 3D image if the model implements Segmenter3D.
func (vm *vizModel) GetObjectPointClouds(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]*viz.Object, error) {
	if vm.segmenter3DFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a 3D segmenter", vm.Named.Name().String())
	}
	ctx, span := trace.StartSpan(ctx, "service::vision::GetObjectPointClouds::"+vm.Named.Name().String())
	defer span.End()
	cam, err := camera.FromRobot(vm.r, cameraName)
	if err != nil {
		return nil, err
	}
	return vm.segmenter3DFunc(ctx, cam)
}

// GetProperties returns a Properties object that details the vision capabilities of the model.
func (vm *vizModel) GetProperties(ctx context.Context, extra map[string]interface{}) (*Properties, error) {
	_, span := trace.StartSpan(ctx, "service::vision::GetProperties::"+vm.Named.Name().String())
	defer span.End()

	return &vm.properties, nil
}

func (vm *vizModel) CaptureAllFromCamera(
	ctx context.Context,
	cameraName string,
	opt viscapture.CaptureOptions,
	extra map[string]interface{},
) (viscapture.VisCapture, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::ClassificationsFromCamera::"+vm.Named.Name().String())
	defer span.End()
	cam, err := camera.FromRobot(vm.r, cameraName)
	if err != nil {
		return viscapture.VisCapture{}, errors.Wrapf(err, "could not find camera named %s", cameraName)
	}
	img, release, err := camera.ReadImage(ctx, cam)
	if err != nil {
		return viscapture.VisCapture{}, errors.Wrapf(err, "could not get image from %s", cameraName)
	}
	defer release()
	logger := vm.r.Logger()
	var detections []objectdetection.Detection
	if opt.ReturnDetections {
		if !vm.properties.DetectionSupported {
			logger.Debugf("detections requested but vision model %q does not implement a Detector", vm.Named.Name())
		} else {
			detections, err = vm.Detections(ctx, img, extra)
			if err != nil {
				return viscapture.VisCapture{}, err
			}
		}
	}
	var classifications classification.Classifications
	if opt.ReturnClassifications {
		logger := vm.r.Logger()
		if !vm.properties.ClassificationSupported {
			logger.Debugf("classifications requested in CaptureAll but vision model %q does not implement a Classifier",
				vm.Named.Name())
		} else {
			classifications, err = vm.Classifications(ctx, img, 0, extra)
			if err != nil {
				return viscapture.VisCapture{}, err
			}
		}
	}

	var objPCD []*viz.Object
	if opt.ReturnObject {
		if !vm.properties.ObjectPCDsSupported {
			logger := vm.r.Logger()
			logger.Debugf("object point cloud requested in CaptureAll but vision model %q does not implement a 3D Segmenter", vm.Named.Name())
		} else {
			objPCD, err = vm.GetObjectPointClouds(ctx, cameraName, extra)
			if err != nil {
				return viscapture.VisCapture{}, err
			}
		}
	}
	if !opt.ReturnImage {
		img = nil
	}
	return viscapture.VisCapture{
		Image:           img,
		Detections:      detections,
		Classifications: classifications,
		Objects:         objPCD,
	}, nil
}

func (vm *vizModel) Close(ctx context.Context) error {
	if vm.closerFunc == nil {
		return nil
	}
	return vm.closerFunc(ctx)
}

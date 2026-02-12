// Package vision is the service that allows you to access various computer vision algorithms
// (like detection, segmentation, tracking, etc) that usually only require a camera or image input.
// For more information, see the [vision service docs].
//
// [vision service docs]: https://docs.viam.com/services/vision/
package vision

import (
	"context"
	"image"

	servicepb "go.viam.com/api/service/vision/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
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
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// A Service implements various computer vision algorithms like detection and segmentation.
// For more information, see the [vision service docs].
//
// DetectionsFromCamera example:
//
//	myDetectorService, err := vision.FromProvider(machine, "my_detector")
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
//		myCam, err := camera.FromProvider(machine, "my_camera")
//		if err != nil {
//			logger.Error(err)
//			return
//		}
//		// Get an image from the camera decoded as an image.Image
//		img, err = camera.DecodeImageFromCamera(context.Background(), utils.MimeTypeJPEG, nil, myCam)
//
//		myDetectorService, err := vision.FromProvider(machine, "my_detector")
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
//	myClassifierService, err := vision.FromProvider(machine, "my_classifier")
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
//		myCam, err := camera.FromProvider(machine, "my_camera")
//		if err != nil {
//			logger.Error(err)
//			return
//		}
//		// Get an image from the camera decoded as an image.Image
//		img, err = camera.DecodeImageFromCamera(context.Background(), utils.MimeTypeJPEG, nil, myCam)
//
//		myClassifierService, err := vision.FromProvider(machine, "my_classifier")
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
//	mySegmenterService, err := vision.FromProvider(machine, "my_segmenter")
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

// Deprecated: FromRobot is a helper for getting the named vision service from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// Deprecated: FromDependencies is a helper for getting the named vision service from a collection of dependencies.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// FromProvider is a helper for getting the named Vision service
// from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Service, error) {
	return resource.FromProvider[Service](provider, Named(name))
}

// Properties returns various information regarding the current vision service,
// specifically, which vision tasks are supported by the resource.
type Properties struct {
	ClassificationSupported bool
	DetectionSupported      bool
	ObjectPCDsSupported     bool
}

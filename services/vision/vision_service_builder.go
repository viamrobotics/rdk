package vision

import (
	"context"
	"image"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
	"go.viam.com/rdk/vision/viscapture"
)

// vizModel wraps the vision model with all the service interface methods.
type vizModel struct {
	resource.Named
	resource.AlwaysRebuild
	logger          logging.Logger
	properties      Properties
	closerFunc      func(ctx context.Context) error // close the underlying model
	getCamera       func(cameraName string) (camera.Camera, error)
	classifierFunc  classification.Classifier
	detectorFunc    objectdetection.Detector
	segmenter3DFunc segmentation.Segmenter
	defaultCamera   string
}

// NewService wraps the vision model in the struct that fulfills the vision service interface.
func NewService(
	name resource.Name,
	deps resource.Dependencies,
	logger logging.Logger,
	closer func(ctx context.Context) error,
	cf classification.Classifier,
	df objectdetection.Detector,
	s3f segmentation.Segmenter,
	defaultCamera string,
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

	getCamera := func(cameraName string) (camera.Camera, error) {
		return camera.FromDependencies(deps, cameraName)
	}

	return &vizModel{
		Named:           name.AsNamed(),
		logger:          logger,
		properties:      p,
		closerFunc:      closer,
		getCamera:       getCamera,
		classifierFunc:  cf,
		detectorFunc:    df,
		segmenter3DFunc: s3f,
		defaultCamera:   defaultCamera,
	}, nil
}

// DeprecatedNewService wraps the vision model in the struct that fulfills the vision service
// interface. Register this service with DeprecatedRobotConstructor.
func DeprecatedNewService(
	name resource.Name,
	r robot.Robot,
	c func(ctx context.Context) error,
	cf classification.Classifier,
	df objectdetection.Detector,
	s3f segmentation.Segmenter,
	defaultCamera string,
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

	logger := r.Logger()

	getCamera := func(cameraName string) (camera.Camera, error) {
		return camera.FromRobot(r, cameraName)
	}

	return &vizModel{
		Named:           name.AsNamed(),
		logger:          logger,
		properties:      p,
		closerFunc:      c,
		getCamera:       getCamera,
		classifierFunc:  cf,
		detectorFunc:    df,
		segmenter3DFunc: s3f,
		defaultCamera:   defaultCamera,
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

	if cameraName == "" && vm.defaultCamera == "" {
		return nil, errors.New("no camera name provided and no default camera found")
	} else if cameraName == "" {
		cameraName = vm.defaultCamera
	}
	if vm.detectorFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Detector", vm.Named.Name())
	}

	cam, err := vm.getCamera(cameraName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find camera named %s", cameraName)
	}
	img, err := camera.DecodeImageFromCamera(ctx, "", extra, cam)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get image from %s", cameraName)
	}
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

	if cameraName == "" && vm.defaultCamera == "" {
		return nil, errors.New("no camera name provided and no default camera found")
	} else if cameraName == "" {
		cameraName = vm.defaultCamera
	}
	if vm.classifierFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Classifier", vm.Named.Name())
	}

	cam, err := vm.getCamera(cameraName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find camera named %s", cameraName)
	}
	img, err := camera.DecodeImageFromCamera(ctx, "", extra, cam)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get image from %s", cameraName)
	}

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
	ctx, span := trace.StartSpan(ctx, "service::vision::GetObjectPointClouds::"+vm.Named.Name().String())
	defer span.End()

	if vm.segmenter3DFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a 3D segmenter", vm.Named.Name().String())
	}
	if cameraName == "" && vm.defaultCamera == "" {
		return nil, errors.New("no camera name provided and no default camera found")
	} else if cameraName == "" {
		cameraName = vm.defaultCamera
	}
	cam, err := vm.getCamera(cameraName)
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

	if cameraName == "" && vm.defaultCamera == "" {
		return viscapture.VisCapture{}, errors.New("no camera name provided and no default camera found")
	} else if cameraName == "" {
		cameraName = vm.defaultCamera
	}
	cam, err := vm.getCamera(cameraName)
	if err != nil {
		return viscapture.VisCapture{}, errors.Wrapf(err, "could not find camera named %s", cameraName)
	}
	img, err := camera.DecodeImageFromCamera(ctx, "", extra, cam)
	if err != nil {
		return viscapture.VisCapture{}, errors.Wrapf(err, "could not get image from %s", cameraName)
	}

	var detections []objectdetection.Detection
	if opt.ReturnDetections {
		if !vm.properties.DetectionSupported {
			vm.logger.Debugf("detections requested but vision model %q does not implement a Detector", vm.Named.Name())
		} else {
			detections, err = vm.Detections(ctx, img, extra)
			if err != nil {
				return viscapture.VisCapture{}, err
			}
		}
	}

	var classifications classification.Classifications
	if opt.ReturnClassifications {
		if !vm.properties.ClassificationSupported {
			vm.logger.Debugf("classifications requested in CaptureAll but vision model %q does not implement a Classifier",
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
			vm.logger.Debugf("object point cloud requested in CaptureAll but vision model %q does not implement a 3D Segmenter", vm.Named.Name())
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

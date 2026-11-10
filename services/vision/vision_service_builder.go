package vision

import (
	"context"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/detection3d"
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
	segmenter3DFunc detection3d.Segmenter
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
	s3f detection3d.Segmenter,
	defaultCamera string,
) (Service, error) {
	if cf == nil && df == nil && s3f == nil {
		return nil, errors.Errorf(
			"model %q does not fulfill any method of the vision service. It is neither a detector, nor classifier, nor 3D segmenter", name,
		)
	}

	p := Properties{}
	if cf != nil {
		p.ClassificationSupported = true
	}
	if df != nil {
		p.DetectionSupported = true
	}
	if s3f != nil {
		p.ObjectPCDsSupported = true
		p.Detections3DSupported = true
	}
	if defaultCamera != "" {
		p.DefaultCamera = &defaultCamera
	}

	getCamera := func(cameraName string) (camera.Camera, error) {
		return camera.FromProvider(deps, cameraName)
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
			"model %q does not fulfill any method of the vision service. It is neither a detector, nor classifier, nor 3D segmenter", name,
		)
	}
	var segmenter3D detection3d.Segmenter
	if s3f != nil {
		segmenter3D = detection3d.FromSegmenter(name.ShortName(), s3f)
	}

	p := Properties{}
	if cf != nil {
		p.ClassificationSupported = true
	}
	if df != nil {
		p.DetectionSupported = true
	}
	if s3f != nil {
		p.ObjectPCDsSupported = true
		p.Detections3DSupported = true
	}
	if defaultCamera != "" {
		p.DefaultCamera = &defaultCamera
	}

	logger := r.Logger()

	getCamera := func(cameraName string) (camera.Camera, error) {
		return camera.FromProvider(r, cameraName)
	}

	return &vizModel{
		Named:           name.AsNamed(),
		logger:          logger,
		properties:      p,
		closerFunc:      c,
		getCamera:       getCamera,
		classifierFunc:  cf,
		detectorFunc:    df,
		segmenter3DFunc: segmenter3D,
		defaultCamera:   defaultCamera,
	}, nil
}

// Detections returns the detections of given image if the model implements objectdetector.Detector.
func (vm *vizModel) Detections(
	ctx context.Context,
	img *camera.NamedImage,
	extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Detections::"+vm.Named.Name().String())
	defer span.End()

	if vm.detectorFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Detector", vm.Named.Name())
	}
	if img == nil {
		return nil, errors.New("nil image input to Detections")
	}
	decoded, err := img.Image(ctx)
	if err != nil {
		return nil, err
	}
	return vm.detectorFunc(ctx, decoded)
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
	namedImages, _, err := cam.Images(ctx, nil, extra)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get image from %s", cameraName)
	}
	if len(namedImages) == 0 {
		return nil, errors.Errorf("no images returned from camera %s", cameraName)
	}
	return vm.Detections(ctx, &namedImages[0], extra)
}

// Classifications returns the classifications of given image if the model implements classifications.Classifier.
func (vm *vizModel) Classifications(
	ctx context.Context,
	img *camera.NamedImage,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Classifications::"+vm.Named.Name().String())
	defer span.End()

	if vm.classifierFunc == nil {
		return nil, errors.Errorf("vision model %q does not implement a Classifier", vm.Named.Name())
	}
	if img == nil {
		return nil, errors.New("nil image input to Classifications")
	}
	decoded, err := img.Image(ctx)
	if err != nil {
		return nil, err
	}
	fullClassifications, err := vm.classifierFunc(ctx, decoded)
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
	namedImages, _, err := cam.Images(ctx, nil, extra)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get image from %s", cameraName)
	}
	if len(namedImages) == 0 {
		return nil, errors.Errorf("no images returned from camera %s", cameraName)
	}
	return vm.Classifications(ctx, &namedImages[0], n, extra)
}

// GetObjectPointClouds returns the 3D detections flattened into objects if the model implements a 3D segmenter.
func (vm *vizModel) GetObjectPointClouds(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]*viz.Object, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetObjectPointClouds::"+vm.Named.Name().String())
	defer span.End()

	detections, err := vm.GetDetections3D(ctx, cameraName, extra)
	if err != nil {
		return nil, err
	}
	return detectionsToObjects(detections)
}

// GetDetections3D returns the 3D detections from the given camera if the model implements a 3D segmenter.
func (vm *vizModel) GetDetections3D(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]*detection3d.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetDetections3D::"+vm.Named.Name().String())
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

// detectionsToObjects flattens each detection into the GetObjectPointClouds shape, which holds one geometry and one
// point cloud per object. Every shape in the tree is expressed in the root's parent frame. The object's geometry is the
// root's non-point-cloud geometry, or if it has none, the first part's in transform order; all point clouds are merged.
//
// GetObjectPointClouds callers such as navigation treat an object's geometry as an obstacle and do not check for nil,
// so a detection with only points gets the points' bounding box and a detection with no shapes at all is dropped.
func detectionsToObjects(detections []*detection3d.Detection) ([]*viz.Object, error) {
	objects := make([]*viz.Object, 0, len(detections))
	for i, det := range detections {
		if det == nil {
			return nil, errors.Errorf("3D detection %d is nil", i)
		}
		cloud := pointcloud.NewBasicEmpty()
		var geom spatialmath.Geometry
		// Parents precede their children, so each transform's pose in the root's parent frame is known when it is reached.
		poses := make(map[string]spatialmath.Pose, len(det.Transforms))
		for j, tf := range det.Transforms {
			pose := tf.Pose()
			if pose == nil {
				pose = spatialmath.NewZeroPose()
			}
			if j > 0 {
				parentPose, ok := poses[tf.Parent()]
				if !ok {
					return nil, errors.Errorf("3D detection %d: transform %q has parent %q, which is not an earlier transform",
						i, tf.Name(), tf.Parent())
				}
				pose = spatialmath.Compose(parentPose, pose)
			}
			// A repeated name would silently re-parent every later child that references it.
			if _, ok := poses[tf.Name()]; ok {
				return nil, errors.Errorf("3D detection %d: transform name %q is used more than once", i, tf.Name())
			}
			poses[tf.Name()] = pose

			if tf.Geometry() == nil {
				continue
			}
			shape := tf.Geometry().Transform(pose)
			if points, ok := shape.(pointcloud.PointCloud); ok {
				var setErr error
				points.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
					setErr = cloud.Set(p, d)
					return setErr == nil
				})
				if setErr != nil {
					return nil, errors.Wrapf(setErr, "3D detection %d", i)
				}
			} else if geom == nil {
				geom = shape
			}
		}

		if geom == nil && cloud.Size() == 0 {
			continue
		}
		if geom == nil {
			obj, err := viz.NewObject(cloud)
			if err != nil {
				return nil, errors.Wrapf(err, "3D detection %d", i)
			}
			objects = append(objects, obj)
			continue
		}
		objects = append(objects, &viz.Object{PointCloud: cloud, Geometry: geom})
	}
	return objects, nil
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
	namedImages, _, err := cam.Images(ctx, nil, extra)
	if err != nil {
		return viscapture.VisCapture{}, errors.Wrapf(err, "could not get image from %s", cameraName)
	}
	if len(namedImages) == 0 {
		return viscapture.VisCapture{}, errors.Errorf("no images returned from camera %s", cameraName)
	}
	namedImg := &namedImages[0]

	var detections []objectdetection.Detection
	if opt.ReturnDetections {
		if !vm.properties.DetectionSupported {
			vm.logger.Debugf("detections requested but vision model %q does not implement a Detector", vm.Named.Name())
		} else {
			detections, err = vm.Detections(ctx, namedImg, extra)
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
			classifications, err = vm.Classifications(ctx, namedImg, 0, extra)
			if err != nil {
				return viscapture.VisCapture{}, err
			}
		}
	}

	// Both 3D outputs come from one segmentation so the segmenter runs at most once per capture.
	var objPCD []*viz.Object
	var dets3D []*detection3d.Detection
	if opt.ReturnObject || opt.ReturnDetections3D {
		if !vm.properties.Detections3DSupported {
			vm.logger.Debugf("3D output requested in CaptureAll but vision model %q does not implement a 3D Segmenter", vm.Named.Name())
		} else {
			detections, err := vm.segmenter3DFunc(ctx, cam)
			if err != nil {
				return viscapture.VisCapture{}, err
			}
			if opt.ReturnDetections3D {
				dets3D = detections
			}
			if opt.ReturnObject {
				objPCD, err = detectionsToObjects(detections)
				if err != nil {
					return viscapture.VisCapture{}, err
				}
			}
		}
	}

	var img *camera.NamedImage
	if opt.ReturnImage {
		img = namedImg
	}
	return viscapture.VisCapture{
		Image:           img,
		Detections:      detections,
		Classifications: classifications,
		Objects:         objPCD,
		Detections3D:    dets3D,
	}, nil
}

func (vm *vizModel) Close(ctx context.Context) error {
	if vm.closerFunc == nil {
		return nil
	}
	return vm.closerFunc(ctx)
}

package segmentation

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
)

// DetectionSegmenter will take an objectdetector.Detector and turn it into a Segementer.
// The Projector that is used to build the Segmenter must be associated with the camera that will be given to the Segmenter.
func DetectionSegmenter(detector objectdetection.Detector, proj rimage.Projector, meanK int, sigma float64) (Segmenter, error) {
	if detector == nil {
		return nil, errors.New("detector cannot be nil")
	}
	if proj == nil {
		return nil, errors.New("projector cannot be nil")
	}
	return func(ctx context.Context, cam camera.Camera, params config.AttributeMap) ([]*vision.Object, error) {
		// get the 2D detections
		img, _, err := cam.Next(ctx)
		if err != nil {
			return nil, err
		}
		originalImg := rimage.CloneToImageWithDepth(img)
		dets, err := detector(img) // detector may modify the input image
		if err != nil {
			return nil, err
		}
		return DetectionsToObjects(dets, originalImg, proj, meanK, sigma)
	}, nil
}

// DetectionsToObjects turns 2D detections into 3D objects using the intrinsic camera projection parameters and the image.
func DetectionsToObjects(dets []objectdetection.Detection,
	iwd *rimage.ImageWithDepth,
	proj rimage.Projector,
	meanK int,
	sigma float64,
) ([]*vision.Object, error) {
	statisticalFilter, err := pointcloud.StatisticalOutlierFilter(meanK, sigma)
	if err != nil {
		return nil, err
	}
	// project 2D detections to 3D objects
	objects := make([]*vision.Object, 0, len(dets))
	for _, d := range dets {
		bb := d.BoundingBox()
		pc, err := proj.ImageWithDepthToPointCloud(iwd, *bb)
		if err != nil {
			return nil, err
		}
		filtered, err := statisticalFilter(pc)
		if err != nil {
			return nil, err
		}
		// if object was filtered away, skip it
		if filtered.Size() == 0 {
			continue
		}
		obj, err := vision.NewObject(filtered)
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

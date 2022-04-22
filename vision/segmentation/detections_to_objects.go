package segmentation

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
)

// DetectionSegmenter will take an objectdetector.Detector and turn it into a Segementer.
// The params for the segmenter are "mean_k" and "sigma" for the statistical filter on the point clouds.
func DetectionSegmenter(detector objectdetection.Detector) (Segmenter, []utils.TypedName, error) {
	if detector == nil {
		return nil, nil, errors.New("detector cannot be nil")
	}
	parameters := []utils.TypedName{{"mean_k", "int"}, {"sigma", "float64"}}
	// return the segmenter
	seg := func(ctx context.Context, cam camera.Camera, params config.AttributeMap) ([]*vision.Object, error) {
		meanK := params.Int("mean_k", 0)
		sigma := params.Float64("sigma", 1.5)
		statisticalFilter, err := pointcloud.StatisticalOutlierFilter(meanK, sigma)
		if err != nil {
			return nil, err
		}
		proj := camera.Projector(cam)
		if proj == nil {
			return nil, errors.New("camera projector cannot be nil." +
				"Currently remote cameras are not supported (intrinsics parameters are not transferred over protobuf)")
		}
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
		pcs, err := detectionsToPointClouds(dets, originalImg, proj)
		if err != nil {
			return nil, err
		}
		// filter the point clouds to get rid of outlier points
		objects := make([]*vision.Object, 0, len(pcs))
		for _, pc := range pcs {
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
	return seg, parameters, nil
}

// detectionsToPointClouds turns 2D detections into 3D point clodus using the intrinsic camera projection parameters and the image.
func detectionsToPointClouds(dets []objectdetection.Detection,
	iwd *rimage.ImageWithDepth,
	proj rimage.Projector,
) ([]pointcloud.PointCloud, error) {
	// project 2D detections to 3D pointclouds
	pcs := make([]pointcloud.PointCloud, 0, len(dets))
	for _, d := range dets {
		bb := d.BoundingBox()
		if bb == nil {
			return nil, errors.New("detection bounding box cannot be nil")
		}
		pc, err := proj.ImageWithDepthToPointCloud(iwd, *bb)
		if err != nil {
			return nil, err
		}
		pcs = append(pcs, pc)
	}
	return pcs, nil
}

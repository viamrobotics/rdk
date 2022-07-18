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
		var err error
		filter := func(pc pointcloud.PointCloud) (pointcloud.PointCloud, error) {
			return pc, nil
		}
		if meanK > 0 && sigma > 0.0 {
			filter, err = pointcloud.StatisticalOutlierFilter(meanK, sigma)
			if err != nil {
				return nil, err
			}
		}
		proj, err := cam.GetProperties(ctx)
		if err != nil {
			return nil, err
		}
		// get the 3D detections, and turn them into 2D image and depthmap
		pc, err := cam.NextPointCloud(ctx)
		if err != nil {
			return nil, err
		}
		img, dm, err := proj.PointCloudToRGBD(pc)
		if err != nil {
			return nil, err
		}
		im := rimage.CloneImage(img)
		dets, err := detector(im) // detector may modify the input image
		if err != nil {
			return nil, err
		}
		// TODO(bhaney): Is there a way to just project the detection boxes themselves?
		pcs, err := detectionsToPointClouds(dets, img, dm, proj)
		if err != nil {
			return nil, err
		}
		// filter the point clouds to get rid of outlier points
		objects := make([]*vision.Object, 0, len(pcs))
		for _, pc := range pcs {
			filtered, err := filter(pc)
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
	im *rimage.Image, dm *rimage.DepthMap,
	proj rimage.Projector,
) ([]pointcloud.PointCloud, error) {
	// project 2D detections to 3D pointclouds
	pcs := make([]pointcloud.PointCloud, 0, len(dets))
	for _, d := range dets {
		bb := d.BoundingBox()
		if bb == nil {
			return nil, errors.New("detection bounding box cannot be nil")
		}
		pc, err := proj.RGBDToPointCloud(im, dm, *bb)
		if err != nil {
			return nil, err
		}
		pcs = append(pcs, pc)
	}
	return pcs, nil
}

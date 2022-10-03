package segmentation

import (
	"context"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
)

// DetectionSegmenterConfig are the optional parameters to turn a detector into a segmenter.
type DetectionSegmenterConfig struct {
	DetectorName     string  `json:"detector_name"`
	ConfidenceThresh float64 `json:"confidence_threshold_pct"`
	MeanK            int     `json:"mean_k"`
	Sigma            float64 `json:"sigma"`
}

// ConvertAttributes changes the AttributeMap input into a DetectionSegmenterConfig.
func (dsc *DetectionSegmenterConfig) ConvertAttributes(am config.AttributeMap) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: dsc})
	if err != nil {
		return err
	}
	return decoder.Decode(am)
}

// DetectionSegmenter will take an objectdetector.Detector and turn it into a Segementer.
// The params for the segmenter are "mean_k" and "sigma" for the statistical filter on the point clouds.
func DetectionSegmenter(detector objectdetection.Detector, meanK int, sigma, confidenceThresh float64) (Segmenter, error) {
	var err error
	if detector == nil {
		return nil, errors.New("detector cannot be nil")
	}
	filter := func(pc pointcloud.PointCloud) (pointcloud.PointCloud, error) {
		return pc, nil
	}
	if meanK > 0 && sigma > 0.0 {
		filter, err = pointcloud.StatisticalOutlierFilter(meanK, sigma)
		if err != nil {
			return nil, err
		}
	}
	// return the segmenter
	seg := func(ctx context.Context, cam camera.Camera) ([]*vision.Object, error) {
		proj, err := cam.Projector(ctx)
		if err != nil {
			return nil, err
		}
		// get the 3D detections, and turn them into 2D image and depthmap
		pc, err := cam.NextPointCloud(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "detection segmenter")
		}
		img, dm, err := proj.PointCloudToRGBD(pc)
		if err != nil {
			return nil, err
		}
		im := rimage.CloneImage(img)
		dets, err := detector(ctx, im) // detector may modify the input image
		if err != nil {
			return nil, err
		}
		// TODO(bhaney): Is there a way to just project the detection boxes themselves?
		pcs, err := detectionsToPointClouds(dets, confidenceThresh, img, dm, proj)
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
	return seg, nil
}

// detectionsToPointClouds turns 2D detections into 3D point clodus using the intrinsic camera projection parameters and the image.
func detectionsToPointClouds(
	dets []objectdetection.Detection,
	confidenceThresh float64,
	im *rimage.Image, dm *rimage.DepthMap,
	proj transform.Projector,
) ([]pointcloud.PointCloud, error) {
	// project 2D detections to 3D pointclouds
	pcs := make([]pointcloud.PointCloud, 0, len(dets))
	for _, d := range dets {
		if d.Score() < confidenceThresh {
			continue
		}
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

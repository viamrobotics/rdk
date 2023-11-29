package segmentation

import (
	"context"
	"image"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
)

// DetectionSegmenterConfig are the optional parameters to turn a detector into a segmenter.
type DetectionSegmenterConfig struct {
	resource.TriviallyValidateConfig
	DetectorName     string  `json:"detector_name"`
	ConfidenceThresh float64 `json:"confidence_threshold_pct"`
	MeanK            int     `json:"mean_k"`
	Sigma            float64 `json:"sigma"`
}

// ConvertAttributes changes the AttributeMap input into a DetectionSegmenterConfig.
func (dsc *DetectionSegmenterConfig) ConvertAttributes(am utils.AttributeMap) error {
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
	seg := func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		proj, err := src.Projector(ctx)
		if err != nil {
			return nil, err
		}
		// get the 3D detections, and turn them into 2D image and depthmap
		imgs, _, err := src.Images(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "detection segmenter")
		}
		var img *rimage.Image
		var dmimg image.Image
		for _, i := range imgs {
			this_i := i
			if i.SourceName == "color" {
				img = rimage.ConvertImage(this_i.Image)
			}
			if i.SourceName == "depth" {
				dmimg = this_i.Image
			}
		}
		if img == nil || dmimg == nil {
			return nil, errors.New("source camera's getImages method did not have 'color' and 'depth' images")
		}
		dm, err := rimage.ConvertImageToDepthMap(ctx, dmimg)
		if err != nil {
			return nil, err
		}
		im := rimage.CloneImage(img)
		dets, err := detector(ctx, im) // detector may modify the input image
		if err != nil {
			return nil, err
		}

		objects := make([]*vision.Object, 0, len(dets))
		for _, d := range dets {
			if d.Score() < confidenceThresh {
				continue
			}
			// TODO(bhaney): Is there a way to just project the detection boxes themselves?
			pc, err := detectionToPointCloud(d, img, dm, proj)
			if err != nil {
				return nil, err
			}
			pc, err = filter(pc)
			if err != nil {
				return nil, err
			}
			// if object was filtered away, skip it
			if pc.Size() == 0 {
				continue
			}
			obj, err := vision.NewObjectWithLabel(pc, d.Label(), nil)
			if err != nil {
				return nil, err
			}
			objects = append(objects, obj)
		}
		return objects, nil
	}
	return seg, nil
}

func detectionToPointCloud(
	d objectdetection.Detection,
	im *rimage.Image, dm *rimage.DepthMap,
	proj transform.Projector,
) (pointcloud.PointCloud, error) {
	bb := d.BoundingBox()
	if bb == nil {
		return nil, errors.New("detection bounding box cannot be nil")
	}
	pc, err := proj.RGBDToPointCloud(im, dm, *bb)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

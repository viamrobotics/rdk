package mlvision

import (
	"errors"

	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/segmentation"
)

func attemptToBuild3DSegmenter(mlm mlmodel.Service) (segmentation.Segmenter, error) {
	return nil, errors.New("vision 3D segmenters from ML models are currently not supported")
}

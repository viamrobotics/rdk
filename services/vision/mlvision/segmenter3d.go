package mlvision

import (
	"errors"

	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/segmentation"
)

// TODO: RSDK-2665, build 3D segmenter from ML models.
func attemptToBuild3DSegmenter(mlm mlmodel.Service, nameMap map[string]string) (segmentation.Segmenter, error) {
	return nil, errors.New("vision 3D segmenters from ML models are currently not supported")
}

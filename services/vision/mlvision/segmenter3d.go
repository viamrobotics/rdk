package mlvision

import (
	"errors"
	"sync"

	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/segmentation"
)

// TODO: RSDK-2665, build 3D segmenter from ML models.
func attemptToBuild3DSegmenter(mlm mlmodel.Service, nameMap *sync.Map) (segmentation.Segmenter, error) {
	return nil, errors.New("cannot use model as a 3D segmenter: vision 3D segmenters from ML models are currently not supported")
}

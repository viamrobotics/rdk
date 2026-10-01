package mlvision

import (
	"errors"
	"sync"

	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/vision/detection3d"
)

// TODO: RSDK-2665, build 3D detector from ML models.
func attemptToBuild3DDetector(mlm mlmodel.Service, inNameMap, outNameMap *sync.Map) (detection3d.Detector, error) {
	return nil, errors.New("cannot use model as a 3D detector: vision 3D detectors from ML models are currently not supported")
}

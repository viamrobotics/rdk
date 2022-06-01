package vision

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

func registerTfliteDetector(dm detectorMap, conf *DetectorConfig, logger golog.Logger) error {
	if conf == nil {
		return errors.New("object detection config for tflite detector cannot be nil")
	}
	detector, err := NewTfliteDetector(conf, logger)
	if err != nil {
		return errors.Wrapf(err, "could not register tflite detector %s", conf.Name)
	}

	return dm.registerDetector(conf.Name, detector, logger)

}

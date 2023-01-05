// Package config implements functions to assist with attribute evaluation in the slam service and in testing
package config

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

// SLAMConfigError returns an error specific to a failure in SLAM config.
func SLAMConfigError(configError string) error {
	return errors.Errorf("SLAM Service configuration error: %s", configError)
}

// DetermineDeleteProcessedData will determine the value of the deleteProcessData attribute of slam builtin
// based on the online/offline state and the delete_processed_data input parameter.
func DetermineDeleteProcessedData(logger golog.Logger, deleteData *bool, useLiveData bool) bool {
	var deleteProcessedData bool
	if deleteData == nil {
		deleteProcessedData = useLiveData
	} else {
		deleteProcessedData = *deleteData
		if !useLiveData && deleteProcessedData {
			logger.Debug("a value of true cannot be given for delete_processed_data when in offline mode, setting to false")
			deleteProcessedData = false
		}
	}
	return deleteProcessedData
}

// DetermineUseLiveData will determine the value of the useLiveData attribute of slam builtin
// based on the use_live_data input parameter and sensor list.
func DetermineUseLiveData(logger golog.Logger, liveData *bool, sensors []string) (bool, error) {
	if liveData == nil {
		return false, SLAMConfigError("use_live_data is a required input parameter")
	}
	useLiveData := *liveData
	if useLiveData && len(sensors) == 0 {
		return false, SLAMConfigError("sensors field cannot be empty when use_live_data is set to true")
	}
	return useLiveData, nil
}

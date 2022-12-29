// Package config implements functions to assist with attribute evaluation in the slam service and in testing
package config

import (
	"github.com/edaniels/golog"
)

// DetermineDeleteProcessedData will determine the value of the deleteProcessData attribute of slam builtin
// based on the online/offline state and the delete_processed_data input parameter.
func DetermineDeleteProcessedData(logger golog.Logger, deleteData *bool, offlineFlag bool) bool {
	var deleteProcessedData bool
	if deleteData == nil {
		deleteProcessedData = !offlineFlag
	} else {
		deleteProcessedData = *deleteData
		if offlineFlag && deleteProcessedData {
			logger.Debug("a value of true cannot be given for delete_processed_data when in offline mode, setting to false")
			deleteProcessedData = false
		}
	}
	return deleteProcessedData
}

// Package internal implements a slam service definition with additional exported functions for
// the purpose of testing
package internal

import (
	"context"

	"go.viam.com/rdk/component/camera"
)

// Service in the internal package includes additional exported functions relating to the data and
// slam processes in the slam service. These functions are not exported to the user. This resolves
// a circular import caused by the inject package.
type Service interface {
	StartDataProcess(cancelCtx context.Context, cam camera.Camera)
	StartSLAMProcess(ctx context.Context) ([]string, error)
	StopSLAMProcess() error
}

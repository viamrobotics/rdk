// Package testhelper implements a slam service definition with additional exported functions for
// the purpose of testing
package testhelper

import (
	"bufio"
	"context"

	"github.com/edaniels/gostream"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/components/camera"
)

// Service in the internal package includes additional exported functions relating to the data and
// slam processes in the slam service. These functions are not exported to the user. This resolves
// a circular import caused by the inject package.
type Service interface {
	StartDataProcess(cancelCtx context.Context, cam []camera.Camera, camStreams []gostream.VideoStream, c chan int)
	StartSLAMProcess(ctx context.Context) error
	StopSLAMProcess() error
	Close() error
	GetSLAMProcessConfig() pexec.ProcessConfig
	GetSLAMProcessBufferedLogReader() bufio.Reader
}

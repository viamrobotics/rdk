package camera

import (
	"context"
	"os"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/data"
)

type method int64

const (
	nextPointCloud method = iota
)

func (m method) String() string {
	if m == nextPointCloud {
		return "NextPointCloud"
	}
	return "Unknown"
}

func newNextPointCloudCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, logger golog.Logger) (data.Collector, error) {
	camera, err := assertCamera(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := camera.NextPointCloud(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, nextPointCloud.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, logger), nil
}

func assertCamera(resource interface{}) (Camera, error) {
	cam, ok := resource.(Camera)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return cam, nil
}

// Package wifi implements a wifi strength sensor
package wifi

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var modelname = resource.NewDefaultModel("wifi")

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newWifi(ctx, deps, config.Name, logger)
		}})
}

func newWifi(
	ctx context.Context,
	deps registry.Dependencies,
	name string,
	logger golog.Logger,
) (sensor.Sensor, error) {
	return &wifi{logger: logger}, nil
}

type wifi struct {
	generic.Unimplemented
	logger golog.Logger
}

// Readings returns Wifi strength statistics.
func (s *wifi) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	cmd := "cat /proc/net/wireless | awk 'NR > 2 { print $3 $4 $5 }'"
	out, err := exec.Command("bash", "-c", cmd).Output()

	stats := strings.SplitN(strings.TrimSpace(string(out)), ".", 3)

	link, err := strconv.ParseInt(stats[0], 10, 32)
	if err != nil {
		return nil, err
	}
	level, err := strconv.ParseInt(stats[1], 10, 32)
	if err != nil {
		return nil, err
	}
	noise, err := strconv.ParseInt(stats[2], 10, 32)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	s.logger.Infof("stats: %v", string(out))
	return map[string]interface{}{
		"link":  link,
		"level": level,
		"noise": noise,
	}, nil
}

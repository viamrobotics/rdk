// Package wifi implements a wifi strength sensor
package wifi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var modelname = resource.NewDefaultModel("wifi")

const wirelessInfoPath string = "/proc/net/wireless"

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			_ context.Context,
			_ registry.Dependencies,
			_ config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newWifi(logger, wirelessInfoPath)
		}})
}

func newWifi(logger golog.Logger, path string) (sensor.Sensor, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, errors.Wrap(err, "wifi readings not supported on this system")
	}
	return &wifi{logger: logger, path: path}, nil
}

type wifi struct {
	generic.Unimplemented
	logger golog.Logger

	path string // for testing
}

// Readings returns Wifi strength statistics.
func (sensor *wifi) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	cmd := fmt.Sprintf("cat %s | awk 'NR > 2 { print $3 $4 $5 }'", sensor.path)
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return nil, err
	}

	stats := strings.SplitN(strings.TrimSpace(string(out)), ".", 3)

	link, err := strconv.ParseInt(stats[0], 10, 32)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reading")
	}
	level, err := strconv.ParseInt(stats[1], 10, 32)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reading")
	}
	noise, err := strconv.ParseInt(stats[2], 10, 32)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reading")
	}

	return map[string]interface{}{
		"link":  int(link),
		"level": int(level),
		"noise": int(noise),
	}, nil
}

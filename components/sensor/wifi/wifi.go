// Package wifi implements a wifi strength sensor
package wifi

import (
	"context"
	"os"
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

var modelname = resource.NewDefaultModel("linux-wifi")

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
	if _, err := os.ReadFile(path); err != nil {
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
	dump, err := os.ReadFile(sensor.path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(dump), "\n")
	fields := strings.Fields(lines[len(lines)-1])

	link, err := strconv.ParseInt(strings.TrimRight(fields[2], "."), 10, 32)
	if err != nil {
		return nil, errors.Wrap(err, "invalid link reading")
	}
	level, err := strconv.ParseInt(strings.TrimRight(fields[3], "."), 10, 32)
	if err != nil {
		return nil, errors.Wrap(err, "invalid wifi level reading")
	}
	noise, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return nil, errors.Wrap(err, "invalid wifi noise reading")
	}

	return map[string]interface{}{
		"link":  int(link),
		"level_dB": int(level),
		"noise_dB": int(noise),
	}, nil
}

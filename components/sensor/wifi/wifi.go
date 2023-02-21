//go:build linux

// Package wifi implements a wifi strength sensor
package wifi

import (
	"context"
	"os"
	"path/filepath"
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
	if _, err := os.ReadFile(filepath.Clean(path)); err != nil {
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

	result := make(map[string]interface{})
	lines := strings.Split(strings.TrimSpace(string(dump)), "\n")
	for i, line := range lines {
		if i < 2 {
			continue
		}
		iface, readings, err := sensor.readingsByInterface(line)
		if err != nil {
			return nil, err
		}
		result[iface] = readings
	}

	return result, nil
}

func (sensor *wifi) readingsByInterface(line string) (string, map[string]int, error) {
	fields := strings.Fields(line)

	iface := strings.TrimRight(fields[0], ":")

	link, err := strconv.ParseInt(strings.TrimRight(fields[2], "."), 10, 32)
	if err != nil {
		return "", nil, errors.Wrap(err, "invalid link quality reading")
	}
	level, err := strconv.ParseInt(strings.TrimRight(fields[3], "."), 10, 32)
	if err != nil {
		return "", nil, errors.Wrap(err, "invalid wifi level reading")
	}
	noise, err := strconv.ParseInt(fields[4], 10, 32)
	if err != nil {
		return "", nil, errors.Wrap(err, "invalid wifi noise reading")
	}

	return iface, map[string]int{
		"link_quality": int(link),
		"level_dB":     int(level),
		"noise_dB":     int(noise),
	}, nil
}

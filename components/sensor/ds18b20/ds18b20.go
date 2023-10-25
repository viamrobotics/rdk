// Package ds18b20 implements a 1-wire temperature sensor
package ds18b20

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("ds18b20")

// Config is used for converting config attributes.
type Config struct {
	resource.TriviallyValidateConfig
	UniqueID string `json:"unique_id"`
}

func init() {
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (sensor.Sensor, error) {
				newConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}
				return newSensor(conf.ResourceName(), newConf.UniqueID, logger), nil
			},
		})
}

func newSensor(name resource.Name, id string, logger logging.Logger) sensor.Sensor {
	// temp sensors are in family 28
	return &Sensor{
		Named:         name.AsNamed(),
		logger:        logger,
		OneWireID:     id,
		OneWireFamily: "28",
	}
}

// Sensor is a 1-wire Sensor device.
type Sensor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	OneWireID     string
	OneWireFamily string
	logger        logging.Logger
}

// ReadTemperatureCelsius returns current temperature in celsius.
func (s *Sensor) ReadTemperatureCelsius(ctx context.Context) (float64, error) {
	// logic here is specific to 1-wire protocol, could be abstracted next time we
	// want to build support for a different 1-wire device,
	// or look at support via periph (or other library)
	devPath := fmt.Sprintf("/sys/bus/w1/devices/%s-%s/w1_slave", s.OneWireFamily, s.OneWireID)
	dat, err := os.ReadFile(filepath.Clean(devPath))
	if err != nil {
		return math.NaN(), err
	}
	tempString := strings.TrimSuffix(string(dat), "\n")
	splitString := strings.Split(tempString, "t=")
	if len(splitString) == 2 {
		tempMili, err := strconv.ParseFloat(splitString[1], 32)
		if err != nil {
			return math.NaN(), err
		}
		return tempMili / 1000, nil
	}
	return math.NaN(), errors.New("temperature could not be read")
}

// Readings returns a list containing single item (current temperature).
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	temp, err := s.ReadTemperatureCelsius(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"degrees_celsius": temp}, nil
}

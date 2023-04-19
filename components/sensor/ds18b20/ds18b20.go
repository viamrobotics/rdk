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

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

var modelname = resource.NewDefaultModel("ds18b20")

// Config is used for converting config attributes.
type Config struct {
	UniqueID string `json:"unique_id"`
}

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			return newSensor(conf.ResourceName(), newConf.UniqueID), nil
		}})

	config.RegisterComponentAttributeMapConverter(sensor.Subtype, modelname,
		func(attributes utils.AttributeMap) (interface{}, error) {
			return config.TransformAttributeMapToStruct(&Config{}, attributes)
		})
}

func newSensor(name resource.Name, id string) sensor.Sensor {
	// temp sensors are in family 28
	return &Sensor{
		Named:         name.AsNamed(),
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

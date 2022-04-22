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

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const (
	modelname = "ds18b20"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	UniqueID string `json:"unique_id"`
}

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSensor(config.Name, config.ConvertedAttributes.(*AttrConfig).UniqueID), nil
		}})

	config.RegisterComponentAttributeMapConverter(sensor.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(name string, id string) sensor.MinimalSensor {
	// temp sensors are in family 28
	return &Sensor{Name: name, OneWireID: id, OneWireFamily: "28"}
}

// Sensor is a 1-wire Sensor device.
type Sensor struct {
	Name          string
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

// GetReadings returns a list containing single item (current temperature).
func (s *Sensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	temp, err := s.ReadTemperatureCelsius(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{temp}, nil
}

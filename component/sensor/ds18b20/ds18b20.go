// Package ds18b20 implements a 1-wire temperature sensor
package ds18b20

import (
	"context"
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

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeSensor, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(name string, id string) sensor.Sensor {
	// temp sensors are in family 28
	return &Sensor{Name: name, OneWireID: id, OneWireFamily: "28"}
}

// Sensor is a fake Sensor device that always returns the set location.
type Sensor struct {
	Name          string
	OneWireID     string
	OneWireFamily string
}

// ReadTemperatureCelsius returns temperature in celsius.
func (s *Sensor) ReadTemperatureCelsius(ctx context.Context) (float64, error) {
	/*
	* logic here is specific to 1-wire protocol, could be abstracted next time we
	* want to build support for a different 1-wire device,
	* or look at support via periph (or other library)
	 */
	devPath := "/sys/bus/w1/devices/" + s.OneWireFamily + "-" + s.OneWireID + "/w1_slave"
	dat, err := os.ReadFile(filepath.Clean(devPath))
	if err != nil {
		return math.NaN(), err
	}
	tempString := strings.TrimSuffix(string(dat), "\n")
	tempString = strings.Split(tempString, "t=")[1]
	tempMili, err := strconv.ParseFloat(tempString, 32)
	if err != nil {
		return math.NaN(), err
	}

	return tempMili / 1000, nil
}

// Readings returns a list of all readings.
func (s *Sensor) Readings(ctx context.Context) ([]interface{}, error) {
	temp, err := s.ReadTemperatureCelsius(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{temp}, nil
}

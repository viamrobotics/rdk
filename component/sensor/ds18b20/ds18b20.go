// Package ds18b20 implements a 1-wire temperature sensor
package ds18b20

import (
	"context"
	"math"
	"os"
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
	UniqueId string `json:"unique_id"`
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
			return newSensor(config.Name, config.ConvertedAttributes.(*AttrConfig).UniqueId)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeSensor, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(name string, id string) (sensor.Sensor, error) {
	// temp sensors are in family 28
	return &Sensor{Name: name, OneWireId: id, OneWireFamily: "28"}, nil
}

// Sensor is a fake Sensor device that always returns the set location.
type Sensor struct {
	Name          string
	OneWireId     string
	OneWireFamily string
}

func (s *Sensor) ReadTemperatureCelsius(ctx context.Context) (float64, error) {
	/*
	* logic here is specific to 1-wire protocol, could be abstracted next time we
	* want to build support for a different 1-wire device,
	* or look at support via periph (or other library)
	 */
	devPath := "/sys/bus/w1/devices/" + s.OneWireFamily + "-" + s.OneWireId + "/w1_slave"
	dat, err := os.ReadFile(devPath)
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

func (s *Sensor) Readings(ctx context.Context) ([]interface{}, error) {
	temp, err := s.ReadTemperatureCelsius(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{temp}, nil
}

// Package ds18b20 implements a 1-wire temperature sensor
package ds18b20

import (
	"context"
<<<<<<< HEAD
	"math"
	"os"
=======
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
>>>>>>> 1a12beba46cb857be5071ce3ae508dc366b890b3
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
<<<<<<< HEAD
	UniqueId string `json:"unique_id"`
=======
	UniqueID string `json:"unique_id"`
>>>>>>> 1a12beba46cb857be5071ce3ae508dc366b890b3
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
<<<<<<< HEAD
			return newSensor(config.Name, config.ConvertedAttributes.(*AttrConfig).UniqueId)
=======
			return newSensor(config.Name, config.ConvertedAttributes.(*AttrConfig).UniqueID), nil
>>>>>>> 1a12beba46cb857be5071ce3ae508dc366b890b3
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeSensor, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

<<<<<<< HEAD
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
	devPath := "/sys/bus/w1/devices/" + s.OneWireFamily + "-" + s.OneWireId + "/w1_slave"
	dat, err := os.ReadFile(devPath)
=======
func newSensor(name string, id string) sensor.Sensor {
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
>>>>>>> 1a12beba46cb857be5071ce3ae508dc366b890b3
	if err != nil {
		return math.NaN(), err
	}
	tempString := strings.TrimSuffix(string(dat), "\n")
<<<<<<< HEAD
	tempString = strings.Split(tempString, "t=")[1]
	tempMili, err := strconv.ParseFloat(tempString, 32)
	if err != nil {
		return math.NaN(), err
	}

	return tempMili / 1000, nil
}

func (s *Sensor) Readings(ctx context.Context) ([]interface{}, error) {
=======
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
>>>>>>> 1a12beba46cb857be5071ce3ae508dc366b890b3
	temp, err := s.ReadTemperatureCelsius(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{temp}, nil
}

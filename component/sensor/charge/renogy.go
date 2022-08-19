// Package renogy implements a charge controller sensor
package charge

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/goburrow/modbus"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

var (
	globalMu sync.Mutex
)

// defaults assume the device is connected via UART serial
const (
	modelname    = "renogy"
	path_default = "/dev/serial0"
	baud_default = 9600
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Path string `json:"path"`
	Baud int    `json:"baud"`
}

type charge struct {
	SolarVolt         float32
	SolarAmp          float32
	SolarWatt         float32
	LoadVolt          float32
	LoadAmp           float32
	LoadWatt          float32
	BattVolt          float32
	BattChargePct     uint
	BattDegC          float32
	ControllerDegC    float32
	MaxSolarTodayWatt float32
	MinSolarTodayWatt float32
	MaxBattTodayVolt  float32
	MinBattTodayVolt  float32
}

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
			return newSensor(config.Name, config.ConvertedAttributes.(*AttrConfig).Path, config.ConvertedAttributes.(*AttrConfig).Baud, config.ConvertedAttributes.(*AttrConfig).TestChan, logger)
		}})

	config.RegisterComponentAttributeMapConverter(sensor.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(name string, path string, baud int, logger golog.Logger) (sensor.Sensor, error) {
	if path == "" {
		path = path_default
	}
	if baud == 0 {
		baud = baud_default
	}

	return &Sensor{Name: name, path: path, baud: baud}, nil
}

// Sensor is a serial charge controller
type Sensor struct {
	Name string
	path string
	baud int
	generic.Unimplemented
}

// ReadTemperatureCelsius returns current temperature in celsius.
func (s *Sensor) ReadFromController(ctx context.Context) (charge, error) {
	var chargeRes charge
	handler := modbus.NewRTUClientHandler(s.path)
	handler.BaudRate = s.baud
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	handler.SlaveId = 1
	handler.Timeout = 2 * time.Second

	err := handler.Connect()
	defer handler.Close()
	if err != nil {
		return chargeRes, err
	}
	client := modbus.NewClient(handler)
	results, err := client.ReadDiscreteInputs(15, 2)

	return chargeRes, nil
}

// GetReadings returns a list containing single item (current temperature).
func (s *Sensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	readings, err := s.ReadFromController(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{readings}, nil
}

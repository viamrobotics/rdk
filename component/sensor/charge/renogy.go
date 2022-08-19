// Package renogy implements a charge controller sensor
package charge

import (
	"context"
	"encoding/binary"
	"math"
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
	BattChargePct     float32
	BattDegC          int16
	ControllerDegC    int16
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

// GetReadings returns a list containing single item (current temperature).
func (s *Sensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	readings, err := s.GetControllerOutput(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{readings}, nil
}

// GetControllerOutput returns current readings from the charge controller
func (s *Sensor) GetControllerOutput(ctx context.Context) (charge, error) {
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
	chargeRes.SolarVolt = readRegister(client, 263, 1)
	chargeRes.SolarAmp = readRegister(client, 264, 2)
	chargeRes.SolarWatt = readRegister(client, 265, 1)
	chargeRes.LoadVolt = readRegister(client, 260, 1)
	chargeRes.LoadAmp = readRegister(client, 261, 2)
	chargeRes.LoadWatt = readRegister(client, 262, 1)
	chargeRes.BattVolt = readRegister(client, 257, 1)
	chargeRes.BattChargePct = readRegister(client, 256, 1)
	tempReading := readRegister(client, 259, 1)
	battTempSign := (int16(tempReading) & 0b0000000010000000) >> 7
	battTemp := int16(tempReading) & 0b0000000001111111
	if battTempSign == 1 {
		battTemp = -battTemp
	}
	chargeRes.BattDegC = battTemp
	ctlTempSign := (int32(tempReading) & 0b1000000000000000) >> 15
	ctlTemp := (int16(tempReading) & 0b0111111100000000) >> 8
	if ctlTempSign == 1 {
		ctlTemp = -ctlTemp
	}
	chargeRes.ControllerDegC = ctlTemp
	chargeRes.MaxSolarTodayWatt = readRegister(client, 271, 1)
	chargeRes.MinSolarTodayWatt = readRegister(client, 272, 1)
	chargeRes.MaxBattTodayVolt = readRegister(client, 268, 1)
	chargeRes.MinBattTodayVolt = readRegister(client, 267, 1)

	return chargeRes, nil
}

func readRegister(client modbus.Client, register uint16, precision uint) (result float32) {
	b, err := client.ReadHoldingRegisters(register, 1)
	if err != nil {
		result = 0
	} else {
		result = float32frombytes(b, precision)
	}
	return result
}

func float32frombytes(bytes []byte, precision uint) float32 {
	bits := binary.LittleEndian.Uint32(bytes)
	float := math.Float32frombits(bits)
	ratio := math.Pow(10, float64(precision))
	float = float32(math.Round(float64(float)*ratio) / ratio)
	return float
}

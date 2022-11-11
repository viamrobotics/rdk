// Package charge implements a charge controller sensor
package charge

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/goburrow/modbus"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

var globalMu sync.Mutex

// defaults assume the device is connected via UART serial.
const (
	modelname       = "renogy"
	pathDefault     = "/dev/serial0"
	baudDefault     = 9600
	modbusIDDefault = 1
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Path     string `json:"serial_path"`
	Baud     int    `json:"serial_baud_rate"`
	ModbusID byte   `json:"modbus_id"`
}

// Charge represents a charge state.
type Charge struct {
	SolarVolt             float32
	SolarAmp              float32
	SolarWatt             float32
	LoadVolt              float32
	LoadAmp               float32
	LoadWatt              float32
	BattVolt              float32
	BattChargePct         float32
	BattDegC              int16
	ControllerDegC        int16
	MaxSolarTodayWatt     float32
	MinSolarTodayWatt     float32
	MaxBattTodayVolt      float32
	MinBattTodayVolt      float32
	MaxSolarTodayAmp      float32
	MinSolarTodayAmp      float32
	ChargeTodayWattHrs    float32
	DischargeTodayWattHrs float32
	ChargeTodayAmpHrs     float32
	DischargeTodayAmpHrs  float32
	TotalBattOverCharges  float32
	TotalBattFullCharges  float32
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
			return newSensor(config.Name, config.ConvertedAttributes.(*AttrConfig).Path,
				config.ConvertedAttributes.(*AttrConfig).Baud, config.ConvertedAttributes.(*AttrConfig).ModbusID), nil
		}})

	config.RegisterComponentAttributeMapConverter(sensor.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(name, path string, baud int, modbusID byte) sensor.Sensor {
	if path == "" {
		path = pathDefault
	}
	if baud == 0 {
		baud = baudDefault
	}
	if modbusID == 0 {
		modbusID = modbusIDDefault
	}

	return &Sensor{Name: name, path: path, baud: baud, modbusID: modbusID}
}

// Sensor is a serial charge controller.
type Sensor struct {
	Name     string
	path     string
	baud     int
	modbusID byte
	generic.Unimplemented
}

// Readings returns a list containing single item (current temperature).
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := s.GetControllerOutput(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	j, err := json.Marshal(readings)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(j, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// GetControllerOutput returns current readings from the charge controller.
func (s *Sensor) GetControllerOutput(ctx context.Context) (Charge, error) {
	var chargeRes Charge
	handler := modbus.NewRTUClientHandler(s.path)
	handler.BaudRate = s.baud
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	handler.SlaveId = s.modbusID
	handler.Timeout = 1 * time.Second

	err := handler.Connect()
	if err != nil {
		err = handler.Close()
		return chargeRes, err
	}
	client := modbus.NewClient(handler)
	chargeRes.SolarVolt = readRegister(client, 263, 1)
	chargeRes.SolarAmp = readRegister(client, 264, 2)
	chargeRes.SolarWatt = readRegister(client, 265, 0)
	chargeRes.LoadVolt = readRegister(client, 260, 1)
	chargeRes.LoadAmp = readRegister(client, 261, 2)
	chargeRes.LoadWatt = readRegister(client, 262, 0)
	chargeRes.BattVolt = readRegister(client, 257, 1)
	chargeRes.BattChargePct = readRegister(client, 256, 0)
	tempReading := readRegister(client, 259, 0)
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
	chargeRes.MaxSolarTodayWatt = readRegister(client, 271, 0)
	chargeRes.MinSolarTodayWatt = readRegister(client, 272, 0)
	chargeRes.MaxBattTodayVolt = readRegister(client, 268, 1)
	chargeRes.MinBattTodayVolt = readRegister(client, 267, 1)
	chargeRes.MaxSolarTodayAmp = readRegister(client, 269, 2)
	chargeRes.MinSolarTodayAmp = readRegister(client, 270, 1)
	chargeRes.ChargeTodayAmpHrs = readRegister(client, 273, 0)
	chargeRes.DischargeTodayAmpHrs = readRegister(client, 274, 0)
	chargeRes.ChargeTodayWattHrs = readRegister(client, 275, 0)
	chargeRes.DischargeTodayWattHrs = readRegister(client, 276, 0)
	chargeRes.TotalBattOverCharges = readRegister(client, 278, 0)
	chargeRes.TotalBattFullCharges = readRegister(client, 279, 0)

	err = handler.Close()
	return chargeRes, err
}

func readRegister(client modbus.Client, register uint16, precision uint) (result float32) {
	globalMu.Lock()
	b, err := client.ReadHoldingRegisters(register, 1)
	globalMu.Unlock()
	if err != nil {
		result = 0
	} else {
		if len(b) > 0 {
			result = float32FromBytes(b, precision)
		} else {
			result = 0
		}
	}
	return result
}

func float32FromBytes(bytes []byte, precision uint) float32 {
	i := binary.BigEndian.Uint16(bytes)
	ratio := math.Pow(10, float64(precision))
	return float32(float64(i) / ratio)
}

// Package renogy implements the renogy charge controller sensor.
// renogy wanderer: https://www.renogy.com/content/RNG-CTRL-WND30-LI/WND30-LI-Manual.pdf
// LCD Wanderer: https://ca.renogy.com/content/manual/RNG-CTRL-WND10-Manual.pdf
package renogy

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/goburrow/modbus"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

var (
	globalMu sync.Mutex
	model    = resource.DefaultModelFamily.WithModel("renogy")
)

// defaults assume the device is connected via UART serial.
const (
	pathDefault     = "/dev/serial0"
	baudDefault     = 9600
	modbusIDDefault = 1
)

// Config is used for converting config attributes.
type Config struct {
	resource.TriviallyValidateConfig
	Path     string `json:"serial_path,omitempty"`
	Baud     int    `json:"serial_baud_rate,omitempty"`
	ModbusID byte   `json:"modbus_id,omitempty"`
}

// Charge represents the solar charge controller readings.
type Charge struct {
	SolarVolt             float32
	SolarAmp              float32
	SolarWatt             float32
	LoadVolt              float32
	LoadAmp               float32
	LoadWatt              float32
	BattVolt              float32
	BattChargePct         float32
	BattChargeCurrent     float32
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
	resource.RegisterComponent(
		powersensor.API,
		model,
		resource.Registration[powersensor.PowerSensor, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (powersensor.PowerSensor, error) {
				newConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}
				return newRenogy(conf.ResourceName(), newConf.Path,
					newConf.Baud, newConf.ModbusID), nil
			},
		})
}

func newRenogy(name resource.Name, path string, baud int, modbusID byte) powersensor.PowerSensor {
	if path == "" {
		path = pathDefault
	}
	if baud == 0 {
		baud = baudDefault
	}
	if modbusID == 0 {
		modbusID = modbusIDDefault
	}

	return &Renogy{
		Named:    name.AsNamed(),
		path:     path,
		baud:     baud,
		modbusID: modbusID,
	}
}

// Renogy is a serial charge controller.
type Renogy struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	path     string
	baud     int
	modbusID byte
}

func (r *Renogy) getHandler() *modbus.RTUClientHandler {
	handler := modbus.NewRTUClientHandler(r.path)
	handler.BaudRate = r.baud
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	handler.SlaveId = r.modbusID
	handler.Timeout = 1 * time.Second
	return handler
}

// Voltage returns the voltage of the battery and a boolean IsAc.
func (r *Renogy) Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	handler := r.getHandler()

	err := handler.Connect()
	if err != nil {
		return 0, false, err
	}
	client := modbus.NewClient(handler)

	// Eead the battery voltage.
	volts := readRegister(client, 257, 1)
	isAc := false

	err = handler.Close()
	if err != nil {
		return 0, false, err
	}

	return float64(volts), isAc, nil
}

// Current returns the load's current and boolean isAC.
// If the controller does not have a load input, will return zero.
func (r *Renogy) Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	handler := r.getHandler()

	client := modbus.NewClient(handler)

	// read the load current.
	loadCurrent := readRegister(client, 261, 2)
	isAc := false

	err := handler.Close()
	if err != nil {
		return 0, false, err
	}

	return float64(loadCurrent), isAc, nil
}

// Power returns the power of the load. If the controller does not have a load input, will return zero.
func (r *Renogy) Power(ctx context.Context, extra map[string]interface{}) (float64, error) {
	handler := r.getHandler()
	err := handler.Connect()
	if err != nil {
		return 0, err
	}

	client := modbus.NewClient(handler)

	// reads the load wattage.
	loadPower := readRegister(client, 262, 0)

	err = handler.Close()
	if err != nil {
		return 0, err
	}
	return float64(loadPower), err
}

// Readings returns a list of all readings from the sensor.
func (r *Renogy) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := r.GetControllerOutput(ctx)
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
func (r *Renogy) GetControllerOutput(ctx context.Context) (Charge, error) {
	var chargeRes Charge
	handler := r.getHandler()

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
	chargeRes.BattChargeCurrent = readRegister(client, 258, 0)
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

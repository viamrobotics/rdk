// Package renogy implements the renogy charge controller sensor
// Tested with renogy wanderer model
// Wanderer Manual: https://www.renogy.com/content/RNG-CTRL-WND30-LI/WND30-LI-Manual.pdf
// LCD Wanderer Manual: https://ca.renogy.com/content/manual/RNG-CTRL-WND10-Manual.pdf
package renogy

import (
	"context"
	"encoding/binary"
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
	readings map[string]interface{}
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
const (
	SolarVoltReg             = 263
	SolarAmpReg              = 264
	SolarWattReg             = 265
	LoadVoltReg              = 260
	LoadAmpReg               = 261
	LoadWattReg              = 262
	BattVoltReg              = 257
	BattChargePctReg         = 256
	ControllerDegCReg        = 259
	MaxSolarTodayWattReg     = 271
	MinSolarTodayWattReg     = 272
	MaxBattTodayVoltReg      = 268
	MinBattTodayVoltReg      = 267
	MaxSolarTodayAmpReg      = 269
	MinSolarTodayAmpReg      = 270
	ChargeTodayWattHrsReg    = 273
	DischargeTodayWattHrsReg = 274
	ChargeTodayAmpHrsReg     = 275
	DischargeTodayAmpHrsReg  = 276
	TotalBattOverChargesReg  = 278
	TotalBattFullChargesReg  = 279
)

func init() {
	resource.RegisterComponent(
		powersensor.API,
		model,
		resource.Registration[powersensor.PowerSensor, *Config]{
			Constructor: newRenogy,
		})
}

func newRenogy(_ context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (powersensor.PowerSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	if newConf.Path == "" {
		newConf.Path = pathDefault
	}
	if newConf.Baud == 0 {
		newConf.Baud = baudDefault
	}
	if newConf.ModbusID == 0 {
		newConf.ModbusID = modbusIDDefault
	}

	return &Renogy{
		Named:    conf.ResourceName().AsNamed(),
		path:     newConf.Path,
		baud:     newConf.Baud,
		modbusID: newConf.ModbusID,
	}, nil
}

// Renogy is a serial charge controller.
type Renogy struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger   golog.Logger
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

	// Read the battery voltage.
	volts, err := readRegister(client, 257, 1)
	if err != nil {
		return 0, false, err
	}
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
	loadCurrent, err := readRegister(client, 261, 2)
	if err != nil {
		return 0, false, err
	}
	isAc := false

	err = handler.Close()
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
	loadPower, err := readRegister(client, 262, 0)
	if err != nil {
		return 0, err
	}

	err = handler.Close()
	if err != nil {
		return 0, err
	}
	return float64(loadPower), err
}

// Readings returns a list of all readings from the sensor.
func (r *Renogy) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	handler := r.getHandler()

	err := handler.Connect()
	if err != nil {
		err = handler.Close()
		return nil, err
	}

	readings = make(map[string]interface{})

	client := modbus.NewClient(handler)

	r.addReading(client, SolarVoltReg, 1, "SolarVolt")
	r.addReading(client, SolarAmpReg, 2, "SolarAmp")
	r.addReading(client, SolarWattReg, 0, "SolarWatt")
	r.addReading(client, LoadVoltReg, 1, "LoadVolt")
	r.addReading(client, LoadAmpReg, 2, "LoadAmp")
	r.addReading(client, LoadWattReg, 0, "LoadWatt")
	r.addReading(client, BattVoltReg, 1, "BattVolt")
	r.addReading(client, BattChargePctReg, 0, "BattChargePct")
	r.addReading(client, MaxSolarTodayWattReg, 0, "MaxSolarTodayWatt")
	r.addReading(client, MinSolarTodayWattReg, 0, "MinSolarTodayWatt")
	r.addReading(client, MaxBattTodayVoltReg, 1, "MaxBattTodayVolt")
	r.addReading(client, MinBattTodayVoltReg, 1, "MinBattTodayVolt")
	r.addReading(client, MaxSolarTodayAmpReg, 2, "MaxSolarTodayAmp")
	r.addReading(client, MinSolarTodayAmpReg, 1, "MinSolarTodayAmp")
	r.addReading(client, ChargeTodayAmpHrsReg, 0, "ChargeTodayAmpHrs")
	r.addReading(client, DischargeTodayAmpHrsReg, 0, "DischargeTodayAmpHrs")
	r.addReading(client, ChargeTodayWattHrsReg, 0, "ChargeTodayWattHrs")
	r.addReading(client, DischargeTodayWattHrsReg, 0, "DischargeTodayWattHrs")
	r.addReading(client, TotalBattOverChargesReg, 0, "TotalBattOverCharges")
	r.addReading(client, TotalBattFullChargesReg, 0, "TotalBattFullCharges")

	// Controller and battery temperates require extra math to determine sign and temp.
	tempReading, err := readRegister(client, ControllerDegCReg, 0)
	if err != nil {
		return readings, err
	}

	battTempSign := (int16(tempReading) & 0b0000000010000000) >> 7
	battTemp := int16(tempReading) & 0b0000000001111111
	if battTempSign == 1 {
		battTemp = -battTemp
	}

	readings["BattDegC"] = int32(battTemp)

	ctlTempSign := (int32(tempReading) & 0b1000000000000000) >> 15
	ctlTemp := (int16(tempReading) & 0b0111111100000000) >> 8
	if ctlTempSign == 1 {
		ctlTemp = -ctlTemp
	}
	readings["ControllerDegC"] = int32(ctlTemp)

	err = handler.Close()
	return readings, err
}

func (r *Renogy) addReading(client modbus.Client, register uint16, precision uint, reading string) {
	value, err := readRegister(client, register, precision)
	if err != nil {
		r.logger.Errorf("error getting reading: %s", reading)
		r.logger.Error(err)
	} else {
		readings[reading] = value
	}
}

func readRegister(client modbus.Client, register uint16, precision uint) (result float32, err error) {
	globalMu.Lock()
	b, err := client.ReadHoldingRegisters(register, 1)
	globalMu.Unlock()
	if err != nil {
		return 0, err
	} else {
		if len(b) > 0 {
			result = float32FromBytes(b, precision)
		} else {
			result = 0
		}
	}
	return result, nil
}

func float32FromBytes(bytes []byte, precision uint) float32 {
	i := binary.BigEndian.Uint16(bytes)
	ratio := math.Pow(10, float64(precision))
	return float32(float64(i) / ratio)
}

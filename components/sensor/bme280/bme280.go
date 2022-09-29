// Package bme280 implements a bme280 sensor for temperature, humidity, and pressure.
// Code based on https://github.com/sparkfun/SparkFun_BME280_Arduino_Library (MIT license)
// and also https://github.com/rm-hull/bme280 (MIT License)
package bme280

import (
	"context"
	"fmt"
	"math"
	"time"
	"errors"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const (
	modelname      = "bme280"
	defaultI2Caddr = 0x77
	defaultBaud    = 100000

	// Addresses of bme280 registers.
	Bme280_T1_LSB_Reg       = 0x88
	Bme280_T1_MSB_Reg       = 0x89
	Bme280_T2_LSB_Reg       = 0x8A
	Bme280_T2_MSB_Reg       = 0x8B
	Bme280_T3_LSB_Reg       = 0x8C
	Bme280_T3_MSB_Reg       = 0x8D
	Bme280_P1_LSB_Reg       = 0x8E
	Bme280_P1_MSB_Reg       = 0x8F
	Bme280_P2_LSB_Reg       = 0x90
	Bme280_P2_MSB_Reg       = 0x91
	Bme280_P3_LSB_Reg       = 0x92
	Bme280_P3_MSB_Reg       = 0x93
	Bme280_P4_LSB_Reg       = 0x94
	Bme280_P4_MSB_Reg       = 0x95
	Bme280_P5_LSB_Reg       = 0x96
	Bme280_P5_MSB_Reg       = 0x97
	Bme280_P6_LSB_Reg       = 0x98
	Bme280_P6_MSB_Reg       = 0x99
	Bme280_P7_LSB_Reg       = 0x9A
	Bme280_P7_MSB_Reg       = 0x9B
	Bme280_P8_LSB_Reg       = 0x9C
	Bme280_P8_MSB_Reg       = 0x9D
	Bme280_P9_LSB_Reg       = 0x9E
	Bme280_P9_MSB_Reg       = 0x9F
	Bme280_H1_Reg           = 0xA1
	Bme280_CHIP_ID_Reg          = 0xD0 // Chip ID
	Bme280_RST_Reg              = 0xE0 // Softreset Reg
	Bme280_H2_LSB_Reg       = 0xE1
	Bme280_H2_MSB_Reg       = 0xE2
	Bme280_H3_Reg           = 0xE3
	Bme280_H4_MSB_Reg       = 0xE4
	Bme280_H4_LSB_Reg       = 0xE5
	Bme280_H5_MSB_Reg       = 0xE6
	Bme280_H6_Reg           = 0xE7
	Bme280_CTRL_Humidity_Reg    = 0xF2 // Ctrl Humidity Reg
	Bme280_STAT_Reg             = 0xF3 // Status Reg
	Bme280_CTRL_MEAS_Reg        = 0xF4 // Ctrl Measure Reg
	Bme280_Config_Reg           = 0xF5 // Configuration Reg
	Bme280_Measurements_Reg     = 0xF7 // Measurements register start
	Bme280_Pressure_MSB_Reg     = 0xF7 // Pressure MSB
	Bme280_Pressure_LSB_Reg     = 0xF8 // Pressure LSB
	Bme280_Pressure_XLSB_Reg    = 0xF9 // Pressure XLSB
	Bme280_Temperature_MSB_Reg  = 0xFA // Temperature MSB
	Bme280_Temperature_LSB_Reg  = 0xFB // Temperature LSB
	Bme280_Temperature_XLSB_Reg = 0xFC // Temperature XLSB
	Bme280_Humidity_MSB_Reg     = 0xFD // Humidity MSB
	Bme280_Humidity_LSB_Reg     = 0xFE // Humidity LSB
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	UniqueID string `json:"unique_id"`
	Board    string `json:"board,omitempty"`

	*I2CAttrConfig `json:"i2c_attributes,omitempty"`
}

// I2CAttrConfig is used for converting Serial NMEA MovementSensor config attributes.
type I2CAttrConfig struct {
	I2CBus      string `json:"i2c_bus"`
	I2cAddr     int    `json:"i2c_addr,omitempty"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
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
			return newSensor(ctx, deps, config.Name, config.ConvertedAttributes.(*AttrConfig), logger)
		}})

	config.RegisterComponentAttributeMapConverter(sensor.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(
	ctx context.Context,
	deps registry.Dependencies,
	name string,
	attr *AttrConfig,
	logger golog.Logger,
) (sensor.Sensor, error) {
	b, err := board.FromDependencies(deps, attr.Board)
	if err != nil {
		return nil, fmt.Errorf("bme280 init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", attr.Board)
	}
	i2cbus, ok := localB.I2CByName(attr.I2CAttrConfig.I2CBus)
	if !ok {
		return nil, fmt.Errorf("bme280 init: failed to find i2c bus %s", attr.I2CAttrConfig.I2CBus)
	}
	addr := attr.I2CAttrConfig.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Warn("using i2c address : 0x77")
	}
	baudrate := attr.I2CAttrConfig.I2CBaudRate
	if baudrate == 0 {
		baudrate = defaultBaud
		logger.Warn("using default baudrate : 100000")
	}

	s := &bme280{
		name:   name,
		logger: logger,
		bus:    i2cbus,
		addr:   byte(addr),
		wbaud:  baudrate,
		lastTemp: -999, // initialize to impossible temp
	}
	
	s.reset(ctx)
	time.Sleep(50 * time.Millisecond)
	s.calibration = map[string]int{}
	s.setupCalibration(ctx)
	s.setMode(ctx, 0b11)
	
	s.setStandbyTime(ctx, 0)
	s.setFilter(ctx, 0)
	s.setPressureOverSample(ctx, 1) //Default of 1x oversample
	s.setHumidityOverSample(ctx, 1) //Default of 1x oversample
	s.setTempOverSample(ctx, 1) //Default of 1x oversample
	
	return s, nil
}

// bme280 is a i2c sensor device.
type bme280 struct {
	generic.Unimplemented
	logger golog.Logger

	bus         board.I2C
	addr        byte
	wbaud       int
	name        string
	calibration map[string]int
	lastTemp    float64 // Store raw data from temp for humidity calculations
}

// Readings returns a list containing single item (current temperature).
func (s *bme280) Readings(ctx context.Context) (map[string]interface{}, error) {
	
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		s.logger.Errorf("can't open bme280 i2c %s", err)
		return nil, err
	}
	buffer := []byte{}
	err = handle.Write(ctx, []byte{byte(Bme280_Measurements_Reg)})
	if err != nil {
		s.logger.Debug("Failed to request temperature")
	}
	buffer, err = handle.Read(ctx, 8)
	if err != nil {
		return nil, err
	}
	if len(buffer) != 8 {
		return nil, errors.New("i2c read did not get 8 bytes")
	}
	
	pressure := s.readPressure(buffer)
	temp := s.readTemperatureCelsius(buffer)
	humid := s.readHumidity(buffer)
	dewPt := s.calculateDewPoint(temp, humid)
	return map[string]interface{}{
		"temperature_celsius": temp,
		"dew_point_celsius": dewPt,
		"temperature_farenheit": temp * 1.8 + 32,
		"dew_point_farenheit": dewPt * 1.8 + 32,
		"humidity_pct_rh": humid,
		"pressure_mpa": pressure,
	}, handle.Close()
}

// readPressure returns current pressure in mPa
func (s *bme280) readPressure(buffer []byte) float64 {

	adc := float64((int(buffer[0])<<16 | int(buffer[1])<<8 | int(buffer[2])) >> 4)
	
	// Need temp to calculate humidity
	if s.lastTemp == -999 {
		s.readTemperatureCelsius(buffer)
	}
	
	v1 := s.lastTemp/2. - 64000.
	v2 := v1 * v1 * float64(s.calibration["dig_P6"]) / 32768.0
	v2 = v2 + v1 * float64(s.calibration["dig_P5"]) *2.
	v2 = v2 / 4. + float64(s.calibration["dig_P4"]) * 65536.
	v1 = (float64(s.calibration["dig_P3"]) * v1 * v1 / 524288.0 + float64(s.calibration["dig_P2"]) * v1) / 524288.0
	v1 = (1.0 + v1 / 32768.0) * float64(s.calibration["dig_P1"])
	
	if v1 == 0 {
		return 0
	}
	
	res := 1048576.0 - adc
	res = ((res - v2 / 4096.0) * 6250.0) / v1
	v1 = float64(s.calibration["dig_P9"]) * res * res / 2147483648.0
	v2 = res * float64(s.calibration["dig_P8"]) / 32768.0
	res = res + (v1 + v2 + float64(s.calibration["dig_P7"])) / 16.0
	return res/100.
}

// readTemperatureCelsius returns current temperature in celsius.
func (s *bme280) readTemperatureCelsius(buffer []byte) float64 {
	adc := float64((int(buffer[3])<<16 | int(buffer[4])<<8 | int(buffer[5])) >> 4)
	var1 := (adc/16382 - float64(s.calibration["dig_T1"])/1024) * float64(s.calibration["dig_T2"])
	var2 := math.Pow((adc/131072 - float64(s.calibration["dig_T1"])/8192), 2) * float64(s.calibration["dig_T3"])

	tFine := var1 + var2
	s.lastTemp = tFine
	output := float64(tFine)/5120.
	return output
}

// readHumidity returns current humidity as %RH
func (s *bme280) readHumidity(buffer []byte) float64 {

	adc := float64(int(buffer[6])<<8 | int(buffer[7]))
	
	// Need temp to calculate humidity
	if s.lastTemp == -999 {
		s.readTemperatureCelsius(buffer)
	}
	
	var1 := float64(s.lastTemp) - 76800.
	var1 = (adc - (float64(s.calibration["dig_H4"]) * 64.0 + float64(s.calibration["dig_H5"]) / 16384.0 * var1)) *
		(float64(s.calibration["dig_H2"]) / 65536.0 * (1.0 + float64(s.calibration["dig_H6"]) / 67108864.0 * var1 * (1.0 +
		float64(s.calibration["dig_H3"]) / 67108864.0 * var1)))
	var1 = var1 * (1.0 - (float64(s.calibration["dig_H1"]) * var1 / 524288.0))
	return math.Max(0., math.Min(var1, 100.))
}

// calculateDewPoint returns current dew point in degrees C
func (s *bme280) calculateDewPoint(temp, humid float64) float64 {
	ratio := 373.15 / (273.15 + temp)
	rhs := -7.90298 * (ratio - 1)
	rhs += 5.02808 * math.Log10(ratio)
	rhs += -1.3816e-7 * (math.Pow(10, (11.344 * (1 - 1/ratio ))) - 1)
	rhs += 8.1328e-3 * (math.Pow(10, (-3.49149 * (ratio - 1))) - 1)
	rhs += math.Log10(1013.246)
	
	// factor -3 is to adjust units - Vapor Pressure SVP * humidity
	vp := math.Pow(10, rhs - 3) * humid
	// (2) DEWPOINT = F(Vapor Pressure)
	t := math.Log(vp/0.61078)   // temp var
	return (241.88 * t) / (17.558 - t)
}

func (s *bme280) reset(ctx context.Context) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	err = handle.WriteByteData(ctx, Bme280_RST_Reg, 0xB6)
	if err != nil {
		return err
	}
	return handle.Close()
}

//Set the mode bits in the ctrl_meas register
// Mode 00 = Sleep
// 01 and 10 = Forced
// 11 = Normal mode
func (s *bme280) setMode(ctx context.Context, mode int) error {
	if(mode > 0b11) {
		mode = 0 //Error check. Default to sleep mode
	}
	
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	controlDataByte, err := handle.ReadByteData(ctx, Bme280_CTRL_MEAS_Reg)
	if err != nil {
		return err
	}
	controlData := int(controlDataByte)
	controlData &= ^( (1<<1) | (1<<0) ) //Clear the mode[1:0] bits
	controlData |= mode //Set
	err = handle.WriteByteData(ctx, Bme280_CTRL_MEAS_Reg, byte(controlData))
	if err != nil {
		return err
	}
	return handle.Close()
}

func (s *bme280) currentMode(ctx context.Context) (int, error) {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return -1, err
	}
	controlDataByte, err := handle.ReadByteData(ctx, Bme280_CTRL_MEAS_Reg)
	if err != nil {
		return -1, err
	}

	return (int(controlDataByte) & 0b00000011), handle.Close()
}

func (s *bme280) isMeasuring(ctx context.Context) (bool, error) {
	
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return false, err
	}
	stat, err := handle.ReadByteData(ctx, Bme280_STAT_Reg)
	if err != nil {
		return false, err
	}
	return stat & (1<<3) == 1, handle.Close()

}

func (s *bme280) setStandbyTime(ctx context.Context, val byte) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	
	if val > 0b111 {
		val = 0
	}
	controlData, err := handle.ReadByteData(ctx, Bme280_Config_Reg)
	if err != nil {
		return err
	}
	controlData &= ^( (byte(1)<<7) | (byte(1)<<6) | (byte(1)<<5) )
	controlData |= (val << 5)
	
	err = handle.WriteByteData(ctx, Bme280_Config_Reg, controlData)
	if err != nil {
		return err
	}
	return handle.Close()
}

func (s *bme280) setFilter(ctx context.Context, val byte) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	
	if val > 0b111 {
		val = 0
	}
	controlData, err := handle.ReadByteData(ctx, Bme280_Config_Reg)
	if err != nil {
		return err
	}
	controlData &= ^( (byte(1)<<4) | (byte(1)<<3) | (byte(1)<<2) )
	controlData |= (val << 2)
	
	err = handle.WriteByteData(ctx, Bme280_Config_Reg, controlData)
	if err != nil {
		return err
	}
	return handle.Close()
}

func (s *bme280) setTempOverSample(ctx context.Context, val byte) error {
	mode, err := s.currentMode(ctx)
	if err != nil {
		return err
	}
	err = s.setMode(ctx, 0b00)
	if err != nil {
		return err
	}
	
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	
	controlData, err := handle.ReadByteData(ctx, Bme280_CTRL_MEAS_Reg)
	if err != nil {
		return err
	}
	controlData &= ^( (byte(1)<<7) | (byte(1)<<6) | (byte(1)<<5) )
	controlData |= (val << 5)
	
	err = handle.WriteByteData(ctx, Bme280_CTRL_MEAS_Reg, controlData)
	if err != nil {
		return err
	}
	err = s.setMode(ctx, mode)
	if err != nil {
		return err
	}
	
	return handle.Close()
}

func (s *bme280) setPressureOverSample(ctx context.Context, val byte) error {
	mode, err := s.currentMode(ctx)
	if err != nil {
		return err
	}
	err = s.setMode(ctx, 0b00)
	if err != nil {
		return err
	}
	
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	
	controlData, err := handle.ReadByteData(ctx, Bme280_CTRL_MEAS_Reg)
	if err != nil {
		return err
	}
	controlData &= ^( (byte(1)<<4) | (byte(1)<<3) | (byte(1)<<2) )
	controlData |= (val << 2)
	
	err = handle.WriteByteData(ctx, Bme280_CTRL_MEAS_Reg, controlData)
	if err != nil {
		return err
	}
	err = s.setMode(ctx, mode)
	if err != nil {
		return err
	}
	
	return handle.Close()
}

// 
func (s *bme280) setHumidityOverSample(ctx context.Context, val byte) error {
	mode, err := s.currentMode(ctx)
	if err != nil {
		return err
	}
	err = s.setMode(ctx, 0b00)
	if err != nil {
		return err
	}
	
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	
	controlData, err := handle.ReadByteData(ctx, Bme280_CTRL_Humidity_Reg)
	if err != nil {
		return err
	}
	controlData &= ^( (byte(1)<<2) | (byte(1)<<1) | (byte(1)<<0) )
	controlData |= (val << 0)
	
	err = handle.WriteByteData(ctx, Bme280_CTRL_Humidity_Reg, controlData)
	if err != nil {
		return err
	}
	err = s.setMode(ctx, mode)
	if err != nil {
		return err
	}
	
	return handle.Close()
}

// setupCalibration sets up all calibration data for the chip
func (s *bme280) setupCalibration(ctx context.Context) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}

	if calib, err := handle.ReadWordData(ctx, Bme280_T1_LSB_Reg); err == nil {
		s.calibration["dig_T1"] = int(calib)
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_T2_LSB_Reg); err == nil {
		s.calibration["dig_T2"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_T3_LSB_Reg); err == nil {
		s.calibration["dig_T3"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P1_LSB_Reg); err == nil {
		s.calibration["dig_P1"] = int(calib)
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P2_LSB_Reg); err == nil {
		s.calibration["dig_P2"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P3_LSB_Reg); err == nil {
		s.calibration["dig_P3"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P4_LSB_Reg); err == nil {
		s.calibration["dig_P4"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P5_LSB_Reg); err == nil {
		s.calibration["dig_P5"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P6_LSB_Reg); err == nil {
		s.calibration["dig_P6"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P7_LSB_Reg); err == nil {
		s.calibration["dig_P7"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P8_LSB_Reg); err == nil {
		s.calibration["dig_P8"] = int(int16(calib))
	}else{
		return err
	}
	if calib, err := handle.ReadWordData(ctx, Bme280_P9_LSB_Reg); err == nil {
		s.calibration["dig_P9"] = int(int16(calib))
	}else{
		return err
	}
	
	calib, err := handle.ReadByteData(ctx, Bme280_H1_Reg)
	if err != nil {
		return err
	}
	s.calibration["dig_H1"] = int(calib)
	
	if calib, err := handle.ReadWordData(ctx, Bme280_H2_LSB_Reg); err == nil {
		s.calibration["dig_H2"] = int(int16(calib))
	}else{
		return err
	}
	
	calib, err = handle.ReadByteData(ctx, Bme280_H3_Reg)
	if err != nil {
		return err
	}
	s.calibration["dig_H3"] = int(calib)
	
	calib, err = handle.ReadByteData(ctx, Bme280_H6_Reg)
	if err != nil {
		return err
	}
	s.calibration["dig_H6"] = int(calib)

	r1byte, err := handle.ReadByteData(ctx, Bme280_H4_MSB_Reg)
	if err != nil {
		return err
	}
	r2byte, err := handle.ReadByteData(ctx, Bme280_H4_LSB_Reg)
	if err != nil {
		return err
	}
	s.calibration["dig_H4"] = (int(r1byte) << 4) + int(r2byte & 0x0f)

	r1byte, err = handle.ReadByteData(ctx, Bme280_H5_MSB_Reg)
	if err != nil {
		return err
	}
	r2byte, err = handle.ReadByteData(ctx, Bme280_H4_LSB_Reg)
	if err != nil {
		return err
	}
	s.calibration["dig_H5"] = (int(r1byte) << 4) + int((r2byte >> 4) & 0x0f)
	
	return handle.Close()
}

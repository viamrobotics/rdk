//go:build linux

// Package bme280 implements a bme280 sensor for temperature, humidity, and pressure.
// Code based on https://github.com/sparkfun/SparkFun_bme280_Arduino_Library (MIT license)
// and also https://github.com/rm-hull/bme280 (MIT License)
package bme280

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("bme280")

const (
	defaultI2Caddr = 0x77

	// When mode is set to 0, sensors are off.
	// When mode is set to 1 or 2, sensors are read once and then turn off again.
	// When mode is set to 3, sensors are on and continuously read.
	activeMode = 0b11

	// Addresses of bme280 registers.
	bme280T1LSBReg           = 0x88
	bme280T1MSBReg           = 0x89
	bme280T2LSBReg           = 0x8A
	bme280T2MSBReg           = 0x8B
	bme280T3LSBReg           = 0x8C
	bme280T3MSBReg           = 0x8D
	bme280P1LSBReg           = 0x8E
	bme280P1MSBReg           = 0x8F
	bme280P2LSBReg           = 0x90
	bme280P2MSBReg           = 0x91
	bme280P3LSBReg           = 0x92
	bme280P3MSBReg           = 0x93
	bme280P4LSBReg           = 0x94
	bme280P4MSBReg           = 0x95
	bme280P5LSBReg           = 0x96
	bme280P5MSBReg           = 0x97
	bme280P6LSBReg           = 0x98
	bme280P6MSBReg           = 0x99
	bme280P7LSBReg           = 0x9A
	bme280P7MSBReg           = 0x9B
	bme280P8LSBReg           = 0x9C
	bme280P8MSBReg           = 0x9D
	bme280P9LSBReg           = 0x9E
	bme280P9MSBReg           = 0x9F
	bme280H1Reg              = 0xA1
	bme280CHIPIDReg          = 0xD0 // Chip ID
	bme280RSTReg             = 0xE0 // Softreset Reg
	bme280H2LSBReg           = 0xE1
	bme280H2MSBReg           = 0xE2
	bme280H3Reg              = 0xE3
	bme280H4MSBReg           = 0xE4
	bme280H4LSBReg           = 0xE5
	bme280H5MSBReg           = 0xE6
	bme280H6Reg              = 0xE7
	bme280CTRLHumidityReg    = 0xF2 // Ctrl Humidity Reg
	bme280STATReg            = 0xF3 // Status Reg
	bme280CTRLMEASReg        = 0xF4 // Ctrl Measure Reg
	bme280ConfigReg          = 0xF5 // Configuration Reg
	bme280MeasurementsReg    = 0xF7 // Measurements register start
	bme280PressureMSBReg     = 0xF7 // Pressure MSB
	bme280PressureLSBReg     = 0xF8 // Pressure LSB
	bme280PressureXLSBReg    = 0xF9 // Pressure XLSB
	bme280TemperatureMSBReg  = 0xFA // Temperature MSB
	bme280TemperatureLSBReg  = 0xFB // Temperature LSB
	bme280TemperatureXLSBReg = 0xFC // Temperature XLSB
	bme280HumidityMSBReg     = 0xFD // Humidity MSB
	bme280HumidityLSBReg     = 0xFE // Humidity LSB
)

// Config is used for converting config attributes.
type Config struct {
	I2CBus  string `json:"i2c_bus"`
	I2cAddr int    `json:"i2c_addr,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string
	if len(conf.I2CBus) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c bus")
	}
	return deps, nil
}

func init() {
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (sensor.Sensor, error) {
				newConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}
				return newSensor(ctx, deps, conf.ResourceName(), newConf, logger)
			},
		})
}

func newSensor(
	ctx context.Context,
	deps resource.Dependencies,
	name resource.Name,
	conf *Config,
	logger golog.Logger,
) (sensor.Sensor, error) {
	i2cbus, err := genericlinux.NewI2cBus(conf.I2CBus)
	if err != nil {
		return nil, fmt.Errorf("bme280 init: failed to open i2c bus %s: %w",
			conf.I2CBus, err)
	}

	addr := conf.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Warn("using i2c address : 0x77")
	}

	s := &bme280{
		Named:    name.AsNamed(),
		logger:   logger,
		bus:      i2cbus,
		addr:     byte(addr),
		lastTemp: -999, // initialize to impossible temp
	}

	err = s.reset(ctx)
	if err != nil {
		return nil, err
	}
	// After sending the reset signal above, it takes the chip a short time to be ready to receive commands again
	time.Sleep(100 * time.Millisecond)
	s.calibration = map[string]int{}
	err = s.setupCalibration(ctx)
	if err != nil {
		return nil, err
	}
	err = s.setMode(ctx, activeMode)
	if err != nil {
		return nil, err
	}

	err = s.setStandbyTime(ctx, 0)
	if err != nil {
		return nil, err
	}
	err = s.setFilter(ctx, 0)
	if err != nil {
		return nil, err
	}

	// Oversample means "read the sensor this many times and average the results". 1 is generally fine.
	// Chip inits to 0 for all, which means "do not read this sensor"/
	// humidity
	err = s.setOverSample(ctx, bme280CTRLHumidityReg, 0, 1) // Default of 1x oversample
	if err != nil {
		return nil, err
	}
	// pressure
	err = s.setOverSample(ctx, bme280CTRLMEASReg, 2, 1) // Default of 1x oversample
	if err != nil {
		return nil, err
	}
	// temperature
	err = s.setOverSample(ctx, bme280CTRLMEASReg, 5, 1) // Default of 1x oversample
	if err != nil {
		return nil, err
	}

	return s, nil
}

// bme280 is a i2c sensor device.
type bme280 struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger golog.Logger

	bus         board.I2C
	addr        byte
	calibration map[string]int
	lastTemp    float64 // Store raw data from temp for humidity calculations
}

// Readings returns a list containing single item (current temperature).
func (s *bme280) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		s.logger.Errorf("can't open bme280 i2c %s", err)
		return nil, err
	}
	err = handle.Write(ctx, []byte{byte(bme280MeasurementsReg)})
	if err != nil {
		s.logger.Debug("Failed to request temperature")
	}
	buffer, err := handle.Read(ctx, 8)
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
		"temperature_celsius":    temp,
		"dew_point_celsius":      dewPt,
		"temperature_fahrenheit": temp*1.8 + 32,
		"dew_point_fahrenheit":   dewPt*1.8 + 32,
		"relative_humidity_pct":  humid,
		"pressure_mpa":           pressure,
	}, handle.Close()
}

// readPressure returns current pressure in mPa.
func (s *bme280) readPressure(buffer []byte) float64 {
	adc := float64((int(buffer[0])<<16 | int(buffer[1])<<8 | int(buffer[2])) >> 4)

	// Need temp to calculate humidity
	if s.lastTemp == -999 {
		s.readTemperatureCelsius(buffer)
	}

	v1 := s.lastTemp/2. - 64000.
	v2 := v1 * v1 * float64(s.calibration["digP6"]) / 32768.0
	v2 += v1 * float64(s.calibration["digP5"]) * 2.
	v2 = v2/4. + float64(s.calibration["digP4"])*65536.
	v1 = (float64(s.calibration["digP3"])*v1*v1/524288.0 + float64(s.calibration["digP2"])*v1) / 524288.0
	v1 = (1.0 + v1/32768.0) * float64(s.calibration["digP1"])

	if v1 == 0 {
		return 0
	}

	res := 1048576.0 - adc
	res = ((res - v2/4096.0) * 6250.0) / v1
	v1 = float64(s.calibration["digP9"]) * res * res / 2147483648.0
	v2 = res * float64(s.calibration["digP8"]) / 32768.0
	res += (v1 + v2 + float64(s.calibration["digP7"])) / 16.0
	return res / 100.
}

// readTemperatureCelsius returns current temperature in celsius.
func (s *bme280) readTemperatureCelsius(buffer []byte) float64 {
	adc := float64((int(buffer[3])<<16 | int(buffer[4])<<8 | int(buffer[5])) >> 4)
	var1 := (adc/16382 - float64(s.calibration["digT1"])/1024) * float64(s.calibration["digT2"])
	var2 := math.Pow((adc/131072-float64(s.calibration["digT1"])/8192), 2) * float64(s.calibration["digT3"])

	tFine := var1 + var2
	s.lastTemp = tFine
	output := tFine / 5120.
	return output
}

// readHumidity returns current humidity as %RH.
func (s *bme280) readHumidity(buffer []byte) float64 {
	adc := float64(int(buffer[6])<<8 | int(buffer[7]))

	// Need temp to calculate humidity
	if s.lastTemp == -999 {
		s.readTemperatureCelsius(buffer)
	}

	var1 := s.lastTemp - 76800.
	var1 = (adc - (float64(s.calibration["digH4"])*64.0 + float64(s.calibration["digH5"])/16384.0*var1)) *
		(float64(s.calibration["digH2"]) / 65536.0 * (1.0 + float64(s.calibration["digH6"])/67108864.0*var1*(1.0+
			float64(s.calibration["digH3"])/67108864.0*var1)))
	var1 *= (1.0 - (float64(s.calibration["digH1"]) * var1 / 524288.0))
	return math.Max(0., math.Min(var1, 100.))
}

// calculateDewPoint returns current dew point in degrees C.
func (s *bme280) calculateDewPoint(temp, humid float64) float64 {
	ratio := 373.15 / (273.15 + temp)
	rhs := -7.90298 * (ratio - 1)
	rhs += 5.02808 * math.Log10(ratio)
	rhs += -1.3816e-7 * (math.Pow(10, (11.344*(1-1/ratio))) - 1)
	rhs += 8.1328e-3 * (math.Pow(10, (-3.49149*(ratio-1))) - 1)
	rhs += math.Log10(1013.246)

	// factor -3 is to adjust units - Vapor Pressure SVP * humidity
	vp := math.Pow(10, rhs-3) * humid
	// (2) DEWPOINT = F(Vapor Pressure)
	t := math.Log(vp / 0.61078) // temp var
	denominator := 17.558 - t
	if denominator == 0 {
		// should be impossible
		return 999
	}
	return (241.88 * t) / denominator
}

func (s *bme280) reset(ctx context.Context) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	err = handle.WriteByteData(ctx, bme280RSTReg, 0xB6)
	if err != nil {
		return err
	}
	return handle.Close()
}

// Mode 00 = Sleep
// 01 and 10 = Forced
// 11 = Normal mode.
func (s *bme280) setMode(ctx context.Context, mode int) error {
	if mode > activeMode {
		mode = 0 // Error check. Default to sleep mode
	}

	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}
	controlDataByte, err := handle.ReadByteData(ctx, bme280CTRLMEASReg)
	if err != nil {
		return err
	}
	controlData := int(controlDataByte)

	controlData |= mode // Set
	err = handle.WriteByteData(ctx, bme280CTRLMEASReg, byte(controlData))
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
	controlDataByte, err := handle.ReadByteData(ctx, bme280CTRLMEASReg)
	if err != nil {
		return -1, err
	}

	return (int(controlDataByte) & 0b00000011), handle.Close()
}

func (s *bme280) IsMeasuring(ctx context.Context) (bool, error) {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return false, err
	}
	stat, err := handle.ReadByteData(ctx, bme280STATReg)
	if err != nil {
		return false, err
	}
	return stat&(1<<3) == 1, handle.Close()
}

func (s *bme280) setStandbyTime(ctx context.Context, val byte) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}

	if val > 0b111 {
		val = 0
	}
	controlData, err := handle.ReadByteData(ctx, bme280ConfigReg)
	if err != nil {
		return err
	}
	controlData &= ^((byte(1) << 7) | (byte(1) << 6) | (byte(1) << 5))
	controlData |= (val << 5)

	err = handle.WriteByteData(ctx, bme280ConfigReg, controlData)
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
	controlData, err := handle.ReadByteData(ctx, bme280ConfigReg)
	if err != nil {
		return err
	}
	controlData &= ^((byte(1) << 4) | (byte(1) << 3) | (byte(1) << 2))
	controlData |= (val << 2)

	err = handle.WriteByteData(ctx, bme280ConfigReg, controlData)
	if err != nil {
		return err
	}
	return handle.Close()
}

func (s *bme280) setOverSample(ctx context.Context, addr, offset, val byte) error {
	mode, err := s.currentMode(ctx)
	if err != nil {
		return err
	}

	if err = s.setMode(ctx, 0b00); err != nil {
		return err
	}

	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}

	controlData, err := handle.ReadByteData(ctx, addr)
	if err != nil {
		return err
	}
	controlData &= ^((byte(1) << (offset + 2)) | (byte(1) << (offset + 1)) | (byte(1) << offset))
	controlData |= (val << offset)

	if err = handle.WriteByteData(ctx, addr, controlData); err != nil {
		return err
	}

	if err := handle.Close(); err != nil {
		return err
	}

	if err = s.setMode(ctx, mode); err != nil {
		return err
	}

	return nil
}

// setupCalibration sets up all calibration data for the chip.
func (s *bme280) setupCalibration(ctx context.Context) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		return err
	}

	// A helper function to read 2 bytes from the handle and interpret it as a word
	readWord := func(register byte) (uint16, error) {
		rd, err := handle.ReadBlockData(ctx, register, 2)
		if err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint16(rd), nil
	}

	// Note, some are signed, others are unsigned
	if calib, err := readWord(bme280T1LSBReg); err == nil {
		s.calibration["digT1"] = int(calib)
	} else {
		return err
	}
	if calib, err := readWord(bme280T2LSBReg); err == nil {
		s.calibration["digT2"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280T3LSBReg); err == nil {
		s.calibration["digT3"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P1LSBReg); err == nil {
		s.calibration["digP1"] = int(calib)
	} else {
		return err
	}
	if calib, err := readWord(bme280P2LSBReg); err == nil {
		s.calibration["digP2"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P3LSBReg); err == nil {
		s.calibration["digP3"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P4LSBReg); err == nil {
		s.calibration["digP4"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P5LSBReg); err == nil {
		s.calibration["digP5"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P6LSBReg); err == nil {
		s.calibration["digP6"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P7LSBReg); err == nil {
		s.calibration["digP7"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P8LSBReg); err == nil {
		s.calibration["digP8"] = int(int16(calib))
	} else {
		return err
	}
	if calib, err := readWord(bme280P9LSBReg); err == nil {
		s.calibration["digP9"] = int(int16(calib))
	} else {
		return err
	}

	calib, err := handle.ReadByteData(ctx, bme280H1Reg)
	if err != nil {
		return err
	}
	s.calibration["digH1"] = int(calib)

	if calib, err := readWord(bme280H2LSBReg); err == nil {
		s.calibration["digH2"] = int(int16(calib))
	} else {
		return err
	}

	calib, err = handle.ReadByteData(ctx, bme280H3Reg)
	if err != nil {
		return err
	}
	s.calibration["digH3"] = int(calib)

	calib, err = handle.ReadByteData(ctx, bme280H6Reg)
	if err != nil {
		return err
	}
	s.calibration["digH6"] = int(calib)

	r1byte, err := handle.ReadByteData(ctx, bme280H4MSBReg)
	if err != nil {
		return err
	}
	r2byte, err := handle.ReadByteData(ctx, bme280H4LSBReg)
	if err != nil {
		return err
	}
	s.calibration["digH4"] = (int(r1byte) << 4) + int(r2byte&0x0f)

	r1byte, err = handle.ReadByteData(ctx, bme280H5MSBReg)
	if err != nil {
		return err
	}
	r2byte, err = handle.ReadByteData(ctx, bme280H4LSBReg)
	if err != nil {
		return err
	}
	s.calibration["digH5"] = (int(r1byte) << 4) + int((r2byte>>4)&0x0f)

	return handle.Close()
}

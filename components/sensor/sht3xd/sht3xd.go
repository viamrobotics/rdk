//go:build linux

// Package sht3xd implements a sht3x-d sensor for temperature and humidity
// datasheet can be found at: https://cdn-shop.adafruit.com/product-files/2857/Sensirion_Humidity_SHT3x_Datasheet_digital-767294.pdf
// example repo: https://github.com/esphome/esphome/tree/dev/esphome/components/sht3xd
package sht3xd

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("sensirion-sht3xd")

const (
	defaultI2Caddr = 0x44
	// Addresses of sht3xd registers.
	sht3xdCOMMANDSOFTRESET1 = 0x30
	sht3xdCOMMANDSOFTRESET2 = 0xA2
	sht3xdCOMMANDPOLLINGH1  = 0x24
	sht3xdCOMMANDPOLLINGH2  = 0x00
)

// Config is used for converting config attributes.
type Config struct {
	Board   string `json:"board,omitempty"`
	I2CBus  string `json:"i2c_bus"`
	I2cAddr int    `json:"i2c_addr,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string
	if len(conf.Board) != 0 {
		deps = append(deps, conf.Board)
	}
	if len(conf.I2CBus) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
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
	var i2cbus board.I2C
	var err error
	if conf.Board != "" {
		b, err := board.FromDependencies(deps, conf.Board)
		if err != nil {
			return nil, fmt.Errorf("sht3xd init: failed to find board: %w", err)
		}
		localB, ok := b.(board.LocalBoard)
		if !ok {
			return nil, fmt.Errorf("board %s is not local", conf.Board)
		}
		i2cbus, ok = localB.I2CByName(conf.I2CBus)
		if !ok {
			return nil, fmt.Errorf("sht3xd init: failed to find i2c bus %s", conf.I2CBus)
		}
	} else {
		i2cbus, err = genericlinux.NewI2cBus(conf.I2CBus)
		if err != nil {
			return nil, fmt.Errorf("sht3xd init: failed to find i2c bus %s", conf.I2CBus)
		}
	}
	addr := conf.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Warn("using i2c address : 0x44")
	}

	s := &sht3xd{
		Named:  name.AsNamed(),
		logger: logger,
		bus:    i2cbus,
		addr:   byte(addr),
	}

	err = s.reset(ctx)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// sht3xd is a i2c sensor device that reports temperature and humidity.
type sht3xd struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger golog.Logger

	bus  board.I2C
	addr byte
}

// Readings returns a list containing two items (current temperature and humidity).
func (s *sht3xd) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	tryRead := func() ([]byte, error) {
		handle, err := s.bus.OpenHandle(s.addr)
		if err != nil {
			s.logger.Errorf("can't open sht3xd i2c %s", err)
			return nil, err
		}
		err = handle.Write(ctx, []byte{sht3xdCOMMANDPOLLINGH1, sht3xdCOMMANDPOLLINGH2})
		if err != nil {
			s.logger.Debug("Failed to request temperature")
			return nil, multierr.Append(err, handle.Close())
		}
		buffer, err := handle.Read(ctx, 2)
		if err != nil {
			return nil, multierr.Append(err, handle.Close())
		}
		return buffer, handle.Close()
	}
	buffer, err := tryRead()
	if err != nil {
		// If error, do a soft reset and try again
		err = s.reset(ctx)
		if err != nil {
			return nil, err
		}
		buffer, err = tryRead()
		if err != nil {
			return nil, err
		}
	}
	if len(buffer) != 2 {
		return nil, fmt.Errorf("expected 2 bytes from sht3xd i2c, got %d", len(buffer))
	}
	tempRaw := binary.LittleEndian.Uint16([]byte{0, buffer[0]})
	humidRaw := binary.LittleEndian.Uint16([]byte{0, buffer[1]})

	temp := 175.0*float64(tempRaw)/65535.0 - 45.0
	humid := 100.0 * float64(humidRaw) / 65535.0
	return map[string]interface{}{
		"temperature_celsius":   temp,
		"relative_humidity_pct": humid, // TODO(RSDK-1903)
	}, nil
}

// reset will reset the sensor.
func (s *sht3xd) reset(ctx context.Context) error {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		s.logger.Errorf("can't open sht3xd i2c %s", err)
		return err
	}
	err = handle.Write(ctx, []byte{sht3xdCOMMANDSOFTRESET1, sht3xdCOMMANDSOFTRESET2})
	// wait for chip reset cycle to complete
	time.Sleep(1 * time.Millisecond)
	return multierr.Append(err, handle.Close())
}

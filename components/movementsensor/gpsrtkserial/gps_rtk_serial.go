// Package gpsrtkserial implements a gps using serial connection
package gpsrtkserial

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	ntripClient "go.viam.com/rdk/components/movementsensor/gpsrtk"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-rtk-serial")

var (
	errCorrectionSourceValidation = fmt.Errorf("only serial is supported correction sources for %s", rtkmodel.Name)
	errConnectionTypeValidation   = fmt.Errorf("only serial is supported connection types for %s", rtkmodel.Name)
	errInputProtocolValidation    = fmt.Errorf("only serial is supported input protocols for %s", rtkmodel.Name)
)

const (
	serialStr = "serial"
	ntripStr  = "ntrip"
)

type Config struct {
	NmeaDataSource           string `json:"nmea_data_source"`
	SerialPath               string `json:"serial_path"`
	SerialBaudRate           int    `json:"serial_baud_rate,omitempty"`
	SerialCorrectionPath     string `json:"serial_correction_path,omitempty"`
	SerialCorrectionBaudRate int    `json:"serial_correction_baud_rate,omitempty"`

	*NtripConfig `json:"ntrip_attributes,omitempty"`
}

// NtripConfig is used for converting attributes for a correction source.
type NtripConfig struct {
	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	NtripInputProtocol   string `json:"ntrip_input_protocol,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	dep, err := cfg.validateNmeaDataSource(path)
	if err != nil {
		return nil, err
	}
	if dep != nil {
		deps = append(deps, dep...)
	}

	if cfg.NmeaDataSource == ntripStr {
		dep, err = cfg.validateNtripInputProtocol(path)
		if err != nil {
			return nil, err
		}
	}
	if dep != nil {
		deps = append(deps, dep...)
	}

	return deps, nil
}

func (cfg *Config) validateNmeaDataSource(path string) ([]string, error) {
	switch strings.ToLower(cfg.NmeaDataSource) {
	case serialStr:
		return nil, cfg.ValidateSerialPath(path)
	case "":
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	default:
		return nil, errConnectionTypeValidation
	}
}

// validateNtripInputProtocol validates protocols accepted by this package
func (cfg *Config) validateNtripInputProtocol(path string) ([]string, error) {

	switch cfg.NtripInputProtocol {
	case serialStr:
		return nil, cfg.ValidateSerialPath(path)
	default:
		return nil, errInputProtocolValidation
	}
}

// ValidateSerial ensures all parts of the config are valid.
func (cfg *Config) ValidateSerialPath(path string) error {
	if cfg.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

// ValidateNtrip ensures all parts of the config are valid.
func (cfg *NtripConfig) ValidateNtrip(path string) error {
	if cfg.NtripURL == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "ntrip_url")
	}
	if cfg.NtripInputProtocol == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "ntrip_input_protocol")
	}
	return nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		rtkmodel,
		resource.Registration[movementsensor.MovementSensor, *Config]{})
}

// RTKSerial is an nmea movementsensor model that can intake RTK correction data
type RTKSerial struct {
	resource.Named
	resource.AlwaysRebuild
	logger     golog.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup

	ntripMu     sync.Mutex
	ntripClient *ntripClient.NtripInfo
	ntripStatus bool

	err          movementsensor.LastError
	lastposition movementsensor.LastPosition

	Nmeamovementsensor gpsnmea.NmeaMovementSensor
	InputProtocol      string
	CorrectionWriter   io.ReadWriteCloser
}

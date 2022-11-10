package gpsnmea

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
)

// PmtkI2CNMEAMovementSensor allows the use of any MovementSensor chip that communicates over I2C using the PMTK protocol.
type PmtkI2CNMEAMovementSensor struct {
	generic.Unimplemented
	mu                      sync.RWMutex
	cancelCtx               context.Context
	cancelFunc              func()
	logger                  golog.Logger
	data                    gpsData
	activeBackgroundWorkers sync.WaitGroup

	disableNmea bool
	errMu       sync.Mutex
	lastError   error

	bus   board.I2C
	addr  byte
	wbaud int
}

// NewPmtkI2CGPSNMEA implements a gps that communicates over i2c.
func NewPmtkI2CGPSNMEA(
	ctx context.Context,
	deps registry.Dependencies,
	attr *AttrConfig,
	logger golog.Logger,
) (NmeaMovementSensor, error) {
	b, err := board.FromDependencies(deps, attr.Board)
	if err != nil {
		return nil, fmt.Errorf("gps init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", attr.Board)
	}
	i2cbus, ok := localB.I2CByName(attr.I2CAttrConfig.I2CBus)
	if !ok {
		return nil, fmt.Errorf("gps init: failed to find i2c bus %s", attr.I2CAttrConfig.I2CBus)
	}
	addr := attr.I2CAttrConfig.I2cAddr
	if addr == -1 {
		return nil, errors.New("must specify gps i2c address")
	}
	if attr.I2CAttrConfig.I2CBaudRate == 0 {
		attr.I2CAttrConfig.I2CBaudRate = 38400
		logger.Warn("using default baudrate : 38400")
	}

	disableNmea := attr.DisableNMEA
	if disableNmea {
		logger.Info("SerialNMEAMovementSensor: NMEA reading disabled")
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &PmtkI2CNMEAMovementSensor{
		bus:         i2cbus,
		addr:        byte(addr),
		wbaud:       attr.I2CAttrConfig.I2CBaudRate,
		cancelCtx:   cancelCtx,
		cancelFunc:  cancelFunc,
		logger:      logger,
		disableNmea: disableNmea,
	}

	if err := g.Start(ctx); err != nil {
		return nil, err
	}
	return g, g.lastError
}

func (g *PmtkI2CNMEAMovementSensor) setLastError(err error) {
	g.errMu.Lock()
	defer g.errMu.Unlock()

	g.lastError = err
}

// Start begins reading nmea messages from module and updates gps data.
func (g *PmtkI2CNMEAMovementSensor) Start(ctx context.Context) error {
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.Errorf("can't open gps i2c %s", err)
		return err
	}
	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	baudcmd := fmt.Sprintf("PMTK251,%d", g.wbaud)
	cmd251 := addChk([]byte(baudcmd))
	cmd314 := addChk([]byte("PMTK314,1,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))

	err = handle.Write(ctx, cmd251)
	if err != nil {
		g.logger.Debug("Failed to set baud rate")
	}
	err = handle.Write(ctx, cmd314)
	if err != nil {
		g.logger.Errorf("i2c handle write failed %s", err)
		return err
	}
	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.Errorf("i2c handle write failed %s", err)
		return err
	}
	err = handle.Close()
	if err != nil {
		g.logger.Errorf("failed to close handle: %s", err)
		return err
	}

	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()
		strBuf := ""
		for {
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}

			if !g.disableNmea {
				// Opening an i2c handle blocks the whole bus, so we open/close each loop so other things also have a chance to use it
				handle, err := g.bus.OpenHandle(g.addr)
				if err != nil {
					g.logger.Errorf("can't open gps i2c handle: %s", err)
					g.setLastError(err)
					return
				}
				buffer, err := handle.Read(ctx, 1024)
				hErr := handle.Close()
				if hErr != nil {
					g.logger.Errorf("failed to close handle: %s", hErr)
					g.setLastError(err)
					return
				}
				if err != nil {
					g.logger.Error(err)
					continue
				}
				for _, b := range buffer {
					// PMTK uses CRLF line endings to terminate sentences, but just LF to blank data.
					// Since CR should never appear except at the end of our sentence, we use that to determine sentence end.
					// LF is merely ignored.
					if b == 0x0D {
						if strBuf != "" {
							g.mu.Lock()
							err = g.data.parseAndUpdate(strBuf)
							g.mu.Unlock()
							if err != nil {
								g.logger.Debugf("can't parse nmea : %s, %v", strBuf, err)
							}
						}
						strBuf = ""
					} else if b != 0x0A && b != 0xFF { // adds only valid bytes
						strBuf += string(b)
					}
				}
			}
		}
	})

	return g.lastError
}

// GetBusAddr returns the bus and address that takes in rtcm corrections.
func (g *PmtkI2CNMEAMovementSensor) GetBusAddr() (board.I2C, byte) {
	return g.bus, g.addr
}

// Position returns the current geographic location of the MovementSensor.
func (g *PmtkI2CNMEAMovementSensor) Position(ctx context.Context) (*geo.Point, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.location, g.data.alt, g.lastError
}

// Accuracy returns the accuracy, hDOP and vDOP.
func (g *PmtkI2CNMEAMovementSensor) Accuracy(ctx context.Context) (map[string]float32, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]float32{"hDOP": float32(g.data.hDOP), "vDOP": float32(g.data.vDOP)}, g.lastError
}

// LinearVelocity returns the current speed of the MovementSensor.
func (g *PmtkI2CNMEAMovementSensor) LinearVelocity(ctx context.Context) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return r3.Vector{X: 0, Y: g.data.speed, Z: 0}, g.lastError
}

// AngularVelocity not supported.
func (g *PmtkI2CNMEAMovementSensor) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

// CompassHeading not supported.
func (g *PmtkI2CNMEAMovementSensor) CompassHeading(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return 0, g.lastError
}

// Orientation not supporter.
func (g *PmtkI2CNMEAMovementSensor) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return nil, movementsensor.ErrMethodUnimplementedOrientation
}

// Properties what can I do!
func (g *PmtkI2CNMEAMovementSensor) Properties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
	}, g.lastError
}

// ReadFix returns quality.
func (g *PmtkI2CNMEAMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.fixQuality, g.lastError
}

// Readings will use return all of the MovementSensor Readings.
func (g *PmtkI2CNMEAMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.Readings(ctx, g)
	if err != nil {
		return nil, err
	}

	fix, err := g.ReadFix(ctx)
	if err != nil {
		return nil, err
	}

	readings["fix"] = fix

	return readings, nil
}

// Close shuts down the SerialNMEAMOVEMENTSENSOR.
func (g *PmtkI2CNMEAMovementSensor) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()

	return g.lastError
}

// PMTK checksums commands by XORing together each byte.
func addChk(data []byte) []byte {
	chk := checksum(data)
	newCmd := []byte("$")
	newCmd = append(newCmd, data...)
	newCmd = append(newCmd, []byte("*")...)
	newCmd = append(newCmd, chk)
	return newCmd
}

func checksum(data []byte) byte {
	var chk byte
	for _, b := range data {
		chk ^= b
	}
	return chk
}

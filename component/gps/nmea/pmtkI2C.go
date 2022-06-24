package nmea

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"nmea-pmtkI2C",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newPmtkI2CNMEAGPS(ctx, deps, config, logger)
		}})
}

// This allows the use of any GPS chip that communicates over I2C using the PMTK protocol.
type pmtkI2CNMEAGPS struct {
	generic.Unimplemented
	mu     sync.RWMutex
	bus    board.I2C
	addr   byte
	logger golog.Logger

	data gpsData

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func newPmtkI2CNMEAGPS(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) (gps.LocalGPS, error) {
	b, err := board.FromDependencies(deps, config.Attributes.String("board"))
	if err != nil {
		return nil, fmt.Errorf("gps init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", config.Attributes.String("board"))
	}
	i2cbus, ok := localB.I2CByName(config.Attributes.String("bus"))
	if !ok {
		return nil, fmt.Errorf("gps init: failed to find i2c bus %s", config.Attributes.String("bus"))
	}
	addr := config.Attributes.Int("i2c_addr", -1)
	if addr == -1 {
		return nil, errors.New("must specify gps i2c address")
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &pmtkI2CNMEAGPS{
		bus: i2cbus, addr: byte(addr), cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger,
	}
	g.Start(ctx)

	return g, nil
}

func (g *pmtkI2CNMEAGPS) Start(ctx context.Context) {
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.Fatalf("can't open gps i2c %s", err)
		return
	}
	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	cmd251 := addChk([]byte("PMTK251,115200")) //set baud rate
	cmd314 := addChk([]byte("PMTK314,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))

	err = handle.Write(ctx, cmd251)
	if err != nil {
		g.logger.Debug("Failed to set baud rate")
	}
	err = handle.Write(ctx, cmd314)
	if err != nil {
		g.logger.Fatalf("i2c handle write failed %s", err)
		return
	}
	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.Fatalf("i2c handle write failed %s", err)
		return
	}
	err = handle.Close()
	if err != nil {
		g.logger.Fatalf("failed to close handle: %s", err)
		return
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

			// Opening an i2c handle blocks the whole bus, so we open/close each loop so other things also have a chance to use it
			handle, err := g.bus.OpenHandle(g.addr)
			if err != nil {
				g.logger.Fatalf("can't open gps i2c handle: %s", err)
				return
			}
			buffer, err := handle.Read(context.Background(), 32)
			hErr := handle.Close()
			if hErr != nil {
				g.logger.Fatalf("failed to close handle: %s", hErr)
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
							g.logger.Debugf("can't parse nmea %s : %v", strBuf, err)
							continue
						// }
						} else {
							g.logger.Info(g.ReadLocation(ctx))
						}
					}
					strBuf = ""
				} else if b!=0xFF{
					strBuf += string(b)
				}
			}
		}
	})
}

func (g *pmtkI2CNMEAGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.location, nil
}

func (g *pmtkI2CNMEAGPS) ReadAltitude(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.alt, nil
}

func (g *pmtkI2CNMEAGPS) ReadSpeed(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.speed, nil
}

func (g *pmtkI2CNMEAGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.satsInUse, g.data.satsInView, nil
}

func (g *pmtkI2CNMEAGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.hDOP, g.data.vDOP, nil
}

func (g *pmtkI2CNMEAGPS) ReadValid(ctx context.Context) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.valid, nil
}

func (g *pmtkI2CNMEAGPS) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.fixQuality, nil
}

func (g *pmtkI2CNMEAGPS) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	return nil
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

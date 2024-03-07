// Package gpsnmea implements an NMEA gps.
package gpsnmea

import (
	"context"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/rtkutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// DataReader represents a way to get data from a GPS NMEA device. We can read data from it using
// the channel in Messages, and we can close the device when we're done.
type DataReader interface {
	Messages() chan string
	Close() error
}

// NMEAMovementSensor allows the use of any MovementSensor chip via a DataReader.
type NMEAMovementSensor struct {
	resource.Named
	resource.AlwaysRebuild
	cancelCtx               context.Context
	cancelFunc              func()
	logger                  logging.Logger
	cachedData              rtkutils.CachedData
	activeBackgroundWorkers sync.WaitGroup

	dev DataReader
}

// NewNmeaMovementSensor creates a new movement sensor.
func NewNmeaMovementSensor(
	ctx context.Context, name resource.Name, dev DataReader, logger logging.Logger,
) (NmeaMovementSensor, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &NMEAMovementSensor{
		Named:      name.AsNamed(),
		dev:        dev,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		cachedData: rtkutils.NewCachedData(),
	}

	if err := g.Start(ctx); err != nil {
		return nil, multierr.Combine(err, g.Close(ctx))
	}
	return g, nil
}

// Start begins reading nmea messages from module and updates gps data.
func (g *NMEAMovementSensor) Start(_ context.Context) error {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()

		messages := g.dev.Messages()
		for {
			// First, check if we're supposed to shut down.
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}

			// Next, wait until either we're supposed to shut down or we have new data to process.
			select {
			case <-g.cancelCtx.Done():
				return
			case message := <-messages:
				// Update our struct's gps data in-place
				err := g.cachedData.ParseAndUpdate(message)
				if err != nil {
					g.logger.CWarnf(g.cancelCtx, "can't parse nmea sentence: %#v", err)
					g.logger.Debug("Check: GPS requires clear sky view." +
						"Ensure the antenna is outdoors if signal is weak or unavailable indoors.")
				}
			}
		}
	})

	return nil
}

// Position returns the position and altitide of the sensor, or an error.
func (g *NMEAMovementSensor) Position(
	ctx context.Context, extra map[string]interface{},
) (*geo.Point, float64, error) {
	return g.cachedData.Position(ctx, extra)
}

// Accuracy returns the accuracy map, hDOP, vDOP, Fixquality and compass heading error.
func (g *NMEAMovementSensor) Accuracy(
	ctx context.Context, extra map[string]interface{},
) (*movementsensor.Accuracy, error) {
	return g.cachedData.Accuracy(ctx, extra)
}

// LinearVelocity returns the sensor's linear velocity. It requires having a compass heading, so we
// know which direction our speed is in. We assume all of this speed is horizontal, and not in
// gaining/losing altitude.
func (g *NMEAMovementSensor) LinearVelocity(
	ctx context.Context, extra map[string]interface{},
) (r3.Vector, error) {
	return g.cachedData.LinearVelocity(ctx, extra)
}

// LinearAcceleration returns the sensor's linear acceleration.
func (g *NMEAMovementSensor) LinearAcceleration(
	ctx context.Context, extra map[string]interface{},
) (r3.Vector, error) {
	return g.cachedData.LinearAcceleration(ctx, extra)
}

// AngularVelocity returns the sensor's angular velocity.
func (g *NMEAMovementSensor) AngularVelocity(
	ctx context.Context, extra map[string]interface{},
) (spatialmath.AngularVelocity, error) {
	return g.cachedData.AngularVelocity(ctx, extra)
}

// Orientation returns the sensor's orientation.
func (g *NMEAMovementSensor) Orientation(
	ctx context.Context, extra map[string]interface{},
) (spatialmath.Orientation, error) {
	return g.cachedData.Orientation(ctx, extra)
}

// CompassHeading returns the heading, from the range 0->360.
func (g *NMEAMovementSensor) CompassHeading(
	ctx context.Context, extra map[string]interface{},
) (float64, error) {
	return g.cachedData.CompassHeading(ctx, extra)
}

// ReadFix returns Fix quality of MovementSensor measurements.
func (g *NMEAMovementSensor) ReadFix(ctx context.Context) (int, error) {
	return g.cachedData.ReadFix(ctx)
}

// ReadSatsInView returns the number of satellites in view.
func (g *NMEAMovementSensor) ReadSatsInView(ctx context.Context) (int, error) {
	return g.cachedData.ReadSatsInView(ctx)
}

// Readings will use return all of the MovementSensor Readings.
func (g *NMEAMovementSensor) Readings(
	ctx context.Context, extra map[string]interface{},
) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, g, extra)
	if err != nil {
		return nil, err
	}

	fix, err := g.ReadFix(ctx)
	if err != nil {
		return nil, err
	}
	satsInView, err := g.ReadSatsInView(ctx)
	if err != nil {
		return nil, err
	}
	readings["fix"] = fix
	readings["satellites_in_view"] = satsInView

	return readings, nil
}

// Properties returns what movement sensor capabilities we have.
func (g *NMEAMovementSensor) Properties(
	ctx context.Context, extra map[string]interface{},
) (*movementsensor.Properties, error) {
	return g.cachedData.Properties(ctx, extra)
}

// Close shuts down the NMEAMovementSensor.
func (g *NMEAMovementSensor) Close(ctx context.Context) error {
	g.logger.CDebug(ctx, "Closing NMEAMovementSensor")
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()

	if g.dev != nil {
		if err := g.dev.Close(); err != nil {
			return err
		}
		g.dev = nil
		g.logger.CDebug(ctx, "NMEAMovementSensor Closed")
	}
	return nil
}

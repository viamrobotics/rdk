// Package gpsnmea implements an NMEA gps.
package gpsnmea

import (
	"context"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// NMEAMovementSensor allows the use of any MovementSensor chip via a DataReader.
type NMEAMovementSensor struct {
	resource.Named
	resource.AlwaysRebuild
	logger     logging.Logger
	cachedData *gpsutils.CachedData
}

// newNMEAMovementSensor creates a new movement sensor.
func newNMEAMovementSensor(
	_ context.Context, name resource.Name, dev gpsutils.DataReader, logger logging.Logger,
) (NmeaMovementSensor, error) {
	g := &NMEAMovementSensor{
		Named:      name.AsNamed(),
		logger:     logger,
		cachedData: gpsutils.NewCachedData(dev, logger),
	}

	return g, nil
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

// Readings will use return all of the MovementSensor Readings.
func (g *NMEAMovementSensor) Readings(
	ctx context.Context, extra map[string]interface{},
) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, g, extra)
	if err != nil {
		return nil, err
	}

	commonReadings := g.cachedData.GetCommonReadings(ctx)

	readings["fix"] = commonReadings.FixValue
	readings["satellites_in_view"] = commonReadings.SatsInView
	readings["satellites_in_use"] = commonReadings.SatsInUse

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
	// In some of the unit tests, the cachedData is nil. Only close it if it's not.
	if g.cachedData != nil {
		return g.cachedData.Close(ctx)
	}
	return nil
}

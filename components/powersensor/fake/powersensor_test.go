package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func newTestPowerSensor(t *testing.T, conf *Config) powersensor.PowerSensor {
	t.Helper()
	cfg := resource.Config{
		Name:                "ps",
		API:                 powersensor.API,
		Model:               model,
		ConvertedAttributes: conf,
	}
	ps, err := newFakePowerSensorModel(context.Background(), nil, cfg, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	return ps
}

func TestBatteryDefaults(t *testing.T) {
	ps := newTestPowerSensor(t, &Config{})

	percent, isCharging, err := ps.BatteryPercent(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, percent, test.ShouldEqual, defaultBatteryPercent)
	test.That(t, isCharging, test.ShouldBeFalse)

	props, err := ps.Properties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.IsBatteryPercentSupported, test.ShouldBeTrue)
	test.That(t, props.IsChargingSupported, test.ShouldBeFalse)
}

func TestBatteryConfigured(t *testing.T) {
	percent := 12.5
	isCharging := true
	ps := newTestPowerSensor(t, &Config{BatteryPercent: &percent, IsCharging: &isCharging})

	gotPercent, gotIsCharging, err := ps.BatteryPercent(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotPercent, test.ShouldEqual, percent)
	test.That(t, gotIsCharging, test.ShouldBeTrue)

	props, err := ps.Properties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.IsChargingSupported, test.ShouldBeTrue)
}

func TestValidateBatteryPercent(t *testing.T) {
	for _, percent := range []float64{0, 50, 100} {
		_, _, err := (&Config{BatteryPercent: &percent}).Validate("path")
		test.That(t, err, test.ShouldBeNil)
	}
	for _, percent := range []float64{-1, 100.5} {
		_, _, err := (&Config{BatteryPercent: &percent}).Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "battery_percent must be between 0 and 100")
	}
}

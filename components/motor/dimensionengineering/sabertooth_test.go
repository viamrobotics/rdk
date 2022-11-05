package dimensionengineering_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/dimensionengineering"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

// var txMu sync.Mutex

func checkTx(t *testing.T, resChan chan string, c chan []byte, expects []byte) {
	t.Helper()
	message := <-c
	t.Logf("Expected: %b, Actual %b", expects, message)
	test.That(t, bytes.Compare(message, expects), test.ShouldBeZeroValue)
	resChan <- "DONE"
}

func TestSabertoothMotor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: false,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, m1)

	// This should be the stop command
	checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})

	motor1, ok := m1.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = motor1.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		features, err := motor1.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})

		// Test 0.5 of max power
		test.That(t, motor1.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x3f, 0x3f})

		// Test -0.5 of max power
		test.That(t, motor1.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x3f, 0x40})

		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
	})

	mc2 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  2,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: false,
	}

	m2, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor2", ConvertedAttributes: &mc2}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, m2)

	checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})

	motor2, ok := m2.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = motor2.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		features, err := motor2.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})

		// Test 0.5 of max power
		test.That(t, motor2.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x3f, 0x43})

		// Test -0.5 of max power
		test.That(t, motor2.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x3f, 0x44})

		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})
	})
}

func TestSabertoothMotorDirectionFlip(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: true,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, m1)

	checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})

	motor1, ok := m1.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = motor1.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x00, 0x01})

		// Test 0.5 of max power
		test.That(t, motor1.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x3f, 0x40})

		// Test -0.5 of max power
		test.That(t, motor1.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x3f, 0x3f})

		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x00, 0x01})
	})

	mc2 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  2,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: true,
	}

	m2, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor2", ConvertedAttributes: &mc2}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, m2)

	checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})

	motor2, ok := m2.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = motor2.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		features, err := motor2.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x00, 0x05})

		// Test 0.5 of max power
		test.That(t, motor2.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x3f, 0x44})

		// Test -0.5 of max power
		test.That(t, motor2.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x3f, 0x43})

		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x00, 0x05})
	})
}

func TestSabertoothRampConfig(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 128,
		RampValue:     100,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, m1)

	checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
	checkTx(t, resChan, c, []byte{0x80, 0x10, 0x64, 0x74})

	motor1, ok := m1.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = motor1.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestSabertoothAddressMapping(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, m1)

	checkTx(t, resChan, c, []byte{0x81, 0x00, 0x00, 0x01})
}

func TestInvalidMotorChannel(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  3,
		TestChan:      c,
		SerialAddress: 129,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid channel")
	defer utils.TryClose(ctx, m1)
}

func TestInvalidBaudRate(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
		BaudRate:      1,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid baud_rate")
	defer utils.TryClose(ctx, m1)
}

func TestInvalidSerialAddress(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 140,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid address")
	defer utils.TryClose(ctx, m1)
}

func TestInvalidMinPowerPct(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
		MinPowerPct:   0.7,
		MaxPowerPct:   0.5,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid min_power_pct")
	defer utils.TryClose(ctx, m1)
}

func TestInvalidMaxPowerPct(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
		MinPowerPct:   0.7,
		MaxPowerPct:   1.5,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid max_power_pct")
	defer utils.TryClose(ctx, m1)
}

func TestMultipleInvalidParameters(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(registry.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  3,
		TestChan:      c,
		BaudRate:      10,
		SerialAddress: 140,
		MinPowerPct:   1.7,
		MaxPowerPct:   1.5,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid channel")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid address")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid baud_rate")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid min_power_pct")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid max_power_pct")
	defer utils.TryClose(ctx, m1)
}

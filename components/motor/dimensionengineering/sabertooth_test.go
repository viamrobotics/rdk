package dimensionengineering_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/dimensionengineering"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// var txMu sync.Mutex

var sabertoothModel = resource.DefaultModelFamily.WithModel("de-sabertooth")

func checkTx(t *testing.T, resChan chan string, c chan []byte, expects []byte) {
	t.Helper()
	message := <-c
	t.Logf("Expected: %b, Actual %b", expects, message)
	test.That(t, bytes.Compare(message, expects), test.ShouldBeZeroValue)
	resChan <- "DONE"
}

//nolint:dupl
func TestSabertoothMotor(t *testing.T) {
	ctx := context.Background()
	logger, obs := logging.NewObservedTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: false,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer m1.Close(ctx)

	// This should be the stop command
	checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})

	motor1, ok := m1.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		properties, err := motor1.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeFalse)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

		// Test 0.5 of max power
		test.That(t, motor1.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x3f, 0x3f})

		// Test -0.5 of max power
		test.That(t, motor1.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x3f, 0x40})

		// Test max power
		test.That(t, motor1.SetPower(ctx, 1, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x7f, 0x7f})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")

		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")
	})

	mc2 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  2,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: false,
		MaxRPM:        1,
	}

	m2, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor2", ConvertedAttributes: &mc2}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer m2.Close(ctx)

	checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})

	motor2, ok := m2.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		properties, err := motor2.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeFalse)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

		// Test 0.5 of max power
		test.That(t, motor2.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x3f, 0x43})

		// Test -0.5 of max power
		test.That(t, motor2.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x3f, 0x44})

		// Test max power
		test.That(t, motor2.SetPower(ctx, 1, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x7f, 0x03})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")

		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")
	})
}

//nolint:dupl
func TestSabertoothMotorDirectionFlip(t *testing.T) {
	ctx := context.Background()
	logger, obs := logging.NewObservedTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: true,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer m1.Close(ctx)

	checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})

	motor1, ok := m1.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x00, 0x01})
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

		// Test 0.5 of max power
		test.That(t, motor1.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x3f, 0x40})

		// Test -0.5 of max power
		test.That(t, motor1.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x00, 0x3f, 0x3f})

		// Test max power
		test.That(t, motor1.SetPower(ctx, 1, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x7f, 0x00})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")

		// Test 0 (aka "stop")
		test.That(t, motor1.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x01, 0x00, 0x01})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")
	})

	mc2 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  2,
		TestChan:      c,
		SerialAddress: 128,
		DirectionFlip: true,
		MaxRPM:        1,
	}

	m2, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor2", ConvertedAttributes: &mc2}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer m2.Close(ctx)

	checkTx(t, resChan, c, []byte{0x80, 0x04, 0x00, 0x04})

	motor2, ok := m2.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		properties, err := motor2.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeFalse)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x00, 0x05})
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

		// Test 0.5 of max power
		test.That(t, motor2.SetPower(ctx, 0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x3f, 0x44})

		// Test -0.5 of max power
		test.That(t, motor2.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x04, 0x3f, 0x43})

		// Test max power
		test.That(t, motor2.SetPower(ctx, 1, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x7f, 0x04})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")

		// Test 0 (aka "stop")
		test.That(t, motor2.SetPower(ctx, 0, nil), test.ShouldBeNil)
		checkTx(t, resChan, c, []byte{0x80, 0x05, 0x00, 0x05})
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")
	})
}

func TestSabertoothRampConfig(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 128,
		RampValue:     100,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer m1.Close(ctx)

	checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
	checkTx(t, resChan, c, []byte{0x80, 0x10, 0x64, 0x74})

	_, ok = m1.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestSabertoothAddressMapping(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	resChan := make(chan string, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	m1, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer m1.Close(ctx)

	checkTx(t, resChan, c, []byte{0x81, 0x00, 0x00, 0x01})
}

func TestInvalidMotorChannel(t *testing.T) {
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  3,
		TestChan:      c,
		SerialAddress: 129,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	_, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid channel")
}

func TestInvalidBaudRate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
		BaudRate:      1,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	_, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid baud_rate")
}

func TestInvalidSerialAddress(t *testing.T) {
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 140,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	_, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid address")
}

func TestInvalidMinPowerPct(t *testing.T) {
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
		MinPowerPct:   0.7,
		MaxPowerPct:   0.5,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	_, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid min_power_pct")
}

func TestInvalidMaxPowerPct(t *testing.T) {
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  1,
		TestChan:      c,
		SerialAddress: 129,
		MinPowerPct:   0.7,
		MaxPowerPct:   1.5,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	_, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid max_power_pct")
}

func TestMultipleInvalidParameters(t *testing.T) {
	logger := logging.NewTestLogger(t)
	c := make(chan []byte, 1024)
	deps := make(resource.Dependencies)

	mc1 := dimensionengineering.Config{
		SerialPath:    "testchan",
		MotorChannel:  3,
		TestChan:      c,
		BaudRate:      10,
		SerialAddress: 140,
		MinPowerPct:   1.7,
		MaxPowerPct:   1.5,
		MaxRPM:        1,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, sabertoothModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	_, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc1}, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid channel")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid address")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid baud_rate")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid min_power_pct")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid max_power_pct")
}

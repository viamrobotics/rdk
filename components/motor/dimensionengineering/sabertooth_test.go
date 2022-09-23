package dimensionengineering_test

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/dimensionengineering"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

var txMu sync.Mutex

func checkTx(t *testing.T, resChan chan string, c chan []byte, expects []byte) {
	t.Helper()
	defer txMu.Unlock()
	message := <-c
	t.Logf("Expected: %b, Actual %b", expects, message)
	test.That(t, bytes.Compare(message, expects), test.ShouldBeZeroValue)
	resChan <- "DONE"
}

func TestSabertoothMotor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte)
	resChan := make(chan string, 1024)
	deps := make(registry.Dependencies)

	mc := dimensionengineering.Config{
		SerialDevice: "testchan",
		Channel:      1,
		TestChan:     c,
		Address:      128,
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "de-sabertooth")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	txMu.Lock()
	go checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
	m, err := motorReg.Constructor(context.Background(), deps, config.Component{Name: "motor1", ConvertedAttributes: &mc}, logger)
	test.That(t, err, test.ShouldBeNil)

	_motor, ok := m.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = _motor.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		features, err := _motor.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		txMu.Lock()
		go checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
		test.That(t, _motor.SetPower(ctx, 0, nil), test.ShouldBeNil)

		// Test 0.5 of max power
		txMu.Lock()
		go checkTx(t, resChan, c, []byte{0x80, 0x00, 0x3f, 0x3f})
		test.That(t, _motor.SetPower(ctx, 0.5, nil), test.ShouldBeNil)

		// Test -0.5 of max power
		txMu.Lock()
		go checkTx(t, resChan, c, []byte{0x80, 0x01, 0x3f, 0x40})
		test.That(t, _motor.SetPower(ctx, -0.5, nil), test.ShouldBeNil)

		// Test 0 (aka "stop")
		txMu.Lock()
		go checkTx(t, resChan, c, []byte{0x80, 0x00, 0x00, 0x00})
		test.That(t, _motor.SetPower(ctx, 0, nil), test.ShouldBeNil)
	})
}
